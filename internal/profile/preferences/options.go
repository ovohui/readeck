// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package preferences

// Note: most options are unsigned integers starting at 1.

type (
	listLayout  uint8
	readerWidth uint8
	fontFamily  uint8
	fontSize    uint8
	lineHeight  uint8
	justified   bool
	hyphenated  bool
)

// List layout.
const (
	ListGrid listLayout = iota
	ListCompact
	ListMosaic
)

func (v listLayout) Class() string {
	switch v {
	case ListGrid:
		return "bookmark-list--grid"
	case ListCompact:
		return "bookmark-list--compact"
	case ListMosaic:
		return "bookmark-list--mosaic"
	}
	return ""
}

// Reader witdth.
const (
	WidthRegular readerWidth = 1 + iota
	WidthWide
	WidthFull
)

func (v readerWidth) Class() string {
	switch v {
	case WidthRegular:
		return "bookmark-display--width-regular"
	case WidthWide:
		return "bookmark-display--width-wide"
	case WidthFull:
		return "bookmark-display--width-full"

	}

	return ""
}

// Fonts.
const (
	FontLora fontFamily = iota
	FontPublicSans
	FontMerriweather
	FontInter
	FontPlexSerif
	FontLiterata
	FontLuciole
	FontAtkinsonHyperlegible
	FontJetbrainsMono
)

func (v fontFamily) Class() string {
	switch v {
	case FontLora:
		return "bookmark-display--font-lora"
	case FontPublicSans:
		return "bookmark-display--font-public-sans"
	case FontMerriweather:
		return "bookmark-display--font-merriweather"
	case FontInter:
		return "bookmark-display--font-inter"
	case FontPlexSerif:
		return "bookmark-display--font-plex-serif"
	case FontLiterata:
		return "bookmark-display--font-literata"
	case FontLuciole:
		return "bookmark-display--font-luciole"
	case FontAtkinsonHyperlegible:
		return "bookmark-display--font-atkinson-hyperlegible"
	case FontJetbrainsMono:
		return "bookmark-display--font-jetbrains-mono"
	}

	return ""
}

// Font Sizes.
const (
	FontSize1 fontSize = 1 + iota
	FontSize2
	FontSize3
	FontSize4
	FontSize5
	FontSize6
)

func (v fontSize) Class() string {
	switch v {
	case FontSize1:
		return "bookmark-display--size-1"
	case FontSize2:
		return "bookmark-display--size-2"
	case FontSize3:
		return "bookmark-display--size-3"
	case FontSize4:
		return "bookmark-display--size-4"
	case FontSize5:
		return "bookmark-display--size-5"
	case FontSize6:
		return "bookmark-display--size-6"
	}

	return ""
}

// Line Heights.
const (
	LineHeight1 lineHeight = 1 + iota
	LineHeight2
	LineHeight3
	LineHeight4
	LineHeight5
	LineHeight6
)

func (v lineHeight) Class() string {
	switch v {
	case LineHeight1:
		return "bookmark-display--leading-1"
	case LineHeight2:
		return "bookmark-display--leading-2"
	case LineHeight3:
		return "bookmark-display--leading-3"
	case LineHeight4:
		return "bookmark-display--leading-4"
	case LineHeight5:
		return "bookmark-display--leading-5"
	case LineHeight6:
		return "bookmark-display--leading-6"
	}

	return ""
}

// Justified.
func (v justified) Class() string {
	if v {
		return "bookmark-display--align-justify"
	}
	return "bookmark-display--align-none"
}

// Hyphenation.
func (v hyphenated) Class() string {
	if v {
		return "bookmark-display--hyphen-auto"
	}
	return "bookmark-display--hyphen-none"
}
