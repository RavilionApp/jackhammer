package main

import (
	"bytes"
	"os/exec"
)

type FfmpegBackend struct{}

func (b *FfmpegBackend) setupLogOutput(cmd *exec.Cmd, buffer *bytes.Buffer) {
	cmd.Stderr = buffer
}

func (b *FfmpegBackend) isAvailable() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}
	return true
}

func (b *FfmpegBackend) buildCmd(inputUrl string) (*exec.Cmd, error) {
	args := []string{
		"-i", inputUrl,
		"-vf", "scale=-2:720",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-crf", "23",
		"-profile:v", "high",
		"-level", "4.0",
		"-x264-params", "keyint=48:min-keyint=48:scenecut=0:open_gop=0",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "128k",
		"-ac", "2",
		"-f", "hls",
		"-hls_time", "4",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", "segment_%03d.ts",
		"-hls_playlist_type", "vod",
		"master.m3u8",
	}
	return exec.Command("ffmpeg", args...), nil
}
