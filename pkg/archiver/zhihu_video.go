// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package archiver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/go-shiori/dom"
)

var zhihuVideoPath = regexp.MustCompile(`/video/(\d{6,})`)

// zhihuVideoID returns the numeric id for a www.zhihu.com/video/<id> URL.
func zhihuVideoID(pageURL string) (string, bool) {
	u, err := url.Parse(pageURL)
	if err != nil || !strings.HasSuffix(strings.ToLower(u.Hostname()), "zhihu.com") {
		return "", false
	}
	m := zhihuVideoPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// replaceZhihuVideo resolves the public lens API playlist and downloads the
// best MP4 directly. yt-dlp has no Zhihu extractor (it answers "Unsupported
// URL"), so the normal embedded-video path cannot archive these clips.
func (arc *Archiver) replaceZhihuVideo(ctx context.Context, log *slog.Logger, node *html.Node, id string) bool {
	hdrs := http.Header{}
	hdrs.Set("X-App-Za", "OS=webplayer")
	hdrs.Set("Referer", "https://www.zhihu.com/")
	hdrs.Set("Accept", "application/json")

	body, _, err := arc.fetch(ctx, "https://lens.zhihu.com/api/v4/videos/"+id, hdrs)
	if err != nil {
		log.Warn("zhihu video info failed", slog.String("id", id), slog.Any("err", err))
		return false
	}
	raw, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return false
	}
	var info struct {
		Playlist map[string]struct {
			PlayURL string `json:"play_url"`
		} `json:"playlist"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return false
	}
	mp4 := ""
	for _, q := range []string{"FHD", "HD", "SD", "LD"} {
		if v, ok := info.Playlist[q]; ok && v.PlayURL != "" {
			mp4 = v.PlayURL
			break
		}
	}
	if mp4 == "" {
		return false
	}

	mediaHeaders := http.Header{}
	mediaHeaders.Set("Referer", "https://www.zhihu.com/")
	mediaHeaders.Set("Accept", "video/mp4,video/*;q=0.9,*/*;q=0.8")
	res, err := arc.processURL(ctx, mp4, processOptions{headers: mediaHeaders})
	if err != nil {
		log.Warn("zhihu video download failed", slog.String("id", id), slog.Any("err", err))
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
	poster := dom.GetAttribute(node, "src")
	if poster != "" {
		dom.SetAttribute(video, "poster", poster)
	}
	dom.ReplaceChild(node.Parent, video, node)
	log.Debug("zhihu video archived", slog.String("id", id), slog.String("resource", res.Name))
	return true
}

// processZhihuVideos replaces Zhihu poster placeholders with a locally saved
// MP4. It is independent from the yt-dlp/EnableEmbeds path because yt-dlp has
// no Zhihu extractor and many instances keep embedded videos disabled.
func (arc *Archiver) processZhihuVideos(ctx context.Context, doc *html.Node) {
	log := arc.log()
	for _, node := range dom.QuerySelectorAll(doc, "img[data-iframe-params],img[x-data-iframe-params]") {
		pageURL, ok := embeddedVideoPageURL(iframeParamsSrc(node))
		if !ok {
			continue
		}
		id, ok := zhihuVideoID(pageURL)
		if !ok {
			continue
		}
		arc.replaceZhihuVideo(ctx, log, node, id)
	}
}
