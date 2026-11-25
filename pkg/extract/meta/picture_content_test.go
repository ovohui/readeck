// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract"
)

func TestFirstBodyImage(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "plain",
			body:     `<div><p>x</p><img src="https://cdn.example.org/a/big.jpg"></div>`,
			expected: "https://cdn.example.org/a/big.jpg",
		},
		{
			name: "skips avatar, emoji and small images",
			body: `<div>
				<img class="avatar" src="https://cdn.example.org/user/1.jpg" width="48" height="48">
				<img src="https://cdn.example.org/emoji/smile.png">
				<img src="https://cdn.example.org/site/logo.png" width="120" height="40">
				<img src="https://cdn.example.org/photo/1612/cover.jpg" width="1080" height="1440">
			</div>`,
			expected: "https://cdn.example.org/photo/1612/cover.jpg",
		},
		{
			name: "lazy loaded image",
			body: `<div><img src="data:image/gif;base64,R0lGOD"
				data-src="https://cdn.example.org/lazy/1.jpg" width="800" height="600"></div>`,
			expected: "https://cdn.example.org/lazy/1.jpg",
		},
		{
			name:     "no picture at all",
			body:     `<div><p>只有文字</p><img src="/static/spacer.gif"></div>`,
			expected: "",
		},
		{
			// Xiaohongshu inlines the note: the payload is the only source of
			// a cover, and its base64 must not be mistaken for chrome.
			name:     "inline picture",
			body:     `<figure><img src="data:image/webp;base64,UklGRsy3AgBXRUJQV" alt="穿搭" width="1080" height="1440"></figure>`,
			expected: "data:image/webp;base64,UklGRsy3AgBXRUJQV",
		},
		{
			name: "inline placeholder with a real url",
			body: `<div><img src="data:image/svg+xml;utf8,&lt;svg/&gt;"
				data-src="https://cdn.example.org/photo/2.jpg" width="900" height="1200"></div>`,
			expected: "https://cdn.example.org/photo/2.jpg",
		},
		{
			name:     "inline picture too small to be artwork",
			body:     `<div><img src="data:image/png;base64,iVBORw0KGgo=" width="48" height="48"></div>`,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, firstBodyImage([]byte(test.body)))
		})
	}
}

func makeTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for i := range img.Pix {
		img.Pix[i] = 0xB0
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractContentPicture(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	body := makeTestPNG(t)
	httpmock.RegisterResponder("GET", "https://cdn.example.org/photo/cover.jpg",
		func(*http.Request) (*http.Response, error) {
			resp := httpmock.NewBytesResponse(http.StatusOK, body)
			resp.Header.Set("Content-Type", "image/png")
			return resp, nil
		})

	newExtractor := func(t *testing.T, docType, dom string) *extract.Extractor {
		t.Helper()
		ex, err := extract.New("https://example.org/post")
		require.NoError(t, err)
		d := ex.Drop()
		d.DocumentType = docType
		d.Body = []byte(dom)
		return ex
	}

	t.Run("takes the article picture", func(t *testing.T) {
		assert := require.New(t)
		ex := newExtractor(t, "article", `<div><img class="avatar" src="https://cdn.example.org/users/1.png" width="40" height="40">
			<img src="https://cdn.example.org/photo/cover.jpg" width="1080" height="810"></div>`)

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.Contains(ex.Drop().Pictures, "image")
		assert.Equal("https://cdn.example.org/photo/cover.jpg", ex.Drop().Pictures["image"].Href)
		assert.Contains(ex.Drop().Pictures, "thumbnail")
	})

	t.Run("takes an inline picture", func(t *testing.T) {
		assert := require.New(t)
		src := "data:image/png;base64," + base64.StdEncoding.EncodeToString(makeTestPNG(t))
		ex := newExtractor(t, "article", `<div><p>穿搭</p><img src="`+src+`" width="800" height="600"></div>`)

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.Contains(ex.Drop().Pictures, "image")
		assert.Equal(src, ex.Drop().Pictures["image"].Href)
		assert.Contains(ex.Drop().Pictures, "thumbnail")
		assert.Equal([2]int{380, 285}, ex.Drop().Pictures["thumbnail"].Size)
	})

	// A video page has exactly one picture before it plays: the cover. The
	// bookmark card has nothing else to show.
	t.Run("takes the cover of a video page", func(t *testing.T) {
		assert := require.New(t)
		ex := newExtractor(t, "video", `<figure><img src="https://cdn.example.org/photo/cover.jpg"
			width="1080" height="810" data-iframe-params="src=https%3A%2F%2Fexample.org%2Fvideo%2F1"></figure>`)

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.Contains(ex.Drop().Pictures, "image")
		assert.Contains(ex.Drop().Pictures, "thumbnail")
	})

	t.Run("keeps the declared picture", func(t *testing.T) {
		assert := require.New(t)
		ex := newExtractor(t, "article", `<div><img src="https://cdn.example.org/photo/cover.jpg"></div>`)
		ex.Drop().Pictures["image"] = &extract.Picture{Href: "https://example.org/og.png"}

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.Equal("https://example.org/og.png", ex.Drop().Pictures["image"].Href)
	})

	// Documents that are not read as an article (a raw file, a PDF, an audio
	// track) never show a cover of their own.
	t.Run("skips documents with no article body", func(t *testing.T) {
		assert := require.New(t)
		ex := newExtractor(t, "image", `<div><img src="https://cdn.example.org/photo/cover.jpg"></div>`)

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.NotContains(ex.Drop().Pictures, "image")
	})
}
