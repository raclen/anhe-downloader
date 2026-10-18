package downloader

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var metaClient = &http.Client{
	Timeout: 10 * time.Second,
}

// FetchLyric 跨平台多级获取同步歌词 (LRC 格式)
func FetchLyric(source, songmid, title, singer string) string {
	cleanTitle := sanitizeSearchWord(title)
	cleanSinger := sanitizeSearchWord(singer)
	query := strings.TrimSpace(cleanTitle + " " + cleanSinger)
	if query == "" {
		query = cleanTitle
	}

	// 1. 如果已有明确的 KuGou Hash (32 位 16 进制)
	if source == "kg" || (len(songmid) == 32 && regexp.MustCompile(`^[a-fA-F0-9]{32}$`).MatchString(songmid)) {
		if lrc := fetchKuGouLyricByHash(songmid); lrc != "" {
			return lrc
		}
	}

	// 2. 如果是酷我数字 ID
	if (source == "kw" || source == "") && regexp.MustCompile(`^\d+$`).MatchString(songmid) {
		if lrc := fetchKuWoLyricByMusicID(songmid); lrc != "" {
			return lrc
		}
	}

	// 3. 如果是网易云数字 ID
	if source == "wy" && regexp.MustCompile(`^\d+$`).MatchString(songmid) {
		if lrc := fetchNetEaseLyricByID(songmid); lrc != "" {
			return lrc
		}
	}

	// 3.5 如果是 QQ 音乐 songmid
	if source == "tx" && songmid != "" {
		if lrc := fetchTencentLyricByMid(songmid); lrc != "" {
			return lrc
		}
	}

	// 4. 优先基于 酷狗检索曲目并抓取歌词 (库容最大、最稳定)
	if query != "" {
		if lrc := searchAndFetchKuGouLyric(query); lrc != "" {
			return lrc
		}
	}

	// 5. 备用：酷我全网检索
	if query != "" {
		if lrc := searchAndFetchKuWoLyric(query); lrc != "" {
			return lrc
		}
	}

	// 6. 备用：网易云全网检索
	if query != "" {
		if lrc := searchAndFetchNetEaseLyric(query); lrc != "" {
			return lrc
		}
	}

	return ""
}

// FetchCoverBytes 获取封面二进制图片数据
func FetchCoverBytes(coverURL, source, songmid, title, singer string) []byte {
	// 1. 若已有 coverURL，直接下载
	if coverURL != "" {
		if strings.HasPrefix(coverURL, "//") {
			coverURL = "https:" + coverURL
		}
		if b := downloadImageBytes(coverURL); len(b) > 500 {
			return b
		}
	}

	// 2. 若无 coverURL 或下载失败，全网搜索专辑封面
	cleanTitle := sanitizeSearchWord(title)
	cleanSinger := sanitizeSearchWord(singer)
	query := strings.TrimSpace(cleanTitle + " " + cleanSinger)
	if query == "" {
		return nil
	}

	// 查酷狗
	if cURL := searchKuGouCover(query); cURL != "" {
		if b := downloadImageBytes(cURL); len(b) > 500 {
			return b
		}
	}

	// 查酷我
	if cURL := searchKuWoCover(query); cURL != "" {
		if b := downloadImageBytes(cURL); len(b) > 500 {
			return b
		}
	}

	// 查网易云
	if cURL := searchNetEaseCover(query); cURL != "" {
		if b := downloadImageBytes(cURL); len(b) > 500 {
			return b
		}
	}

	return nil
}

func downloadImageBytes(imgURL string) []byte {
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "http://y.qq.com")

	resp, err := metaClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	return b
}

// 酷狗歌词 (根据 Hash)
func fetchKuGouLyricByHash(hash string) string {
	searchURL := fmt.Sprintf("http://krcs.kugou.com/search?ver=1&man=yes&client=mobi&keyword=&duration=&hash=%s", hash)
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Candidates []struct {
			ID        string `json:"id"`
			AccessKey string `json:"accesskey"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Candidates) == 0 {
		return ""
	}

	cand := res.Candidates[0]
	downURL := fmt.Sprintf("http://krcs.kugou.com/download?ver=1&client=mobi&id=%s&accesskey=%s&fmt=lrc", cand.ID, cand.AccessKey)
	downReq, _ := http.NewRequest("GET", downURL, nil)
	downReq.Header.Set("User-Agent", "Mozilla/5.0")
	downResp, err := metaClient.Do(downReq)
	if err != nil {
		return ""
	}
	defer downResp.Body.Close()

	var downRes struct {
		Content string `json:"content"`
		Charset string `json:"charset"`
	}
	if err := json.NewDecoder(downResp.Body).Decode(&downRes); err != nil || downRes.Content == "" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(downRes.Content)
	if err != nil {
		return ""
	}

	var result string
	if strings.Contains(strings.ToLower(downRes.Charset), "gb") || !utf8.Valid(decoded) {
		if utf8Bytes, gErr := simplifiedchinese.GB18030.NewDecoder().Bytes(decoded); gErr == nil {
			result = string(utf8Bytes)
		} else {
			result = string(decoded)
		}
	} else {
		result = string(decoded)
	}

	return strings.TrimPrefix(result, "\ufeff")
}

// 酷狗歌曲检索并获取歌词
func searchAndFetchKuGouLyric(keyword string) string {
	searchURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/search/song?format=json&keyword=%s&page=1&pagesize=1", url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Data struct {
			Info []struct {
				Hash string `json:"hash"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Data.Info) == 0 {
		return ""
	}

	return fetchKuGouLyricByHash(res.Data.Info[0].Hash)
}

// 酷狗歌曲检索封面
func searchKuGouCover(keyword string) string {
	searchURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/search/song?format=json&keyword=%s&page=1&pagesize=1", url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Data struct {
			Info []struct {
				AlbumSizableCover string `json:"album_sizable_cover"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Data.Info) == 0 {
		return ""
	}

	cover := res.Data.Info[0].AlbumSizableCover
	if cover != "" {
		return strings.ReplaceAll(cover, "{size}", "400")
	}
	return ""
}

// 酷我歌词 (根据 MusicId)
func fetchKuWoLyricByMusicID(musicID string) string {
	urlStr := fmt.Sprintf("http://m.kuwo.cn/newh5/singles/songinfoandlrc?musicId=%s", musicID)
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Referer", "http://m.kuwo.cn")

	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Data struct {
			LrcList []struct {
				LineLyric string `json:"lineLyric"`
				Time      string `json:"time"`
			} `json:"lrclist"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Data.LrcList) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, item := range res.Data.LrcList {
		var sec float64
		_, _ = fmt.Sscanf(item.Time, "%f", &sec)
		m := int(sec) / 60
		s := int(sec) % 60
		ms := int((sec - float64(int(sec))) * 100)
		sb.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", m, s, ms, item.LineLyric))
	}
	return sb.String()
}

// 酷我搜索并抓取歌词
func searchAndFetchKuWoLyric(keyword string) string {
	urlStr := fmt.Sprintf("http://search.kuwo.cn/r.s?client=kt&all=%s&pn=0&rn=1&vipver=1&ft=music&encoding=utf8&rformat=json&mobi=1", url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	fixed := strings.ReplaceAll(string(body), `'`, `"`)
	var res struct {
		AbsList []struct {
			DCTargetID string `json:"DC_TARGETID"`
		} `json:"abslist"`
	}
	if err := json.Unmarshal([]byte(fixed), &res); err != nil || len(res.AbsList) == 0 {
		return ""
	}

	return fetchKuWoLyricByMusicID(res.AbsList[0].DCTargetID)
}

// 酷我搜索封面
func searchKuWoCover(keyword string) string {
	urlStr := fmt.Sprintf("http://search.kuwo.cn/r.s?client=kt&all=%s&pn=0&rn=1&vipver=1&ft=music&encoding=utf8&rformat=json&mobi=1", url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	fixed := strings.ReplaceAll(string(body), `'`, `"`)
	var res struct {
		AbsList []struct {
			HtsMVPIC         string `json:"hts_MVPIC"`
			WebAlbumPicShort string `json:"web_albumpic_short"`
		} `json:"abslist"`
	}
	if err := json.Unmarshal([]byte(fixed), &res); err != nil || len(res.AbsList) == 0 {
		return ""
	}

	item := res.AbsList[0]
	if item.HtsMVPIC != "" {
		return item.HtsMVPIC
	}
	if item.WebAlbumPicShort != "" {
		return "https://img4.kuwo.cn/star/albumcover/" + item.WebAlbumPicShort
	}
	return ""
}

// 网易云歌词 (根据 ID)
func fetchNetEaseLyricByID(songID string) string {
	urlStr := fmt.Sprintf("https://music.163.com/api/song/lyric?id=%s&lv=1&kv=1&tv=-1", songID)
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Lrc struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return ""
	}
	return res.Lrc.Lyric
}

// 网易云搜索并抓取歌词
func searchAndFetchNetEaseLyric(keyword string) string {
	urlStr := fmt.Sprintf("https://music.163.com/api/search/get/web?csrf_token=&hlpretag=&hlposttag=&s=%s&type=1&offset=0&total=true&limit=1", url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Result struct {
			Songs []struct {
				ID int64 `json:"id"`
			} `json:"songs"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Result.Songs) == 0 {
		return ""
	}

	return fetchNetEaseLyricByID(fmt.Sprintf("%d", res.Result.Songs[0].ID))
}

// 网易云搜索封面
func searchNetEaseCover(keyword string) string {
	urlStr := fmt.Sprintf("https://music.163.com/api/search/get/web?csrf_token=&hlpretag=&hlposttag=&s=%s&type=1&offset=0&total=true&limit=1", url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Result struct {
			Songs []struct {
				Album struct {
					PicURL string `json:"picUrl"`
				} `json:"album"`
			} `json:"songs"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Result.Songs) == 0 {
		return ""
	}

	return res.Result.Songs[0].Album.PicURL
}

// QQ 音乐歌词 (根据 songmid)
func fetchTencentLyricByMid(songmid string) string {
	urlStr := fmt.Sprintf("https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=%s&format=json&nobase64=0&g_tk=5381",
		url.QueryEscape(songmid))
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://y.qq.com/portal/player.html")

	resp, err := metaClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Retcode int    `json:"retcode"`
		Lyric   string `json:"lyric"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || res.Lyric == "" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(res.Lyric)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(string(decoded), "\ufeff")
}

func sanitizeSearchWord(s string) string {
	re := regexp.MustCompile(`[《》【】\(\)\[\]（）]`)
	s = re.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
