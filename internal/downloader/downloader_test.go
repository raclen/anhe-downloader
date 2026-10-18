package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitTaskStatus 等待任务进入期望状态，超时则测试失败。
func waitTaskStatus(t *testing.T, e *Engine, id string, want TaskStatus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		task, ok := e.tasks[id]
		status := TaskStatus("")
		if ok {
			status = task.Status
		}
		e.mu.RUnlock()

		if ok && status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	e.mu.RLock()
	status := TaskStatus("<missing>")
	if task, ok := e.tasks[id]; ok {
		status = task.Status
	}
	e.mu.RUnlock()
	t.Fatalf("任务 %s 未进入状态 %s，当前为 %s", id, want, status)
}

func taskSnapshot(e *Engine, id string) (TaskInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	task, ok := e.tasks[id]
	if !ok {
		return TaskInfo{}, false
	}
	return *task, true
}

// slowAudioServer 持续缓慢输出数据，便于测试下载中取消。
func slowAudioServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 1024)
		for i := 0; i < 1024; i++ {
			if _, err := w.Write(buf); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
}

func TestEnqueueGeneratesUniqueTaskID(t *testing.T) {
	e := NewEngine(0)
	dir := t.TempDir()

	first := e.Enqueue(&DownloadRequest{URL: "http://example.invalid/a.mp3", Title: "A", SaveDir: dir})
	second := e.Enqueue(&DownloadRequest{URL: "http://example.invalid/b.mp3", Title: "B", SaveDir: dir})

	if first.ID == "" || second.ID == "" {
		t.Fatalf("服务端应生成非空任务 ID: %q %q", first.ID, second.ID)
	}
	if first.ID == second.ID {
		t.Fatalf("不同任务不应共用 ID: %s", first.ID)
	}
	if first.Format != "mp3" {
		t.Fatalf("format 缺省应为 mp3，实际 %q", first.Format)
	}
	if filepath.Ext(first.SavePath) != ".mp3" {
		t.Fatalf("savePath 扩展名错误: %s", first.SavePath)
	}
}

func TestCancelDownloadingTaskCleansTempFile(t *testing.T) {
	srv := slowAudioServer(t)
	defer srv.Close()

	e := NewEngine(1)
	dir := t.TempDir()
	task := e.Enqueue(&DownloadRequest{
		TaskID:  "cancel-delay",
		URL:     srv.URL + "/slow.mp3",
		Title:   "慢速测试",
		Singer:  "测试",
		Format:  "mp3",
		SaveDir: dir,
	})

	waitTaskStatus(t, e, task.ID, StatusDownloading)

	// 等待至少写入一部分数据，确保临时文件已经产生
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if snap, ok := taskSnapshot(e, task.ID); ok && snap.DownloadedBytes > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !e.CancelTask(task.ID) {
		t.Fatal("下载中的任务应当允许取消")
	}
	waitTaskStatus(t, e, task.ID, StatusPaused)

	snap, _ := taskSnapshot(e, task.ID)
	if snap.ErrorMsg != "" {
		t.Fatalf("取消后不应保留错误信息: %q", snap.ErrorMsg)
	}
	if snap.Speed != "已取消" {
		t.Fatalf("取消后速度文案错误: %q", snap.Speed)
	}
	if _, err := os.Stat(snap.SavePath + ".downloading"); !os.IsNotExist(err) {
		t.Fatalf("取消后临时文件应被删除: %s", snap.SavePath+".downloading")
	}
	if _, err := os.Stat(snap.SavePath); !os.IsNotExist(err) {
		t.Fatalf("取消后不应生成最终文件: %s", snap.SavePath)
	}
}

func TestCancelPendingTaskIsSkipped(t *testing.T) {
	srv := slowAudioServer(t)
	defer srv.Close()

	e := NewEngine(1)
	dir := t.TempDir()
	running := e.Enqueue(&DownloadRequest{
		TaskID:  "blocker",
		URL:     srv.URL + "/slow.mp3",
		Title:   "占位任务",
		Format:  "mp3",
		SaveDir: dir,
	})
	waitTaskStatus(t, e, running.ID, StatusDownloading)

	pending := e.Enqueue(&DownloadRequest{
		TaskID:  "pending-cancel",
		URL:     srv.URL + "/pending.mp3",
		Title:   "排队取消",
		Format:  "mp3",
		SaveDir: dir,
	})
	if !e.CancelTask(pending.ID) {
		t.Fatal("排队中的任务应当允许取消")
	}

	// 给 worker 足够时间，如果逻辑有误它会开始下载并改写状态
	time.Sleep(300 * time.Millisecond)
	snap, _ := taskSnapshot(e, pending.ID)
	if snap.Status != StatusPaused {
		t.Fatalf("排队任务取消后应保持 paused，实际 %s", snap.Status)
	}
	if _, err := os.Stat(snap.SavePath + ".downloading"); !os.IsNotExist(err) {
		t.Fatalf("排队任务取消后不应创建临时文件: %s", snap.SavePath+".downloading")
	}

	e.CancelTask(running.ID)
	waitTaskStatus(t, e, running.ID, StatusPaused)
}

func TestCancelCompletedTaskReturnsFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3\x04\x00\x00\x00\x00\x00\x00fake-mp3-payload"))
	}))
	defer srv.Close()

	e := NewEngine(1)
	task := e.Enqueue(&DownloadRequest{
		TaskID:  "completed-cancel",
		URL:     srv.URL + "/fast.mp3",
		Title:   "完成测试",
		Singer:  "测试",
		Format:  "mp3",
		SaveDir: t.TempDir(),
	})

	waitTaskStatus(t, e, task.ID, StatusCompleted)
	if e.CancelTask(task.ID) {
		t.Fatal("已完成任务不应允许取消")
	}
}
