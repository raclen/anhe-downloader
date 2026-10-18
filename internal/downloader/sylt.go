package downloader

import (
	"encoding/binary"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// lrcTimeTag 匹配 LRC 时间标签，如 [01:23.45] / [01:23.456] / [01:23]
var lrcTimeTag = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)

// lrcWordTag 匹配增强型 LRC 的行内逐字时间标签 <01:23.45>
var lrcWordTag = regexp.MustCompile(`<\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?>`)

type syltText struct {
	Text string
	Time uint32 // 毫秒
}

// parseLRCTexts 解析 LRC 为 (时间, 文本) 列表
func parseLRCTexts(lrc string) []syltText {
	var out []syltText
	for _, line := range strings.Split(strings.ReplaceAll(lrc, "\r\n", "\n"), "\n") {
		matches := lrcTimeTag.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		text := strings.TrimSpace(lrcWordTag.ReplaceAllString(lrcTimeTag.ReplaceAllString(line, ""), ""))
		if text == "" {
			continue
		}
		for _, m := range matches {
			min, _ := strconv.Atoi(line[m[2]:m[3]])
			sec, _ := strconv.Atoi(line[m[4]:m[5]])
			ms := 0
			if m[6] >= 0 {
				frac := line[m[6]:m[7]]
				switch len(frac) {
				case 1:
					ms, _ = strconv.Atoi(frac)
					ms *= 100
				case 2:
					ms, _ = strconv.Atoi(frac)
					ms *= 10
				default:
					ms, _ = strconv.Atoi(frac)
				}
			}
			out = append(out, syltText{Text: text, Time: uint32(min*60*1000 + sec*1000 + ms)})
		}
	}
	return out
}

// buildSyltBody 构建 ID3v2.3 SYLT 帧体：
// 文本编码(UTF-16) + 语言 + 时间戳格式(毫秒) + 内容类型(歌词) + 描述 + (文本, 时间戳)*
func buildSyltBody(lrc string) []byte {
	texts := parseLRCTexts(lrc)
	if len(texts) == 0 {
		return nil
	}

	buf := make([]byte, 0, 256)
	buf = append(buf, 0x01)                   // UTF-16 with BOM
	buf = append(buf, []byte("chi")...)       // 语言
	buf = append(buf, 0x02)                   // 时间戳格式：毫秒
	buf = append(buf, 0x01)                   // 内容类型：歌词
	buf = append(buf, 0xFF, 0xFE, 0x00, 0x00) // 空描述

	for _, item := range texts {
		buf = appendUTF16String(buf, item.Text)
		var ts [4]byte
		binary.BigEndian.PutUint32(ts[:], item.Time)
		buf = append(buf, ts[:]...)
	}
	return buf
}

// appendUTF16String 追加带 BOM 和双字节终止符的 UTF-16LE 字符串
func appendUTF16String(buf []byte, s string) []byte {
	buf = append(buf, 0xFF, 0xFE)
	for _, u := range utf16.Encode([]rune(s)) {
		buf = append(buf, byte(u), byte(u>>8))
	}
	buf = append(buf, 0x00, 0x00)
	return buf
}
