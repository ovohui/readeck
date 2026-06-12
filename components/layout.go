// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package components

import "github.com/a-h/templ"

// Page renders a page with a side menu.
func Page(title string, sidemenu templ.Component) templ.Component {
	return defaultLayout.Page(title, sidemenu)
}

// PageStd renders a page with an optional side menu.
// The content has a standard maximum width.
func PageStd(title string, sidemenu templ.Component) templ.Component {
	return defaultLayout.PageStd(title, sidemenu)
}

// Layout is our base layout for every Readeck page.
type Layout struct {
	Head       templ.Component
	AccessMenu [][2]string
}

// defaultLayout is an instance of [Layout] that we reuse
// about everywhere.
var defaultLayout = NewLayout()

// NewLayout returns a new [Layout].
func NewLayout() (l *Layout) {
	return &Layout{
		Head: templ.NopComponent,
	}
}

// AuthLayout is the layout used on all the authentication pages.
func AuthLayout(title string) templ.Component {
	return defaultLayout.AuthBase(title)
}

// AuthError is the component for the authentication error pages.
func AuthError(status int) templ.Component {
	return defaultLayout.AuthError(status)
}
