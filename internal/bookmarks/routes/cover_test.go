// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCoverFiles(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for x := 0; x < 1000; x++ {
		src.Set(x, 0, color.RGBA{R: 255, A: 255})
	}
	buf := &bytes.Buffer{}
	require.NoError(t, png.Encode(buf, src))

	files, err := buildCoverFiles(buf.Bytes())
	require.NoError(t, err)
	require.Contains(t, files, "image")
	require.Contains(t, files, "thumbnail")

	assert.Equal(t, "img/image.jpg", files["image"].Name)
	assert.Equal(t, [2]int{1000, 500}, files["image"].Size)
	assert.Equal(t, "img/thumbnail.jpg", files["thumbnail"].Name)
	assert.Equal(t, [2]int{380, 190}, files["thumbnail"].Size)

	_, _, err = image.Decode(bytes.NewReader(files["thumbnail"].Data))
	assert.NoError(t, err)
}

func TestBuildCoverFilesRejectsGarbage(t *testing.T) {
	_, err := buildCoverFiles([]byte("not an image"))
	assert.Error(t, err)
}
