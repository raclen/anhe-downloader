package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"anhe-downloader/internal/downloader"
)

func newTestServer(t *testing.T) (*Server, *downloader.Engine) {
	t.Helper()
	engine := downloader.NewEngine(0)
	scriptPath := filepath.Join(t.TempDir(), "custom-source.js")
	return NewServer(engine, nil, scriptPath), engine
}

func postJSON(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeTask(t *testing.T, rec *httptest.ResponseRecorder) downloader.TaskInfo {
	t.Helper()
	var task downloader.TaskInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatalf("响应不是合法任务 JSON: %v, body=%s", err, rec.Body.String())
	}
	return task
}

func TestDownloadRejectsInvalidPayload(t *testing.T) {
	s, _ := newTestServer(t)
	validURL := `"url":"https://example.com/a.mp3"`

	cases := []struct {
		name string
		body string
	}{
		{"空对象", `{}`},
		{"缺少 url", `{"title":"测试"}`},
		{"非法协议", `{"url":"ftp://example.com/a.mp3","title":"测试"}`},
		{"相对路径", `{"url":"/a.mp3","title":"测试"}`},
		{"不支持格式", `{` + validURL + `,"title":"测试","format":"exe"}`},
		{"无可用文件名", `{` + validURL + `}`},
		{"点文件名", `{` + validURL + `,"title":"测试","fileName":"."}`},
		{"纯点文件名", `{` + validURL + `,"title":"测试","fileName":"..."}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, s, "/api/download", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("期望 400，实际 %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDownloadDefaultsFormatAndFileName(t *testing.T) {
	s, _ := newTestServer(t)
	rec := postJSON(t, s, "/api/download", `{
		"url":"https://example.com/a.mp3",
		"title":"测试歌曲",
		"singer":"测试歌手",
		"saveDir":"`+strings.ReplaceAll(t.TempDir(), `\`, `\\`)+`"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d: %s", rec.Code, rec.Body.String())
	}

	task := decodeTask(t, rec)
	if task.ID == "" {
		t.Fatal("缺省 taskId 时服务端应生成任务 ID")
	}
	if task.Format != "mp3" {
		t.Fatalf("缺省 format 应为 mp3，实际 %q", task.Format)
	}
	if !strings.HasSuffix(task.SavePath, "测试歌手 - 测试歌曲.mp3") {
		t.Fatalf("缺省文件名错误: %s", task.SavePath)
	}
}

func TestDownloadKeepsUniqueIDs(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{
		"taskId":"fixed-id",
		"url":"https://example.com/a.mp3",
		"title":"测试歌曲",
		"singer":"测试歌手",
		"fileName":"测试",
		"saveDir":"` + strings.ReplaceAll(t.TempDir(), `\`, `\\`) + `"
	}`

	first := decodeTask(t, postJSON(t, s, "/api/download", body))
	second := decodeTask(t, postJSON(t, s, "/api/download", body))

	if first.ID != "fixed-id" {
		t.Fatalf("首次请求应保留客户端 taskId，实际 %q", first.ID)
	}
	if second.ID == "" || second.ID == first.ID {
		t.Fatalf("重复 taskId 应生成新 ID，首次 %q 第二次 %q", first.ID, second.ID)
	}
}

func TestCancelTaskValidation(t *testing.T) {
	s, _ := newTestServer(t)

	if rec := postJSON(t, s, "/api/cancel-task", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("缺少 id 应返回 400，实际 %d", rec.Code)
	}

	rec := postJSON(t, s, "/api/cancel-task", `{"id":"not-exist"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("未知任务应返回 success:false，实际 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCancelPendingTaskViaHTTP(t *testing.T) {
	s, engine := newTestServer(t)
	task := engine.Enqueue(&downloader.DownloadRequest{
		TaskID:  "pending-http",
		URL:     "https://example.com/a.mp3",
		Title:   "测试",
		Format:  "mp3",
		SaveDir: t.TempDir(),
	})

	rec := postJSON(t, s, "/api/cancel-task", `{"id":"pending-http"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("排队任务应可取消，实际 %d: %s", rec.Code, rec.Body.String())
	}

	tasks := engine.GetTasks()
	if len(tasks) != 1 || tasks[0].ID != task.ID || tasks[0].Status != downloader.StatusPaused {
		t.Fatalf("任务状态应变为 paused，实际 %+v", tasks)
	}
}

func TestResolveAndLyricRequireIdentifier(t *testing.T) {
	s, _ := newTestServer(t)

	for _, path := range []string{"/api/resolve", "/api/lyric"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s 缺少参数应返回 400，实际 %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestDownloadBodyLimitDoesNotPanic(t *testing.T) {
	s, _ := newTestServer(t)
	// 超过 8MB 的请求体被截断，应作为非法 JSON 返回 400，而不是 OOM
	big := bytes.Repeat([]byte("a"), 9<<20)
	rec := postJSON(t, s, "/api/download", string(big))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超大请求体应返回 400，实际 %d", rec.Code)
	}
}
