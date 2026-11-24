// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package bleach is a simple HTML sanitizer tool.
package bleach

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/go-shiori/dom"
)

// attrBlocker is a function that returns true for HTML attribute names that should be stripped.
type attrBlocker func(string) bool

// Policy holds the cleaning rules and provides methods to
// perform the DOM cleaning.
type Policy struct {
	blockAttrs []attrBlocker
}

// New instantiates a new cleaning Policy.
func New(blockAttrs []attrBlocker) Policy {
	return Policy{blockAttrs: blockAttrs}
}

// DefaultPolicy is the default bleach policy.
var DefaultPolicy = New(
	[]attrBlocker{
		// Remove "class", "style", "rel", and "sizes" attributes
		func(attr string) bool {
			switch attr {
			case "class", "style", "rel", "sizes":
				return true
			default:
				return false
			}
		},
		// Remove "data-*" attributes except some "data-readeck-" ones
		func(attr string) bool {
			if !strings.HasPrefix(attr, "data-") {
				return false
			}
			switch attr {
			case "data-readeck-width", "data-readeck-height",
				// "data-iframe-params" marks a thumbnail as the stand-in for
				// a video player: removing players from the extracted
				// content also removes the <iframe> it came from, so the
				// archiver finds the page to download through this attribute
				// instead. It is only ever written by a content script or by
				// the browser extension.
				"data-iframe-params":
				return false
			default:
				return true
			}
		},
		// Remove all on* (JS events) attributes
		func(attr string) bool {
			if len(attr) < 3 {
				return false
			}
			r := attr[2]
			return attr[0:2] == "on" && r >= 'a' && r <= 'z'
		},
	},
)

// SanitizeString replaces any control character in a string by a space.
func SanitizeString(s string) string {
	return ctrlReplacer.Replace(s)
}

// Clean removes unwanted tags and attributes from the document.
func (p Policy) Clean(top *html.Node) {
	var traverse func(*html.Node)
	traverse = func(parent *html.Node) {
		var nextSibling *html.Node
		for node := parent.FirstChild; node != nil; node = nextSibling {
			nextSibling = node.NextSibling
			if node.Type != html.ElementNode {
				continue
			}

			// Remove tags explicitly marked for removal.
			rule, exists := p.getTagRule(node)
			if rule&tagRemove > 0 {
				node.Parent.RemoveChild(node)
				continue
			}

			// Rename disallowed or unrecognized tags.
			if !exists || (rule&tagRename > 0 && rule&tagIsBlock > 0) {
				// a block or unknown tag becomes a div
				node.Data = "div"
				node.DataAtom = atom.Div
			} else if rule&tagRename > 0 {
				// otherwise, a span
				node.Data = "span"
				node.DataAtom = atom.Span
			}

			traverse(node)

			// Apply blockAttrs rules on element attributes.
			for i := len(node.Attr) - 1; i >= 0; i-- {
				for _, r := range p.blockAttrs {
					if r(node.Attr[i].Key) {
						node.Attr = append(node.Attr[0:i], node.Attr[i+1:]...)
						break
					}
				}
			}

			if p.isEmptyNode(node, rule) {
				node.Parent.RemoveChild(node)
				continue
			}

			if node.DataAtom == atom.A && dom.HasAttribute(node, "href") {
				dom.SetAttribute(node, "rel", "nofollow noopener noreferrer")
			}
		}
	}
	traverse(top)
}

// isEmptyNode returns true for elements with no child elements nodes and blank text content, with a
// few exceptions such as tags that were marked as [tagKeepEmpty].
func (p Policy) isEmptyNode(node *html.Node, rule tagRule) bool {
	// Never remove the document body.
	if node.DataAtom == atom.Body {
		return false
	}

	// Keep <a name> tags
	if node.DataAtom == atom.A && dom.GetAttribute(node, "name") != "" {
		return false
	}

	// Keep tags that are explicitly allowed to be empty, e.g. <hr>
	if rule&tagKeepEmpty > 0 {
		return false
	}

	isBlockTag := rule&tagIsBlock > 0

	// Inspect element contents
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.ElementNode:
			// Keep tags with child elements
			return false
		case html.TextNode:
			if isBlockTag {
				if hasPrintableText(child) {
					// Keep block elements with non-blank text.
					return false
				}
			} else if len(child.Data) > 0 {
				// Keep inline elements with any text, including spaces.
				return false
			}
		}
	}

	return true
}

func (p Policy) getTagRule(node *html.Node) (tagRule, bool) {
	if node.DataAtom == 0 {
		// Nodes that were created manually or by using [dom.CreateElement] might have their
		// DataAtom field uninitialized.
		node.DataAtom = atom.Lookup([]byte(node.Data))
	}
	if rule, exists := elementMap[node.DataAtom]; exists {
		return rule, true
	}
	rule, exists := elementMap2[node.Data]
	return rule, exists
}

func hasPrintableText(node *html.Node) bool {
	for _, r := range node.Data {
		if !isHTMLSpace(r) {
			return true
		}
	}
	return false
}

// isHTMLSpace returns true if a rune is a space as defined by the HTML spec.
func isHTMLSpace(r rune) bool {
	if uint32(r) <= unicode.MaxLatin1 {
		switch r {
		case '\t', '\n', '\r', ' ':
			return true
		}
		return false
	}
	return false
}
