// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oidc_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/auth/oidc"
	"codeberg.org/readeck/readeck/internal/auth/users"
	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestProvisioning(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	providerInfo := oidc.Providers.Add(
		"test", "OIDC Test", "https://example.org", "client-id", "client-secret",
		[][2]string{
			{"roles:admin", "admin"},
			{"roles:user", "user"},
		},
		true,
	)
	defer oidc.Providers.Clear()

	user1, err := NewTestUser("bob", "bob@example.org", "abcd", "user")
	require.NoError(t, err)

	user2, err := NewTestUser("bob-oidc", "bob-oidc@example.org", "abcd", "user")
	require.NoError(t, err)
	user2.User.ExtID = new(uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://example.org#bob-oidc")).String())
	require.NoError(t, user2.User.Save())

	tests := []struct {
		name   string
		info   *oidc.UserInfo
		assert func(t *testing.T, user *users.User, err error)
	}{
		{
			name: "new user default group",
			info: &oidc.UserInfo{
				Subject:  "alice1",
				Username: "alice1",
				Email:    "alice1@example.org",
			},
			assert: func(t *testing.T, user *users.User, err error) {
				require.NoError(t, err)
				assert.Equal(t, "alice1", user.Username)
				assert.Equal(t, "alice1@example.org", user.Email)
				assert.Equal(t, "user", user.Group)
			},
		},
		{
			name: "new user with oidc group",
			info: &oidc.UserInfo{
				Subject:  "alice2",
				Username: "alice2",
				Email:    "alice2@example.org",
				Groups:   []string{"roles:user", "roles:admin"},
			},
			assert: func(t *testing.T, user *users.User, err error) {
				require.NoError(t, err)
				assert.Equal(t, "alice2", user.Username)
				assert.Equal(t, "alice2@example.org", user.Email)
				assert.Equal(t, "admin", user.Group)
			},
		},
		{
			name: "new user with custom claims",
			info: &oidc.UserInfo{
				Subject:      app.Users["user"].User.Username,
				Username:     app.Users["user"].User.Username,
				Email:        app.Users["user"].User.Email,
				ReadeckUser:  "new-user",
				ReadeckEmail: "user@example.net",
				ReadeckGroup: "admin",
			},
			assert: func(t *testing.T, user *users.User, err error) {
				require.NoError(t, err)
				assert.Equal(t, "new-user", user.Username)
				assert.Equal(t, "user@example.net", user.Email)
				assert.Equal(t, "admin", user.Group)
			},
		},
		{
			name: "provision existing user",
			info: &oidc.UserInfo{
				Subject:  "bob",
				Username: "bob",
				Email:    "bob-new-email@example.org",
			},
			assert: func(t *testing.T, user *users.User, err error) {
				require.NoError(t, err)
				assert.Equal(t, "bob", user.Username)
				assert.Equal(t, "bob-new-email@example.org", user.Email)
				assert.Equal(t, "user", user.Group)
				assert.Equal(t, user1.User.ID, user.ID)
			},
		},
		{
			name: "update existing user",
			info: &oidc.UserInfo{
				Subject:  "bob-oidc",
				Username: "new-bob",
				Email:    "bob@example.com",
			},
			assert: func(t *testing.T, user *users.User, err error) {
				require.NoError(t, err)
				assert.Equal(t, "new-bob", user.Username)
				assert.Equal(t, "bob@example.com", user.Email)
				assert.Equal(t, "user", user.Group)
				assert.Equal(t, user2.User.ID, user.ID)
			},
		},
		{
			name: "conflicting information",
			info: &oidc.UserInfo{
				Subject:  app.Users["admin"].User.Username,
				Username: app.Users["admin"].User.Username,
				Email:    app.Users["user"].User.Email,
			},
			assert: func(t *testing.T, _ *users.User, err error) {
				require.Error(t, err, "more than one user is")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.info.Provider = providerInfo
			test.info.Issuer = providerInfo.URL

			f := test.info.Form()
			require.True(t, f.IsValid())
			user, err := f.Execute()
			test.assert(t, user, err)
		})
	}
}
