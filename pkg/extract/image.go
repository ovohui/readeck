// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/readeck/readeck/pkg/img"
	"github.com/gabriel-vasile/mimetype"
)

const (
	// ImageSizeThumbnail is the width of a thumbnail image.
	ImageSizeThumbnail = 380

	// ImageSizeRegular is the width of a regular image.
	ImageSizeRegular = 800

	// ImageSizeWide is the width of a bigger image.
	// Its dimension matches 48rem (on a 16px basis)
	// on an HDPI screen: 48 × 16px × 2.
	ImageSizeWide = 1536
)

// NewRemoteImage loads an image and returns a new img.Image instance.
// If the image is a GIF, it returns its first frame only.
func NewRemoteImage(ctx context.Context, client *http.Client, src string) (img.Image, error) {
	if client == nil {
		client = http.DefaultClient
	}

	if src == "" {
		return nil, errors.New("No image URL")
	}

	// Send the request with a specific Accept header
	header, _ := CheckRequestHeader(ctx)
	if header == nil {
		header = http.Header{}
	}
	header.Set("Accept", "image/webp,image/svg+xml,image/*,*/*;q=0.8")
	ctx = WithRequestHeader(ctx, header)
	ctx = WithRequestType(ctx, ImageRequest)

	rsp, err := Fetch(ctx, client, src)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close() //nolint:errcheck

	if rsp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Invalid response status (%d)", rsp.StatusCode)
	}

	buf := new(bytes.Buffer)
	mtype, err := mimetype.DetectReader(io.TeeReader(rsp.Body, buf))
	if err != nil {
		return nil, err
	}

	im, err := img.New(mtype.String(), io.MultiReader(buf, rsp.Body))
	if err != nil {
		return nil, err
	}

	// Image is a GIF, use its first frame only.
	if x, ok := im.(img.MultiFrameImage); ok {
		im = x.FirstFrame()
	}
	return im, nil
}

// Picture is a remote picture.
type Picture struct {
	Href   string
	Type   string
	Size   [2]int
	format string
	bytes  []byte
}

// NewPicture returns a new Picture instance from a given
// URL and its base.
func NewPicture(src string, base *url.URL) (*Picture, error) {
	// An inline picture carries its own data and has nothing to resolve.
	// Handing it to base.Parse would turn it into "data:image/webp;base64..."
	// with the payload eaten by path normalisation.
	if isDataURI(src) {
		return &Picture{Href: src}, nil
	}

	href, err := base.Parse(src)
	if err != nil {
		return nil, err
	}

	return &Picture{
		Href: href.String(),
	}, nil
}

// Load loads the image remotely and fit it into the given
// boundaries size.
func (p *Picture) Load(ctx context.Context, client *http.Client, size uint, toFormat string) error {
	ri, err := p.open(ctx, client)
	if err != nil {
		return err
	}
	defer ri.Close() //nolint:errcheck

	err = img.Pipeline(ri, pClean, pComp, pQual, pFit(size, 0), pFormat(toFormat))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	ct, err := ri.Encode(&buf)
	if err != nil {
		return err
	}

	p.bytes = buf.Bytes()
	p.Size = [2]int{int(ri.Width()), int(ri.Height())}
	p.Type = ct
	p.format = ri.Format()
	return nil
}

// open returns the image to work on, decoding a data: URL instead of
// fetching it.
//
// Pages saved by the browser extension, and mobile share links that inlined
// their pictures to save requests, keep the image in the document itself as
// data:image/webp;base64,... - there is no URL to ask for.
func (p *Picture) open(ctx context.Context, client *http.Client) (img.Image, error) {
	if data, declared, err := decodeDataURI(p.Href); err == nil {
		// Trust the bytes over the label: pages in the wild declare
		// image/jpeg over a WebP payload often enough to matter.
		types := []string{}
		if d := mimetype.Detect(data); d != nil {
			types = append(types, d.String())
		}
		if declared != "" && (len(types) == 0 || types[0] != declared) {
			types = append(types, declared)
		}

		var lastErr error
		for _, t := range types {
			im, err := img.New(t, bytes.NewReader(data))
			if err != nil {
				lastErr = err
				continue
			}
			if x, ok := im.(img.MultiFrameImage); ok {
				im = x.FirstFrame()
			}
			return im, nil
		}
		if lastErr == nil {
			lastErr = errors.New("no image handler for inline picture")
		}
		return nil, lastErr
	}
	return NewRemoteImage(ctx, client, p.Href)
}

// isDataURI reports whether the source is an inline picture.
func isDataURI(s string) bool {
	return len(s) > 5 && strings.EqualFold(s[:5], "data:")
}

// decodeDataURI splits an inline picture into its bytes and media type.
func decodeDataURI(s string) ([]byte, string, error) {
	if !isDataURI(s) {
		return nil, "", errors.New("not a data URI")
	}

	meta, payload, found := strings.Cut(s[5:], ",")
	if !found {
		return nil, "", errors.New("malformed data URI")
	}

	// "image/webp;base64" or "image/svg+xml;charset=utf-8"
	params := strings.Split(meta, ";")
	mtype := strings.TrimSpace(params[0])
	if !strings.HasPrefix(strings.ToLower(mtype), "image/") {
		return nil, "", fmt.Errorf("data URI is not an image: %s", mtype)
	}

	base64Encoded := false
	for _, x := range params[1:] {
		if strings.EqualFold(strings.TrimSpace(x), "base64") {
			base64Encoded = true
		}
	}

	if !base64Encoded {
		// Percent-encoded payload (SVG usually).
		data, err := url.PathUnescape(payload)
		if err != nil {
			return nil, "", err
		}
		return []byte(data), mtype, nil
	}

	// Base64 in a data URI may be wrapped across lines in hand-written pages.
	clean := strings.NewReplacer("\n", "", "\r", "", " ", "", "\t", "").Replace(payload)
	data, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(clean)
		if err != nil {
			return nil, "", err
		}
	}
	return data, mtype, nil
}

// Copy returns a resized copy of the image, as a new Picture instance.
func (p *Picture) Copy(size uint, toFormat string) (*Picture, error) {
	ri, err := img.New(p.Type, bytes.NewReader(p.bytes))
	if err != nil {
		return nil, err
	}
	defer ri.Close() //nolint:errcheck

	res := &Picture{Href: p.Href}
	err = img.Pipeline(ri, pClean, pComp, pQual, pFit(size, 0), pFormat(toFormat))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	ct, err := ri.Encode(&buf)
	if err != nil {
		return nil, err
	}

	res.bytes = buf.Bytes()
	res.Size = [2]int{int(ri.Width()), int(ri.Height())}
	res.Type = ct
	res.format = ri.Format()
	return res, nil
}

// Name returns the given name of the picture with the correct
// extension.
func (p *Picture) Name(name string) string {
	return fmt.Sprintf("%s.%s", name, p.format)
}

// Bytes returns the image data.
func (p *Picture) Bytes() []byte {
	return p.bytes
}

// Encoded returns a base64 encoded string of the image.
func (p *Picture) Encoded() string {
	if len(p.bytes) == 0 {
		return ""
	}

	return base64.StdEncoding.EncodeToString(p.bytes)
}

func pFormat(f string) img.ImageFilter {
	return func(im img.Image) error {
		return im.SetFormat(f)
	}
}

func pFit(w, h uint) img.ImageFilter {
	return func(im img.Image) error {
		return img.Fit(im, w, h)
	}
}

func pComp(im img.Image) error {
	return im.SetCompression(img.CompressionBest)
}

func pClean(im img.Image) error {
	return im.Clean()
}

func pQual(im img.Image) error {
	return im.SetQuality(75)
}
