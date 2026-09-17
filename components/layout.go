// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package components

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/a-h/templ/runtime"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/http/csp"
)

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

// AuthPage is the layout used on all the authentication pages.
func AuthPage(title string) templ.Component {
	return NewLayout().AuthPage(title)
}

// AuthError is the component for the authentication error pages.
func AuthError(status int) templ.Component {
	return NewLayout().AuthError(status)
}

// CSPMeta retrieves the CSP directives from the HTTP Header in the component
// [io.Writer] and, when it found something, injects the string in a meta tag.
var CSPMeta = templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
	var policy string
	if w, ok := w.(*runtime.Buffer); ok {
		if w, ok := w.Underlying.(http.ResponseWriter); ok {
			policy = w.Header().Get(csp.HeaderName)
		}
	}

	if len(policy) == 0 {
		return nil
	}

	p := new(strings.Builder)
	for x := range strings.SplitSeq(policy, ";") {
		switch n := strings.SplitN(strings.TrimSpace(x), " ", 2); n[0] {
		case "report-uri", "frame-ancestors", "sandbox":
		default:
			p.WriteString(x)
			p.WriteRune(';')
		}
	}

	_, err := fmt.Fprintf(
		w, `<meta http-equiv="%s" content="%s">`,
		csp.HeaderName, html.EscapeString(p.String()),
	)
	return err
})
