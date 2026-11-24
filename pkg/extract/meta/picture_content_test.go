// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
