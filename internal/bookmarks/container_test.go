// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/configs"
)

func TestRewriteArticle(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	dir := t.TempDir()
	oldStorage := configs.Config.Bookmarks.StoragePath
	configs.Config.Bookmarks.StoragePath = dir
	t.Cleanup(func() { configs.Config.Bookmarks.StoragePath = oldStorage })

	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	require.NoError(err)
	zw := zip.NewWriter(f)
	entries := map[string][]byte{
		"index.html":          []byte("<!DOCTYPE html><html><body><p>old body</p></body></html>"),
		"_resources/pic.jpg":  []byte("jpeg-bytes"),
		"props.json":          []byte(`{"title":"x"}`),
		"_resources/film.mp4": []byte("mp4-bytes"),
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(err)
		_, err = w.Write(content)
		require.NoError(err)
	}
	require.NoError(zw.Close())
	require.NoError(f.Close())

	b := &Bookmark{
		FilePath: "test",
		Files: BookmarkFiles{
			"article": &BookmarkFile{Name: "index.html"},
		},
	}

	// Read the current body.
	body, err := b.ArticleBody()
	require.NoError(err)
	assert.Equal("<p>old body</p>", body)

	// Rewrite it.
	require.NoError(b.RewriteArticle(`<p>new body</p><img src="./_resources/pic.jpg">`))

	body, err = b.ArticleBody()
	require.NoError(err)
	assert.Equal(`<p>new body</p><img src="./_resources/pic.jpg">`, body)

	// Every other entry must survive untouched.
	zr, err := zip.OpenReader(zipPath)
	require.NoError(err)
	defer zr.Close() //nolint:errcheck

	got := map[string][]byte{}
	for _, entry := range zr.File {
		rc, err := entry.Open()
		require.NoError(err)
		data, err := io.ReadAll(rc)
		require.NoError(err)
		rc.Close() //nolint:errcheck
		got[entry.Name] = data
	}

	for name, content := range entries {
		if name == "index.html" {
			assert.Contains(string(got[name]), "new body")
			continue
		}
		assert.True(bytes.Equal(content, got[name]), "entry %s was altered", name)
	}
}

func TestExtractHTMLBody(t *testing.T) {
	assert := assert.New(t)
	assert.Equal("<p>x</p>", ExtractHTMLBody("<!DOCTYPE html><html><body><p>x</p></body></html>"))
	assert.Equal("plain", ExtractHTMLBody("plain"))
}
