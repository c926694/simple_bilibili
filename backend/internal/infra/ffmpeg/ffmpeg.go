package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"simple_bilibili/backend/internal/config"
)

var (
	DefaultMaxConcurrent, DefaultThreads = resolveTranscodeResources(runtime.GOMAXPROCS(0))
)

func resolveTranscodeResources(cpus int) (int, int) {
	budget := max(2, cpus-2)
	maxConcurrent := min(8, max(1, budget/4))
	threads := min(8, max(2, budget/maxConcurrent))
	return maxConcurrent, threads
}

var ErrInvalidMedia = errors.New("视频文件损坏或没有可用的视频流")

type Meta struct {
	DurationSec int
	Width       int
	Height      int
	HasAudio    bool
	VideoCodec  string
	PixFmt      string
	AudioCodec  string
}

type Quality struct {
	Name   string
	Height int
}

type Client struct {
	binary      string
	probeBinary string
}

func New(cfg config.FFmpeg) *Client {
	return &Client{binary: cfg.Binary, probeBinary: cfg.ProbeBinary}
}

func (c *Client) Probe(ctx context.Context, inputPath string) (Meta, error) {
	output, err := run(ctx, c.probeBinary, "-v", "error", "-show_entries", "format=duration:stream=codec_type,codec_name,pix_fmt,width,height,duration:stream_disposition=attached_pic", "-of", "json", inputPath)
	if err != nil {
		return Meta{}, err
	}
	var probe struct {
		Format  struct{ Duration string }
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			PixFmt      string `json:"pix_fmt"`
			Width       int
			Height      int
			Duration    string
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			}
		}
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		return Meta{}, fmt.Errorf("解析 ffprobe 结果: %w", err)
	}
	meta := Meta{}
	duration := probe.Format.Duration
	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" && !meta.HasAudio {
			meta.HasAudio = true
			meta.AudioCodec = strings.ToLower(stream.CodecName)
		}
		if stream.CodecType == "video" && stream.Disposition.AttachedPic == 0 && meta.Height == 0 {
			meta.Width, meta.Height = stream.Width, stream.Height
			meta.VideoCodec = strings.ToLower(stream.CodecName)
			meta.PixFmt = strings.ToLower(stream.PixFmt)
			if duration == "" || duration == "N/A" {
				duration = stream.Duration
			}
		}
	}
	if meta.Width < 2 || meta.Height < 2 {
		return Meta{}, ErrInvalidMedia
	}
	seconds, err := strconv.ParseFloat(duration, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || seconds > math.MaxInt32 {
		return Meta{}, ErrInvalidMedia
	}
	meta.DurationSec = max(1, int(seconds))
	return meta, nil
}

func (c *Client) ExtractCover(ctx context.Context, inputPath, outputPath string, durationSec int) error {
	at := "0"
	if durationSec > 1 {
		at = "1"
	}
	_, err := run(ctx, c.binary, "-nostdin", "-v", "error", "-xerror", "-y", "-ss", at, "-i", inputPath, "-map", "0:V:0", "-frames:v", "1", "-update", "1", "-q:v", "3", outputPath)
	return err
}

func PlannedQualities(meta Meta) []Quality {
	qualities := make([]Quality, 0, 3)
	for _, height := range []int{1080, 720, 480} {
		if height != 480 && meta.Height < height {
			continue
		}
		target := min(meta.Height, height)
		target -= target % 2
		qualities = append(qualities, Quality{Name: strconv.Itoa(height) + "p", Height: target})
	}
	return qualities
}

func (c *Client) TranscodeQuality(ctx context.Context, inputPath, outDir string, meta Meta, quality Quality, threads int) error {
	dir := filepath.Join(outDir, quality.Name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("创建 HLS 目录: %w", err)
	}
	n := strconv.Itoa(threads)
	copyVideo := quality.Height == meta.Height && meta.VideoCodec == "h264" && meta.PixFmt == "yuv420p"
	args := []string{"-nostdin", "-v", "error", "-xerror", "-y"}
	if !copyVideo {
		args = append(args, "-threads", n)
	}
	args = append(args, "-i", inputPath, "-map", "0:V:0")
	if copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-vf", fmt.Sprintf("scale=-2:%d", quality.Height), "-filter_threads", n,
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-threads", n, "-preset", "veryfast",
			"-force_key_frames", "expr:gte(t,n_forced*4)")
	}
	args = appendAudioArgs(args, meta)
	args = append(args, "-f", "hls", "-hls_time", "4", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(dir, "seg_%05d.ts"), filepath.Join(dir, "index.m3u8"))
	_, err := run(ctx, c.binary, args...)
	return err
}

func (c *Client) TranscodeLowerQualities(ctx context.Context, inputPath, outDir string, meta Meta, qualities []Quality, threads int) error {
	if len(qualities) == 0 {
		return nil
	}
	if len(qualities) == 1 {
		return c.TranscodeQuality(ctx, inputPath, outDir, meta, qualities[0], threads)
	}
	for _, quality := range qualities {
		if err := os.MkdirAll(filepath.Join(outDir, quality.Name), 0o750); err != nil {
			return fmt.Errorf("创建 HLS 目录: %w", err)
		}
	}
	n := strconv.Itoa(threads)
	filter := buildSplitFilter(qualities)
	args := []string{"-nostdin", "-v", "error", "-xerror", "-y", "-threads", n, "-i", inputPath, "-filter_complex", filter, "-filter_threads", n}
	for i, quality := range qualities {
		dir := filepath.Join(outDir, quality.Name)
		args = append(args, "-map", fmt.Sprintf("[out%d]", i), "-c:v", "libx264", "-pix_fmt", "yuv420p",
			"-threads", n, "-preset", "veryfast", "-force_key_frames", "expr:gte(t,n_forced*4)")
		args = appendAudioArgs(args, meta)
		args = append(args, "-f", "hls", "-hls_time", "4", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments",
			"-hls_segment_filename", filepath.Join(dir, "seg_%05d.ts"), filepath.Join(dir, "index.m3u8"))
	}
	_, err := run(ctx, c.binary, args...)
	return err
}

func (c *Client) TranscodeHLS(ctx context.Context, inputPath, outDir string, meta Meta, threads int) ([]Quality, error) {
	qualities := PlannedQualities(meta)
	if len(qualities) == 0 {
		return nil, nil
	}
	if err := c.TranscodeQuality(ctx, inputPath, outDir, meta, qualities[0], threads); err != nil {
		return nil, err
	}
	if err := c.TranscodeLowerQualities(ctx, inputPath, outDir, meta, qualities[1:], threads); err != nil {
		return nil, err
	}
	return qualities, nil
}

func appendAudioArgs(args []string, meta Meta) []string {
	if !meta.HasAudio {
		return append(args, "-an")
	}
	if meta.AudioCodec == "aac" {
		return append(args, "-map", "0:a:0", "-c:a", "copy")
	}
	return append(args, "-map", "0:a:0", "-c:a", "aac")
}

func buildSplitFilter(qualities []Quality) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[0:V:0]split=%d", len(qualities))
	for i := range qualities {
		fmt.Fprintf(&b, "[v%d]", i)
	}
	for i, quality := range qualities {
		fmt.Fprintf(&b, ";[v%d]scale=-2:%d[out%d]", i, quality.Height, i)
	}
	return b.String()
}

func run(ctx context.Context, binary string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		stderr := strings.TrimSpace(string(exit.Stderr))
		for _, problem := range []string{"invalid data found", "moov atom not found", "error while decoding", "invalid nal unit", "matches no streams"} {
			if strings.Contains(strings.ToLower(stderr), problem) {
				return nil, fmt.Errorf("%w: %s", ErrInvalidMedia, stderr)
			}
		}
		return nil, fmt.Errorf("执行 %s: %w，%s", filepath.Base(binary), err, stderr)
	}
	return nil, fmt.Errorf("执行 %s: %w", filepath.Base(binary), err)
}
