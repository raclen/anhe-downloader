package downloader

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 本文件负责把 MP3 元数据写成 ID3v2.3。
// 说明：Windows 资源管理器、Windows Media Player 以及大量车载/国产播放器
// 只识别 ID3v2.3，ID3v2.4 的 APIC 封面会被直接忽略，因此这里自行构造标签，
// 保证标签版本、编码、帧长度都严格符合 v2.3 规范。

const id3v23HeaderSize = 10

// syncSafeInt 解析 ID3v2 的 syncsafe 长度（每字节仅低 7 位有效）
func syncSafeInt(b []byte) int {
	if len(b) < 4 {
		return 0
	}
	return int(b[0]&0x7F)<<21 | int(b[1]&0x7F)<<14 | int(b[2]&0x7F)<<7 | int(b[3]&0x7F)
}

func putSyncSafe(b []byte, v int) {
	b[0] = byte((v >> 21) & 0x7F)
	b[1] = byte((v >> 14) & 0x7F)
	b[2] = byte((v >> 7) & 0x7F)
	b[3] = byte(v & 0x7F)
}

// id3v23Frame 构造一个 v2.3 帧：10 字节帧头 + 帧体（长度为大端 uint32，不是 syncsafe）
func id3v23Frame(id string, body []byte) []byte {
	frame := make([]byte, 10+len(body))
	copy(frame[0:4], id)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(body)))
	frame[8] = 0
	frame[9] = 0
	copy(frame[10:], body)
	return frame
}

// utf16TextBody 文本帧帧体：编码字节 + BOM + UTF-16LE + 终止符
func utf16TextBody(s string) []byte {
	body := []byte{0x01}
	return appendUTF16String(body, s)
}

// usltBody 非同步歌词帧帧体：编码 + 语言 + 内容描述 + 歌词正文
func usltBody(lrc string) []byte {
	body := []byte{0x01}
	body = append(body, []byte("chi")...)
	body = append(body, 0xFF, 0xFE, 0x00, 0x00) // 空内容描述
	return appendUTF16String(body, lrc)
}

// apicBody 封面帧帧体：编码(Latin-1) + MIME + 0 + 图片类型 + 描述 + 0 + 图片数据
func apicBody(img []byte) []byte {
	mime := detectImageMime(img)
	if mime == "" {
		mime = "image/jpeg"
	}
	body := []byte{0x00}
	body = append(body, []byte(mime)...)
	body = append(body, 0x00)
	body = append(body, 0x03) // Picture type 3 = Front Cover
	body = append(body, 0x00) // 空描述
	body = append(body, img...)
	return body
}

// detectImageMime 通过文件头识别图片 MIME，返回空字符串表示不是可识别图片
func detectImageMime(b []byte) string {
	if len(b) < 12 {
		return ""
	}
	switch {
	case b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return "image/png"
	case b[0] == 'G' && b[1] == 'I' && b[2] == 'F' && b[3] == '8':
		return "image/gif"
	case b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P':
		return "image/webp"
	case b[0] == 'B' && b[1] == 'M':
		return "image/bmp"
	default:
		return ""
	}
}

// buildID3v23Tag 生成完整的 ID3v2.3 标签（含标签头）
func buildID3v23Tag(req *DownloadRequest, cover []byte) []byte {
	var frames []byte

	addText := func(id, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		frames = append(frames, id3v23Frame(id, utf16TextBody(value))...)
	}

	addText("TIT2", req.Title)
	addText("TPE1", req.Singer)
	addText("TALB", req.Album)

	if req.EmbedLyrics && strings.TrimSpace(req.Lyric) != "" {
		frames = append(frames, id3v23Frame("USLT", usltBody(req.Lyric))...)
		if sylt := buildSyltBody(req.Lyric); sylt != nil {
			frames = append(frames, id3v23Frame("SYLT", sylt)...)
		}
	}

	if len(cover) > 0 {
		frames = append(frames, id3v23Frame("APIC", apicBody(cover))...)
	}

	tag := make([]byte, id3v23HeaderSize, id3v23HeaderSize+len(frames))
	copy(tag[0:3], "ID3")
	tag[3] = 0x03 // v2.3
	tag[4] = 0x00 // 修订号
	tag[5] = 0x00 // 无 unsynchronisation / 扩展头
	putSyncSafe(tag[6:10], len(frames))
	return append(tag, frames...)
}

// writeMP3WithTag 用新标签替换文件开头已有的 ID3v2 标签，音频数据原样保留
func writeMP3WithTag(filePath string, tag []byte) error {
	src, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer src.Close()

	header := make([]byte, id3v23HeaderSize)
	n, readErr := io.ReadFull(src, header)
	offset := int64(0)
	if readErr == nil && n == id3v23HeaderSize && string(header[0:3]) == "ID3" {
		offset = int64(id3v23HeaderSize) + int64(syncSafeInt(header[6:10]))
		// v2.4 标签可能带 10 字节 footer
		if header[5]&0x10 != 0 {
			offset += id3v23HeaderSize
		}
	}
	if _, err := src.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	info, err := src.Stat()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(filePath), ".anhe-tag-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(tag); err != nil {
		return err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Windows 上目标文件仍被打开时 rename 会返回 Access denied，先关闭再重试
	_ = src.Close()
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		return fmt.Errorf("设置文件权限失败: %w", err)
	}

	var renameErr error
	for attempt := 0; attempt < 5; attempt++ {
		if renameErr = os.Rename(tmpPath, filePath); renameErr == nil {
			return nil
		}
		time.Sleep(time.Duration(80*(attempt+1)) * time.Millisecond)
	}
	return renameErr
}
