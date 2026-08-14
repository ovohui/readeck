// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package users

import (
	"strings"
	"unicode"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/glob"
)

// Error definitions.
var (
	ErrInvalidUsername  = forms.Gettext(`username is not valid`)
	ErrBlockedUsername  = forms.Gettext("username is not available")
	ErrBlockedEmailAddr = forms.ErrInvalidEmail
)

func init() {
	forms.RegisterTaggedValidator(func(name, _ string, tc *forms.TagContext) (forms.Validator, bool) {
		switch name {
		case "is_valid_password":
			return forms.TypedValidator(
				func(v string) bool {
					if strings.TrimSpace(v) == "" {
						return false
					}
					return len(v) >= 8
				},
				forms.Gettext("password must be at least 8 characters long"),
			), true
		case "is_valid_email":
			return forms.ValueValidatorFunc[string](func(f forms.Binder, v string) error {
				if err := forms.IsEmail.ValidateValue(f, v); err != nil {
					return err
				}

				for _, blocked := range configs.Config.Accounts.EmailDenyList {
					if glob.Glob(blocked, v) {
						return ErrBlockedEmailAddr
					}
				}

				return nil
			}), true
		case "is_valid_username":
			return forms.ValueValidatorFunc[string](func(f forms.Binder, v string) error {
				if f.IsNil() {
					return nil
				}

				if len(v) < 3 {
					return ErrInvalidUsername
				}

				for _, x := range v {
					if unicode.Is(unicode.C, x) || unicode.Is(unicode.Space, x) {
						return ErrInvalidUsername
					}
				}

				for _, blocked := range configs.Config.Accounts.UsernameDenyList {
					if glob.Glob(blocked, v) {
						return ErrBlockedUsername
					}
				}

				if !strings.ContainsRune(v, '@') {
					return nil
				}

				// Username contains "@". There must be an email field and both
				// values must be equal.
				email, ok := tc.Form.Fields()["email"]
				if !ok || email.V() != f.V() {
					return ErrInvalidUsername
				}

				return nil
			}), true
		case "group_choices":
			g := GroupList(forms.GetTranslator(tc.Context), "@group", nil)
			choices := make([]forms.ValueChoice[string], 0, len(g)+1)
			choices = append(choices, forms.Choice(
				forms.GetTranslator(tc.Context).Pgettext("role", "no group"),
				"none",
			))

			for _, x := range g {
				choices = append(choices, forms.Choice(x[1], x[0]))
			}
			forms.Choices(tc.Field, choices...)
			tc.Field.(*forms.TextField).Set("user")

			return nil, true
		default:
			return nil, false
		}
	})
}
