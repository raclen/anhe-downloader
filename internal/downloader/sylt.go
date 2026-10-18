package downloader

import (
	"encoding/binary"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/bogem/id3v2/v2"
)

// lrcTimeTag 匹配 LRC 时间标签，如 [01:23.45] / [01:23.456] / [01:23]
var lrcTimeTag = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)

// lrcWordTag 匹配增强型 LRC 的行内逐字时间标签 <01:23.45>
var lrcWordTag = regexp.MustCompile(`<\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?>`)

type syltText struct {
	Text string
	Time uint32 // 毫秒
}

// syltFrame ID3v2 SYLT（同步歌词）帧，用于让播放器按时间轴滚动显示内嵌歌词
type syltFrame struct {
	Language string
	Texts    []syltText
	body     []byte
}

func newSyltFrame(lrc string) *syltFrame {
	texts := parseLRCTexts(lrc)
	if len(texts) == 0 {
		return nil
	}
	f := &syltFrame{Language: "chi", Texts: texts}
	f.body = f.build()
	return f
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

func (f *syltFrame) build() []byte {
	buf := make([]byte, 0, 256)
	// 文本编码：UTF-16 with BOM（v2.3/v2.4 通用）
	buf = append(buf, 0x01)
	lang := f.Language
	if len(lang) != 3 {
		lang = "chi"
	}
	buf = append(buf, []byte(strings.ToLower(lang))...)
	// 时间戳格式：0x02 = 毫秒
	buf = append(buf, 0x02)
	// 内容类型：0x01 = 歌词
	buf = append(buf, 0x01)
	// 描述信息（空字符串，UTF-16 BOM + 终止符）
	buf = append(buf, 0xFF, 0xFE, 0x00, 0x00)

	for _, item := range f.Texts {
		buf = appendUTF16(buf, item.Text)
		var ts [4]byte
		binary.BigEndian.PutUint32(ts[:], item.Time)
		buf = append(buf, ts[:]...)
	}
	return buf
}

func appendUTF16(buf []byte, s string) []byte {
	// 每个字符串单独带 BOM，兼容各播放器解析
	buf = append(buf, 0xFF, 0xFE)
	for _, u := range utf16.Encode([]rune(s)) {
		buf = append(buf, byte(u), byte(u>>8))
	}
	buf = append(buf, 0x00, 0x00)
	return buf
}

func (f *syltFrame) Size() int {
	return len(f.body)
}

func (f *syltFrame) UniqueIdentifier() string {
	return ""
}

func (f *syltFrame) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(f.body)
	return int64(n), err
}

// addLyricsFrames 同时写入 USLT（非同步歌词）与 SYLT（同步歌词），最大化播放器兼容性
func addLyricsFrames(tag *id3v2.Tag, lrc string) {
	tag.DeleteFrames("USLT")
	tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
		Encoding:          id3v2.EncodingUTF16,
		Language:          "chi",
		ContentDescriptor: "",
		Lyrics:            lrc,
	})

	if sylt := newSyltFrame(lrc); sylt != nil {
		tag.DeleteFrames("SYLT")
		tag.AddFrame("SYLT", sylt)
	}
}
