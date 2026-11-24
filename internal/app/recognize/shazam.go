package recognize

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Match 是识别结果。
type Match struct {
	Matched  bool   `json:"matched"`
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Cover    string `json:"cover,omitempty"`
	ShazamID string `json:"shazamId,omitempty"`
	OffsetMs int    `json:"offsetMs,omitempty"`
}

// endpoint 是 Shazam 的识别接口（公开 App 接口，无需密钥）。
const endpoint = "https://amp.shazam.com/discovery/v5/zh-CN/CN/android/-/tag/%s/%s" +
	"?sync=true&webv3=true&sampling=true&connected=&shazam-ssl-override=false&hd=true"

var httpClient = &http.Client{Timeout: 25 * time.Second}

// Identify 对 16kHz 单声道 PCM 做识别。proxyURL 为空时直连（网络被墙时可在设置里填手动代理）。
func Identify(ctx context.Context, pcm []int16, proxyURL string) (*Match, error) {
	sig, samples, err := GenerateSignature(pcm)
	if err != nil {
		return nil, err
	}
	ts := time.Now().UnixMilli()
	payload := map[string]any{
		"geolocation": map[string]any{"altitude": 0, "latitude": 0, "longitude": 0},
		"signature": map[string]any{
			"samplems":  samples * 1000 / SampleRate,
			"timestamp": ts,
			"uri":       "data:audio/vnd.shazam.sig;base64," + base64.StdEncoding.EncodeToString(sig),
		},
		"timestamp": ts,
		"timezone":  "Asia/Shanghai",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	client, err := clientFor(proxyURL)
	if err != nil {
		return nil, err
	}

	// 识别接口偶发连不上：重试 3 次（每次新 UUID）
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 800 * time.Millisecond):
			}
		}
		m, err := doIdentify(ctx, client, body)
		if err == nil {
			return m, nil
		}
		lastErr = err
		if !isTransient(err) {
			break
		}
	}
	return nil, lastErr
}

func doIdentify(ctx context.Context, client *http.Client, body []byte) (*Match, error) {
	url := fmt.Sprintf(endpoint, newUUID(), newUUID())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Shazam/14.1.0 Android/13")
	req.Header.Set("X-Shazam-Platform", "ANDROID")
	req.Header.Set("X-Shazam-AppVersion", "14.1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("识曲请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("识曲服务返回 %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return parseResponse(raw)
}

// clientFor 按需构建带代理的 HTTP 客户端。
func clientFor(proxyURL string) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return httpClient, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址无效: %w", err)
	}
	return &http.Client{
		Timeout:   25 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(u)},
	}, nil
}

func isTransient(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "识曲请求失败") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection")
}

type shazamResponse struct {
	Matches []struct {
		ID     string  `json:"id"`
		Offset float64 `json:"offset"`
	} `json:"matches"`
	Track *struct {
		Title    string `json:"title"`
		Subtitle string `json:"subtitle"`
		Key      string `json:"key"`
		Images   struct {
			CoverArt   string `json:"coverart"`
			CoverArtHQ string `json:"coverarthq"`
			Background string `json:"background"`
		} `json:"images"`
		Sections []struct {
			Type     string `json:"type"`
			Metadata []struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"metadata"`
		} `json:"sections"`
	} `json:"track"`
}

func parseResponse(raw []byte) (*Match, error) {
	var r shazamResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("识曲响应解析失败: %w", err)
	}
	if r.Track == nil || strings.TrimSpace(r.Track.Title) == "" {
		return &Match{Matched: false}, nil
	}
	m := &Match{
		Matched:  true,
		Title:    strings.TrimSpace(r.Track.Title),
		Artist:   strings.TrimSpace(r.Track.Subtitle),
		ShazamID: r.Track.Key,
	}
	if m.Cover == "" {
		m.Cover = r.Track.Images.CoverArtHQ
	}
	if m.Cover == "" {
		m.Cover = r.Track.Images.CoverArt
	}
	for _, sec := range r.Track.Sections {
		for _, md := range sec.Metadata {
			switch strings.ToLower(md.Title) {
			case "album":
				if m.Album == "" {
					m.Album = strings.TrimSpace(md.Text)
				}
			}
		}
	}
	if len(r.Matches) > 0 {
		m.OffsetMs = int(math.Round(r.Matches[0].Offset * 1000))
	}
	return m, nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// DecodePCM16 把 base64 的 s16le 字节流解成采样切片。
func DecodePCM16(b64 string) ([]int16, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("音频数据解码失败: %w", err)
	}
	if len(raw) < 2 {
		return nil, ErrTooShort
	}
	out := make([]int16, len(raw)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return out, nil
}

// ResampleMono 把单声道 PCM 从 from 采样率线性重采样到 to。
func ResampleMono(pcm []int16, from, to int) []int16 {
	if from == to || from <= 0 || len(pcm) == 0 {
		return pcm
	}
	n := len(pcm)
	outN := int(float64(n) * float64(to) / float64(from))
	if outN <= 0 {
		return nil
	}
	out := make([]int16, outN)
	for i := 0; i < outN; i++ {
		src := float64(i) * float64(from) / float64(to)
		i0 := int(src)
		if i0 >= n-1 {
			i0 = n - 2
		}
		if i0 < 0 {
			i0 = 0
		}
		frac := src - float64(i0)
		a := float64(pcm[i0])
		b := float64(pcm[i0+1])
		out[i] = int16(a + (b-a)*frac)
	}
	return out
}
