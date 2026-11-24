// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"bytes"
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

	t.Run("keeps the declared picture", func(t *testing.T) {
		assert := require.New(t)
		ex := newExtractor(t, "article", `<div><img src="https://cdn.example.org/photo/cover.jpg"></div>`)
		ex.Drop().Pictures["image"] = &extract.Picture{Href: "https://example.org/og.png"}

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.Equal("https://example.org/og.png", ex.Drop().Pictures["image"].Href)
	})

	t.Run("skips videos", func(t *testing.T) {
		assert := require.New(t)
		ex := newExtractor(t, "video", `<div><img src="https://cdn.example.org/photo/cover.jpg"></div>`)

		ExtractContentPicture(ex.NewProcessMessage(extract.StepFinish), nil)

		assert.NotContains(ex.Drop().Pictures, "image")
	})
}
