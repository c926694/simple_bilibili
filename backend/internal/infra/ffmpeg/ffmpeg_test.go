package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"simple_bilibili/backend/internal/config"
)

func TestTranscodeQualityAndLowerQualities(t *testing.T) {
	ffmpegBin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("本机未安装 ffmpeg")
	}
	ffprobeBin, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("本机未安装 ffprobe")
	}

	dir := t.TempDir()
	sample := filepath.Join(dir, "sample.mp4")
	args := []string{
		"-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=12",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", sample,
	}
	if out, err := exec.CommandContext(t.Context(), ffmpegBin, args...).CombinedOutput(); err != nil {
		t.Fatalf("生成测试视频失败: %v, %s", err, out)
	}

	client := New(config.FFmpeg{Binary: ffmpegBin, ProbeBinary: ffprobeBin})
	meta, err := client.Probe(t.Context(), sample)
	if err != nil {
		t.Fatalf("探测视频失败: %v", err)
	}
	if meta.Height != 720 || meta.VideoCodec != "h264" || meta.PixFmt != "yuv420p" || !meta.HasAudio || meta.AudioCodec != "aac" {
		t.Fatalf("探测编码与分辨率不符: %+v", meta)
	}

	qualities := PlannedQualities(meta)
	if len(qualities) != 2 || qualities[0].Name != "720p" || qualities[1].Name != "480p" {
		t.Fatalf("清晰度规划不符: %+v", qualities)
	}

	outDir := filepath.Join(dir, "hls")
	if err := client.TranscodeQuality(t.Context(), sample, outDir, meta, qualities[0], 4); err != nil {
		t.Fatalf("原画流拷贝切片失败: %v", err)
	}
	playlist, err := os.ReadFile(filepath.Join(outDir, "720p", "index.m3u8"))
	if err != nil || !strings.Contains(string(playlist), "#EXTM3U") || !strings.Contains(string(playlist), "#EXT-X-ENDLIST") {
		t.Fatalf("720p 播放列表不合法: %v", err)
	}

	if err := client.TranscodeLowerQualities(t.Context(), sample, outDir, meta, qualities[1:], 4); err != nil {
		t.Fatalf("低清晰度转码失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "480p", "index.m3u8")); err != nil {
		t.Fatalf("480p 播放列表缺失: %v", err)
	}
}

func TestResolveTranscodeResources(t *testing.T) {
	cases := []struct {
		cpus        int
		wantSlots   int
		wantThreads int
	}{
		{cpus: 1, wantSlots: 1, wantThreads: 2},
		{cpus: 2, wantSlots: 1, wantThreads: 2},
		{cpus: 4, wantSlots: 1, wantThreads: 2},
		{cpus: 8, wantSlots: 1, wantThreads: 6},
		{cpus: 10, wantSlots: 2, wantThreads: 4},
		{cpus: 16, wantSlots: 3, wantThreads: 4},
		{cpus: 32, wantSlots: 7, wantThreads: 4},
		{cpus: 64, wantSlots: 8, wantThreads: 7},
	}
	for _, tc := range cases {
		slots, threads := resolveTranscodeResources(tc.cpus)
		if slots != tc.wantSlots || threads != tc.wantThreads {
			t.Fatalf("cpus=%d: got (%d, %d), want (%d, %d)", tc.cpus, slots, threads, tc.wantSlots, tc.wantThreads)
		}
	}
	if DefaultMaxConcurrent < 1 || DefaultThreads < 2 {
		t.Fatalf("自动初始化转码资源非法: slots=%d threads=%d", DefaultMaxConcurrent, DefaultThreads)
	}
}
