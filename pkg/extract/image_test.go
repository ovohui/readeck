// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
	. "codeberg.org/readeck/readeck/pkg/extract/testing" //revive:disable:dot-imports
	"codeberg.org/readeck/readeck/pkg/img"
)

func TestRemoteImage(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/bogus", NewFileResponder("images/bogus"))
	httpmock.RegisterResponder("GET", "/404", httpmock.NewJsonResponderOrPanic(404, ""))
	httpmock.RegisterResponder("GET", "/error", httpmock.NewErrorResponder(errors.New("HTTP")))

	formats := []string{"jpeg", "png", "gif", "ico", "bmp"}
	for _, name := range formats {
		name = "/img." + name
		httpmock.RegisterResponder("GET", name, NewFileResponder("images/"+name))
	}

	t.Run("RemoteImage", func(t *testing.T) {
		t.Run("errors", func(t *testing.T) {
			tests := []struct {
				name string
				path string
				err  string
			}{
				{"url", "", "No image URL"},
				{"404", "/404", "Invalid response status (404)"},
				{"http", "/error", `Get "/error": HTTP`},
				{"bogus", "/bogus", "no img handler for application/octet-stream"},
			}

			for _, x := range tests {
				t.Run(x.name, func(t *testing.T) {
					ri, err := extract.NewRemoteImage(context.Background(), nil, x.path)
					require.Nil(t, ri)
					if ri != nil {
						defer ri.Close() //nolint:errcheck
					}
					require.Equal(t, x.err, err.Error())
				})
			}
		})

		for _, format := range formats {
			t.Run(format, func(t *testing.T) {
				ri, err := extract.NewRemoteImage(context.Background(), nil, "/img."+format)
				require.NoError(t, err)
				defer ri.Close() //nolint:errcheck
				require.Equal(t, format, ri.Format())
			})
		}

		t.Run("fit", func(t *testing.T) {
			assert := require.New(t)
			ri, _ := extract.NewRemoteImage(context.Background(), nil, "/img.png")
			defer ri.Close() //nolint:errcheck

			w := ri.Width()
			h := ri.Height()
			assert.Equal([]uint{240, 181}, []uint{w, h})

			assert.NoError(img.Fit(ri, uint(24), uint(24)))
			assert.Equal(uint(24), ri.Width())
			assert.Equal(uint(18), ri.Height())

			assert.NoError(img.Fit(ri, 240, 240))
			assert.Equal(uint(24), ri.Width())
			assert.Equal(uint(18), ri.Height())
		})

		t.Run("encode", func(t *testing.T) {
			tests := []struct {
				name     string
				path     string
				format   string
				expected string
			}{
				{"auto-png", "/img.png", "", "image/png"},
				{"jpeg-jpeg", "/img.jpeg", "jpeg", "image/jpeg"},
				{"gif-png", "/img.gif", "gif", "image/png"},
				{"png-png", "/img.png", "png", "image/png"},
				{"png-gif", "/img.png", "gif", "image/gif"},
			}

			for _, x := range tests {
				t.Run(x.format, func(t *testing.T) {
					assert := require.New(t)
					ri, err := extract.NewRemoteImage(context.Background(), nil, x.path)
					assert.NoError(err)
					defer func() {
						if err := ri.Close(); err != nil {
							panic(err)
						}
					}()
					assert.NoError(ri.SetFormat(x.format))

					var buf bytes.Buffer
					ct, err := ri.Encode(&buf)
					assert.NoError(err)
					assert.Equal(x.expected, ct)

					_, format, _ := image.DecodeConfig(bytes.NewReader(buf.Bytes()))
					assert.Equal(strings.TrimPrefix(x.expected, "image/"), format)
				})
			}
		})
	})
}

func TestPicture(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/img", NewFileResponder("images/img.png"))

	base, _ := url.Parse("http://x/index.html")

	t.Run("URL error", func(t *testing.T) {
		p, err := extract.NewPicture("/\b0x7f", base)
		require.Nil(t, p)
		require.Error(t, err)
	})

	t.Run("HTTP error", func(t *testing.T) {
		p, _ := extract.NewPicture("/nowhere", base)
		err := p.Load(context.Background(), nil, 100, "")
		require.Error(t, err)
	})

	t.Run("Load", func(t *testing.T) {
		assert := require.New(t)
		p, _ := extract.NewPicture("/img", base)

		assert.Empty(p.Encoded())

		err := p.Load(context.Background(), nil, 100, "")
		assert.NoError(err)

		assert.Equal([2]int{100, 75}, p.Size)
		assert.Equal("image/png", p.Type)

		header := []byte{137, 80, 78, 71, 13, 10, 26, 10}
		assert.Equal(header, p.Bytes()[0:8])
		assert.Equal("iVBORw0K", p.Encoded()[0:8])
	})
}

// Chinese mobile sites inline their pictures in the page itself: there is no
// URL to request, and the bookmark cover has to come out of the bytes.
func TestPictureFromDataURI(t *testing.T) {
	assert := require.New(t)

	raw, err := os.ReadFile("test-fixtures/images/img.png")
	assert.NoError(err)
	src := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)

	p, err := extract.NewPicture(src, mustParseURL("https://example.org/post"))
	assert.NoError(err)
	assert.Equal(src, p.Href, "an inline picture must not be resolved against the page")

	assert.NoError(p.Load(context.Background(), nil, 380, ""))
	assert.Equal("image/png", p.Type)
	assert.NotEmpty(p.Bytes())
	assert.True(p.Size[0] <= 380 && p.Size[0] > 0, "width %d", p.Size[0])

	thumb, err := p.Copy(200, "")
	assert.NoError(err)
	assert.True(thumb.Size[0] <= 200)
}

func TestPictureFromDataURIRejectsNonImages(t *testing.T) {
	assert := require.New(t)

	for _, src := range []string{
		"data:text/plain;base64,SGVsbG8=",
		"data:image/png;base64,not-base64-at-all!!",
		"data:image/png",
	} {
		p, err := extract.NewPicture(src, mustParseURL("https://example.org/post"))
		assert.NoError(err)
		assert.Error(p.Load(context.Background(), nil, 380, ""), src)
	}
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
