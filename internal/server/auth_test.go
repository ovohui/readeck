// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/base58"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestTokenAuth(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	unknownToken, err := configs.Keys.TokenKey().Encode(base58.NewUUID())
	require.NoError(t, err)

	t1 := &tokens.Token{
		UserID:      &app.Users["user"].User.ID,
		Application: "test",
		IsEnabled:   true,
	}
	require.NoError(t, tokens.Tokens.Create(t1))
	t1Encoded, err := configs.Keys.TokenKey().Encode(t1.UID)
	require.NoError(t, err)

	exp := time.Now().UTC().Add(-time.Minute)
	t2 := &tokens.Token{
		UserID:      &app.Users["user"].User.ID,
		Application: "test",
		IsEnabled:   true,
		Expires:     &exp,
	}
	require.NoError(t, tokens.Tokens.Create(t2))
	t2Encoded, err := configs.Keys.TokenKey().Encode(t2.UID)
	require.NoError(t, err)

	client := app.Client()

	tests := []struct {
		name   string
		req    func(r *http.Request)
		assert func(assert *require.Assertions, rsp *Response)
	}{
		{
			name: "no token",
			req:  func(*http.Request) {},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "empty bearer",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer")
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "space bearer",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer   ")
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "bearer",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+t1Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "bearer unknown",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+unknownToken)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "bearer expired",
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+t2Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
		{
			name: "basic auth",
			req: func(r *http.Request) {
				r.SetBasicAuth("", t1Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "basic auth any user",
			req: func(r *http.Request) {
				r.SetBasicAuth("alice and bob", t1Encoded)
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(200, rsp.StatusCode)
			},
		},
		{
			name: "basic auth empty",
			req: func(r *http.Request) {
				r.SetBasicAuth("", "")
			},
			assert: func(assert *require.Assertions, rsp *Response) {
				assert.Equal(401, rsp.StatusCode)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := client.NewRequest("GET", "/api/profile", nil)
			require.NoError(t, err)
			test.req(req)

			rsp := client.Request(t, req)
			test.assert(require.New(t), rsp)
		})
	}
}

func TestTokenScopes(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	tUserAdminScope := &tokens.Token{
		UserID:      &app.Users["user"].User.ID,
		Application: "test",
		IsEnabled:   true,
		Roles:       types.Strings{"admin:read"},
	}
	require.NoError(t, tokens.Tokens.Create(tUserAdminScope))
	tokenUserAdminScope, err := configs.Keys.TokenKey().Encode(tUserAdminScope.UID)
	require.NoError(t, err)

	tAdminAdminScope := &tokens.Token{
		UserID:      &app.Users["admin"].User.ID,
		Application: "test",
		IsEnabled:   true,
		Roles:       types.Strings{"admin:read"},
	}
	require.NoError(t, tokens.Tokens.Create(tAdminAdminScope))
	tokenAdminAdminScope, err := configs.Keys.TokenKey().Encode(tAdminAdminScope.UID)
	require.NoError(t, err)

	client := app.Client()

	tests := []struct {
		name   string
		token  string
		method string
		path   string
		status int
	}{
		{
			name:   "user profile",
			token:  tokenUserAdminScope,
			method: "GET",
			path:   "/api/profile",
			status: 200,
		},
		{
			name:   "user with admin scope",
			token:  tokenUserAdminScope,
			method: "GET",
			path:   "/api/admin/users",
			status: 403,
		},
		{
			name:   "admin with admin scope",
			token:  tokenAdminAdminScope,
			method: "GET",
			path:   "/api/admin/users",
			status: 200,
		},
		{
			name:   "admin no write",
			token:  tokenAdminAdminScope,
			method: "POST",
			path:   "/api/admin/users",
			status: 403,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := client.NewRequest(test.method, test.path, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+test.token)
			rsp := client.Request(t, req)
			require.Equal(t, test.status, rsp.StatusCode)
		})
	}
}
