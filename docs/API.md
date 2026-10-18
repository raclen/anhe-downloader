# 本地 API

服务默认监听 `127.0.0.1:45678`（被占用时自动换端口），所有接口仅在本机可访问。

- 基础地址：`http://127.0.0.1:45678`
- 编码：JSON 使用 UTF-8
- CORS：允许任意来源（仅本地服务）
- 错误约定：业务错误通常返回 `4xx` 且响应体为纯文本错误信息

---

## GET /api/config

读取运行配置与音源脚本状态。

```json
{
  "appName": "风宝音乐下载器",
  "version": "1.3.1",
  "defaultDownloadDir": "C:\\Users\\<user>\\Music\\AnHeMusic",
  "hasScript": true,
  "scriptPath": "custom-source.js"
}
```

---

## GET /api/search

多源检索歌曲。

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `keyword` | 是 | 歌名 / 歌手 / 专辑 |
| `source` | 否 | `all`（默认）/ `tx` / `kg` / `kw` / `wy` |
| `page` | 否 | 页码，默认 1 |
| `pagesize` | 否 | 每页数量，默认 30，最大 100 |

响应：

```json
{
  "list": [
    {
      "id": "tx_0039MnYb0qxYhV",
      "source": "tx",
      "name": "晴天",
      "singer": "周杰伦",
      "album": "叶惠美",
      "duration": 269,
      "cover": "https://y.qq.com/music/photo_new/T002R500x500M000000MkMni19ClKG.jpg",
      "songmid": "0039MnYb0qxYhV",
      "types": ["128k", "320k", "flac"]
    }
  ],
  "total": 997
}
```

```bash
curl "http://127.0.0.1:45678/api/search?keyword=%E6%99%B4%E5%A4%A9&source=tx&page=1&pagesize=5"
```

---

## GET /api/resolve

解析音频直链。服务端会依次尝试原平台、酷我、酷狗、网易云，并过滤 VIP 试听占位片段。

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `source` | 否 | 平台标识 |
| `songmid` | 否 | 平台歌曲 ID / Hash |
| `title` | 是 | 歌名（跨源匹配用） |
| `singer` | 是 | 歌手（跨源匹配用） |
| `quality` | 否 | `128k` / `320k` / `flac`，默认 320k |

成功响应：

```json
{
  "url": "https://.../xxx.mp3",
  "quality": "320k",
  "format": "mp3",
  "resolver": "酷我直链"
}
```

失败示例（HTTP 404）：

```text
《晴天》暂无可下载的完整音源（VIP / 付费或版权限制），可换其他版本试试
```

---

## GET /api/lyric

抓取 LRC 歌词，按平台 ID 精确获取，失败时按歌名 + 歌手全网检索。

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `source` | 否 | 平台标识 |
| `songmid` | 否 | 平台歌曲 ID |
| `title` | 是 | 歌名 |
| `singer` | 是 | 歌手 |

```json
{ "lyric": "[00:00.00]晴天 - 周杰伦\n[00:02.25]词：周杰伦\n..." }
```

---

## POST /api/download

创建下载任务，立即返回任务信息，实际下载异步进行。

```json
{
  "taskId": "task_1750000000000_ab12",
  "url": "https://.../xxx.mp3",
  "title": "晴天",
  "singer": "周杰伦",
  "album": "叶惠美",
  "quality": "320k",
  "format": "mp3",
  "coverUrl": "https://.../cover.jpg",
  "lyric": "[00:00.00]...",
  "saveDir": "C:\\Users\\<user>\\Music\\AnHeMusic",
  "fileName": "周杰伦 - 晴天",
  "embedCover": true,
  "embedLyrics": true,
  "saveLrcFile": false,
  "source": "tx",
  "songmid": "0039MnYb0qxYhV"
}
```

| 字段 | 说明 |
| --- | --- |
| `format` | `mp3` / `flac`，决定扩展名与标签写入方式 |
| `embedCover` | 写入 `APIC` / FLAC Picture |
| `embedLyrics` | 写入 `USLT` + `SYLT` / Vorbis Comment |
| `saveLrcFile` | 额外生成同名 `.lrc` |

---

## GET /api/tasks

返回全部任务（数组）：

```json
[
  {
    "id": "task_1750000000000_ab12",
    "title": "晴天",
    "singer": "周杰伦",
    "album": "叶惠美",
    "quality": "320k",
    "format": "mp3",
    "coverUrl": "https://.../cover.jpg",
    "savePath": "C:\\Users\\<user>\\Music\\AnHeMusic\\周杰伦 - 晴天.mp3",
    "status": "completed",
    "progress": 100,
    "downloadedBytes": 10500000,
    "totalBytes": 10500000,
    "speed": "已完成",
    "createdAt": 1750000000
  }
]
```

`status` 取值：`pending` / `downloading` / `tagging` / `completed` / `error` / `paused`。

---

## POST /api/cancel-task

```json
{ "id": "task_1750000000000_ab12" }
```

响应：`{ "success": true }`。取消后会删除 `.downloading` 临时文件。

---

## /api/source-script

| 方法 | 说明 |
| --- | --- |
| `GET` | 返回当前音源脚本源码（`application/javascript`） |
| `POST` | 请求体为原始 JS 文本，保存为 `custom-source.js` 并设为当前脚本 |
| `DELETE` | 删除自定义脚本，回退内置音源 |

POST 成功响应：

```json
{ "success": true, "path": "custom-source.js", "size": 38858 }
```

```bash
curl -X POST --data-binary @my-source.js \
  -H "Content-Type: application/javascript" \
  http://127.0.0.1:45678/api/source-script
```

---

## GET /api/select-dir

调起原生 Windows 目录选择器，返回：

```json
{ "selectedDir": "D:\\Music" }
```

用户取消时 `selectedDir` 为空字符串。

---

## POST /api/open-folder

在资源管理器中打开目录或选中文件。

```json
{ "path": "C:\\Users\\<user>\\Music\\AnHeMusic\\周杰伦 - 晴天.mp3" }
```

响应：`{ "success": true }`。
