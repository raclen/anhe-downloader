package downloader

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	flacpicture "github.com/go-flac/flacpicture"
	flacvorbis "github.com/go-flac/flacvorbis"
	flac "github.com/go-flac/go-flac"
)

type TaskStatus string

const (
	StatusPending     TaskStatus = "pending"
	StatusDownloading TaskStatus = "downloading"
	StatusTagging     TaskStatus = "tagging"
	StatusCompleted   TaskStatus = "completed"
	StatusError       TaskStatus = "error"
	StatusPaused      TaskStatus = "paused"
)

type DownloadRequest struct {
	TaskID      string `json:"taskId"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Singer      string `json:"singer"`
	Album       string `json:"album"`
	Quality     string `json:"quality"`
	Format      string `json:"format"`
	CoverURL    string `json:"coverUrl"`
	Lyric       string `json:"lyric"`
	SaveDir     string `json:"saveDir"`
	FileName    string `json:"fileName"`
	EmbedCover  bool   `json:"embedCover"`
	EmbedLyrics bool   `json:"embedLyrics"`
	SaveLrcFile bool   `json:"saveLrcFile"`
	Source      string `json:"source"`
	Songmid     string `json:"songmid"`
}

type TaskInfo struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Singer          string     `json:"singer"`
	Album           string     `json:"album"`
	Quality         string     `json:"quality"`
	Format          string     `json:"format"`
	CoverURL        string     `json:"coverUrl"`
	SavePath        string     `json:"savePath"`
	Status          TaskStatus `json:"status"`
	Progress        float64    `json:"progress"`
	DownloadedBytes int64      `json:"downloadedBytes"`
	TotalBytes      int64      `json:"totalBytes"`
	Speed           string     `json:"speed"`
	ErrorMsg        string     `json:"errorMsg,omitempty"`
	CreatedAt       int64      `json:"createdAt"`

	cancelFunc context.CancelFunc
	lastBytes  int64
	lastTime   time.Time
}

type Engine struct {
	mu         sync.RWMutex
	tasks      map[string]*TaskInfo
	queue      chan *DownloadRequest
	httpClient *http.Client
}

func NewEngine(maxConcurrent int) *Engine {
	e := &Engine{
		tasks: make(map[string]*TaskInfo),
		queue: make(chan *DownloadRequest, 200),
		httpClient: &http.Client{
			Timeout: 0, // 流式下载不设全局单一超时，依赖 Context 控制
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: true,
			},
		},
	}

	for i := 0; i < maxConcurrent; i++ {
		go e.worker()
	}

	// 速度采样定时器
	go e.speedTicker()

	return e
}

func (e *Engine) Enqueue(req *DownloadRequest) *TaskInfo {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 格式化输出文件路径
	ext := "." + req.Format
	if !strings.HasSuffix(strings.ToLower(req.FileName), ext) {
		req.FileName = req.FileName + ext
	}

	// 清洗文件名中的 Windows 非法字符
	req.FileName = sanitizeFileName(req.FileName)
	savePath := filepath.Join(req.SaveDir, req.FileName)

	task := &TaskInfo{
		ID:        req.TaskID,
		Title:     req.Title,
		Singer:    req.Singer,
		Album:     req.Album,
		Quality:   req.Quality,
		Format:    req.Format,
		CoverURL:  req.CoverURL,
		SavePath:  savePath,
		Status:    StatusPending,
		Progress:  0,
		Speed:     "0 KB/s",
		CreatedAt: time.Now().Unix(),
		lastTime:  time.Now(),
	}

	e.tasks[req.TaskID] = task
	e.queue <- req
	return task
}

func (e *Engine) GetTasks() []*TaskInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]*TaskInfo, 0, len(e.tasks))
	for _, t := range e.tasks {
		list = append(list, t)
	}
	return list
}

func (e *Engine) CancelTask(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t, ok := e.tasks[id]; ok {
		if t.cancelFunc != nil {
			t.cancelFunc()
		}
		t.Status = StatusPaused
		t.Speed = "已取消"
		return true
	}
	return false
}

func (e *Engine) worker() {
	for req := range e.queue {
		e.processDownload(req)
	}
}

func (e *Engine) processDownload(req *DownloadRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	task, exists := e.tasks[req.TaskID]
	if !exists {
		e.mu.Unlock()
		cancel()
		return
	}
	task.cancelFunc = cancel
	task.Status = StatusDownloading
	e.mu.Unlock()

	err := e.downloadFile(ctx, req, task)
	if err != nil {
		e.mu.Lock()
		task.Status = StatusError
		task.ErrorMsg = err.Error()
		task.Speed = "下载失败"
		e.mu.Unlock()
		return
	}

	// 下载完成，进入标签处理
	e.mu.Lock()
	task.Status = StatusTagging
	task.Speed = "写入标签与封面"
	e.mu.Unlock()

	e.tagFile(req, task.SavePath)

	e.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = 100
	task.Speed = "已完成"
	e.mu.Unlock()
}

func (e *Engine) downloadFile(ctx context.Context, req *DownloadRequest, task *TaskInfo) error {
	_ = os.MkdirAll(req.SaveDir, 0755)

	tmpPath := task.SavePath + ".downloading"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URL, nil)
	if err != nil {
		return fmt.Errorf("构建请求失败: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	httpReq.Header.Set("Referer", refererForURL(req.URL))

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	total := resp.ContentLength
	atomic.StoreInt64(&task.TotalBytes, total)

	buf := make([]byte, 64*1024)
	var downloaded int64

	for {
		select {
		case <-ctx.Done():
			_ = out.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("任务已终止")
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("写入磁盘失败: %w", writeErr)
			}
			downloaded += int64(n)
			atomic.StoreInt64(&task.DownloadedBytes, downloaded)

			if total > 0 {
				prog := float64(downloaded) / float64(total) * 100
				e.mu.Lock()
				task.Progress = prog
				e.mu.Unlock()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("流读取异常: %w", readErr)
		}
	}

	_ = out.Close()

	// 移除可能存在的已有文件，原子重命名完成
	_ = os.Remove(task.SavePath)
	if err := os.Rename(tmpPath, task.SavePath); err != nil {
		return fmt.Errorf("重命名目标文件失败: %w", err)
	}

	return nil
}

func (e *Engine) tagFile(req *DownloadRequest, filePath string) {
	// 1. 如果请求需要歌词且 req.Lyric 为空，自动从后端多渠道拉取同步歌词
	if (req.EmbedLyrics || req.SaveLrcFile) && strings.TrimSpace(req.Lyric) == "" {
		req.Lyric = FetchLyric(req.Source, req.Songmid, req.Title, req.Singer)
	}

	// 2. 如果需要保存单独的 .lrc 文件
	if req.SaveLrcFile && strings.TrimSpace(req.Lyric) != "" {
		lrcPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".lrc"
		_ = os.WriteFile(lrcPath, []byte(req.Lyric), 0644)
	}

	// 3. 获取封面二进制数据 (如果启用内嵌封面)
	var coverBytes []byte
	if req.EmbedCover {
		coverBytes = FetchCoverBytes(req.CoverURL, req.Source, req.Songmid, req.Title, req.Singer)
	}

	// 4. 根据扩展名处理 ID3 / FLAC 元数据
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".mp3" {
		// 统一写 ID3v2.3：Windows 与大多数车载/国产播放器不识别 v2.4 的封面帧
		tagBytes := buildID3v23Tag(req, coverBytes)
		if err := writeMP3WithTag(filePath, tagBytes); err != nil {
			log.Printf("MP3 标签写入失败: %s: %v", filePath, err)
		}
	} else if ext == ".flac" {
		f, err := flac.ParseFile(filePath)
		if err == nil {
			var newMeta []*flac.MetaDataBlock
			var existingCmt *flacvorbis.MetaDataBlockVorbisComment

			for _, block := range f.Meta {
				if block.Type == flac.VorbisComment {
					if cmt, cErr := flacvorbis.ParseFromMetaDataBlock(*block); cErr == nil {
						existingCmt = cmt
					}
				} else if block.Type == flac.Picture {
					// 过滤原图，统一写入高清封面
				} else {
					newMeta = append(newMeta, block)
				}
			}

			var cmt *flacvorbis.MetaDataBlockVorbisComment
			if existingCmt != nil {
				cmt = existingCmt
			} else {
				cmt = flacvorbis.New()
			}

			_ = cmt.Add(flacvorbis.FIELD_TITLE, req.Title)
			_ = cmt.Add(flacvorbis.FIELD_ARTIST, req.Singer)
			_ = cmt.Add(flacvorbis.FIELD_ALBUM, req.Album)
			if req.EmbedLyrics && strings.TrimSpace(req.Lyric) != "" {
				_ = cmt.Add("LYRICS", req.Lyric)
				_ = cmt.Add("UNSYNCEDLYRICS", req.Lyric)
			}
			cmBlock := cmt.Marshal()
			newMeta = append(newMeta, &cmBlock)

			// Picture Block
			if len(coverBytes) > 0 {
				mime := "image/jpeg"
				if len(coverBytes) > 8 && coverBytes[0] == 0x89 && coverBytes[1] == 'P' && coverBytes[2] == 'N' && coverBytes[3] == 'G' {
					mime = "image/png"
				}
				pic, pErr := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front Cover", coverBytes, mime)
				if pErr == nil {
					pBlock := pic.Marshal()
					newMeta = append(newMeta, &pBlock)
				}
			}

			f.Meta = newMeta
			_ = f.Save(filePath)
		}
	}
}

func (e *Engine) speedTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		e.mu.Lock()
		now := time.Now()
		for _, task := range e.tasks {
			if task.Status != StatusDownloading {
				continue
			}

			currentBytes := atomic.LoadInt64(&task.DownloadedBytes)
			deltaBytes := currentBytes - task.lastBytes
			duration := now.Sub(task.lastTime).Seconds()
			if duration <= 0 {
				duration = 1
			}

			speedBps := float64(deltaBytes) / duration
			task.Speed = formatSpeed(speedBps)
			task.lastBytes = currentBytes
			task.lastTime = now
		}
		e.mu.Unlock()
	}
}

func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	} else if bytesPerSec >= 1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.0f B/s", bytesPerSec)
}

func sanitizeFileName(name string) string {
	illegal := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, ch := range illegal {
		name = strings.ReplaceAll(name, ch, "_")
	}
	return strings.TrimSpace(name)
}

// refererForURL 按音频域名设置合适的 Referer，避免第三方 CDN 拒绝请求
func refererForURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "https://y.qq.com/"
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "kuwo"):
		return "https://www.kuwo.cn/"
	case strings.Contains(host, "kugou"):
		return "https://www.kugou.com/"
	case strings.Contains(host, "163.com"):
		return "https://music.163.com/"
	case strings.Contains(host, "qq.com"):
		return "https://y.qq.com/"
	default:
		return "https://y.qq.com/"
	}
}
