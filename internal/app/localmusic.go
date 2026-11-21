package app

// 本地音乐：文件夹递归扫描、标签读取、时长解析、封面缓存。

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dhowden/tag"
)

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".m4a": true, ".aac": true,
	".wav": true, ".ogg": true, ".opus": true,
}

// LocalLibrary 本地曲库（内存态 + dataDir/library.json 缓存）。
type LocalLibrary struct {
	mu       sync.Mutex
	dataDir  string
	coverDir string
	tracks   []Track
	byID     map[string]Track
}

// NewLocalLibrary 创建曲库。
func NewLocalLibrary(dataDir, coverDir string) *LocalLibrary {
	return &LocalLibrary{dataDir: dataDir, coverDir: coverDir, byID: map[string]Track{}}
}

// LoadCache 载入持久化的曲库缓存。
func (l *LocalLibrary) LoadCache() {
	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(l.dataDir, "library.json"))
	if err != nil {
		return
	}
	var tracks []Track
	if err := json.Unmarshal(data, &tracks); err != nil {
		return
	}
	l.tracks = tracks
	l.byID = map[string]Track{}
	for _, t := range tracks {
		l.byID[t.Id] = t
	}
}

func (l *LocalLibrary) saveCacheLocked() {
	data, err := json.MarshalIndent(l.tracks, "", " ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(l.dataDir, "library.json"), data, 0o644)
}

// Rescan 重扫全部文件夹（替换曲库）。
func (l *LocalLibrary) Rescan(folders []string) {
	var tracks []Track
	var walkMu sync.Mutex
	for _, root := range folders {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// 跳过明显无关目录
				name := d.Name()
				if strings.HasPrefix(name, ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !audioExts[ext] {
				return nil
			}
			t := readLocalTrack(path, ext)
			walkMu.Lock()
			tracks = append(tracks, t)
			walkMu.Unlock()
			return nil
		})
	}
	sort.SliceStable(tracks, func(i, j int) bool {
		return strings.ToLower(tracks[i].Title) < strings.ToLower(tracks[j].Title)
	})
	l.mu.Lock()
	l.tracks = tracks
	l.byID = map[string]Track{}
	for _, t := range tracks {
		l.byID[t.Id] = t
	}
	l.saveCacheLocked()
	l.mu.Unlock()
}

// Tracks 返回全部曲目快照。
func (l *LocalLibrary) Tracks() []Track {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Track, len(l.tracks))
	copy(out, l.tracks)
	return out
}

// FindTrack 按 id 查找。
func (l *LocalLibrary) FindTrack(id string) (Track, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.byID[id]
	return t, ok
}

// localTrackID 由路径生成稳定 id。
func localTrackID(path string) string {
	sum := sha1.Sum([]byte(path))
	return "local-" + hex.EncodeToString(sum[:8])
}

// readLocalTrack 读取单个文件的标签与时长。
func readLocalTrack(path, ext string) Track {
	id := localTrackID(path)
	track := Track{
		Kind: "local", Id: id, Path: path, Ext: strings.TrimPrefix(ext, "."),
		Title: strings.TrimSuffix(filepath.Base(path), ext),
	}
	if st, err := os.Stat(path); err == nil {
		_ = st
	}

	if f, err := os.Open(path); err == nil {
		meta, err := tag.ReadFrom(f)
		f.Close()
		if err == nil {
			if meta.Title() != "" {
				track.Title = meta.Title()
			}
			if meta.Artist() != "" {
				for _, a := range strings.Split(meta.Artist(), "/") {
					track.Artists = append(track.Artists, NameRef{Name: strings.TrimSpace(a)})
				}
			}
			if meta.Album() != "" {
				track.Album = meta.Album()
			}
			if tn, _ := meta.Track(); tn > 0 {
				track.TrackNum = tn
			}
		}
	}
	track.DurationMs = audioDuration(path, ext)
	return track
}

// audioDuration 解析音频时长（FLAC STREAMINFO / MP3 估算 / MP4 mvhd）。
func audioDuration(path, ext string) int {
	switch ext {
	case ".flac":
		return flacDuration(path)
	case ".mp3":
		return mp3Duration(path)
	case ".m4a", ".mp4", ".aac":
		return mp4Duration(path)
	case ".wav":
		return wavDuration(path)
	}
	return 0
}

// flacDuration 从 "fLaC" + STREAMINFO 块解析。
func flacDuration(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil || string(header) != "fLaC" {
		return 0
	}
	block := make([]byte, 38)
	if _, err := f.Read(block); err != nil {
		return 0
	}
	// STREAMINFO: 最后 8 字节 = 36bit totalSamples + 20bit sampleRate
	sampleRate := int(block[10])<<12 | int(block[11])<<4 | int(block[12])>>4
	totalSamples := (int(block[13]&0x0F) << 32) | int(block[14])<<24 | int(block[15])<<16 | int(block[16])<<8 | int(block[17])
	if sampleRate <= 0 || totalSamples <= 0 {
		return 0
	}
	return int(int64(totalSamples) / int64(sampleRate) * 1000)
}

// mp3Duration 按平均比特率估算（Xing/Info 帧或文件大小）。
func mp3Duration(path string) int {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	// 粗略估算：320kbps 或更低，取 192kbps 经验值太不准；改用 ID3 后首个帧头。
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	// 跳过 ID3v2
	head := make([]byte, 10)
	if _, err := f.Read(head); err != nil {
		return 0
	}
	skip := int64(0)
	if string(head[:3]) == "ID3" {
		sz := int64(head[6])<<21 | int64(head[7])<<14 | int64(head[8])<<7 | int64(head[9])
		skip = 10 + sz
	}
	if _, err := f.Seek(skip, 0); err != nil {
		return 0
	}
	// 搜帧头
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	for i := 0; i+4 <= n; i++ {
		if buf[i] == 0xFF && buf[i+1]&0xE0 == 0xE0 {
			ver := (buf[i+1] >> 3) & 0x03
			layer := (buf[i+1] >> 1) & 0x03
			if ver == 3 && layer == 1 { // MPEG1 Layer3
				bitrates := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
				brIdx := int(buf[i+2] >> 4)
				if brIdx <= 0 || brIdx >= len(bitrates) {
					continue
				}
				bitrateKbps := bitrates[brIdx]
				if bitrateKbps == 0 {
					continue
				}
				totalBytes := st.Size() - skip
				return int(totalBytes * 8 / int64(bitrateKbps) / 1000)
			}
		}
	}
	return 0
}

// mp4Duration 从 moov/mvhd 解析。
func mp4Duration(path string) int {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 128 {
		return 0
	}
	idx := strings.Index(string(data[:min(len(data), 512)]), "mvhd")
	if idx < 0 {
		return 0
	}
	base := idx + 4 // box header 前 4 字节是 size，我们直接从 'mvhd' 后取
	if base+20 > len(data) {
		return 0
	}
	version := data[base+4]
	if version == 1 {
		timescale := binary.BigEndian.Uint32(data[base+24 : base+28])
		duration := binary.BigEndian.Uint64(data[base+28 : base+36])
		if timescale == 0 {
			return 0
		}
		return int(duration / uint64(timescale) * 1000)
	}
	timescale := binary.BigEndian.Uint32(data[base+16 : base+20])
	duration := binary.BigEndian.Uint32(data[base+20 : base+24])
	if timescale == 0 {
		return 0
	}
	return int(uint64(duration) / uint64(timescale) * 1000)
}

// wavDuration 从 fmt 块解析。
func wavDuration(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	st, _ := f.Stat()
	head := make([]byte, 44)
	if _, err := f.Read(head); err != nil {
		return 0
	}
	if string(head[0:4]) != "RIFF" || string(head[8:12]) != "WAVE" {
		return 0
	}
	byteRate := binary.LittleEndian.Uint32(head[28:32])
	if byteRate == 0 {
		return 0
	}
	return int(int64(st.Size()-44) * 1000 / int64(byteRate))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CoverDataURL 返回本地曲目内嵌封面的 data URL（缓存到 covers/）。
func (l *LocalLibrary) CoverDataURL(trackID string) (string, error) {
	l.mu.Lock()
	track, ok := l.byID[trackID]
	l.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("track not found")
	}
	// 先查缓存
	cachePath := filepath.Join(l.coverDir, trackID+".jpg")
	if data, err := os.ReadFile(cachePath); err == nil {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
	}
	f, err := os.Open(track.Path)
	if err != nil {
		return "", err
	}
	meta, err := tag.ReadFrom(f)
	f.Close()
	if err != nil {
		return "", err
	}
	pic := meta.Picture()
	if pic == nil {
		return "", fmt.Errorf("no embedded cover")
	}
	_ = os.WriteFile(cachePath, pic.Data, 0o644)
	return "data:" + pic.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(pic.Data), nil
}
