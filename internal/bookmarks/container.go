// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	rxHTMLStart = regexp.MustCompile(`^(?s)(.*?)<body>`)
	rxHTMLEnd   = regexp.MustCompile(`</body>\s*</html>\s*$`)
)

// BookmarkContainer is a wrapper around zip.ReadCloser
// to handle a bookmark's zipfile.
type BookmarkContainer struct {
	*zip.ReadCloser
	articleFilename string
	articleContent  *strings.Builder
}

// OpenContainer opens the bookmark's zipfile and returns a new
// bookmarkContainer instance.
func (b *Bookmark) OpenContainer() (*BookmarkContainer, error) { //revive:disable:unexported-return
	p := b.GetFilePath()
	if p == "" {
		return nil, os.ErrNotExist
	}

	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, err
	}

	res := &BookmarkContainer{
		ReadCloser:     z,
		articleContent: new(strings.Builder),
	}
	if a, ok := b.Files["article"]; ok {
		res.articleFilename = a.Name
	}

	return res, nil
}

// Lookup return a [*zip.File] with the given name, when it exists.
func (c *BookmarkContainer) Lookup(name string) (*zip.File, bool) {
	for _, entry := range c.File {
		if entry.Name == name {
			return entry, true
		}
	}
	return nil, false
}

// ListResources returns a list of files located under "_resources/".
func (c *BookmarkContainer) ListResources() []*zip.File {
	res := []*zip.File{}
	for _, entry := range c.File {
		if !strings.HasSuffix(entry.Name, "/") && strings.HasPrefix(entry.Name, resourceDirName) {
			res = append(res, entry)
		}
	}

	return res
}

// LoadArticle loads the bookmark´s article when it exists.
func (c *BookmarkContainer) LoadArticle() error {
	if c.articleFilename == "" {
		return os.ErrNotExist
	}

	fp, err := c.Open(c.articleFilename)
	if err != nil {
		return err
	}

	_, err = io.Copy(c.articleContent, fp)

	return err
}

// ReplaceLinks replaces all the link to _resources/* in the article content.
func (c *BookmarkContainer) ReplaceLinks(fn func(name string) string) (err error) {
	args := []string{}
	for _, x := range c.ListResources() {
		args = append(args, "./"+x.Name, fn("./"+x.Name))
	}

	replacer := strings.NewReplacer(args...)
	res := replacer.Replace(c.articleContent.String())
	c.articleContent.Reset()
	_, err = c.articleContent.WriteString(res)
	return
}

// ExtractBody extract the content of the article's HTML body.
func (c *BookmarkContainer) ExtractBody() (err error) {
	res := ExtractHTMLBody(c.articleContent.String())
	c.articleContent.Reset()
	_, err = c.articleContent.WriteString(res)
	return
}

// GetArticle returns a string of the article's HTML.
func (c *BookmarkContainer) GetArticle() string {
	return c.articleContent.String()
}

// GetFile returns a file's content.
func (c *BookmarkContainer) GetFile(name string) ([]byte, error) {
	fd, err := c.Open(name)
	if err != nil {
		return nil, err
	}
	defer fd.Close() //nolint:errcheck
	return io.ReadAll(fd)
}

// ExtractHTMLBody returns the given string's content that's inside
// the body element.
func ExtractHTMLBody(text string) string {
	res := rxHTMLStart.ReplaceAllString(text, "")
	res = rxHTMLEnd.ReplaceAllString(res, "")
	return res
}

// ArticleBody returns the archived article's body HTML, ready to be edited.
func (b *Bookmark) ArticleBody() (string, error) {
	c, err := b.OpenContainer()
	if err != nil {
		return "", err
	}
	defer c.Close() //nolint:errcheck

	if err := c.LoadArticle(); err != nil {
		return "", err
	}

	return strings.TrimSpace(ExtractHTMLBody(c.GetArticle())), nil
}

// RewriteArticle replaces the body of the archived article with bodyHTML.
//
// The archive is a zip file; the other entries (images, log, props) are copied
// with their raw compressed stream so that only the article is rewritten.
func (b *Bookmark) RewriteArticle(bodyHTML string) error {
	p := b.GetFilePath()
	if p == "" {
		return os.ErrNotExist
	}

	articleName := "index.html"
	if a, ok := b.Files["article"]; ok {
		articleName = a.Name
	}

	zr, err := zip.OpenReader(p)
	if err != nil {
		return err
	}

	tmpName, err := b.writeArchive(zr, articleName, bodyHTML)
	// The original archive must be closed before it can be replaced (Windows
	// keeps the file locked while a handle is open).
	if cErr := zr.Close(); err == nil {
		err = cErr
	}
	if err != nil {
		if tmpName != "" {
			os.Remove(tmpName) //nolint:errcheck
		}
		return err
	}

	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName) //nolint:errcheck
		return err
	}
	return nil
}

// writeArchive writes the archive to a temporary file and returns its name.
// Only the article entry is rewritten, everything else is copied as is.
func (b *Bookmark) writeArchive(zr *zip.ReadCloser, articleName, bodyHTML string) (string, error) {
	p := b.GetFilePath()

	tmp, err := os.CreateTemp(filepath.Dir(p), "article-*.zip")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()

	zw := zip.NewWriter(tmp)
	doc := []byte("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"><title></title></head><body>" +
		bodyHTML + "</body></html>")

	var found bool
	for _, entry := range zr.File {
		if entry.Name == articleName {
			found = true
			w, err := zw.CreateHeader(&zip.FileHeader{
				Name:     entry.Name,
				Method:   zip.Deflate,
				Modified: time.Now(),
			})
			if err != nil {
				tmp.Close() //nolint:errcheck
				return tmpName, err
			}
			if _, err = w.Write(doc); err != nil {
				tmp.Close() //nolint:errcheck
				return tmpName, err
			}
			continue
		}

		// Copy the entry without recompressing it.
		rc, err := entry.OpenRaw()
		if err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
		hdr := entry.FileHeader
		w, err := zw.CreateRaw(&hdr)
		if err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
		if _, err = io.Copy(w, rc); err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
	}

	if !found {
		// No article in the archive yet: add it as a new entry.
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     articleName,
			Method:   zip.Deflate,
			Modified: time.Now(),
		})
		if err == nil {
			_, err = w.Write(doc)
		}
		if err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
	}

	if err := zw.Close(); err != nil {
		tmp.Close() //nolint:errcheck
		return tmpName, err
	}
	if err := tmp.Close(); err != nil {
		return tmpName, err
	}

	return tmpName, nil
}
