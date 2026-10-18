# 构建与开发

## 1. 环境要求

| 组件 | 版本 |
| --- | --- |
| Windows | 10 / 11 |
| Go | 1.26+ |
| Node.js | 20+ |
| pnpm | 9+ |
| WebView2 | Windows 10/11 自带 |

克隆仓库：

```bash
git clone https://github.com/raclen/anhe-downloader.git
cd anhe-downloader
```

---

## 2. 构建前端

```bash
cd frontend
pnpm install
pnpm build
cd ..
```

产物输出到 `frontend/dist`，该目录已提交在仓库中，因此只改后端时可以直接跳到下一步。

> 修改任何 `frontend/src` 文件后，必须重新执行 `pnpm build` 再编译 exe，否则内嵌的仍是旧界面。

---

## 3. 构建桌面程序

```bash
go build -ldflags="-H=windowsgui -s -w" -o anhe-downloader.exe .
```

- `-H=windowsgui`：去掉控制台黑框；
- `-s -w`：裁剪符号表与调试信息，减小体积；
- 图标与版本信息来自仓库中已提交的 `rsrc_windows_*.syso`。

构建完成后双击 `anhe-downloader.exe` 即可运行。

---

## 4. 开发调试

### 方式 A：只调后端

```bash
go run . -headless -port 45678
```

浏览器打开 `http://127.0.0.1:45678`，使用已构建的 `frontend/dist`。

### 方式 B：前端热更新

终端 1（后端）：

```bash
go run . -headless -port 45678
```

终端 2（前端）：

```bash
cd frontend
pnpm dev
```

浏览器打开 `http://127.0.0.1:5173`。Vite 已配置 `'/api' -> http://127.0.0.1:45678` 代理，前端改动会实时热更新。

### 常用参数

```bash
anhe-downloader.exe -headless -port 45678
```

| 参数 | 说明 |
| --- | --- |
| `-headless` | 不创建 WebView2 窗口，只启动本地 HTTP 服务 |
| `-port` | 指定端口，默认优先 `45678`，被占用时自动随机 |

---

## 5. 图标与版本信息

仓库已包含 `rsrc_windows_386.syso` / `rsrc_windows_amd64.syso`。仅在修改图标或版本信息时需要重新生成，可使用 [go-winres](https://github.com/tc-hib/go-winres) 基于 `winres/winres.json` 生成后再提交 syso 文件。

---

## 6. 发布清单

1. 更新 `CHANGELOG.md`，确定版本号；
2. `cd frontend && pnpm build`；
3. `go build ./...` 与 `go vet ./...` 通过；
4. `go build -ldflags="-H=windowsgui -s -w" -o anhe-downloader.exe .`；
5. 冒烟测试：
   - `/api/search?source=tx` 能返回结果；
   - `/api/resolve` 能拿到完整音频直链；
   - 下载后用 `ffprobe` 确认时长，用 mutagen 确认 `USLT` / `SYLT` / `APIC` 存在且封面可被 `PIL` 打开；
6. 提交代码并推送；
7. `gh release create vX.Y.Z anhe-downloader.exe` 上传构建产物。

---

## 7. 目录说明

| 路径 | 说明 |
| --- | --- |
| `main.go` | 桌面宿主、嵌入前端、启动服务 |
| `internal/downloader` | 下载引擎、元数据抓取、ID3v2.3 / FLAC 写入 |
| `internal/server` | HTTP API、搜索、解析、音源脚本管理 |
| `frontend/src` | React 源码 |
| `frontend/dist` | 构建产物（提交到仓库） |
| `docs/` | 使用、构建、API、FAQ 文档 |
| `winres/` | 图标与 Windows 资源定义 |
