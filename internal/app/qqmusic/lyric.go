package qqmusic

// 歌词：获取（Web 匿名信封）→ QRC 3DES 解密 + zlib 解压 → LRC/QRC 解析。

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/hex"
	"io"
	"sort"
	"strconv"
	"strings"
)

const qrc3DESKey = "!@#)(*$%123ZXC!@!@#)(NHL"
const maxQRCDecompressed = 4 * 1024 * 1024

// qrcDecrypt 解密 QRC 密文（hex → 3DES-ECB → zlib），失败返回空串。
func qrcDecrypt(encrypted string) string {
	if encrypted == "" {
		return ""
	}
	cipherBytes, err := hex.DecodeString(encrypted)
	if err != nil || len(cipherBytes) == 0 || len(cipherBytes)%8 != 0 {
		return ""
	}
	schedule := tripledesKeySetup([]byte(qrc3DESKey), desDecrypt)
	out := make([]byte, 0, len(cipherBytes))
	for i := 0; i+8 <= len(cipherBytes); i += 8 {
		block := tripledesCrypt(cipherBytes[i:i+8], &schedule)
		out = append(out, block[:]...)
	}
	decoder, err := zlib.NewReader(bytes.NewReader(out))
	if err != nil {
		return ""
	}
	limited := io.LimitReader(decoder, maxQRCDecompressed+1)
	decoded, err := io.ReadAll(limited)
	closeErr := decoder.Close()
	if err != nil && closeErr != nil {
		return ""
	}
	if len(decoded) > maxQRCDecompressed {
		return ""
	}
	return string(decoded)
}

// LyricWord 逐字词条。
type LyricWord struct {
	StartMs int    `json:"startMs"`
	EndMs   int    `json:"endMs"`
	Text    string `json:"text"`
}

// LyricLine 歌词行。
type LyricLine struct {
	StartMs      int         `json:"startMs"`
	EndMs        int         `json:"endMs"`
	Text         string      `json:"text"`
	Translation  string      `json:"translation,omitempty"`
	Romanization string      `json:"romanization,omitempty"`
	Words        []LyricWord `json:"words,omitempty"`
}

// LyricDocument 歌词文档。
type LyricDocument struct {
	SongID   string `json:"songId"`
	SyncMode string `json:"syncMode"` // word | line | unsynchronized
	Lines    []LyricLine `json:"lines"`
}

// GetLyrics 获取歌词（qrc + 翻译 + 罗马音）。
func (c *Client) GetLyrics(songMid string) (*LyricDocument, error) {
	param := map[string]interface{}{
		"lrc_t":                 0,
		"qrc":                   1,
		"qrc_t":                 0,
		"roma":                  1,
		"roma_t":                0,
		"trans":                 1,
		"trans_t":               0,
		"needSingingAnnotations": false,
		"type":                  0,
		"crypt":                 1,
		"songMID":               songMid,
		"songMid":               songMid,
	}
	data, err := c.cgiAnonymousWeb("music.musichallSong.PlayLyricInfo", "GetPlayLyricInfo", param)
	if err != nil {
		return nil, err
	}

	doc := &LyricDocument{SongID: songMid, SyncMode: "unsynchronized"}

	mainText := decodeLyricField(stringAt(data, "lyric"))
	transText := decodeLyricField(stringAt(data, "trans"))
	romaText := decodeLyricField(stringAt(data, "roma"))
	// 字段可能是 QRC XML（LyricContent 属性）——先提取
	if xmlContent := qrcXMLContent(mainText); xmlContent != "" {
		mainText = xmlContent
	}

	// 判断逐字（QRC 行首是 [start,end]）或行级（LRC [mm:ss.xx]）
	if isQRCBody(mainText) {
		doc.Lines = parseQRC(mainText)
		doc.SyncMode = "word"
	} else if strings.Contains(mainText, "[") {
		doc.Lines = parseLRC(mainText)
		doc.SyncMode = "line"
	}

	// 翻译 / 罗马音按时间戳对齐
	if doc.SyncMode != "unsynchronized" {
		var transLines, romaLines []lrcLine
		if transText != "" {
			transText = qrcXMLContent(transText)
			transLines = parseLRCRaw(transText)
		}
		if romaText != "" {
			romaText = qrcXMLContent(romaText)
			romaLines = parseLRCRaw(romaText)
		}
		for i := range doc.Lines {
			line := &doc.Lines[i]
			for _, tl := range transLines {
				if abs(tl.timeMs-line.StartMs) <= 400 {
					line.Translation = tl.text
					break
				}
			}
			for _, rl := range romaLines {
				if abs(rl.timeMs-line.StartMs) <= 400 {
					line.Romanization = rl.text
					break
				}
			}
		}
	}

	if len(doc.Lines) == 0 {
		doc.SyncMode = "unsynchronized"
		if plain := strings.TrimSpace(mainText); plain != "" {
			// 无时间轴纯文本兜底
			for _, ln := range strings.Split(plain, "\n") {
				ln = strings.TrimSpace(ln)
				if ln != "" {
					doc.Lines = append(doc.Lines, LyricLine{Text: ln})
				}
			}
		}
	}
	return doc, nil
}

// decodeLyricField 处理加密 hex / base64 / 纯文本三种形态，返回可读歌词文本。
// 顺序：hex 密文（qrc）→ base64 结构化文本 → 原样。
func decodeLyricField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if decoded := qrcDecrypt(value); decoded != "" {
		return decoded
	}
	if t := structuredBase64(value); t != "" {
		return t
	}
	return value
}

// structuredBase64 仅当 base64 解码结果为结构化歌词（[ 开头或 QRC XML）时才接受。
func structuredBase64(value string) string {
	if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "<") {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	s := string(decoded)
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "<") {
		return s
	}
	return ""
}

// qrcXMLContent 从 QRC XML 文本中提取 LyricContent。
func qrcXMLContent(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "<") {
		return ""
	}
	const openTag = "LyricContent=\""
	idx := strings.Index(trimmed, openTag)
	if idx < 0 {
		return ""
	}
	start := idx + len(openTag)
	end := strings.Index(trimmed[start:], "\"")
	if end < 0 {
		return ""
	}
	content := trimmed[start : start+end]
	// XML 转义还原
	content = strings.ReplaceAll(content, "&apos;", "'")
	content = strings.ReplaceAll(content, "&quot;", "\"")
	content = strings.ReplaceAll(content, "&lt;", "<")
	content = strings.ReplaceAll(content, "&gt;", ">")
	content = strings.ReplaceAll(content, "&amp;", "&")
	return content
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---- LRC 解析 ----

type lrcLine struct {
	timeMs int
	text   string
}

// isQRCBody 判断文本是否为 QRC 逐字格式（行首 [start,end]）。
func isQRCBody(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		closeIdx := strings.Index(line, "]")
		if closeIdx < 0 {
			continue
		}
		inner := line[1:closeIdx]
		if strings.Contains(inner, ",") {
			parts := strings.SplitN(inner, ",", 2)
			startOK := isAllDigits(strings.TrimSpace(parts[0]))
			endOK := isAllDigits(strings.TrimSpace(parts[1]))
			if startOK && endOK {
				return true
			}
		}
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseLRCRaw 解析 LRC 文本为时间/文本对。
func parseLRCRaw(text string) []lrcLine {
	var lines []lrcLine
	offset := 0
	for _, raw := range strings.Split(text, "\n") {
		item := strings.TrimRight(raw, "\r")
		rIndex := strings.LastIndex(item, "]")
		if rIndex < 0 {
			continue
		}
		left := item[:rIndex+1]
		right := strings.TrimSpace(item[rIndex+1:])
		tmpTimes := strings.Split(strings.TrimLeft(left, "["), "]")
		found := false
		for _, tag := range tmpTimes {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			lower := strings.ToLower(tag)
			if strings.HasPrefix(lower, "offset:") {
				if o, err := strconv.Atoi(strings.TrimSpace(tag[7:])); err == nil {
					offset = o
				}
				continue
			}
			if t, ok := lrcTimeToMs(tag); ok {
				adjusted := t - offset
				if right != "" {
					lines = append(lines, lrcLine{timeMs: adjusted, text: right})
					found = true
				}
			}
		}
		_ = found
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].timeMs < lines[j].timeMs })
	return lines
}

// parseLRC 解析为 LyricLine。
func parseLRC(text string) []LyricLine {
	raw := parseLRCRaw(text)
	lines := make([]LyricLine, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, LyricLine{StartMs: l.timeMs, Text: l.text})
	}
	return lines
}

// lrcTimeToMs 解析 [mm:ss.xx] 为毫秒。
func lrcTimeToMs(tag string) (int, bool) {
	colon := strings.Index(tag, ":")
	if colon < 0 {
		return 0, false
	}
	minutes, err := strconv.Atoi(strings.TrimSpace(tag[:colon]))
	if err != nil {
		return 0, false
	}
	rest := tag[colon+1:]
	dot := strings.Index(rest, ".")
	if dot < 0 {
		dot = len(rest)
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(rest[:dot]))
	if err != nil {
		return 0, false
	}
	centis := 0
	if dot < len(rest) {
		cs := strings.TrimSpace(rest[dot+1:])
		if len(cs) > 2 {
			cs = cs[:2]
		}
		if cs != "" {
			centis, _ = strconv.Atoi(cs)
			if len(cs) == 1 {
				centis *= 10
			}
		}
	}
	return minutes*60000 + seconds*1000 + centis*10, true
}

// ---- QRC 解析 ----

// parseQRC 解析 [start,end]word(dur_cs)... 格式。
func parseQRC(text string) []LyricLine {
	var lines []LyricLine
	offset := 0
	for _, raw := range strings.Split(text, "\n") {
		item := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "[offset:") && strings.HasSuffix(item, "]") {
			if o, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(item, "[offset:"), "]"))); err == nil {
				offset = o
			}
			continue
		}
		if !strings.HasPrefix(item, "[") {
			continue
		}
		closeIdx := strings.Index(item, "]")
		if closeIdx < 0 {
			continue
		}
		rangePart := item[1:closeIdx]
		parts := strings.Split(rangePart, ",")
		start, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		end := start
		if len(parts) > 1 {
			end, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
		body := item[closeIdx+1:]

		var words []LyricWord
		pos := 0
		cursorMs := start
		var lineText strings.Builder
		for pos < len(body) {
			open := strings.Index(body[pos:], "(")
			if open < 0 {
				break
			}
			open += pos
			word := body[pos:open]
			closeP := strings.Index(body[open:], ")")
			if closeP < 0 {
				break
			}
			closeP += open
			inner := body[open+1 : closeP]
			durStr := strings.TrimSpace(inner)
			if comma := strings.Index(durStr, ","); comma >= 0 {
				durStr = strings.TrimSpace(durStr[:comma])
			}
			dur, _ := strconv.Atoi(durStr)
			durMs := dur * 10
			if word != "" {
				lineText.WriteString(word)
				words = append(words, LyricWord{
					StartMs: cursorMs - offset,
					EndMs:   cursorMs - offset + durMs,
					Text:    word,
				})
			}
			cursorMs += durMs
			pos = closeP + 1
		}
		if lineText.Len() > 0 {
			lines = append(lines, LyricLine{
				StartMs: start - offset,
				EndMs:   end - offset,
				Text:    lineText.String(),
				Words:   words,
			})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].StartMs < lines[j].StartMs })
	return lines
}
