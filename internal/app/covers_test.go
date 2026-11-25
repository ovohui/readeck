// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstPictureName(t *testing.T) {
	tests := []struct {
		name     string
		article  string
		expected string
	}{
		{
			name:     "first archived picture",
			article:  `<article><p>text</p><img src="./_resources/AbC123.jpg"><img src="./_resources/DeF456.jpg"></article>`,
			expected: "_resources/AbC123.jpg",
		},
		{
			name:     "skips remote and icon images",
			article:  `<article><img src="/style/logo.png"><img src="https://cdn.example.org/a.jpg"><img src="./_resources/XyZ789.jpg"></article>`,
			expected: "_resources/XyZ789.jpg",
		},
		{
			name:     "no picture at all",
			article:  `<article><p>只有文字</p></article>`,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, firstPictureName(test.article))
		})
	}
}

func TestPictureSize(t *testing.T) {
	assert := require.New(t)

	src := image.NewRGBA(image.Rect(0, 0, 1146, 717))
	var buf bytes.Buffer
	assert.NoError(png.Encode(&buf, src))

	assert.Equal([2]int{1146, 717}, pictureSize(bytes.NewReader(buf.Bytes())))
	// An unreadable entry is not a reason to skip the cover.
	assert.Equal([2]int{}, pictureSize(bytes.NewReader([]byte("not an image"))))
}

func TestPictureType(t *testing.T) {
	assert := require.New(t)
	assert.Equal("image/jpeg", pictureType("_resources/abc"))
	assert.Equal("image/png", pictureType("_resources/abc.png"))
}
