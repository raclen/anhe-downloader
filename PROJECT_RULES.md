# PROJECT_RULES · anhe-downloader

本文件记录长期有效、可复用的结论与不可破坏的边界。新任务开始前先读这里。

## 架构边界

- 桌面端 = Go 单二进制 + 内嵌 `frontend/dist`（`//go:embed all:frontend/dist`）+ native WebView2。
- 改前端后必须 `cd frontend && pnpm build`，再 `go build -ldflags "-H windowsgui -s -w" -o anhe-downloader.exe .`，否则 exe 里仍是旧 dist。
- 音源解析统一放在 Go 后端 `/api/resolve`，浏览器端只做展示与兜底。原因：`antiserver.kuwo.cn` 等接口**没有 CORS 头**，前端直接 fetch 会失败。

## 音源现状（2026-10 实测）

- `长青SVIP音源v1.3.0.protected.js` 的直链服务 `yinyue.haitangw.net/{kg,kw,qq,wy,mg}/*.php` 全部 404，脚本里的远端配置 `https://13413.kstore.vip/lxmusic/changqing.json` 只剩版本号。不要把下载可用性寄托在该脚本上，只能作为可更换音源的一层。
- 酷我 `anti.s`：免费曲目返回完整 MP3；**VIP 曲目会返回约 11 秒 / 181KB 的占位音频（所有 VIP 曲目返回同一文件）**。解析时必须用 `Range: bytes=0-1` 探测总大小，小于 900KB 视为试听片段并拒绝。
- 网易云 `music.163.com/song/media/outer/url?id=<id>.mp3`：免费曲目可用，VIP 返回 HTML（探测时按 Content-Type 拒绝）。
- 酷狗 `https://m.kugou.com/app/i/getSongInfo.php?cmd=playInfo&hash=<hash>`：免费曲目返回 `url`，付费返回 `error="需要付费"`。
- QQ 官方 `vkey.GetVkeyServer` 对免费曲目只给 `C400*.m4a`，320K MP3 需 VIP；因此 QQ 源以检索 + 跨源（酷我/酷狗/网易）匹配 MP3 为主。
- 跨源匹配必须校验歌手（`pickBestSong`），歌手对不上就放弃，**不能用别人的翻唱充数**。

## 元数据规则

- 目标交付物：**一份 MP3 内含封面 + 歌词**。
  - MP3：ID3 `TIT2/TPE1/TALB` + `USLT`（非同步）+ `SYLT`（同步滚动，毫秒时间戳）+ `APIC` 前置封面。
  - FLAC：Vorbis Comment 的 `LYRICS` / `UNSYNCEDLYRICS` + Picture Block。
- `.lrc` 独立文件默认关闭（`saveLrcFile: false`），仅在用户手动开启时额外生成。
- 默认首选音质为 `320k`（实际由解析结果决定，可能降级为 128k MP3）。

## 接口备忘

- QQ 搜索：`https://c.y.qq.com/soso/fcgi-bin/client_search_cp?p=&n=&w=&format=json&cr=1&t=0&aggr=1&lossless=0&catZhida=1&new_json=1`，需 `Referer: https://y.qq.com/`。注意 `file.size_128mp3/size_320mp3/size_flac` 是**数字**，别声明成 string。
- QQ 歌词：`https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=&format=json&nobase64=0&g_tk=5381`，返回 base64。
- QQ 封面：`https://y.qq.com/music/photo_new/T002R500x500M000<album.mid>.jpg`。
- 酷我封面：`hts_MVPIC` 是 MV 截图（可能是 324×183），必须优先用 `web_albumpic_short` 专辑封面并升级到 500×500。

## 下载任务与取消（2026-10 冒烟测试修复）

- `/api/download` 必须校验 `url`（仅 http/https）、`format`（mp3/flac）、`fileName`（缺省用 `singer - title`，仍为空则 400）；`taskId` 缺省或重复时由服务端生成，**禁止空 ID 入任务表**，否则多个任务会互相覆盖。
- 取消下载时 `resp.Body.Read` 会返回 `context canceled`，必须统一归类为 `errTaskCancelled`，不能当网络错误写 `error`。
- `downloadFile` 用 `defer` 在未成功时删除 `.downloading` 临时文件；`processDownload` 仅在任务仍为 `pending` 时启动，避免排队期取消后又开跑。
- 仅 `pending` / `downloading` 可取消；`tagging` / `completed` / `error` 返回 `success:false`，前端按返回值提示。
- `source=all` 聚合结果必须按 `pagesize` 截断，否则 `pagesize` 参数形同虚设。

## 验证清单

- `go build ./...` 通过；前端 `pnpm build` 通过。
- 抽样实测 `/api/search?source=tx`、`/api/resolve`、`/api/lyric`、`/api/download`。
- 用 `ffprobe` 确认下载时长不是 11 秒片段；用 `mutagen` / `id3v2` 确认 `USLT`、`SYLT`、`APIC` 均存在。

## 自定义音源（更换音源脚本）

- 前端 `Upload` 的 `beforeUpload` 不能只在内存里 `initFromScript` 然后调 `onReloadSource()`：后者会从后端重新拉取脚本，把刚加载的自定义脚本覆盖掉，表现就是「提示成功但实际没换」。
- 正确流程：① 前端先 `initFromScript` 验证是否注册了 `lx.on('request')`；② `POST /api/source-script`（原始 JS 文本）持久化；③ 再 `GET /api/source-script` 重新加载并刷新 `soundSourceMeta`。
- 自定义脚本固定保存为程序目录下的 `custom-source.js`；`main.go` 启动时优先加载它，`DELETE /api/source-script` 删除并回退内置长青音源。
- `SoundSourceEngine.initFromScript` 必须先清空旧的 `requestHandler`，并在脚本未注册处理器时返回 `false`，否则旧音源残留会让校验假成功。

## 仓库与文档

- 公开仓库不包含第三方音源脚本（`长青*.js` / `*.protected.js` 已在 `.gitignore`），也不提交 `custom-source.js`、`anhe-downloader.exe` 与 `temp/`。
- `frontend/dist` 会提交，保证克隆后可直接 `go build`；改前端后务必重新 `pnpm build` 并一起提交。
- 文档分工：`README.md` 作为落地页；`docs/USAGE.md` 使用指南、`docs/BUILD.md` 构建开发、`docs/API.md` 接口说明、`docs/FAQ.md` 常见问题；`CHANGELOG.md` 记录版本变更；`PROJECT_RULES.md` 记录开发约束与接口备忘。
- 提交信息使用中文，格式建议：`feat: ...` / `fix: ...` / `docs: ...` / `chore: ...`。
- 发布前检查：`go build ./...`、`cd frontend && pnpm build`、`ffprobe` 验证时长、`mutagen` 验证 `USLT/SYLT/APIC`。

## MP3 标签必须写 ID3v2.3

- Windows 资源管理器、WMP、椒盐音乐以及大量车载播放器**不读 ID3v2.4 的 `APIC`**，只认 ID3v2.3。因此 MP3 标签由 `internal/downloader/id3v23.go` 自行构造，禁止改回第三方库的默认 v2.4 写法。
- 旧库（bogem/id3v2）在 UTF-16 帧里会多写 NUL：`APIC` 图片数据前会出现 `0x00`，导致图片无法识别；`USLT` 也会乱码。自研写入器已规避。
- 标签替换流程：剥离文件头已有 ID3v2 标签 → 写新标签 → `io.Copy` 音频 → 关闭源文件 → `rename`（Windows 需重试）。
- 封面统一取 `500×500`，写入前用文件头校验 MIME；酷我 `120/240/300` 尺寸段自动替换为 `500`。
