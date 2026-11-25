// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"strings"

	"github.com/cristalhq/acmd"
	"github.com/doug-martin/goqu/v9"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/pkg/img"
)

func init() {
	commands = append(commands, acmd.Command{
		Name:        "covers",
		Description: "Give bookmarks saved without a cover the first picture of their archive",
		ExecFunc:    runCovers,
	})
}

// runCovers picks a cover for bookmarks that have none, from the pictures
// already stored in their own archive.
//
// The cover comes from the page's social card, then from the first picture of
// the article. Pages that inline their pictures (Xiaohongshu) or that were
// saved before those rules existed sit in the list as a grey placeholder while
// the archive below them holds the images. Nothing is downloaded again here:
// the first picture the archived article shows becomes the cover, which is
// what a reader would call the picture of the page.
func runCovers(_ context.Context, args []string) error {
	flags := &appFlags{}
	fs := flags.Flags()
	dryRun := fs.Bool("dry-run", false, "list what would change without saving")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if err := appPreRun(flags); err != nil {
		return err
	}
	defer appPostRun()

	var items []*bookmarks.Bookmark
	ds := bookmarks.Bookmarks.Query().
		Where(goqu.C("state").Eq(bookmarks.StateLoaded))
	if err := ds.ScanStructs(&items); err != nil {
		return err
	}

	var fixed, skipped, failed int
	for _, b := range items {
		if _, ok := b.Files["thumbnail"]; ok {
			continue
		}
		if _, ok := b.Files["article"]; !ok {
			skipped++
			continue
		}

		name, size, err := firstArchivedPicture(b)
		if err != nil {
			slog.Warn("covers: unreadable archive", slog.String("id", b.UID), slog.Any("err", err))
			failed++
			continue
		}
		if name == "" {
			skipped++
			continue
		}

		if *dryRun {
			println("•", b.UID, "->", name)
			fixed++
			continue
		}

		b.Files["thumbnail"] = &bookmarks.BookmarkFile{
			Name: name,
			Type: pictureType(name),
			Size: size,
		}
		if err := b.Save(); err != nil {
			slog.Warn("covers: cannot save", slog.String("id", b.UID), slog.Any("err", err))
			failed++
			continue
		}
		println("•", b.UID, "->", name)
		fixed++
	}

	slog.Info("covers", slog.Int("set", fixed), slog.Int("without_picture", skipped), slog.Int("errors", failed))
	println("covers:", fixed, "set,", skipped, "without a picture in the archive,", failed, "errors")
	return nil
}

// firstArchivedPicture returns the name of the first picture the archived
// article shows, and its dimensions when they can be read.
func firstArchivedPicture(b *bookmarks.Bookmark) (string, [2]int, error) {
	size := [2]int{}

	c, err := b.OpenContainer()
	if err != nil {
		return "", size, err
	}
	defer c.Close() //nolint:errcheck

	if err := c.LoadArticle(); err != nil {
		return "", size, err
	}

	name := firstPictureName(c.GetArticle())
	if name == "" {
		return "", size, nil
	}

	// Dimensions are cosmetic: the card keeps its own ratio when it does not
	// know them, and a picture too big to decode is still a cover.
	if f, ok := c.Lookup(name); ok && f.UncompressedSize64 < 12<<20 {
		if r, err := f.Open(); err == nil {
			size = pictureSize(r)
			r.Close() //nolint:errcheck
		}
	}

	return name, size, nil
}

// firstPictureName reads the first <img> of an archived article that points
// inside the archive. Images are stored as "./_resources/<hash>", in the order
// they appear in the article.
func firstPictureName(article string) string {
	doc, err := html.Parse(strings.NewReader(article))
	if err != nil {
		return ""
	}

	prefix := bookmarks.ResourceDirName() + "/"
	for _, n := range dom.GetElementsByTagName(doc, "img") {
		src := strings.TrimPrefix(strings.TrimSpace(dom.GetAttribute(n, "src")), "./")
		if len(src) > len(prefix) && strings.HasPrefix(src, prefix) {
			return src
		}
	}
	return ""
}

// pictureSize decodes a stored picture just far enough to know its width and
// height, returning zeros when the file cannot be read.
func pictureSize(r io.Reader) [2]int {
	data, err := io.ReadAll(io.LimitReader(r, 12<<20))
	if err != nil {
		return [2]int{}
	}

	mtype := mimetype.Detect(data)
	im, err := img.New(mtype.String(), bytes.NewReader(data))
	if err != nil {
		return [2]int{}
	}
	defer im.Close() //nolint:errcheck

	return [2]int{int(im.Width()), int(im.Height())}
}

// pictureType returns the content type the archive stores the picture as.
// The archiver only keeps JPEG and PNG (WebP and the rest are converted).
func pictureType(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".png") {
		return "image/png"
	}
	return "image/jpeg"
}
