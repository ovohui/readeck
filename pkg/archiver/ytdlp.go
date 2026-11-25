// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// YtdlpConfig configures the optional yt-dlp integration used to download
// videos that are embedded through a player (YouTube, Bilibili, ...).
type YtdlpConfig struct {
	// Path is the yt-dlp executable. Empty disables the feature.
	Path string
	// MaxHeight caps the downloaded video height in pixels (0 = best).
	MaxHeight int
	// Timeout is the maximum duration of a single download.
	Timeout time.Duration
	// CookiesFile is an optional Netscape cookies file passed to yt-dlp.
	CookiesFile string
	// ExtraArgs holds additional yt-dlp arguments (already split).
	ExtraArgs []string
}

// WithYtdlp enables embedded video downloading with yt-dlp.
func WithYtdlp(cfg YtdlpConfig) Option {
	return func(arc *Archiver) {
		arc.ytdlp = &cfg
	}
}

// rxYtEmbedded matches the /embed/<id> path of a YouTube iframe.
var rxYtEmbedded = regexp.MustCompile(`^/embed/([a-zA-Z0-9_-]{6,})`)

// rxBiliVideo matches a Bilibili video page path.
var rxBiliVideo = regexp.MustCompile(`/(?:video|bangumi/play)/((?:BV[0-9A-Za-z]{8,})|(?:av\d+))`)

// rxZhihuVideo matches a Zhihu clip path: /video/<id> or /zvideo/<id>.
var rxZhihuVideo = regexp.MustCompile(`/(?:video|zvideo)/(\d+)`)

// supportedEmbedHosts lists hosts whose players yt-dlp knows how to handle.
// A host matches when it is equal to an entry or is one of its subdomains.
var supportedEmbedHosts = []string{
	"youtube.com",
	"youtube-nocookie.com",
	"youtu.be",
	"bilibili.com",
	"b23.tv",
	"vimeo.com",
	"dailymotion.com",
	"dai.ly",
	"youku.com",
	"weibo.com",
	"ixigua.com",
	"douyin.com",
	"tiktok.com",
	"twitch.tv",
	"ted.com",
	"nicovideo.jp",
	"rutube.ru",
	"vk.com",
	// Xiaohongshu video notes render their player with JavaScript and the
	// stream URL is signed, so the archived page has nothing to play. yt-dlp
	// resolves the note on its own, and it does so without cookies.
	"xiaohongshu.com",
	"xhslink.com",
	// Zhihu answers embed their clip in a player the extractor strips. The
	// canonical /video/<id> page resolves without the answer shell.
	"zhihu.com",
}

// isSupportedEmbedHost reports whether host (or one of its parents) is handled
// by yt-dlp.
func isSupportedEmbedHost(host string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	for _, h := range supportedEmbedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// embeddedVideoPageURL converts a player URL into the canonical page URL that
// yt-dlp expects. It returns false when the URL is not a supported player.
func embeddedVideoPageURL(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}

	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if !isSupportedEmbedHost(host) {
		return "", false
	}

	switch {
	case host == "youtu.be":
		id := strings.Trim(u.Path, "/")
		if id == "" {
			return "", false
		}
		return "https://www.youtube.com/watch?v=" + id, true

	case strings.HasSuffix(host, "youtube.com"), strings.HasSuffix(host, "youtube-nocookie.com"):
		if m := rxYtEmbedded.FindStringSubmatch(u.Path); m != nil {
			return "https://www.youtube.com/watch?v=" + m[1], true
		}
		if id := u.Query().Get("v"); id != "" {
			return "https://www.youtube.com/watch?v=" + id, true
		}
		return "", false

	case strings.HasSuffix(host, "bilibili.com"):
		// Player URLs carry bvid/aid as query parameters.
		if bvid := u.Query().Get("bvid"); bvid != "" {
			return "https://www.bilibili.com/video/" + bvid, true
		}
		if aid := u.Query().Get("aid"); aid != "" {
			return "https://www.bilibili.com/video/av" + aid, true
		}
		if m := rxBiliVideo.FindStringSubmatch(u.Path); m != nil {
			return "https://www.bilibili.com/video/" + m[1], true
		}
		return u.String(), true

	case strings.HasSuffix(host, "zhihu.com"):
		// An answer page is handed over untouched: yt-dlp's Zhihu extractor
		// accepts /question/<id>/answer/<id> as well as /video/<id>.
		if m := rxZhihuVideo.FindStringSubmatch(u.Path); m != nil {
			return "https://www.zhihu.com/video/" + m[1], true
		}
		return u.String(), true

	case strings.HasSuffix(host, "vimeo.com"):
		// player.vimeo.com/video/<id>
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, p := range parts {
			if p == "video" && i+1 < len(parts) {
				return "https://vimeo.com/" + parts[i+1], true
			}
		}
		if len(parts) > 0 && isDigits(parts[len(parts)-1]) {
			return "https://vimeo.com/" + parts[len(parts)-1], true
		}
		return u.String(), true
	}

	// Everything else: hand the player URL to yt-dlp as-is. It resolves the
	// canonical page itself for most supported extractors.
	return u.String(), true
}

// isDigits reports whether s is a non empty sequence of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// maxEmbeddedVideosPerPage caps how many player videos are downloaded for a
// single page. Archiving is synchronous, so an article with dozens of videos
// must not hold the queue for hours.
const maxEmbeddedVideosPerPage = 3

// injectForcedVideo appends a player iframe for arc.forcedVideo when the
// document has none. Video pages (Bilibili, ...) build their player with
// JavaScript, so the extracted HTML is often empty and there is nothing for
// yt-dlp to hook on.
func (arc *Archiver) injectForcedVideo(doc *html.Node) {
	if arc.forcedVideo == "" || arc.ytdlp == nil || arc.ytdlp.Path == "" {
		return
	}

	for _, node := range dom.GetElementsByTagName(doc, "iframe") {
		if _, ok := embeddedVideoPageURL(dom.GetAttribute(node, "src")); ok {
			return
		}
	}

	// A thumbnail placeholder is a player too: the Xiaohongshu content script
	// (and the browser extension) leave one on the note cover instead of an
	// iframe, because players are stripped from the extracted content.
	// Forcing another one here would archive the clip twice and show two
	// players.
	for _, node := range dom.QuerySelectorAll(doc, "img[data-iframe-params],img[x-data-iframe-params]") {
		if _, ok := embeddedVideoPageURL(iframeParamsSrc(node)); ok {
			return
		}
	}

	body := dom.QuerySelector(doc, "body")
	if body == nil {
		return
	}

	figure := dom.CreateElement("figure")
	iframe := dom.CreateElement("iframe")
	dom.SetAttribute(iframe, "src", arc.forcedVideo)
	dom.SetAttribute(iframe, "data-forced-video", "1")
	dom.AppendChild(figure, iframe)
	dom.AppendChild(body, figure)
}

// processEmbeddedVideos downloads videos behind player iframes (and the
// thumbnail placeholders produced by the embedded video content script) and
// replaces them with a local <video> element.
// It must run before the concurrent resource processing loop, since it
// rewrites the document tree.
func (arc *Archiver) processEmbeddedVideos(ctx context.Context, doc *html.Node) {
	if arc.ytdlp == nil || arc.ytdlp.Path == "" || arc.flags&EnableEmbeds == 0 {
		return
	}

	log := arc.log()
	done := 0

	// iframes with a supported player.
	for _, node := range dom.GetElementsByTagName(doc, "iframe") {
		if done >= maxEmbeddedVideosPerPage {
			return
		}
		src := dom.GetAttribute(node, "src")
		if src == "" {
			src = dom.GetAttribute(node, "data-src")
		}
		pageURL, ok := embeddedVideoPageURL(src)
		if !ok {
			continue
		}
		if arc.replaceEmbedWithVideo(ctx, log, node, pageURL) {
			done++
		}
	}

	// Thumbnails created by the "video-embedded" content script. The real
	// player URL is stored as a query string in data-iframe-params.
	for _, node := range dom.QuerySelectorAll(doc, "img[data-iframe-params],img[x-data-iframe-params]") {
		if done >= maxEmbeddedVideosPerPage {
			return
		}
		pageURL, ok := embeddedVideoPageURL(iframeParamsSrc(node))
		if !ok {
			continue
		}
		if arc.replaceEmbedWithVideo(ctx, log, node, pageURL) {
			done++
		}
	}
}

// iframeParamsSrc returns the player URL carried by a thumbnail placeholder.
// The attribute name comes back as "data-iframe-params" once the extractor has
// restored the "x-data-" prefix it uses to shield it from bleach.
func iframeParamsSrc(node *html.Node) string {
	raw := dom.GetAttribute(node, "data-iframe-params")
	if raw == "" {
		raw = dom.GetAttribute(node, "x-data-iframe-params")
	}
	if raw == "" {
		return ""
	}
	params, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	return params.Get("src")
}

// replaceEmbedWithVideo runs yt-dlp for pageURL and, on success, replaces node
// with a <video> element pointing at the archived file.
// It reports whether the node was replaced.
func (arc *Archiver) replaceEmbedWithVideo(ctx context.Context, log *slog.Logger, node *html.Node, pageURL string) bool {
	if node.Parent == nil {
		return false
	}

	res, err := arc.downloadWithYtdlp(ctx, pageURL)
	if err != nil {
		log.Warn(
			"yt-dlp download failed",
			slog.String("url", pageURL),
			slog.Any("err", err),
		)
		// The iframe we injected ourselves has no archived document behind
		// it: leaving it in place gives the reader a "connection refused"
		// box pointing at a path that does not exist. Replace it with a link
		// to the page instead.
		if dom.GetAttribute(node, "data-forced-video") == "1" {
			replaceWithLink(node, pageURL)
		}
		return false
	}

	video := dom.CreateElement("video")
	dom.SetAttribute(video, "src", res.Value())
	dom.SetAttribute(video, "controls", "")
	dom.SetAttribute(video, "playsinline", "")
	dom.SetAttribute(video, "preload", "metadata")
	if w := dom.GetAttribute(node, "width"); isDigits(w) {
		dom.SetAttribute(video, "width", w)
	}
	if h := dom.GetAttribute(node, "height"); isDigits(h) {
		dom.SetAttribute(video, "height", h)
	}
	if poster := dom.GetAttribute(node, "poster"); poster != "" {
		dom.SetAttribute(video, "poster", poster)
	}

	parent := node.Parent
	dom.ReplaceChild(parent, video, node)

	log.Debug(
		"embedded video archived",
		slog.String("url", pageURL),
		slog.String("resource", res.Name),
	)

	return true
}

// replaceWithLink swaps a player iframe for a plain link to the same page, so
// a failed download leaves something clickable instead of a broken frame.
func replaceWithLink(node *html.Node, href string) {
	parent := node.Parent
	if parent == nil {
		return
	}

	figure := dom.CreateElement("figure")
	link := dom.CreateElement("a")
	dom.SetAttribute(link, "href", href)
	dom.SetAttribute(link, "rel", "noopener")
	dom.SetAttribute(link, "target", "_blank")
	dom.AppendChild(link, &html.Node{Type: html.TextNode, Data: "视频：打开原页面"})
	dom.AppendChild(figure, link)
	dom.ReplaceChild(parent, figure, node)
}

// videoResourceURL gives the downloaded file its own slot in the collector.
// Archiving the page and archiving its video both happen under the page URL,
// and a shared key made the HTML step believe the page was already saved: the
// zip ended up with the mp4 but no index.html, so the reader was blank.
func videoResourceURL(pageURL string) string {
	sep := "?"
	if strings.Contains(pageURL, "?") {
		sep = "&"
	}
	return pageURL + sep + "__readeck_video=1"
}

// downloadWithYtdlp runs yt-dlp in a temporary directory and stores the
// resulting file as an archive resource.
func (arc *Archiver) downloadWithYtdlp(ctx context.Context, pageURL string) (*Resource, error) {
	// When the page is archived twice (re-fetch, manual re-run) reuse the file
	// instead of pulling hundreds of megabytes again.
	key := videoResourceURL(pageURL)
	if res, ok := arc.collector.Get(key); ok && res.Saved() &&
		strings.HasPrefix(res.ContentType, "video/") {
		return res, nil
	}

	// A video download can be much longer than the archiver's HTTP timeout,
	// so it must not be bound to the parent deadline.
	dlCtx := context.WithoutCancel(ctx)
	timeout := arc.ytdlp.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	dlCtx, cancel := context.WithTimeout(dlCtx, timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "readeck-ytdlp-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	format := "bv*+ba/b"
	if h := arc.ytdlp.MaxHeight; h > 0 {
		format = fmt.Sprintf("bv*[height<=%d]+ba/b[height<=%d]/bv*+ba/b", h, h)
	}

	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--no-progress",
		"--no-part",
		"--no-mtime",
		"--retries", "3",
		"--socket-timeout", "30",
		"--merge-output-format", "mp4",
		// Sites publish the same clip as H.264 and H.265. Without a preference
		// yt-dlp takes the "better" one, which on a phone-oriented site is
		// almost always HEVC at a higher resolution. Chrome and Edge only
		// decode HEVC when the operating system ships the codec pack, so the
		// archived clip showed a black frame and never started - it read as a
		// broken video rather than as the wrong codec. Prefer H.264/AAC, the
		// pair every browser plays out of the box.
		"-S", "vcodec:h264,acodec:aac",
		"-f", format,
		"-o", filepath.Join(dir, "video.%(ext)s"),
	}
	if arc.ytdlp.CookiesFile != "" {
		if _, err := os.Stat(arc.ytdlp.CookiesFile); err == nil {
			args = append(args, "--cookies", arc.ytdlp.CookiesFile)
		}
	}
	if len(arc.ytdlp.ExtraArgs) > 0 {
		args = append(args, arc.ytdlp.ExtraArgs...)
	}
	args = append(args, "--", pageURL)

	cmd := exec.CommandContext(dlCtx, arc.ytdlp.Path, args...) //nolint:gosec // path from configuration
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 500 {
			msg = msg[len(msg)-500:]
		}
		if errors.Is(dlCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("yt-dlp timed out after %s", timeout)
		}
		return nil, fmt.Errorf("yt-dlp: %w: %s", err, msg)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var file string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "video.") {
			continue
		}
		if strings.HasSuffix(e.Name(), ".part") || strings.HasSuffix(e.Name(), ".ytdl") {
			continue
		}
		file = filepath.Join(dir, e.Name())
		break
	}
	if file == "" {
		return nil, errors.New("yt-dlp produced no file")
	}

	// The format sort above is a preference, not a guarantee: when a site only
	// publishes HEVC the download comes back unplayable in every browser that
	// lacks a system codec pack. Rewrite such a file to H.264/AAC once, here,
	// so the archive always holds something a reader can actually play.
	if playable, err := arc.ensurePlayable(ctx, file); err != nil {
		return nil, err
	} else {
		file = playable
	}

	f, err := os.Open(file) //nolint:gosec // file comes from our temp dir
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	res := &Resource{
		url:         key,
		Name:        arc.collector.Name(pageURL) + ".mp4",
		ContentType: "video/mp4",
		Size:        st.Size(),
	}

	return arc.saveResource(ctx, f, res)
}

// playableVideoCodecs lists the codecs a browser plays inside an MP4 without
// extra software. HEVC (hvc1/hev1) and anything exotic are missing on purpose:
// Chrome and Edge only decode HEVC when the OS has a codec pack installed, and
// the resulting "black frame that never starts" is indistinguishable from a
// broken download.
var playableVideoCodecs = map[string]bool{
	"h264": true,
	"av1":  true,
	"vp9":  true,
}

// playableAudioCodecs lists the audio codecs supported in an MP4 across
// browsers. AC-3/E-AC-3 (common in DASH manifests) are not.
var playableAudioCodecs = map[string]bool{
	"aac":  true,
	"mp3":  true,
	"opus": true,
}

// probeStreams is the subset of ffprobe's JSON output this package needs.
type probeStreams struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
	} `json:"streams"`
}

// parseProbeStreams returns the codec name of the first video and the first
// audio stream. Either is empty when the file has no such stream.
func parseProbeStreams(data []byte) (video, audio string) {
	var out probeStreams
	if err := json.Unmarshal(data, &out); err != nil {
		return "", ""
	}
	for _, s := range out.Streams {
		switch s.CodecType {
		case "video":
			if video == "" {
				video = s.CodecName
			}
		case "audio":
			if audio == "" {
				audio = s.CodecName
			}
		}
	}
	return video, audio
}

// isPlayable reports whether a probed file can be played by a browser as is.
// A file with no video stream is never playable; a missing audio track is
// normal (many short clips are silent) and does not require a rewrite.
func isPlayable(video, audio string) bool {
	if video == "" || !playableVideoCodecs[video] {
		return false
	}
	return audio == "" || playableAudioCodecs[audio]
}

// ensurePlayable probes the downloaded file and, when its codecs cannot be
// played in a browser, transcodes it to H.264/AAC. Any failure leaves the
// original file in place: a wrong codec is better than a missing video.
func (arc *Archiver) ensurePlayable(ctx context.Context, file string) (string, error) {
	log := arc.log()

	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		log.Debug("ffprobe not available, keeping the downloaded file", slog.Any("err", err))
		return file, nil
	}

	probe, err := exec.CommandContext(ctx, ffprobe, //nolint:gosec // path from PATH
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name",
		"-of", "json",
		file,
	).Output()
	if err != nil {
		log.Warn("cannot probe the downloaded video", slog.Any("err", err))
		return file, nil
	}

	video, audio := parseProbeStreams(probe)
	log.Debug("downloaded video", slog.String("vcodec", video), slog.String("acodec", audio))

	if video == "" {
		return "", errors.New("the downloaded file has no video stream")
	}
	if isPlayable(video, audio) {
		return file, nil
	}

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Warn("ffmpeg not available, keeping an unplayable video",
			slog.String("vcodec", video), slog.Any("err", err))
		return file, nil
	}

	out := filepath.Join(filepath.Dir(file), "playable.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-y", "-i", file,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
	}
	if audio != "" {
		args = append(args, "-c:a", "aac", "-b:a", "128k")
	} else {
		args = append(args, "-an")
	}
	args = append(args, out)

	if err := exec.CommandContext(ctx, ffmpeg, args...).Run(); err != nil { //nolint:gosec // path from PATH
		log.Warn("cannot transcode the downloaded video",
			slog.String("vcodec", video), slog.Any("err", err))
		return file, nil
	}

	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		log.Warn("transcoded video is empty")
		return file, nil
	}

	log.Debug("transcoded the downloaded video to h264",
		slog.String("from", video), slog.String("file", out))
	return out, nil
}
