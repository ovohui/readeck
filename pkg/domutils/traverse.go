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

// FindBody finds the first body element contained by node.
func FindBody(node *html.Node) *html.Node {
	var traverse func(*html.Node) *html.Node
	traverse = func(n *html.Node) *html.Node {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode {
				continue
			}
			switch child.Data {
			case "body":
				return child
			case "head":
				// No need to descend into the head element
				continue
			default:
				if found := traverse(child); found != nil {
					return found
				}
			}
		}
		return nil
	}
	return traverse(node)
}

// WrapContents moves all child nodes of parent to target. Afterwards, target will become the only
// child of parent.
func WrapContents(parent *html.Node, target *html.Node) {
	for child := parent.FirstChild; child != nil; {
		nextSibling := child.NextSibling
		parent.RemoveChild(child)
		target.AppendChild(child)
		child = nextSibling
	}
	if target.Parent != nil {
		target.Parent.RemoveChild(target)
	}
	parent.AppendChild(target)
}

// CreateElement creates a HTML element node based on tagName.
func CreateElement(tagName string) *html.Node {
	return &html.Node{
		Type:     html.ElementNode,
		Data:     tagName,
		DataAtom: atom.Lookup([]byte(tagName)),
	}
}
