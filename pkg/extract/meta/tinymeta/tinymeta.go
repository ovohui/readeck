// SPDX-FileCopyrightText: © 2026 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package tinymeta implements reading metadata from HTML documents without having to parse the
// entire DOM in memory.
package tinymeta

import (
	"io"
	"iter"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Scan returns an iterator that scans HTML tokens from r and yields key-value pairs representing
// common document metadata found in head, such as "title" or "og:description". Scanning will
// automatically stop as soon as the first body element is encountered.
func Scan(r io.Reader) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		z := html.NewTokenizer(r)
		inTitle := false

		for {
			if z.Next() == html.ErrorToken {
				return
			}

			tt := z.Token()

			switch tt.Type {
			case html.TextToken:
				if inTitle && !yield("title", tt.Data) {
					return
				}

			case html.EndTagToken:
				if tt.DataAtom == atom.Title {
					inTitle = false
				}

			case html.StartTagToken, html.SelfClosingTagToken:
				switch tt.DataAtom {
				case atom.Title:
					inTitle = true
				case atom.Noscript:
					z.NextIsNotRawText()
				case atom.Meta:
					var metaName string
					var metaContent string
					for _, attr := range tt.Attr {
						switch attr.Key {
						case "name", "property", "http-equiv":
							metaName = attr.Val
						case "content":
							metaContent = attr.Val
						}
					}
					if metaName != "" && metaContent != "" && !yield(metaName, metaContent) {
						return
					}
				case atom.Html, atom.Head, atom.Base, atom.Link, atom.Style, atom.Script, atom.Template:
					// Ignore other content in head while scanning for meta tags.
				default:
					// We've reached non-head content.
					return
				}
			}
		}
	}
}
