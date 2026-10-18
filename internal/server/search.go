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
	"sync"
	"time"
)

type SearchSong struct {
	ID       string   `json:"id"`
	Source   string   `json:"source"`
	Name     string   `json:"name"`
	Singer   string   `json:"singer"`
	Album    string   `json:"album"`
	Duration int      `json:"duration"`
	Cover    string   `json:"cover"`
	Songmid  string   `json:"songmid"`
	Types    []string `json:"types"`
}

type SearchResponse struct {
	List  []SearchSong `json:"list"`
	Total int          `json:"total"`
}

var httpClient = &http.Client{
	Timeout: 8 * time.Second,
}

// 酷狗音乐搜索
func searchKuGou(keyword string, page, pageSize int) ([]SearchSong, int, error) {
	apiURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/search/song?format=json&keyword=%s&page=%d&pagesize=%d&showtype=1",
		url.QueryEscape(keyword), page, pageSize)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var kgData struct {
		Data struct {
			Total int `json:"total"`
			Info  []struct {
				Hash       string `json:"hash"`
				SongName   string `json:"songname"`
				SingerName string `json:"singername"`
				AlbumName  string `json:"album_name"`
				Duration   int    `json:"duration"`
				SqHash     string `json:"sqhash"`
				Hash320    string `json:"320hash"`
				TransParam struct {
					UnionCover string `json:"union_cover"`
				} `json:"trans_param"`
			} `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &kgData); err != nil {
		return nil, 0, err
	}

	var list []SearchSong
	for _, item := range kgData.Data.Info {
		types := []string{"128k"}
		if item.Hash320 != "" || item.Hash != "" {
			types = append(types, "320k")
		}
		if item.SqHash != "" {
			types = append(types, "flac")
		}

		cover := strings.Replace(item.TransParam.UnionCover, "{size}", "400", -1)
		cover = strings.Replace(cover, "\\/", "/", -1)

		list = append(list, SearchSong{
			ID:       fmt.Sprintf("kg_%s", item.Hash),
			Source:   "kg",
			Name:     cleanHTMLTags(item.SongName),
			Singer:   cleanHTMLTags(item.SingerName),
			Album:    cleanHTMLTags(item.AlbumName),
			Duration: item.Duration,
			Cover:    cover,
			Songmid:  item.Hash,
			Types:    types,
		})
	}

	return list, kgData.Data.Total, nil
}

// 酷我音乐搜索
func searchKuWo(keyword string, page, pageSize int) ([]SearchSong, int, error) {
	pn := page - 1
	if pn < 0 {
		pn = 0
	}
	apiURL := fmt.Sprintf("http://search.kuwo.cn/r.s?client=kt&all=%s&pn=%d&rn=%d&vipver=1&ft=music&encoding=utf8&rformat=json&mobi=1",
		url.QueryEscape(keyword), pn, pageSize)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	// 酷我有时返回非标准单引号 JSON，先尝试直接解析，失败则规整化为双引号
	bodyStr := string(bodyBytes)
	var kwData struct {
		Total   string `json:"TOTAL"`
		Abslist []struct {
			SongName         string `json:"SONGNAME"`
			Artist           string `json:"ARTIST"`
			Album            string `json:"ALBUM"`
			Duration         string `json:"DURATION"`
			DcTargetId       string `json:"DC_TARGETID"`
			MInfo            string `json:"MINFO"`
			HtsMVPIC         string `json:"hts_MVPIC"`
			WebAlbumPicShort string `json:"web_albumpic_short"`
		} `json:"abslist"`
	}

	err = json.Unmarshal([]byte(bodyStr), &kwData)
	if err != nil {
		// 替换单引号为双引号
		normalized := strings.Replace(bodyStr, "'", "\"", -1)
		_ = json.Unmarshal([]byte(normalized), &kwData)
	}

	var list []SearchSong
	for _, item := range kwData.Abslist {
		dur, _ := strconv.Atoi(item.Duration)
		rid := strings.TrimPrefix(item.DcTargetId, "MUSIC_")

		types := []string{"128k", "320k"}
		if strings.Contains(item.MInfo, "format:flac") || strings.Contains(item.MInfo, "alflac") {
			types = append(types, "flac")
		}

		cover := strings.Replace(item.HtsMVPIC, "/star/albumcover/120/", "/star/albumcover/500/", 1)
		if cover == "" && item.WebAlbumPicShort != "" {
			// web_albumpic_short 形如 "120/s4s70/24/xxx.jpg"，只替换尺寸段
			cover = fmt.Sprintf("https://img4.kuwo.cn/star/albumcover/%s", item.WebAlbumPicShort)
			cover = strings.Replace(cover, "/star/albumcover/120/", "/star/albumcover/500/", 1)
			cover = strings.Replace(cover, "/star/albumcover/240/", "/star/albumcover/500/", 1)
			cover = strings.Replace(cover, "/star/albumcover/300/", "/star/albumcover/500/", 1)
		}

		list = append(list, SearchSong{
			ID:       fmt.Sprintf("kw_%s", rid),
			Source:   "kw",
			Name:     cleanHTMLTags(item.SongName),
			Singer:   cleanHTMLTags(item.Artist),
			Album:    cleanHTMLTags(item.Album),
			Duration: dur,
			Cover:    cover,
			Songmid:  rid,
			Types:    types,
		})
	}

	total, _ := strconv.Atoi(kwData.Total)
	return list, total, nil
}

// 网易云音乐搜索
func searchNetEase(keyword string, page, pageSize int) ([]SearchSong, int, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	apiURL := fmt.Sprintf("https://music.163.com/api/search/get/web?s=%s&type=1&offset=%d&total=true&limit=%d",
		url.QueryEscape(keyword), offset, pageSize)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var wyData struct {
		Result struct {
			SongCount int `json:"songCount"`
			Songs     []struct {
				ID       int64  `json:"id"`
				Name     string `json:"name"`
				Duration int    `json:"duration"`
				Artists  []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Album struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"album"`
			} `json:"songs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(bodyBytes, &wyData); err != nil {
		return nil, 0, err
	}

	var list []SearchSong
	for _, s := range wyData.Result.Songs {
		var singers []string
		for _, a := range s.Artists {
			singers = append(singers, a.Name)
		}
		singer := strings.Join(singers, " / ")
		if singer == "" {
			singer = "群星"
		}

		list = append(list, SearchSong{
			ID:       fmt.Sprintf("wy_%d", s.ID),
			Source:   "wy",
			Name:     s.Name,
			Singer:   singer,
			Album:    s.Album.Name,
			Duration: s.Duration / 1000,
			Cover:    s.Album.PicURL,
			Songmid:  strconv.FormatInt(s.ID, 10),
			Types:    []string{"128k", "320k", "flac"},
		})
	}

	return list, wyData.Result.SongCount, nil
}

// QQ 音乐搜索（new_json 接口，返回 songmid / 专辑 mid / 时长 / 可用音质）
func searchTencent(keyword string, page, pageSize int) ([]SearchSong, int, error) {
	if page < 1 {
		page = 1
	}
	apiURL := fmt.Sprintf("https://c.y.qq.com/soso/fcgi-bin/client_search_cp?p=%d&n=%d&w=%s&format=json&cr=1&t=0&aggr=1&lossless=0&catZhida=1&new_json=1",
		page, pageSize, url.QueryEscape(keyword))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://y.qq.com/")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var txData struct {
		Data struct {
			Song struct {
				Total int `json:"totalnum"`
				List  []struct {
					Mid      string `json:"mid"`
					Name     string `json:"name"`
					Title    string `json:"title"`
					Interval int    `json:"interval"`
					Singer   []struct {
						Name string `json:"name"`
					} `json:"singer"`
					Album struct {
						Name string `json:"name"`
						Mid  string `json:"mid"`
					} `json:"album"`
					File struct {
						Size128  int64 `json:"size_128mp3"`
						Size320  int64 `json:"size_320mp3"`
						SizeFlac int64 `json:"size_flac"`
					} `json:"file"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &txData); err != nil {
		return nil, 0, err
	}

	var list []SearchSong
	for _, item := range txData.Data.Song.List {
		var singers []string
		for _, s := range item.Singer {
			if s.Name != "" {
				singers = append(singers, s.Name)
			}
		}
		singer := strings.Join(singers, " / ")
		if singer == "" {
			singer = "群星"
		}
		name := cleanHTMLTags(item.Name)
		if name == "" {
			name = cleanHTMLTags(item.Title)
		}

		cover := ""
		if item.Album.Mid != "" {
			cover = fmt.Sprintf("https://y.qq.com/music/photo_new/T002R500x500M000%s.jpg", item.Album.Mid)
		}

		types := []string{"128k"}
		if item.File.Size320 > 0 {
			types = append(types, "320k")
		}
		if item.File.SizeFlac > 0 {
			types = append(types, "flac")
		}

		list = append(list, SearchSong{
			ID:       fmt.Sprintf("tx_%s", item.Mid),
			Source:   "tx",
			Name:     name,
			Singer:   cleanHTMLTags(singer),
			Album:    cleanHTMLTags(item.Album.Name),
			Duration: item.Interval,
			Cover:    cover,
			Songmid:  item.Mid,
			Types:    types,
		})
	}

	return list, txData.Data.Song.Total, nil
}

// 全网并发聚合搜索
func searchAllSources(keyword string, page, pageSize int) ([]SearchSong, int) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	var allSongs []SearchSong
	seen := make(map[string]bool)

	subPageSize := pageSize
	if subPageSize < 15 {
		subPageSize = 15
	}

	addSongs := func(songs []SearchSong) {
		mu.Lock()
		defer mu.Unlock()
		for _, s := range songs {
			// 规范化去重 Key (歌曲名 + 歌手首名)
			key := strings.ToLower(strings.TrimSpace(s.Name)) + "_" + strings.ToLower(strings.TrimSpace(s.Singer))
			if !seen[key] {
				seen[key] = true
				allSongs = append(allSongs, s)
			}
		}
	}

	// 1. 酷狗
	wg.Add(1)
	go func() {
		defer wg.Done()
		if list, _, err := searchKuGou(keyword, page, subPageSize); err == nil && len(list) > 0 {
			addSongs(list)
		}
	}()

	// 2. 酷我
	wg.Add(1)
	go func() {
		defer wg.Done()
		if list, _, err := searchKuWo(keyword, page, subPageSize); err == nil && len(list) > 0 {
			addSongs(list)
		}
	}()

	// 3. 网易云
	wg.Add(1)
	go func() {
		defer wg.Done()
		if list, _, err := searchNetEase(keyword, page, subPageSize); err == nil && len(list) > 0 {
			addSongs(list)
		}
	}()

	// 4. QQ 音乐
	wg.Add(1)
	go func() {
		defer wg.Done()
		if list, _, err := searchTencent(keyword, page, subPageSize); err == nil && len(list) > 0 {
			addSongs(list)
		}
	}()

	wg.Wait()
	return allSongs, len(allSongs)
}

var tagReg = regexp.MustCompile(`<[^>]*>`)

func cleanHTMLTags(str string) string {
	cleaned := tagReg.ReplaceAllString(str, "")
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	return strings.TrimSpace(cleaned)
}
