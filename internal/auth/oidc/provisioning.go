// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oidc

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// ProvisioningForm is the form used to provisioon a user.
type ProvisioningForm struct {
	forms.Form
	userInfo *UserInfo

	ID       forms.TextField `json:"id"       validate:"trim required"`
	Username forms.TextField `json:"username" validate:"trim is_valid_username"`
	Email    forms.TextField `json:"email"    validate:"trim is_valid_email"`
	Group    forms.TextField `json:"group"    validate:"required_or_nil group_choices"`
}

// UserInfo contains the information gathered after the token exchange.
type UserInfo struct {
	Provider     *providerInfo `json:"-"`
	Issuer       string        `json:"iss"`
	Subject      string        `json:"sub"`
	Username     string        `json:"preferred_username"`
	Email        string        `json:"email"`
	Groups       []string      `json:"groups"`
	ReadeckUser  string        `json:"readeck_user"`
	ReadeckEmail string        `json:"readeck_email"`
	ReadeckGroup string        `json:"readeck_group"`
}

func (ui *UserInfo) id() uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(ui.Issuer+"#"+ui.Subject))
}

func (ui *UserInfo) username() string {
	switch {
	case ui.ReadeckUser != "":
		return ui.ReadeckUser
	case ui.Username != "":
		return ui.Username
	default:
		return ui.Email
	}
}

func (ui *UserInfo) email() string {
	switch {
	case ui.ReadeckEmail != "":
		return ui.ReadeckEmail
	default:
		return ui.Email
	}
}

func (ui *UserInfo) group() string {
	if ui.ReadeckGroup != "" {
		return ui.ReadeckGroup
	}

	for _, g := range ui.Provider.Groups {
		if slices.Contains(ui.Groups, g[0]) {
			return g[1]
		}
	}
	return "user"
}

// Form returns a bound [ProvisioningForm] using the struct values.
func (ui *UserInfo) Form() *ProvisioningForm {
	// Get some corner cases out the way first
	username := ui.username()
	email := ui.email()

	switch {
	case strings.ContainsRune(username, '@'):
		email = username
	case username == "":
		username = email
	}

	q := url.Values{
		"id":       {ui.id().String()},
		"username": {username},
		"email":    {email},
		"group":    {ui.group()},
	}

	f := forms.New[ProvisioningForm](context.Background())
	f.userInfo = ui
	forms.BindValues(q, f)

	return f
}

func (f *ProvisioningForm) errors() error {
	if len(f.Errors()) > 0 {
		return f.Errors()
	}

	for name, field := range f.Fields() {
		if len(field.Errors()) > 0 {
			return forms.Errors{fmt.Errorf("%s: %s", name, field.Errors())}
		}
	}

	return nil
}

// Execute performs the user provisioning.
// It creates or updates a user depending on whether it exists already.
func (f *ProvisioningForm) Execute() (*users.User, error) {
	tx, err := db.Q().Begin()
	if err != nil {
		return nil, err
	}

	user := new(users.User)
	err = tx.Wrap(func() error {
		// Check if we have a user with the OIDC ID
		exists, err := tx.From(db.TableUser).
			Where(goqu.C("ext_id").Eq(f.ID.String())).
			Prepared(true).
			ScanStruct(user)
		if err != nil {
			return err
		}

		// No user, check if we have users with the username or email
		if !exists {
			userList := []*users.User{}
			err = tx.From(db.TableUser).
				Where(
					goqu.Or(
						goqu.C("username").Eq(f.Username.Value()),
						goqu.C("email").Eq(f.Email.Value()),
					),
				).
				Prepared(true).
				ScanStructs(&userList)
			if err != nil {
				return err
			}

			switch {
			case len(userList) == 1:
				user = userList[0]
			case len(userList) == 0:
				user = new(users.User)
			default:
				return fmt.Errorf("more than one user is associated with %s and %s",
					f.Username.Value(), f.Email.Value(),
				)
			}
		}

		now := time.Now().UTC()
		if user.IsAnonymous() {
			// Create user
			if !f.userInfo.Provider.ProvisioningEnabled {
				return errNoProvisioning
			}

			user.Username = f.Username.Value()
			user.Email = f.Email.Value()
			user.Group = f.Group.Value()
			user.SetSeed()
			user.SetPassword(users.MakePassword(64)) //nolint:errcheck
			user.Created = now
			user.Updated = now
			user.LastLogin = now
			user.UID = base58.NewUUID()

			user.ExtID = new(f.ID.Value())
			user.ExtInfo = &users.ExtInfo{
				AddedOn: user.Created,
				Issuer:  f.userInfo.Issuer,
			}

			ds := tx.Insert(db.TableUser).Rows(user).Prepared(true)
			id, err := db.InsertWithID(ds, "id")
			if err != nil {
				return err
			}
			user.ID = id
		} else {
			// Update with new information
			user.Updated = now
			user.LastLogin = now
			user.Username = f.Username.Value()
			user.Email = f.Email.Value()
			user.Group = f.Group.Value()
			user.ExtID = new(f.ID.Value())

			if user.ExtInfo == nil {
				user.ExtInfo = &users.ExtInfo{
					AddedOn: now,
				}
			}
			user.ExtInfo.Issuer = f.userInfo.Issuer

			_, err = tx.Update(db.TableUser).
				Set(user).Where(goqu.C("id").Eq(user.ID)).
				Prepared(true).
				Executor().Exec()
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}
