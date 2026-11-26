// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package archiver

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// A video archived by yt-dlp must not occupy the page URL slot: doing so made
// saveHTML believe the document was already stored and the zip ended up with
// the mp4 but no index.html.
func TestArchiveDocumentWritesHTMLWithYtdlpVideo(t *testing.T) {
	pageURL := "https://www.xiaohongshu.com/explore/6afabf31000000001803b7b0?xsec_token=ABC&xsec_source=pc_feed"
	uri, err := url.Parse(pageURL)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	col := NewZipCollector(zw, http.DefaultClient)
	arc := New(WithCollector(col))

	// Stand in for the file yt-dlp already downloaded.
	video := &Resource{
		url:         videoResourceURL(pageURL),
		Name:        "./_resources/video.mp4",
		ContentType: "video/mp4",
		Size:        1024,
		saved:       true,
	}
	col.Set(video.url, video)

	doc := `<html><body><p>note</p></body></html>`
	if err := arc.ArchiveReader(context.Background(), strings.NewReader(doc), uri, "index.html"); err != nil {
		t.Fatalf("ArchiveReader: %v", err)
	}

	saved, ok := col.Get(requestURI(pageURL))
	if !ok {
		t.Fatal("the page URL holds no resource")
	}
	if saved.Name != "index.html" {
		t.Fatalf("page URL holds %q, want index.html", saved.Name)
	}
	if saved.ContentType != "text/html" {
		t.Fatalf("content type = %q, want text/html", saved.ContentType)
	}

	// The video keeps its own slot.
	got, ok := col.Get(videoResourceURL(pageURL))
	if !ok || got.Name != "./_resources/video.mp4" {
		t.Fatalf("video resource was overwritten: %+v", got)
	}
}

// A forum attachment carries the picture inline in src and a hotlink-protected
// copy in file=. The inline copy must win; preferring the remote one loses the
// GIF when the CDN answers with a challenge.
func TestConvertLazyImageAttrsKeepsInlineImage(t *testing.T) {
	payload := strings.Repeat("A", 200)
	doc, err := html.Parse(strings.NewReader(
		`<html><body><img src="data:image/gif;base64,` + payload +
			`" file="https://cdn.example.com/a.gif"></body></html>`,
	))
	if err != nil {
		t.Fatal(err)
	}

	arc := New()
	arc.convertLazyImageAttrs(context.Background(), doc)

	img := dom.QuerySelector(doc, "img")
	if img == nil {
		t.Fatal("image disappeared")
	}
	if src := dom.GetAttribute(img, "src"); !strings.HasPrefix(src, "data:image/gif") {
		t.Fatalf("src = %q, want the inline GIF", src)
	}
}

// Re-archiving a page that already has its HTML must stay idempotent.
func TestArchiveDocumentSkipsSavedHTML(t *testing.T) {
	pageURL := "https://example.com/post"
	uri, err := url.Parse(pageURL)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	col := NewZipCollector(zw, http.DefaultClient)
	arc := New(WithCollector(col))

	for range 2 {
		err := arc.ArchiveReader(context.Background(), strings.NewReader("<html><body><p>x</p></body></html>"), uri, "index.html")
		if err != nil {
			t.Fatalf("ArchiveReader: %v", err)
		}
	}

	count := 0
	for range col.Resources() {
		count++
	}
	if count != 1 {
		t.Fatalf("saved %d resources, want 1", count)
	}

	if err := zw.Close(); err != nil && err != io.ErrClosedPipe {
		t.Fatalf("close zip: %v", err)
	}
}
