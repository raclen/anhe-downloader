<p align="center">
  <img src="icon.png" width="96" alt="风宝音乐下载器图标">
</p>

<h1 align="center">风宝音乐下载器 · AnHe Music Downloader</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Platform-Windows%2010%2B-blue?style=flat-square&logo=windows" alt="Platform">
  <img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/React-18%20%2B%20Vite%206-61DAFB?style=flat-square&logo=react" alt="React">
  <img src="https://img.shields.io/badge/Binary-~9.5MB-brightgreen?style=flat-square" alt="Binary Size">
  <img src="https://img.shields.io/badge/License-MIT-yellow?style=flat-square" alt="License">
</p>

<p align="center">
  基于 <b>Go + 原生 Edge WebView2 + React 18</b> 的轻量 Windows 桌面音乐下载器<br>
  多源聚合检索 · 歌单批量导入 · 单文件 MP3 自动内嵌封面与滚动歌词
</p>

---

## 📖 简介

风宝音乐下载器是一个面向 Windows 10/11 的轻量桌面应用：

- 后端使用 Go 编写，编译为**单个 exe**（约 9.5MB），前端资源通过 `go:embed` 打包进二进制；
- 界面基于原生 WebView2，启动快、内存占用低，不需要安装 Electron 或浏览器环境；
- 默认把下载结果整理成**一份 MP3**：标题 / 歌手 / 专辑 + 高清封面 + USLT 歌词 + SYLT 同步滚动歌词；
- 支持洛雪（LX Music）自定义音源脚本，可上传、持久化、随时恢复内置音源；
- 音源解析全部在本地后端完成，规避第三方接口的 CORS 限制，并自动过滤 VIP 试听占位片段。

> 本项目仅供个人学习与音频技术研究使用，请勿用于商业传播。详见文末[免责声明](#-免责声明)。

---

## ✨ 核心特性

### 🚀 轻量原生架构
- Go + WebView2 单文件桌面程序，免去 Electron 动辄 150MB+ 的体积；
- 运行时内存约 35MB ~ 60MB，启动秒开、无黑色控制台窗口；
- 前端静态资源内嵌进 exe，双击即用，可随意移动目录。

### 🔍 多源聚合搜索
- 支持 **QQ 音乐、网易云音乐、酷我音乐、酷狗音乐** 单源检索与全网聚合去重；
- 搜索结果包含专辑、时长、封面与可用音质标签；
- 支持粘贴网易云歌单链接 / 歌单 ID 批量解析导入。

### ⚡ 异步流式下载
- Go 协程池 + `atomic` 状态管理，支持 1~5 并发下载；
- 实时显示进度、速度与总大小；
- 先写 `.downloading` 临时文件，完成后原子重命名，异常退出不产生半损坏文件。

### 🏷️ 单文件 MP3（封面 + 歌词）
- MP3：ID3v2 `TIT2` / `TPE1` / `TALB` + `USLT`（非同步歌词）+ `SYLT`（同步滚动歌词）+ `APIC`（前置封面）；
- FLAC：Vorbis Comment（`LYRICS` / `UNSYNCEDLYRICS`）+ Picture Block 封面；
- `.lrc` 独立文件**默认关闭**，需要兼容老式车载播放器时可在设置中开启。

### 🎵 洛雪音源沙箱
- 原生模拟 `globalThis.lx` 运行时（`on('request')` / `send('inited')` / `request`）；
- 上传脚本先校验是否注册 `request` 处理器，通过后保存为 `custom-source.js` 并立即生效，重启后继续使用；
- 内置备用解析链（酷我 → 酷狗 → 网易云，按歌名 + 歌手严格匹配），音源脚本失效时仍可下载免费曲目；
- 「恢复内置音源」一键删除自定义脚本并回退。

### 🎛️ 简洁的日间 UI
- React 18 + Vite 6 + Ant Design 5；
- 双栏布局、磨砂玻璃质感；
- 底部迷你试听播放器（进度拖拽、音量调节）。

---

## 🖥️ 快速开始

### 方式一：直接运行（推荐）

1. 从 [Releases](../../releases) 下载 `anhe-downloader.exe`（或自行编译）；
2. 将 exe 放到任意可写目录，双击运行，或双击 `启动.bat`。

> Windows 10 / 11 已内置 Edge WebView2 运行时，一般无需额外安装。

### 方式二：无窗口 / 浏览器模式

```bash
anhe-downloader.exe -headless -port 45678
```

然后浏览器打开 `http://127.0.0.1:45678`。

### 首次使用

1. 在「搜索」页输入歌名或歌手，选择平台或使用「全网聚合」；
2. 勾选歌曲后点「下载」，或单曲点下载按钮；
3. 「下载管理」查看进度，完成后点「定位文件」打开保存目录；
4. 需要更换音源时，在「设置 → 洛雪音源解析系统」上传 `.js` 脚本。

默认保存目录：`%USERPROFILE%\Music\AnHeMusic`，可在设置中修改。

---

## 🛠️ 项目结构

```text
anhe-downloader/
├── main.go                       # 桌面宿主：嵌入 dist、WebView2 窗口、启动本地服务
├── internal/
│   ├── downloader/
│   │   ├── downloader.go         # 下载引擎、并发调度、ID3v2 / FLAC 元数据写入
│   │   ├── metadata.go           # 多平台歌词与封面抓取（含 QQ 音乐歌词接口）
│   │   └── sylt.go               # LRC 解析与 ID3v2 SYLT 同步歌词帧
│   └── server/
│       ├── server.go             # HTTP API、目录选择、任务与音源脚本管理
│       ├── search.go             # 酷狗 / 酷我 / 网易云 / QQ 音乐检索
│       └── resolve.go            # 服务端统一直链解析、跨源匹配、试听片段过滤
├── frontend/                     # React 18 + TypeScript + Ant Design 5
│   ├── src/
│   │   ├── services/
│   │   │   ├── soundSourceEngine.ts  # 洛雪 lx 沙箱与解析调度
│   │   │   └── musicApi.ts           # 聚合搜索、歌单解析、歌词抓取
│   │   ├── views/                    # 搜索 / 批量导入 / 下载管理 / 设置
│   │   ├── components/MiniPlayer.tsx # 底部试听播放器
│   │   ├── App.tsx
│   │   └── App.css
│   └── dist/                     # 前端构建产物（已内嵌进 exe）
├── winres/                       # Windows 图标资源
├── rsrc_windows_*.syso           # 编译进 exe 的图标/版本信息
├── icon.png / icon.ico
├── 启动.bat
├── go.mod
├── README.md
├── CHANGELOG.md
├── PROJECT_RULES.md              # 开发约束与音源接口备忘
└── LICENSE
```

> 仓库**不附带**第三方音源脚本。运行时可自行放置 `长青SVIP音源v1.3.0.protected.js` 等合法获取的洛雪音源，或通过设置页上传。

---

## 📦 从源码构建

### 环境要求

- Windows 10 / 11
- Go 1.26+
- Node.js 20+ 与 pnpm

### 1. 构建前端

```bash
cd frontend
pnpm install
pnpm build
cd ..
```

### 2. 构建桌面程序

```bash
go build -ldflags="-H=windowsgui -s -w" -o anhe-downloader.exe .
```

> `frontend/dist` 已提交在仓库中，仅改后端时可直接执行第 2 步；改前端后必须重新 `pnpm build` 再编译 exe。

### 3. 开发调试

```bash
# 终端 A：前端热更新
cd frontend && pnpm dev

# 终端 B：后端（浏览器访问 http://127.0.0.1:45678）
go run . -headless -port 45678
```

---

## 🔌 本地 API

服务仅监听 `127.0.0.1`，所有接口均为本地调用：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/config` | 默认下载目录、音源脚本状态 |
| GET | `/api/search` | 聚合搜索（`keyword` / `source` / `page` / `pagesize`） |
| GET | `/api/resolve` | 解析音频直链（`source` / `songmid` / `title` / `singer` / `quality`） |
| GET | `/api/lyric` | 抓取 LRC 歌词 |
| POST | `/api/download` | 创建下载任务 |
| GET | `/api/tasks` | 查询任务列表与进度 |
| POST | `/api/cancel-task` | 取消任务 |
| GET / POST / DELETE | `/api/source-script` | 读取 / 上传 / 恢复音源脚本 |
| GET | `/api/select-dir` | 调起原生目录选择器 |
| POST | `/api/open-folder` | 在资源管理器中定位文件 |

---

## 🎼 音源脚本说明

- 遵循洛雪自定义音源协议：脚本通过 `globalThis.lx.on('request', handler)` 注册解析器，平台标识通常为 `kw` / `kg` / `wy` / `tx` / `mg`。
- 上传流程：浏览器内先验证脚本能注册 `request` 处理器 → `POST /api/source-script` 持久化 → 重新加载并刷新状态。
- 保存位置：程序目录下的 `custom-source.js`（已加入 `.gitignore`），启动时优先加载。
- 恢复：设置页「恢复内置音源」或 `DELETE /api/source-script`。
- 安全提示：音源脚本是可直接执行的 JavaScript，请只加载可信来源的脚本。

---

## ⚠️ 已知限制

- **VIP / 付费曲目**：各平台免费接口通常只返回短试听片段，解析器会用 Range 探测音频大小并拒绝这类片段，提示「暂无可下载的完整音源」。这是版权限制，不是程序 bug。
- **第三方直链服务不稳定**：部分音源脚本依赖的外部解析服务可能随时失效（例如长青音源的 `yinyue.haitangw.net` 已返回 404），此时会自动回退到内置跨源解析链。
- **版权与地区限制**：不同地区、不同时间的接口可用性会变化，个别歌曲可能搜索得到但无法解析。
- **仅支持 Windows**：桌面窗口依赖 WebView2；Linux/macOS 可尝试 `-headless` 模式自行构建。

---

## ❓ 常见问题

<details>
<summary>为什么下载后只有一个 MP3，没有 .lrc？</summary>

这是默认行为：歌词已经以 `USLT` + `SYLT` 内嵌进 MP3，封面写入 `APIC`。若播放器需要外挂歌词，请在设置中开启「额外另存 .lrc 歌词文件」。
</details>

<details>
<summary>播放器看不到内嵌歌词怎么办？</summary>

请确认播放器支持 ID3v2 歌词帧（如 foobar2000、Poweramp、海贝音乐等）。部分老式车载播放器只认同名 `.lrc`，此时开启外挂歌词即可。
</details>

<details>
<summary>搜索有 QQ 音乐，但下载提示没有完整音源？</summary>

QQ 官方接口对免费曲目只提供 m4a，对 VIP 曲目需要登录凭证。程序会尝试跨源匹配酷我 / 酷狗 / 网易云的 MP3；若全部失败，说明该曲目受版权保护，暂时无法下载完整版本。
</details>

<details>
<summary>更换音源后重启又变回去了？</summary>

请使用设置页的「更换音源脚本」上传，脚本会保存为程序目录的 `custom-source.js` 并在启动时优先加载。直接替换内置脚本文件不会被持久化。
</details>

---

## 📚 文档索引

- [README.md](README.md)：项目介绍、构建与使用说明
- [CHANGELOG.md](CHANGELOG.md)：版本变更记录
- [PROJECT_RULES.md](PROJECT_RULES.md)：开发约束、音源接口备忘与已知坑
- [LICENSE](LICENSE)：MIT 开源许可证

---

## ⚖️ 免责声明

1. 本软件仅供个人学习、技术研究与音频技术交流使用。
2. 音频、封面、歌词等内容的版权均归原权利人所有。
3. 请在法律法规允许的范围内合理使用，请勿用于商业盈利或非法传播。
4. 使用者需自行承担因使用本软件产生的一切后果。

---

## 🤝 鸣谢

- [LX Music Desktop（洛雪音乐助手）](https://github.com/lyswhut/lx-music-desktop)
- [go-webview2](https://github.com/jchv/go-webview2)
- [bogem/id3v2](https://github.com/bogem/id3v2)
- [go-flac](https://github.com/go-flac/go-flac)
- [OuterTune](https://github.com/OuterTune/OuterTune)

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 开源。
