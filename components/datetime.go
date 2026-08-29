// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package components

import (
	"context"
	"time"

	"codeberg.org/readeck/readeck/pkg/strftime"
)

// These are the date formats used on Readeck's frontend.
// In English, they translate like this:
//
// dateFmt         : 28 Aug 2026
// dateTimeFmt     : 28 Aug 2026, 15:37
// dateTimeFullFmt : 28 Aug 2026, 15:37:23 UTC
//
// Translations should match their respective formats and can't start with an article.
var (
	dateFmt         = strftime.Pgettext("datetime", "%e %b %Y")
	dateTimeFmt     = strftime.Pgettext("datetime", "%e %b %Y, %H:%M")
	dateTimeFullFmt = strftime.Pgettext("datetime", "%e %b %Y, %H:%M:%S %Z")
)

// Strftime calls [strftime.Formatter.Strftime] with the context's locale.
func Strftime(ctx context.Context, t time.Time, f string) string {
	return strftime.New(L(ctx)).Strftime(f, t)
}

// formatTime calls [strftime.Formatter.Format] with the context's locale.
func formatTime(ctx context.Context, t time.Time, tr strftime.Translatable) string {
	l := L(ctx)
	return strftime.New(l).Strftime(tr.Translate(l), t)
}

// Date returns the localized and formatted date only without tz change.
func Date(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t, dateFmt)
}

// DateTime returns the localized and formatted date and time without tz change.
func DateTime(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t, dateTimeFmt)
}

// DateTimeFull returns the localized and formatted date and time without tz change.
func DateTimeFull(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t, dateTimeFullFmt)
}

// LocalDate returns the localized and formatted local date only.
func LocalDate(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t.Local(), dateFmt)
}

// LocalDateTime returns the localized and formatted local date and time.
func LocalDateTime(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t.Local(), dateTimeFmt)
}

// LocalDateTimeFull returns the localized and formatted local date and time with tz info.
func LocalDateTimeFull(ctx context.Context, t time.Time) string {
	return formatTime(ctx, t.Local(), dateTimeFullFmt)
}
