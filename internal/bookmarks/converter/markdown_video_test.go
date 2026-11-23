// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/pkg/http/request"
)

// TestVideoMarkdown checks both renderings of an archived video.
func TestVideoMarkdown(t *testing.T) {
	if got, want := videoMarkdown("multipart", "_resources/abc.mp4", ""), "![[abc.mp4]]\n\n"; got != want {
		t.Errorf("multipart render:\n got %q\nwant %q", got, want)
	}
	got := videoMarkdown("", "_resources/abc.mp4", "https://example.org/bm/x/_resources/abc.mp4")
	want := "<video controls preload=\"metadata\" src=\"https://example.org/bm/x/_resources/abc.mp4\"></video>\n\n"
	if got != want {
		t.Errorf("plain render:\n got %q\nwant %q", got, want)
	}
}

// TestWriteArticleWithArchivedVideo renders a full markdown export of a video
// bookmark that carries a downloaded mp4, and prints the result.
func TestWriteArticleWithArchivedVideo(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")

	// GetFilePath() joins the configured storage path with the bookmark's
	// file path, so point the storage at the temporary directory.
	oldStorage := configs.Config.Bookmarks.StoragePath
	configs.Config.Bookmarks.StoragePath = dir
	t.Cleanup(func() { configs.Config.Bookmarks.StoragePath = oldStorage })

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	f, err := zw.Create("index.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("<p>本期斥资 5000+ 拿下两张 2080Ti。</p>")); err != nil {
		t.Fatal(err)
	}
	if f, err = zw.Create("_resources/kW1video.mp4"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("fake mp4")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(zipPath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	b := &bookmarks.Bookmark{
		UID:          "5hKePJ3JSTkRPvQ8Qipm5z",
		FilePath:     "test",
		Title:        "听劝买了两张2080Ti 22G！",
		SiteName:     "哔哩哔哩",
		Site:         "www.bilibili.com",
		URL:          "https://www.bilibili.com/video/BV1GXaA6jEoK/",
		DocumentType: "video",
		Created:      time.Now(),
		Files: bookmarks.BookmarkFiles{
			"article": &bookmarks.BookmarkFile{Name: "index.html"},
		},
	}

	ctx := requestContext(t, "http://readeck.test/api/bookmarks/5hKe/article.md")
	// dataset.NewBookmark needs a running task manager, which a unit test does
	// not have; build the view model directly instead.
	item := &dataset.Bookmark{
		Bookmark:     b,
		ID:           b.UID,
		Title:        b.Title,
		SiteName:     b.SiteName,
		Site:         b.Site,
		URL:          b.URL,
		DocumentType: b.DocumentType,
		Created:      b.Created,
	}

	e := NewMarkdownExporter()
	out := new(bytes.Buffer)
	if err := e.writeArticle(ctx, out, item, true); err != nil {
		t.Fatalf("writeArticle: %v", err)
	}
	t.Logf("plain markdown export://n%s", out.String())

	if !bytes.Contains(out.Bytes(), []byte("<video controls")) {
		t.Errorf("markdown output has no video tag://n%s", out.String())
	}

	// Same bookmark, zip/multipart export: the video becomes a companion file.
	zout := new(bytes.Buffer)
	if err := e.writeArticle(withExportType(ctx, "multipart"), zout, item, false); err != nil {
		t.Fatalf("writeArticle (zip): %v", err)
	}
	t.Logf("zip markdown export://n%s", zout.String())
	if !bytes.Contains(zout.Bytes(), []byte("![[kW1video.mp4]]")) {
		t.Errorf("zip output has no obsidian embed://n%s", zout.String())
	}
}

// requestContext returns a context carrying a request URL, as the export
// routes always do (urls.AbsoluteURLContext depends on it).
func requestContext(t *testing.T, target string) context.Context {
	t.Helper()
	var ctx context.Context
	h := request.InitRequest()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	if ctx == nil {
		t.Fatal("request context not initialized")
	}
	return ctx
}
