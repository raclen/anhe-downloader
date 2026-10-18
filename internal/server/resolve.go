package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ResolveResult 音源直链解析结果
type ResolveResult struct {
	URL      string `json:"url"`
	Quality  string `json:"quality"`
	Format   string `json:"format"`
	Resolver string `json:"resolver"`
}

var digitsOnly = regexp.MustCompile(`^\d+$`)

// 低于该大小的音频视为「试听占位片段」（酷我对付费曲目会返回约 11 秒的占位 MP3）
const minFullAudioBytes int64 = 900 * 1024

// resolveAudio 统一音源解析入口（服务端执行，彻底规避浏览器 CORS）
func resolveAudio(source, songmid, title, singer, quality string) (*ResolveResult, error) {
	quality = normalizeQuality(quality)
	var candidates []*ResolveResult

	add := func(r *ResolveResult) {
		if r != nil && r.URL != "" {
			candidates = append(candidates, r)
		}
	}

	// 1. 原平台直链（酷我 / 网易云 / 酷狗）
	switch source {
	case "kw":
		if digitsOnly.MatchString(songmid) {
			if r, err := resolveKuWo(songmid, quality); err == nil {
				add(r)
			}
		}
	case "wy":
		if r, err := resolveNetEase(songmid); err == nil {
			add(r)
		}
	case "kg":
		if songmid != "" {
			if r, err := resolveKuGouHash(songmid, quality); err == nil {
				add(r)
			}
		}
	}

	// 2. 跨源兜底：酷我 → 酷狗 → 网易云（按歌名 + 歌手匹配）
	if r, err := resolveCrossKuWo(title, singer, quality); err == nil {
		add(r)
	}
	if r, err := resolveCrossKuGou(title, singer, quality); err == nil {
		add(r)
	}
	if r, err := resolveCrossNetEase(title, singer); err == nil {
		add(r)
	}

	// 3. 选中第一个「非试听片段」的可用直链
	var unknown *ResolveResult
	shortOnly := false
	for _, cand := range candidates {
		size, state := probeAudio(cand.URL)
		if state == probeInvalid {
			continue
		}
		if state == probeUnknown {
			// 探测失败（可能仅是不支持 Range），保留作兜底
			if unknown == nil {
				unknown = cand
			}
			continue
		}
		if size > 0 && size < minFullAudioBytes {
			shortOnly = true
			continue
		}
		return cand, nil
	}

	if unknown != nil {
		return unknown, nil
	}
	if shortOnly {
		return nil, fmt.Errorf("《%s》暂无可下载的完整音源（VIP / 付费或版权限制），可换其他版本试试", strings.TrimSpace(title))
	}

	return nil, fmt.Errorf("暂未找到《%s》可用的音源直链", strings.TrimSpace(title))
}

const (
	probeOK = iota
	probeUnknown
	probeInvalid
)

// probeAudio 探测音频大小（只读 1 字节，避免整段下载）
func probeAudio(rawURL string) (int64, int) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return 0, probeInvalid
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Range", "bytes=0-1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, probeUnknown
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, probeInvalid
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/json") {
		return 0, probeInvalid
	}

	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if idx := strings.LastIndex(cr, "/"); idx >= 0 {
			if total, err := strconv.ParseInt(cr[idx+1:], 10, 64); err == nil {
				return total, probeOK
			}
		}
	}
	return resp.ContentLength, probeOK
}

// resolveCrossKuWo 通过「歌名 + 歌手」在酷我检索同曲目并解析 MP3 直链
func resolveCrossKuWo(title, singer, quality string) (*ResolveResult, error) {
	rid, found := searchKuWoBestMatch(title, singer)
	if !found {
		return nil, fmt.Errorf("酷我未匹配到曲目")
	}
	return resolveKuWo(rid, quality)
}

// resolveKuWo 酷我 anti.s 直链解析
func resolveKuWo(rid, quality string) (*ResolveResult, error) {
	br := ""
	if quality != "128k" {
		br = "320k"
	}
	apiURL := fmt.Sprintf("http://antiserver.kuwo.cn/anti.s?type=convert_url&rid=%s&format=mp3&br=%s&response=url",
		url.QueryEscape(rid), br)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return nil, err
	}
	playURL := strings.TrimSpace(string(body))
	if !strings.HasPrefix(playURL, "http") {
		return nil, fmt.Errorf("酷我直链解析失败: %s", playURL)
	}

	actual := "128k"
	if br == "320k" {
		actual = "320k"
	}
	return &ResolveResult{
		URL:      playURL,
		Quality:  actual,
		Format:   detectURLFormat(playURL),
		Resolver: "酷我直链",
	}, nil
}

// resolveNetEase 网易云官方外链
func resolveNetEase(songID string) (*ResolveResult, error) {
	if !digitsOnly.MatchString(songID) {
		return nil, fmt.Errorf("无效的网易云 ID")
	}
	return &ResolveResult{
		URL:      fmt.Sprintf("https://music.163.com/song/media/outer/url?id=%s.mp3", songID),
		Quality:  "128k",
		Format:   "mp3",
		Resolver: "网易云直链",
	}, nil
}

// resolveCrossNetEase 按歌名 + 歌手检索网易云再取直链
func resolveCrossNetEase(title, singer string) (*ResolveResult, error) {
	list, _, err := searchNetEase(strings.TrimSpace(title+" "+firstArtistName(singer)), 1, 5)
	if err != nil || len(list) == 0 {
		return nil, fmt.Errorf("网易云未匹配到曲目")
	}
	best, ok := pickBestSong(list, title, singer)
	if !ok {
		return nil, fmt.Errorf("网易云未匹配到曲目")
	}
	return resolveNetEase(best.Songmid)
}

// resolveCrossKuGou 按歌名 + 歌手检索酷狗，并请求播放直链
func resolveCrossKuGou(title, singer, quality string) (*ResolveResult, error) {
	list, _, err := searchKuGou(strings.TrimSpace(title+" "+firstArtistName(singer)), 1, 10)
	if err != nil || len(list) == 0 {
		return nil, fmt.Errorf("酷狗未匹配到曲目")
	}
	best, ok := pickBestSong(list, title, singer)
	if !ok {
		return nil, fmt.Errorf("酷狗未匹配到曲目")
	}
	return resolveKuGouHash(best.Songmid, quality)
}

// resolveKuGouHash 请求酷狗播放直链
func resolveKuGouHash(hash, quality string) (*ResolveResult, error) {
	apiURL := "https://m.kugou.com/app/i/getSongInfo.php?cmd=playInfo&hash=" + url.QueryEscape(hash)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info struct {
		URL       string  `json:"url"`
		BitRate   float64 `json:"bitRate"`
		FileName  string  `json:"fileName"`
		Error     string  `json:"error"`
		Privilege int     `json:"privilege"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	if strings.TrimSpace(info.URL) == "" {
		return nil, fmt.Errorf("酷狗直链不可用: %s", info.Error)
	}

	actual := "128k"
	if info.BitRate >= 300 {
		actual = "320k"
	}
	return &ResolveResult{
		URL:      info.URL,
		Quality:  actual,
		Format:   detectURLFormat(info.URL),
		Resolver: "酷狗直链",
	}, nil
}

// searchKuWoBestMatch 检索酷我并做歌名/歌手相似度匹配
func searchKuWoBestMatch(title, singer string) (string, bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", false
	}
	firstSinger := firstArtistName(singer)
	list, _, err := searchKuWo(strings.TrimSpace(title+" "+firstSinger), 1, 10)
	if err != nil || len(list) == 0 {
		return "", false
	}
	best, ok := pickBestSong(list, title, singer)
	if !ok {
		return "", false
	}
	return best.Songmid, true
}

// pickBestSong 歌名/歌手相似度匹配，返回最可能的目标曲目
func pickBestSong(list []SearchSong, title, singer string) (SearchSong, bool) {
	if len(list) == 0 {
		return SearchSong{}, false
	}
	tKey := normalizeKey(title)
	aKey := normalizeKey(firstArtistName(singer))

	// 1. 精确命中：歌名完全一致 + 歌手包含匹配
	for _, s := range list {
		if normalizeKey(s.Name) == tKey && (aKey == "" || strings.Contains(normalizeKey(s.Singer), aKey)) {
			return s, true
		}
	}
	// 2. 宽松命中：歌名互相包含 + 歌手匹配（无歌手信息时才允许仅凭歌名）
	for _, s := range list {
		sName := normalizeKey(s.Name)
		sArtist := normalizeKey(s.Singer)
		if !strings.Contains(sName, tKey) && !strings.Contains(tKey, sName) {
			continue
		}
		if aKey == "" || strings.Contains(sArtist, aKey) || strings.Contains(aKey, sArtist) {
			return s, true
		}
	}
	// 3. 歌手完全对不上时宁可不下载，也不拿别人的翻唱充数
	return SearchSong{}, false
}

func firstArtistName(singer string) string {
	s := strings.ReplaceAll(singer, "、", "/")
	s = strings.ReplaceAll(s, ",", "/")
	s = strings.ReplaceAll(s, "，", "/")
	s = strings.ReplaceAll(s, "&", "/")
	s = strings.ReplaceAll(s, " feat.", "/")
	s = strings.ReplaceAll(s, " ft.", "/")
	parts := strings.Split(s, "/")
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			return v
		}
	}
	return strings.TrimSpace(singer)
}

func normalizeKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeQuality(q string) string {
	switch strings.ToLower(strings.TrimSpace(q)) {
	case "128", "128k", "128kbps":
		return "128k"
	default:
		return "320k"
	}
}

func detectURLFormat(rawURL string) string {
	lower := strings.ToLower(rawURL)
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	switch {
	case strings.HasSuffix(lower, ".flac"):
		return "flac"
	case strings.HasSuffix(lower, ".m4a"), strings.HasSuffix(lower, ".mp4"):
		return "m4a"
	default:
		return "mp3"
	}
}

// handleResolve HTTP 入口
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source := strings.TrimSpace(q.Get("source"))
	songmid := strings.TrimSpace(q.Get("songmid"))
	title := strings.TrimSpace(q.Get("title"))
	singer := strings.TrimSpace(q.Get("singer"))
	if songmid == "" && title == "" {
		http.Error(w, "缺少 title 或 songmid，无法解析音源", http.StatusBadRequest)
		return
	}
	result, err := resolveAudio(source, songmid, title, singer, q.Get("quality"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, result)
}
