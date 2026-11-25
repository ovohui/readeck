// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: MIT

package archiver

import "testing"

const probeFixture = `{
  "streams": [
    {"codec_type": "video", "codec_name": "hevc"},
    {"codec_type": "audio", "codec_name": "aac"}
  ]
}`

const probeFixtureNoAudio = `{
  "streams": [
    {"codec_type": "video", "codec_name": "h264"}
  ]
}`

const probeFixtureTwoVideos = `{
  "streams": [
    {"codec_type": "video", "codec_name": "mjpeg"},
    {"codec_type": "video", "codec_name": "h264"},
    {"codec_type": "audio", "codec_name": "ac3"}
  ]
}`

func TestParseProbeStreams(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		video string
		audio string
	}{
		{"hevc with aac", probeFixture, "hevc", "aac"},
		{"video only", probeFixtureNoAudio, "h264", ""},
		{"first video wins", probeFixtureTwoVideos, "mjpeg", "ac3"},
		{"not json", "ffprobe: not found", "", ""},
		{"empty", "", "", ""},
	}

	for _, x := range tests {
		t.Run(x.name, func(t *testing.T) {
			video, audio := parseProbeStreams([]byte(x.data))
			if video != x.video || audio != x.audio {
				t.Fatalf("got (%q, %q), want (%q, %q)", video, audio, x.video, x.audio)
			}
		})
	}
}

func TestIsPlayable(t *testing.T) {
	tests := []struct {
		video, audio string
		playable     bool
	}{
		{"h264", "aac", true},
		{"h264", "", true}, // silent clips are fine
		{"av1", "opus", true},
		{"vp9", "aac", true},
		{"hevc", "aac", false}, // what a phone-oriented site hands out by default
		{"h264", "ac3", false},
		{"", "aac", false},
		{"mpeg4", "", false},
	}

	for _, x := range tests {
		if got := isPlayable(x.video, x.audio); got != x.playable {
			t.Errorf("isPlayable(%q, %q) = %v, want %v", x.video, x.audio, got, x.playable)
		}
	}
}
