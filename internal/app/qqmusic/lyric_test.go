package qqmusic

import (
	"strings"
	"testing"
)

// QRC 线上真实片段（[行开始,行时长] + 词(词开始,词时长)），用于锁定解析语义。
const qrcSample = `[ti:新月的摇篮曲（其一）：伴月同眠 Lullaby of the New Moon (I): Somnias a Luna]
[ar:HOYO-MiX]
[offset:0]
[0,1803]新(0,69)月(69,69)的(139,69)摇(208,69)篮(278,69)曲(347,69)（(416,69)其(486,69)一(555,69)）：(625,69)伴(694,69)月(763,69)同(833,69)眠(902,69) (971,69)Lullaby (1041,69)of (1110,69)the (1180,69)New (1249,69)Moon ((1318,69)I): (1388,69)Somnias (1457,69)a (1527,69)Luna (1596,69)- (1666,69)HOYO-(1735,69)MiX(1804,69)
[1804,624]作(1804,69)词(1874,69) (1943,69)Lyricist：(2012,69)项(2082,69)柳(2151,69) (2221,69)Hsiang (2290,69)Liu(2359,69)`

func TestParseQRCTiming(t *testing.T) {
	if !isQRCBody(qrcSample) {
		t.Fatal("isQRCBody should detect QRC body")
	}
	lines := parseQRC(qrcSample)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %+v", len(lines), lines)
	}

	l0 := lines[0]
	if l0.StartMs != 0 || l0.EndMs != 1803 {
		t.Fatalf("line0 time = [%d,%d], want [0,1803]", l0.StartMs, l0.EndMs)
	}
	if !strings.Contains(l0.Text, "(I)") {
		t.Fatalf("line0 text lost literal parentheses: %q", l0.Text)
	}
	if !strings.Contains(l0.Text, "Lullaby of the New Moon") {
		t.Fatalf("line0 text broken: %q", l0.Text)
	}
	if len(l0.Words) == 0 {
		t.Fatal("line0 has no words")
	}
	// 词时间是整首歌的绝对时间，且时长按 (start,dur) 解释
	w0, w1 := l0.Words[0], l0.Words[1]
	if w0.Text != "新" || w0.StartMs != 0 || w0.EndMs != 69 {
		t.Fatalf("word0 = %+v, want 新 [0,69]", w0)
	}
	if w1.Text != "月" || w1.StartMs != 69 || w1.EndMs != 138 {
		t.Fatalf("word1 = %+v, want 月 [69,138]", w1)
	}

	l1 := lines[1]
	if l1.StartMs != 1804 || l1.EndMs != 1804+624 {
		t.Fatalf("line1 time = [%d,%d], want [1804,2428]", l1.StartMs, l1.EndMs)
	}
	if len(l1.Words) < 4 {
		t.Fatalf("line1 words = %d, want >=4", len(l1.Words))
	}
	if got := l1.Words[0]; got.StartMs != 1804 || got.EndMs != 1873 || got.Text != "作" {
		t.Fatalf("line1 word0 = %+v", got)
	}
	// 每个词的结束都不应早于开始（历史 bug：EndMs 变成负的）
	for _, ln := range lines {
		if ln.EndMs < ln.StartMs {
			t.Fatalf("line %q end<start: [%d,%d]", ln.Text, ln.StartMs, ln.EndMs)
		}
		for _, w := range ln.Words {
			if w.EndMs < w.StartMs {
				t.Fatalf("word %q end<start: [%d,%d]", w.Text, w.StartMs, w.EndMs)
			}
		}
	}
}

func TestParseQRCWithoutWordTags(t *testing.T) {
	lines := parseQRC("[1000,500]纯文本行")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	if lines[0].Text != "纯文本行" || lines[0].EndMs != 1500 || len(lines[0].Words) != 0 {
		t.Fatalf("unexpected line: %+v", lines[0])
	}
}
