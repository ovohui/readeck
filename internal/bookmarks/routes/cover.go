// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	_ "image/gif" // GIF decoder
	_ "image/png" // PNG decoder
	"io"
	"net/http"
	"net/url"
	"strings"

	xdraw "golang.org/x/image/draw"

	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
	"codeberg.org/readeck/readeck/internal/httpclient"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"

	_ "golang.org/x/image/bmp"  // BMP decoder
	_ "golang.org/x/image/tiff" // TIFF decoder
	_ "golang.org/x/image/webp" // WebP decoder
)

const maxCoverSize = 20 << 20

type coverForm struct {
	forms.Form
	File   forms.FileField    `json:"cover_file"`
	URL    forms.TextField    `json:"cover_url" validate:"trim max_len:2048"`
	Remove forms.BooleanField `json:"remove_cover"`
}

// bookmarkCover shows and updates a bookmark's cover. The picture is stored
// inside the bookmark archive, so it behaves exactly like a cover the
// extractor found: cards, the reading page and exports all use it.
func (h *viewsRouter) bookmarkCover(w http.ResponseWriter, r *http.Request) {
	b := getBookmark(r.Context())
	item := dataset.NewBookmark(r.Context(), b)
	f := forms.New[coverForm](r.Context())

	status := http.StatusOK
	if r.Method == http.MethodPost {
		forms.Bind(r, f)
		status = http.StatusUnprocessableEntity
		if f.IsValid() {
			var err error
			switch {
			case f.Remove.Value():
				err = b.RemoveCover()
			case f.File.IsBound() && !f.File.IsEmpty():
				var data []byte
				data, err = readUploadedFile(f)
				if err == nil {
					var files map[string]bookmarks.CoverFile
					files, err = buildCoverFiles(data)
					if err == nil {
						err = b.SetCover(files)
					}
				}
			case strings.TrimSpace(f.URL.Value()) != "":
				var data []byte
				data, err = fetchCoverImage(r, f.URL.Value())
				if err == nil {
					var files map[string]bookmarks.CoverFile
					files, err = buildCoverFiles(data)
					if err == nil {
						err = b.SetCover(files)
					}
				}
			default:
				err = forms.Gettext("Choose an image file or enter an image URL.")
			}

			if err == nil {
				_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("The cover was updated."))
				server.Redirect(w, r, "/bookmarks/"+b.UID)
				return
			}
			f.AddErrors(err)
		}
	}

	server.RenderComponent(w, r, status, Views{}.bookmarkCover(item, f))
}

func readUploadedFile(f *coverForm) ([]byte, error) {
	rc, err := f.File.Value().Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(rc, maxCoverSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCoverSize {
		return nil, forms.Gettext("This image is too large.")
	}
	return data, nil
}

func fetchCoverImage(r *http.Request, rawURL string) ([]byte, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, forms.Gettext("Enter a valid image URL.")
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*")

	resp, err := httpclient.New().Do(req)
	if err != nil {
		return nil, forms.Gettext("Unable to download this image.")
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, forms.Gettext("Unable to download this image.")
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverSize+1))
	if err != nil {
		return nil, forms.Gettext("Unable to download this image.")
	}
	if len(data) > maxCoverSize {
		return nil, forms.Gettext("This image is too large.")
	}
	return data, nil
}

// buildCoverFiles decodes the picture and produces the two entries stored in
// the archive: a full-size cover (up to 1600px) and a 380px thumbnail.
func buildCoverFiles(data []byte) (map[string]bookmarks.CoverFile, error) {
	if len(data) > maxCoverSize {
		return nil, forms.Gettext("This image is too large.")
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, forms.Gettext("Unable to read this image. Use a JPEG, PNG, GIF, WebP, BMP or TIFF file.")
	}

	imageData, imageSize, err := resizeCover(src, 1600, 88)
	if err != nil {
		return nil, forms.Gettext("Unable to read this image. Use a JPEG, PNG, GIF, WebP, BMP or TIFF file.")
	}
	thumbData, thumbSize, err := resizeCover(src, 380, 82)
	if err != nil {
		return nil, forms.Gettext("Unable to read this image. Use a JPEG, PNG, GIF, WebP, BMP or TIFF file.")
	}

	return map[string]bookmarks.CoverFile{
		"image": {
			Name: "img/image.jpg",
			Type: "image/jpeg",
			Data: imageData,
			Size: imageSize,
		},
		"thumbnail": {
			Name: "img/thumbnail.jpg",
			Type: "image/jpeg",
			Data: thumbData,
			Size: thumbSize,
		},
	}, nil
}

func resizeCover(src image.Image, maxSide uint, quality int) ([]byte, [2]int, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, [2]int{}, errors.New("empty image")
	}

	scale := float64(maxSide) / float64(w)
	if hs := float64(maxSide) / float64(h); hs < scale {
		scale = hs
	}
	if scale > 1 {
		scale = 1
	}

	nw, nh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	// JPEG has no alpha: put transparent pictures on white instead of black.
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, [2]int{}, err
	}
	return buf.Bytes(), [2]int{nw, nh}, nil
}
