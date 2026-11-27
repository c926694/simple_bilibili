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
	"strconv"
	"strings"

	"simple_bilibili/backend/internal/config"
)

var ErrInvalidMedia = errors.New("视频文件损坏或没有可用的视频流")

type Meta struct {
	DurationSec int
	Width       int
	Height      int
	HasAudio    bool
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
	output, err := run(ctx, c.probeBinary, "-v", "error", "-show_entries", "format=duration:stream=codec_type,width,height,duration:stream_disposition=attached_pic", "-of", "json", inputPath)
	if err != nil {
		return Meta{}, err
	}
	var probe struct {
		Format  struct{ Duration string }
		Streams []struct {
			CodecType   string `json:"codec_type"`
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
		if stream.CodecType == "audio" {
			meta.HasAudio = true
		}
		if stream.CodecType == "video" && stream.Disposition.AttachedPic == 0 && meta.Height == 0 {
			meta.Width, meta.Height = stream.Width, stream.Height
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

func (c *Client) TranscodeHLS(ctx context.Context, inputPath, outDir string, meta Meta, threads int) ([]Quality, error) {
	qualities := make([]Quality, 0, 3)
	for _, height := range []int{1080, 720, 480} {
		if height != 480 && meta.Height < height {
			continue
		}
		target := min(meta.Height, height)
		target -= target % 2
		quality := Quality{Name: strconv.Itoa(height) + "p", Height: target}
		dir := filepath.Join(outDir, quality.Name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("创建 HLS 目录: %w", err)
		}
		n := strconv.Itoa(threads)
		args := []string{"-nostdin", "-v", "error", "-xerror", "-y", "-threads", n, "-i", inputPath, "-map", "0:V:0",
			"-vf", fmt.Sprintf("scale=-2:%d", target), "-filter_threads", n, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-threads", n,
			"-preset", "fast", "-force_key_frames", "expr:gte(t,n_forced*4)"}
		if meta.HasAudio {
			args = append(args, "-map", "0:a:0", "-c:a", "aac")
		} else {
			args = append(args, "-an")
		}
		args = append(args, "-f", "hls", "-hls_time", "4", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments",
			"-hls_segment_filename", filepath.Join(dir, "seg_%05d.ts"), filepath.Join(dir, "index.m3u8"))
		if _, err := run(ctx, c.binary, args...); err != nil {
			return nil, err
		}
		qualities = append(qualities, quality)
	}
	return qualities, nil
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
