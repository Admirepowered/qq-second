package app

// 听歌识曲绑定：前端录音/选文件得到 PCM → 生成 Shazam 指纹 → 识别 → 映射 QQ 曲库。

import (
	"context"
	"strings"
	"time"

	"qqsecond/internal/app/qqmusic"
	"qqsecond/internal/app/recognize"
)

// IdentifyResult 识曲结果：Shazam 元信息 + QQ 音乐曲库候选。
type IdentifyResult struct {
	Matched bool            `json:"matched"`
	Title   string          `json:"title,omitempty"`
	Artist  string          `json:"artist,omitempty"`
	Album   string          `json:"album,omitempty"`
	Cover   string          `json:"cover,omitempty"`
	Message string          `json:"message,omitempty"`
	Songs   []qqmusic.Track `json:"songs"`
}

// IdentifyAudio 识别一段音频。
// pcmBase64 是 s16le 单声道 PCM 的 base64；sampleRate 为原始采样率（非 16k 时后端重采样）。
func (a *App) IdentifyAudio(pcmBase64 string, sampleRate int) (*IdentifyResult, error) {
	pcm, err := recognize.DecodePCM16(pcmBase64)
	if err != nil {
		return nil, err
	}
	if sampleRate <= 0 {
		sampleRate = recognize.SampleRate
	}
	if sampleRate != recognize.SampleRate {
		pcm = recognize.ResampleMono(pcm, sampleRate, recognize.SampleRate)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	match, err := recognize.Identify(ctx, pcm, a.manualProxy())
	if err != nil {
		return nil, err
	}

	result := &IdentifyResult{
		Matched: match.Matched,
		Title:   match.Title,
		Artist:  match.Artist,
		Album:   match.Album,
		Cover:   match.Cover,
		Songs:   []qqmusic.Track{},
	}
	if !match.Matched {
		result.Message = "没有识别到这首歌，靠近声源或换一段再试"
		return result, nil
	}

	// 用「标题 + 歌手」在 QQ 音乐曲库找可播放的版本
	a.ensureQQ()
	query := strings.TrimSpace(match.Title + " " + match.Artist)
	if a.qq != nil && query != "" {
		if res, err := a.qq.Search("song", query, 1); err == nil && res != nil && len(res.Songs) > 0 {
			result.Songs = res.Songs
			if len(result.Songs) > 8 {
				result.Songs = result.Songs[:8]
			}
		}
	}
	if len(result.Songs) == 0 {
		result.Message = "已识别，但 QQ 音乐曲库没有找到匹配曲目"
	}
	return result, nil
}
