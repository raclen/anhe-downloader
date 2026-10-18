package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"anhe-downloader/internal/downloader"
)

type Server struct {
	engine            *downloader.Engine
	embedFS           fs.FS
	scriptMu          sync.RWMutex
	scriptPath        string
	bundledScriptPath string
	customScriptPath  string
	musicDir          string
	defaultDir        string
}

func NewServer(engine *downloader.Engine, embedFS fs.FS, scriptPath string) *Server {
	// 默认音乐下载目录：用户的 Music 目录下的 AnHeMusic
	home, err := os.UserHomeDir()
	defaultMusicDir := "C:\\Users\\Administrator\\Music"
	if err == nil {
		defaultMusicDir = filepath.Join(home, "Music", "AnHeMusic")
	} else {
		defaultMusicDir = "E:\\Music\\Downloads"
	}
	_ = os.MkdirAll(defaultMusicDir, 0755)

	customScriptPath := filepath.Join(filepath.Dir(scriptPath), "custom-source.js")
	bundledScriptPath := filepath.Join(filepath.Dir(scriptPath), "长青SVIP音源v1.3.0.protected.js")

	return &Server{
		engine:            engine,
		embedFS:           embedFS,
		scriptPath:        scriptPath,
		bundledScriptPath: bundledScriptPath,
		customScriptPath:  customScriptPath,
		defaultDir:        defaultMusicDir,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/resolve", s.handleResolve)
	mux.HandleFunc("/api/lyric", s.handleLyric)
	mux.HandleFunc("/api/select-dir", s.handleSelectDir)
	mux.HandleFunc("/api/open-folder", s.handleOpenFolder)
	mux.HandleFunc("/api/source-script", s.handleSourceScript)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/cancel-task", s.handleCancelTask)

	// 前端静态文件托管
	if s.embedFS != nil {
		fileServer := http.FileServer(http.FS(s.embedFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			// 如果文件在 embedFS 中不存在，重定向回 index.html (SPA 支持)
			if f, err := s.embedFS.Open(path); err != nil {
				r.URL.Path = "/"
			} else {
				_ = f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return s.corsMiddleware(mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	scriptPath := s.currentScriptPath()
	resp := map[string]interface{}{
		"defaultDownloadDir": s.defaultDir,
		"hasScript":          fileExists(scriptPath),
		"scriptPath":         scriptPath,
		"version":            "1.3.0",
		"appName":            "风宝音乐下载器",
	}
	writeJSON(w, resp)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		writeJSON(w, SearchResponse{List: []SearchSong{}, Total: 0})
		return
	}
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pagesize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}

	var list []SearchSong
	var total int
	var err error

	switch source {
	case "kg":
		list, total, err = searchKuGou(keyword, page, pageSize)
	case "kw":
		list, total, err = searchKuWo(keyword, page, pageSize)
	case "wy":
		list, total, err = searchNetEase(keyword, page, pageSize)
	case "tx":
		list, total, err = searchTencent(keyword, page, pageSize)
	case "all":
		fallthrough
	default:
		list, total = searchAllSources(keyword, page, pageSize)
	}

	if err != nil && len(list) == 0 {
		// 仅全网聚合模式降级；指定单一平台时直接返回空，避免串源展示
		if source == "all" || source == "" {
			list, total = searchAllSources(keyword, page, pageSize)
		}
	}

	writeJSON(w, SearchResponse{
		List:  list,
		Total: total,
	})
}

func (s *Server) handleSourceScript(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(s.currentScriptPath())
		if err != nil {
			// 尝试当前目录长青音源
			data, err = os.ReadFile("长青SVIP音源v1.3.0.protected.js")
		}
		if err != nil {
			http.Error(w, "未找到音源脚本: "+err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(data)

	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			http.Error(w, "读取上传脚本失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			http.Error(w, "音源脚本内容为空", http.StatusBadRequest)
			return
		}

		target := s.customScriptPath
		tmp := target + ".tmp"
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			http.Error(w, "创建音源目录失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(tmp, body, 0644); err != nil {
			http.Error(w, "写入音源脚本失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, target); err != nil {
			_ = os.Remove(tmp)
			http.Error(w, "保存音源脚本失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.scriptMu.Lock()
		s.scriptPath = target
		s.scriptMu.Unlock()

		writeJSON(w, map[string]interface{}{
			"success": true,
			"path":    target,
			"size":    len(body),
		})

	case http.MethodDelete:
		if err := os.Remove(s.customScriptPath); err != nil && !os.IsNotExist(err) {
			http.Error(w, "删除自定义音源失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.scriptMu.Lock()
		s.scriptPath = s.bundledScriptPath
		s.scriptMu.Unlock()
		writeJSON(w, map[string]interface{}{
			"success": true,
			"path":    s.bundledScriptPath,
		})

	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "不支持的请求方法", http.StatusMethodNotAllowed)
	}
}

func (s *Server) currentScriptPath() string {
	s.scriptMu.RLock()
	defer s.scriptMu.RUnlock()
	return s.scriptPath
}

func (s *Server) handleLyric(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	songmid := r.URL.Query().Get("songmid")
	title := r.URL.Query().Get("title")
	singer := r.URL.Query().Get("singer")
	lrc := downloader.FetchLyric(source, songmid, title, singer)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"lyric": lrc,
	})
}

func (s *Server) handleSelectDir(w http.ResponseWriter, r *http.Request) {
	// 调用 Windows 原生 COM IFileOpenDialog 对话框，无 CMD 窗口，无中文乱码
	dir, err := SelectFolderNative("请选择音乐保存目录")
	if err != nil {
		log.Printf("选择目录异常: %v", err)
	}
	// 用户取消选择时返回空字符串，前端保持原配置不变
	if strings.TrimSpace(dir) == "" {
		writeJSON(w, map[string]string{"selectedDir": ""})
		return
	}
	writeJSON(w, map[string]string{"selectedDir": dir})
}

func (s *Server) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	target := body.Path
	if target == "" {
		target = s.defaultDir
	}
	target = filepath.Clean(target)

	// 存在文件则选中文件，否则打开目录
	if fi, err := os.Stat(target); err == nil {
		if fi.IsDir() {
			_ = exec.Command("explorer.exe", target).Start()
		} else {
			_ = exec.Command("explorer.exe", "/select,", target).Start()
		}
	} else {
		parent := filepath.Dir(target)
		_ = os.MkdirAll(parent, 0755)
		_ = exec.Command("explorer.exe", parent).Start()
	}

	writeJSON(w, map[string]bool{"success": true})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	var req downloader.DownloadRequest
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.SaveDir == "" {
		req.SaveDir = s.defaultDir
	}

	task := s.engine.Enqueue(&req)
	writeJSON(w, task)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.engine.GetTasks()
	writeJSON(w, tasks)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ok := s.engine.CancelTask(body.ID)
	writeJSON(w, map[string]bool{"success": ok})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
