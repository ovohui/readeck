// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package preferences

import (
	"context"

	"codeberg.org/readeck/readeck/pkg/forms"
)

type choices[T comparable] forms.ValueChoices[T]

func (c choices[T]) Classes() []string {
	res := make([]string, len(c))
	for i, x := range c {
		if x, ok := any(x.Value).(interface{ Class() string }); ok {
			res[i] = x.Class()
		}
	}

	return res
}

// WidthChoices contains the reader width possible choices.
var WidthChoices = choices[readerWidth]{
	forms.Choice("", WidthRegular),
	forms.Choice("", WidthWide),
	forms.Choice("", WidthFull),
}

// FontChoices contains the reader font family possible choices.
var FontChoices = choices[fontFamily]{
	forms.Choice("Lora", FontLora),
	forms.Choice("Public Sans", FontPublicSans),
	forms.Choice("Merriweather", FontMerriweather),
	forms.Choice("Inter", FontInter),
	forms.Choice("Plex Serif", FontPlexSerif),
	forms.Choice("Literata", FontLiterata),
	forms.Choice("Luciole", FontLuciole),
	forms.Choice("Atkinson Hyperlegible", FontAtkinsonHyperlegible),
	forms.Choice("Jetbrains Mono", FontJetbrainsMono),
}

// FontSizeChoices contains the reader font size possible choices.
var FontSizeChoices = choices[fontSize]{
	forms.Choice("", FontSize1),
	forms.Choice("", FontSize2),
	forms.Choice("", FontSize3),
	forms.Choice("", FontSize4),
	forms.Choice("", FontSize5),
	forms.Choice("", FontSize6),
}

// LineHeightChoices contains the reader line height possible choices.
var LineHeightChoices = choices[lineHeight]{
	forms.Choice("", LineHeight1),
	forms.Choice("", LineHeight2),
	forms.Choice("", LineHeight3),
	forms.Choice("", LineHeight4),
	forms.Choice("", LineHeight5),
	forms.Choice("", LineHeight6),
}

// JustifiedChoices contains the reader justified possible choices.
var JustifiedChoices = choices[justified]{
	forms.Choice("", justified(false)),
	forms.Choice("", justified(true)),
}

// HyphenatedChoices contains the reader hyphenated possible choices.
var HyphenatedChoices = choices[hyphenated]{
	forms.Choice("", hyphenated(false)),
	forms.Choice("", hyphenated(true)),
}

// Form is the client-bound preferences form.
type Form struct {
	forms.Form
	Bookmark     bookmarkPrefForm     `json:"bookmark"`
	BookmarkList bookmarkListPrefForm `json:"bookmark_list"`
}

type bookmarkPrefForm struct {
	ReaderWidth forms.NumberField[uint] `json:"reader_width" validate:"gte:1 lte:3"`
	FontFamily  forms.NumberField[uint] `json:"font_family"  validate:"gte:0 lte:8"`
	FontSize    forms.NumberField[int]  `json:"font_size"    validate:"gte:1 lte:6"`
	LineHeight  forms.NumberField[int]  `json:"line_height"  validate:"gte:1 lte:6"`
	Justified   forms.BooleanField      `json:"justified"`
	Hyphenated  forms.BooleanField      `json:"hyphenated"`
	HideSidebar forms.BooleanField      `json:"hide_sidebar"`
}

type bookmarkListPrefForm struct {
	Layout forms.NumberField[uint] `json:"layout" validate:"gte:0 lte:2"`
}

// NewForm returns a [Form] instance with values set by the given [Data] instance.
func NewForm(ctx context.Context, p *Data) *Form {
	f := forms.New[Form](ctx)

	f.Bookmark.ReaderWidth.Set(uint(p.Bookmark.ReaderWidth))
	f.Bookmark.FontFamily.Set(uint(p.Bookmark.FontFamily))
	f.Bookmark.FontSize.Set(int(p.Bookmark.FontSize))
	f.Bookmark.LineHeight.Set(int(p.Bookmark.LineHeight))
	f.Bookmark.Justified.Set(bool(p.Bookmark.Justified))
	f.Bookmark.Hyphenated.Set(bool(p.Bookmark.Hyphenated))
	f.Bookmark.HideSidebar.Set(p.Bookmark.HideSidebar)

	f.BookmarkList.Layout.Set(uint(p.BookmarkList.Layout))

	return f
}

// Update updates the values of the given [Data] instance.
func (f *Form) Update(p *Data) {
	if f.Bookmark.ReaderWidth.IsBound() {
		p.Bookmark.ReaderWidth = readerWidth(f.Bookmark.ReaderWidth.Value())
	}
	if f.Bookmark.FontFamily.IsBound() {
		p.Bookmark.FontFamily = fontFamily(f.Bookmark.FontFamily.Value())
	}
	if f.Bookmark.FontSize.IsBound() {
		p.Bookmark.FontSize = fontSize(f.Bookmark.FontSize.Value())
	}
	if f.Bookmark.LineHeight.IsBound() {
		p.Bookmark.LineHeight = lineHeight(f.Bookmark.LineHeight.Value())
	}
	if f.Bookmark.Justified.IsBound() {
		p.Bookmark.Justified = justified(f.Bookmark.Justified.Value())
	}
	if f.Bookmark.Hyphenated.IsBound() {
		p.Bookmark.Hyphenated = hyphenated(f.Bookmark.Hyphenated.Value())
	}

	if f.Bookmark.HideSidebar.IsBound() {
		p.Bookmark.HideSidebar = f.Bookmark.HideSidebar.Value()
	}

	if f.BookmarkList.Layout.IsBound() {
		p.BookmarkList.Layout = listLayout(f.BookmarkList.Layout.Value())
	}
}
