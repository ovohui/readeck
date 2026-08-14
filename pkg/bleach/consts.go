// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bleach

import (
	"strings"

	"golang.org/x/net/html/atom"
)

type tagRule uint8

const (
	tagKeep = 1 << iota
	tagKeepEmpty
	tagRemove
	tagRename
	tagIsBlock
)

// elementMap is the map of all known elements
// and the [tagRule]s that apply to them.
// As per https://developer.mozilla.org/en-US/docs/Web/HTML/Element
var elementMap = map[atom.Atom]tagRule{
	atom.A:          tagKeep,
	atom.Abbr:       tagKeep,
	atom.Acronym:    tagKeep,
	atom.Address:    tagKeep | tagIsBlock,
	atom.Applet:     tagRemove,
	atom.Area:       tagKeep | tagKeepEmpty,
	atom.Article:    tagKeep | tagIsBlock,
	atom.Aside:      tagKeep | tagIsBlock,
	atom.Audio:      tagRemove,
	atom.B:          tagKeep,
	atom.Base:       tagRemove | tagKeepEmpty,
	atom.Bdi:        tagKeep,
	atom.Bdo:        tagKeep,
	atom.Big:        tagKeep,
	atom.Blockquote: tagKeep | tagIsBlock,
	atom.Body:       tagKeep,
	atom.Br:         tagKeep | tagKeepEmpty,
	atom.Button:     tagRemove,
	atom.Canvas:     tagRemove | tagIsBlock,
	atom.Caption:    tagKeep,
	atom.Center:     tagKeep,
	atom.Cite:       tagKeep,
	atom.Code:       tagKeep,
	atom.Col:        tagKeep | tagKeepEmpty,
	atom.Command:    tagKeepEmpty,
	atom.Colgroup:   tagKeep,
	atom.Data:       tagKeep,
	atom.Datalist:   tagKeep,
	atom.Dd:         tagKeep | tagIsBlock,
	atom.Del:        tagKeep,
	atom.Details:    tagKeep,
	atom.Dfn:        tagKeep,
	atom.Dialog:     tagRemove,
	atom.Dir:        tagKeep,
	atom.Div:        tagKeep | tagIsBlock,
	atom.Dl:         tagKeep | tagIsBlock,
	atom.Dt:         tagKeep | tagIsBlock,
	atom.Em:         tagKeep,
	atom.Embed:      tagRemove | tagKeepEmpty,
	atom.Fieldset:   tagRename | tagIsBlock,
	atom.Figcaption: tagKeep | tagIsBlock,
	atom.Figure:     tagKeep | tagIsBlock,
	atom.Font:       tagRename,
	atom.Footer:     tagKeep | tagIsBlock,
	atom.Form:       tagRename | tagIsBlock,
	atom.Frame:      tagRemove,
	atom.Frameset:   tagRemove,
	atom.H1:         tagKeep | tagIsBlock,
	atom.H2:         tagKeep | tagIsBlock,
	atom.H3:         tagKeep | tagIsBlock,
	atom.H4:         tagKeep | tagIsBlock,
	atom.H5:         tagKeep | tagIsBlock,
	atom.H6:         tagKeep | tagIsBlock,
	atom.Head:       tagRemove,
	atom.Header:     tagKeep | tagIsBlock,
	atom.Hgroup:     tagKeep,
	atom.Hr:         tagKeep | tagKeepEmpty | tagIsBlock,
	atom.Html:       tagKeep,
	atom.I:          tagKeep,
	atom.Iframe:     tagRemove,
	atom.Image:      tagKeep,
	atom.Img:        tagKeep | tagKeepEmpty,
	atom.Input:      tagRemove | tagKeepEmpty,
	atom.Ins:        tagKeep,
	atom.Kbd:        tagKeep,
	atom.Keygen:     tagKeepEmpty,
	atom.Label:      tagKeep,
	atom.Legend:     tagKeep,
	atom.Li:         tagKeep | tagIsBlock,
	atom.Link:       tagRemove | tagKeepEmpty,
	atom.Main:       tagKeep | tagIsBlock,
	atom.Map:        tagKeep,
	atom.Mark:       tagKeep,
	atom.Marquee:    tagKeep,
	atom.Menu:       tagKeep,
	atom.Menuitem:   tagKeep | tagKeepEmpty,
	atom.Meta:       tagRemove | tagKeepEmpty,
	atom.Meter:      tagKeep,
	atom.Nav:        tagKeep | tagIsBlock,
	atom.Nobr:       tagKeep,
	atom.Noembed:    tagRename,
	atom.Noframes:   tagRename,
	atom.Noscript:   tagRename | tagIsBlock,
	atom.Object:     tagRemove,
	atom.Ol:         tagKeep | tagIsBlock,
	atom.Optgroup:   tagKeep,
	atom.Option:     tagKeep,
	atom.Output:     tagKeep,
	atom.P:          tagKeep | tagIsBlock,
	atom.Param:      tagRemove | tagKeepEmpty,
	atom.Picture:    tagKeep,
	atom.Plaintext:  tagKeep,
	atom.Pre:        tagKeep | tagIsBlock,
	atom.Progress:   tagKeep,
	atom.Q:          tagKeep,
	atom.Rb:         tagKeep,
	atom.Rp:         tagKeep,
	atom.Rt:         tagKeep,
	atom.Rtc:        tagKeep,
	atom.Ruby:       tagKeep,
	atom.S:          tagKeep,
	atom.Samp:       tagKeep,
	atom.Script:     tagRemove,
	atom.Search:     tagKeep,
	atom.Section:    tagKeep | tagIsBlock,
	atom.Select:     tagRemove,
	atom.Slot:       tagRemove,
	atom.Small:      tagKeep,
	atom.Source:     tagRemove | tagKeepEmpty,
	atom.Span:       tagKeep,
	atom.Strike:     tagKeep,
	atom.Strong:     tagKeep,
	atom.Style:      tagRemove,
	atom.Sub:        tagKeep,
	atom.Summary:    tagKeep,
	atom.Sup:        tagKeep,
	atom.Table:      tagKeep | tagIsBlock,
	atom.Tbody:      tagKeep,
	atom.Td:         tagKeep | tagKeepEmpty,
	atom.Template:   tagRemove,
	atom.Textarea:   tagRemove,
	atom.Tfoot:      tagKeep | tagIsBlock,
	atom.Th:         tagKeep | tagKeepEmpty,
	atom.Thead:      tagKeep,
	atom.Time:       tagKeep,
	atom.Title:      tagRemove,
	atom.Tr:         tagKeep,
	atom.Track:      tagRemove | tagKeepEmpty,
	atom.Tt:         tagKeep,
	atom.U:          tagKeep,
	atom.Ul:         tagKeep | tagIsBlock,
	atom.Var:        tagKeep,
	atom.Video:      tagRemove | tagIsBlock,
	atom.Wbr:        tagKeep | tagKeepEmpty,
	atom.Xmp:        tagKeep,

	// MathML Core elements
	// curl -fsSL https://www.w3.org/Math/RelaxNG/mathml4/mathml4-core.rnc | grep -oE 'element [a-z-]+' | cut -d' ' -f2 | grep -v '^none$' | sort
	atom.Annotation:    tagKeep,
	atom.AnnotationXml: tagKeep,
	atom.Math:          tagKeep,
	atom.Mi:            tagKeep,
	atom.Mn:            tagKeep,
	atom.Mo:            tagKeep,
	atom.Ms:            tagKeep,
	atom.Mtext:         tagKeep,
}

// Tag names for which the [atom] package does not have any constants.
var elementMap2 = map[string]tagRule{
	"portal": tagRemove,

	// MathML Core
	"maction":       tagKeep,
	"merror":        tagKeep,
	"mfrac":         tagKeep,
	"mmultiscripts": tagKeep,
	"mover":         tagKeep,
	"mpadded":       tagKeep,
	"mphantom":      tagKeep,
	"mprescripts":   tagKeep | tagKeepEmpty,
	"mroot":         tagKeep,
	"mrow":          tagKeep | tagKeepEmpty,
	"mspace":        tagKeep | tagKeepEmpty,
	"msqrt":         tagKeep,
	"mstyle":        tagKeep,
	"msub":          tagKeep,
	"msubsup":       tagKeep,
	"msup":          tagKeep,
	"mtable":        tagKeep,
	"mtd":           tagKeep | tagKeepEmpty,
	"mtr":           tagKeep,
	"munder":        tagKeep,
	"munderover":    tagKeep,
	"semantics":     tagKeep,
}

var excludedChars = [][2]int{
	// CO block, except 0x09 (tab), 0x0A (LF), 0x0D (CR)
	{0x00, 0x08},
	{0x0B, 0x0C},
	{0x0E, 0x1F},

	// C1 block, except 0x85 (next line)
	{0x7F, 0x84},
	{0x86, 0x9F},

	// Surrogates
	{0xFDD0, 0xFDDF},
	{0xFFFE, 0xFFFF},

	{0x1FFFE, 0x1FFFF},
	{0x2FFFE, 0x2FFFF},
	{0x3FFFE, 0x3FFFF},
	{0x4FFFE, 0x4FFFF},
	{0x5FFFE, 0x5FFFF},
	{0x6FFFE, 0x6FFFF},
	{0x7FFFE, 0x7FFFF},
	{0x8FFFE, 0x8FFFF},
	{0x9FFFE, 0x9FFFF},
	{0xAFFFE, 0xAFFFF},
	{0xBFFFE, 0xBFFFF},
	{0xCFFFE, 0xCFFFF},
	{0xDFFFE, 0xDFFFF},
	{0xEFFFE, 0xEFFFF},
	{0xFFFFE, 0xFFFFF},
	{0x10FFFE, 0x10FFFF},
}

// ctrlReplacer is a string replacer for all invalid XML characters.
var ctrlReplacer = func() *strings.Replacer {
	repl := []string{}
	for _, t := range excludedChars {
		for i := t[0]; i <= t[1]; i++ {
			repl = append(repl, string(rune(i)), "")
		}
	}

	return strings.NewReplacer(repl...)
}()
