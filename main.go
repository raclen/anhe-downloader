package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"anhe-downloader/internal/downloader"
	"anhe-downloader/internal/server"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed all:frontend/dist
var embeddedFrontend embed.FS

func main() {
	headlessFlag := flag.Bool("headless", false, "以无窗口后台服务模式运行")
	portFlag := flag.Int("port", 0, "指定本地服务端口 (默认动态分配)")
	flag.Parse()

	// 1. 初始化并发下载引擎 (默认 3 并发)
	engine := downloader.NewEngine(3)

	// 2. 剥离 frontend/dist 前缀
	distFS, err := fs.Sub(embeddedFrontend, "frontend/dist")
	if err != nil {
		log.Fatalf("无法加载嵌入的前端资源: %v", err)
	}

	// 3. 寻找音源脚本（优先用户上传的自定义音源，其次内置长青音源）
	scriptPath := "长青SVIP音源v1.3.0.protected.js"
	exePath, _ := os.Executable()
	if !fileExists(scriptPath) {
		scriptPath = filepath.Join(filepath.Dir(exePath), "长青SVIP音源v1.3.0.protected.js")
	}
	customScriptPath := filepath.Join(filepath.Dir(scriptPath), "custom-source.js")
	if fileExists(customScriptPath) {
		scriptPath = customScriptPath
	}

	// 4. 创建服务器实例
	srv := server.NewServer(engine, distFS, scriptPath)

	// 5. 监听可用端口
	port := *portFlag
	var listener net.Listener
	if port > 0 {
		listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	} else {
		// 优先尝试 45678，若占用则随机分配
		listener, err = net.Listen("tcp", "127.0.0.1:45678")
		if err != nil {
			listener, err = net.Listen("tcp", "127.0.0.1:0")
		}
	}
	if err != nil {
		log.Fatalf("绑定本地网络端口失败: %v", err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	appURL := fmt.Sprintf("http://127.0.0.1:%d", actualPort)
	fmt.Printf("♪ 风宝音乐下载器服务已启动: %s\n", appURL)

	httpServer := &http.Server{
		Handler: srv.Handler(),
	}

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP 服务异常退出: %v", err)
		}
	}()

	// 6. 如果指定 headless 或无 GUI 环境，则仅常驻服务
	if *headlessFlag {
		fmt.Println("处于无窗口服务模式，请在浏览器中打开以上地址。")
		select {}
	}

	// 7. 在 Windows 10+ 环境下启动原生 WebView2 独立桌面客户端
	if runtime.GOOS == "windows" {
		w := webview2.NewWithOptions(webview2.WebViewOptions{
			Debug:     false,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  "风宝音乐下载器 ♪",
				Width:  1240,
				Height: 820,
				IconId: 2,
				Center: true,
			},
		})

		if w != nil {
			defer w.Destroy()
			w.SetSize(1240, 820, webview2.HintNone)
			w.Navigate(appURL)
			w.Run()
			_ = httpServer.Close()
			return
		}
	}

	// 如果 WebView2 窗口未能创建（例如远程非交互终端），备用唤起默认浏览器
	fmt.Println("正在打开系统浏览器...")
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", appURL).Start()
	for {
		time.Sleep(1 * time.Hour)
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
