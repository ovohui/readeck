// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard_test

import (
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestPermissions(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	users := []string{"admin", "staff", "user", "disabled", ""}
	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			app.Client(WithSession(user)).Sequence(t,
				RT(
					WithTarget("/dashboard"),
					WithAssert(func(t *testing.T, r *Response) {
						switch user {
						case "admin", "staff", "user":
							r.AssertStatus(t, 200)
						case "disabled":
							r.AssertStatus(t, 403)
						case "":
							r.AssertStatus(t, 303)
						}
					}),
				),
			)
		})
	}
}
