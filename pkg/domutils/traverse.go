// SPDX-FileCopyrightText: © 2026 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package domutils provides optimized traversal functions for parsed HTML DOM trees and should be
// preferred whenever possible over "github.com/go-shiori/dom" and "github.com/antchfx/htmlquery".
package domutils

import (
	"iter"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// EachElement yields every element node in the given DOM. The yielded node is safe to detach from
// the DOM mid-iteration.
func EachElement(node *html.Node) iter.Seq[*html.Node] {
	return func(yield func(*html.Node) bool) {
		var traverse func(*html.Node) bool
		traverse = func(n *html.Node) bool {
			for child := n.FirstChild; child != nil; {
				nextSibling := child.NextSibling
				if child.Type == html.ElementNode {
					if !yield(child) {
						return false
					}
					if child.Parent != nil && !traverse(child) {
						return false
					}
				}
				child = nextSibling
			}
			return true
		}
		_ = traverse(node)
	}
}

// EachElementByTag yields every element node matching the tag name in the given DOM. The yielded
// node is safe to detach from the DOM mid-iteration.
func EachElementByTag(node *html.Node, tagName atom.Atom) iter.Seq[*html.Node] {
	return func(yield func(*html.Node) bool) {
		for n := range EachElement(node) {
			if n.DataAtom == tagName && !yield(n) {
				return
			}
		}
	}
}
