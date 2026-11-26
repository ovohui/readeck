// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/go-shiori/dom"
	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/img"
)

const (
	resourceDirName = "_resources"
)

var (
	// We can't process too many images at the same time if we don't want to overload
	// the system and freeze everything just because the image processing has way
	// too much work to do.
	imgSem = semaphore.NewWeighted(2)
	imgCtx = context.Background()

	archiveFlags = archiver.EnableImages | archiver.EnableBestImage | archiver.EnableDataAttributes
)

// currentArchiveFlags returns the archiver flags for the current configuration.
// When video downloading is enabled, media resources are also downloaded into
// the bookmark archive.
// SiteCookiesFile returns a Netscape cookies file holding the cookies saved in
// Settings > Site cookies. It is set at startup by internal/app, because the
// sitecookies package imports internal/server, which imports this one.
var SiteCookiesFile func() (string, error)

func currentArchiveFlags() archiver.ArchiveFlag {
	flags := archiveFlags
	if configs.Config.Bookmarks.DownloadVideos {
		flags |= archiver.EnableMedia
		// Keep embedded players (e.g. Zhihu/Bilibili video iframes). Without
		// EnableEmbeds the archiver strips every <iframe>, which is why saved
		// answers with videos showed no player at all. The iframe src stays a
		// live URL, so the video remains playable when online.
		flags |= archiver.EnableEmbeds
	}
	return flags
}

// ResourceDirName returns the resource folder name in an archive.
func ResourceDirName() string {
	return resourceDirName
}

// interface guards.
var (
	_ archiver.Collector        = &bookmarkCollector{}
	_ archiver.ConvertCollector = &bookmarkCollector{}
	_ archiver.LoggerCollector  = &bookmarkCollector{}
)

// for these formats, we copy them directly if they don't need resizing.
// Note: as tempting as allowing webp can be, it must be converted
// to either jpeg or png for maximum EPUB compatibility.
var directCopyFormats = map[string]struct{}{
	"image/gif":  {},
	"image/jpeg": {},
	"image/png":  {},
	// Animated WebP/AVIF cannot be decoded by x/image (and AVIF at all), but a
	// browser can. Forums increasingly serve a .gif URL as animated WebP; the
	// old code tried to decode it, failed, and removed the picture.
	"image/webp": {},
	"image/avif": {},
}

type bookmarkCollector struct {
	archiver.ZipCollector
	logger *slog.Logger
}

func (c *bookmarkCollector) Log() *slog.Logger {
	return c.logger
}

func (c *bookmarkCollector) Name(uri string) string {
	return "./" + path.Join(
		ResourceDirName(),
		base58.EncodeUUID(uuid.NewSHA1(uuid.NameSpaceURL, []byte(uri))),
	)
}

func (c *bookmarkCollector) Convert(ctx context.Context, res *archiver.Resource, r io.ReadCloser) (io.ReadCloser, error) {
	return ConvertCollectedImage(ctx, c, res, r)
}

// ConvertCollectedImage copies or convert a collected image.
// When an image fits in [extract.ImageSizeWide] and is in a compatible format, its
// reader is returned directly. Otherwise, it's converted to a suitable format.
func ConvertCollectedImage(ctx context.Context, c archiver.Collector, res *archiver.Resource, r io.ReadCloser) (io.ReadCloser, error) {
	if strings.Split(res.ContentType, "/")[0] != "image" {
		return r, nil
	}

	node := archiver.GetNodeContext(ctx)
	l := archiver.Logger(c).With(slog.Any("url", archiver.URLLogValue(res.URL())))
	readabilityEnabled, _ := contents.IsReadabilityEnabled(ctx)

	// First pass ignoring icons
	if readabilityEnabled && contents.IsIcon(node, res.Width, res.Height, 64) {
		l.Debug(
			"remove icon image",
			slog.Int("w", res.Width),
			slog.Int("h", res.Height),
		)
		return nil, archiver.ErrRemoveSrc
	}

	// Direct copy of small enough images and in a compatible format.
	if _, ok := directCopyFormats[res.ContentType]; ok && res.Width <= extract.ImageSizeWide {
		l.Debug("copy image", slog.Group(
			"resource",
			slog.String("type", res.ContentType),
			slog.Int("w", res.Width),
			slog.Int("h", res.Height),
		))
		return r, nil
	}

	err := imgSem.Acquire(imgCtx, 1)
	if err != nil {
		return nil, archiver.ErrRemoveSrc
	}
	defer imgSem.Release(1)

	buf := new(bytes.Buffer)
	im, err := img.New(res.ContentType, io.TeeReader(r, buf))
	r.Close() //nolint:errcheck
	if err != nil {
		l.Warn(
			"open image",
			slog.String("format", res.ContentType),
			slog.Any("err", err),
		)
		return nil, archiver.ErrRemoveSrc
	}
	defer func() {
		if err := im.Close(); err != nil {
			l.Warn("closing image", slog.Any("err", err))
		}
	}()

	// Direct copy of animated images
	if im, ok := im.(img.MultiFrameImage); ok && im.Frames() > 1 {
		l.Debug("copy image", slog.Group(
			"resource",
			slog.String("type", im.ContentType()),
			slog.Int("w", int(im.Width())),
			slog.Int("h", int(im.Height())),
		))
		return io.NopCloser(buf), nil
	}

	// Second pass ignoring icons
	if readabilityEnabled && contents.IsIcon(node, int(im.Width()), int(im.Height()), 64) {
		l.Debug(
			"remove icon image",
			slog.Int("w", int(im.Width())),
			slog.Int("h", int(im.Height())),
		)
		return nil, archiver.ErrRemoveSrc
	}

	if err = img.Pipeline(
		im,
		func(im img.Image) error { return im.Clean() },
		func(im img.Image) error { return im.SetQuality(75) },
		func(im img.Image) error { return im.SetCompression(img.CompressionBest) },
		func(im img.Image) error { return img.Fit(im, extract.ImageSizeWide, 0) },
	); err != nil {
		l.Warn("convert image", slog.Any("err", err))
		return nil, archiver.ErrRemoveSrc
	}

	if node != nil && dom.TagName(node) == "img" {
		dom.SetAttribute(node, "width", strconv.FormatUint(uint64(im.Width()), 10))
		dom.SetAttribute(node, "height", strconv.FormatUint(uint64(im.Height()), 10))
	}

	buf.Reset()
	ct, err := im.Encode(buf)
	if err != nil {
		l.Warn("encode image", slog.Any("err", err))
		return nil, archiver.ErrRemoveSrc
	}

	l.Debug(
		"convert image",
		slog.Group(
			"resource",
			slog.String("type", res.ContentType),
			slog.Int("w", res.Width),
			slog.Int("h", res.Height),
		),
		slog.Group(
			"image",
			slog.String("type", ct),
			slog.Int("w", int(im.Width())),
			slog.Int("h", int(im.Height())),
		),
	)

	res.ContentType = ct
	res.Width = int(im.Width())
	res.Height = int(im.Height())
	res.Name = strings.TrimSuffix(res.Name, path.Ext(res.Name))
	res.Name += archiver.GetExtension(res.ContentType)

	return io.NopCloser(buf), nil
}

// ArchiveDocument runs the archiver and returns a the number of saved resources.
func ArchiveDocument(ctx context.Context, zw *zip.Writer, ex *extract.Extractor) (int, error) {
	collector := &bookmarkCollector{
		ZipCollector: *archiver.NewZipCollector(
			zw,
			ex.Client(),
			archiver.WithTimeout(50*time.Second),
		),
		logger: ex.Log(),
	}

	opts := []archiver.Option{
		archiver.WithFlags(currentArchiveFlags()),
		archiver.WithConcurrency(8),
		archiver.WithCollector(collector),
	}
	if configs.Config.Bookmarks.DownloadEmbeddedVideos {
		// yt-dlp runs as a separate process and never sees the cookie
		// injection the extractor's HTTP client gets. A site that answers an
		// anonymous request with a login wall (Zhihu) also refuses to hand
		// over its clip, so hand the saved site cookies to yt-dlp as well.
		// An explicitly configured file still wins.
		cookiesFile := configs.Config.Bookmarks.YtdlpCookiesFile
		if cookiesFile == "" && SiteCookiesFile != nil {
			if f, err := SiteCookiesFile(); err == nil {
				cookiesFile = f
			} else {
				slog.Debug("no site cookie for yt-dlp", slog.Any("err", err))
			}
		}
		opts = append(opts, archiver.WithYtdlp(archiver.YtdlpConfig{
			Path:        configs.Config.Bookmarks.YtdlpPath,
			MaxHeight:   configs.Config.Bookmarks.YtdlpMaxHeight,
			Timeout:     time.Duration(configs.Config.Bookmarks.YtdlpTimeout) * time.Second,
			CookiesFile: cookiesFile,
			ExtraArgs:   strings.Fields(configs.Config.Bookmarks.YtdlpExtraArgs),
		}))
		// Video bookmarks (Bilibili, ...) have no player in their extracted
		// HTML: ask the archiver to rebuild one from the page URL.
		if ex.Drop().DocumentType == "video" {
			opts = append(opts, archiver.WithForcedVideo(ex.Drop().URL.String()))
		}
	}

	arc := archiver.New(opts...)

	err := arc.ArchiveReader(ctx, bytes.NewReader(ex.HTML), ex.Drop().URL, "index.html")
	if err != nil {
		return 0, err
	}

	count := 0
	for res := range collector.Resources() {
		if res.Saved() {
			count++
		}
	}

	return count, nil
}
