// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package components

import (
	"context"
	"io"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// Markdown returns a [templ.Component] that renders the
// given markdown text to HTML.
func Markdown(text string) templ.Component {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.NewLinkify(
				extension.WithLinkifyAllowedProtocols([]string{"http:", "https:"}),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
		),
	)

	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		return md.Convert([]byte(text), w)
	})
}
