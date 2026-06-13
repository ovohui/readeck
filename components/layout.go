// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package components

import "github.com/a-h/templ"

// Layout is our base layout for every Readeck page.
type Layout struct {
	head           templ.Component
	sideMenu       templ.Component
	accessMenu     [][2]string
	bodyClasses    templ.CSSClasses
	contentClasses templ.CSSClasses
}

// LayoutOption is an option function for [Layout].
type LayoutOption func(*Layout)

// NewLayout returns a new [Layout].
func NewLayout(options ...LayoutOption) (l *Layout) {
	l = &Layout{}
	for _, fn := range options {
		fn(l)
	}
	return l
}

// WithSideMenu sets the layout's side menu.
func WithSideMenu(c templ.Component) LayoutOption {
	return func(l *Layout) {
		l.sideMenu = c
	}
}

// WithAccessMenu sets the layout's quick access menu.
func WithAccessMenu(m [][2]string) LayoutOption {
	return func(l *Layout) {
		l.accessMenu = m
	}
}

// WithHead sets the component added at the end of the layout's head element.
func WithHead(c templ.Component) LayoutOption {
	return func(l *Layout) {
		l.head = c
	}
}

// WithContentClasses sets the extra CSS classes on the "#content" element.
func WithContentClasses(items ...any) LayoutOption {
	return func(l *Layout) {
		l.contentClasses = templ.CSSClasses(items)
	}
}

// WithBodyClasses sets the extra CSS classes on the body element.
func WithBodyClasses(items ...any) LayoutOption {
	return func(l *Layout) {
		l.bodyClasses = templ.CSSClasses(items)
	}
}

// Page renders a page with a side menu.
func Page(title string, options ...LayoutOption) templ.Component {
	return NewLayout(options...).Page(title)
}

// PageStd renders a page with an optional side menu.
// The content has a standard maximum width.
func PageStd(title string, options ...LayoutOption) templ.Component {
	l := NewLayout(options...)
	l.contentClasses = append(l.contentClasses, "max-w-std")
	return l.Page(title)
}

// AuthLayout is the layout used on all the authentication pages.
func AuthPage(title string) templ.Component {
	return NewLayout().AuthPage(title)
}

// AuthError is the component for the authentication error pages.
func AuthError(status int) templ.Component {
	return NewLayout().AuthError(status)
}
