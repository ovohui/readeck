// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package oidc_test

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jose-util/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/auth/oidc"
	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

type oidcServer struct {
	ts       *httptest.Server
	router   *http.ServeMux
	url      string
	key      jose.JSONWebKey
	clientID string
	nonce    string
}

func newOIDCServer() *oidcServer {
	srv := &oidcServer{}

	_, privKey, err := generator.NewSigningKey(jose.RS256, 2048)
	if err != nil {
		panic(err)
	}

	srv.key = jose.JSONWebKey{
		Key:       privKey,
		KeyID:     "test",
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}

	srv.router = &http.ServeMux{}
	srv.router.HandleFunc("GET /.well-known/openid-configuration", srv.wellKnown)
	srv.router.HandleFunc("GET /login/oauth/keys", srv.keys)
	srv.router.HandleFunc("GET /login/oauth/authorize", srv.authorize)
	srv.router.HandleFunc("POST /login/oauth/access_token", srv.token)
	srv.router.HandleFunc("GET /login/oauth/userinfo", srv.userInfo)

	srv.ts = httptest.NewUnstartedServer(srv.router)

	return srv
}

func (s *oidcServer) start(clientID string) func() {
	s.ts.Start()
	s.url = s.ts.URL
	s.clientID = clientID
	s.nonce = ""

	oidc.Providers.Add("OIDC Test", s.url, s.clientID, "client-secret", [][2]string{}, true)
	return s.stop
}

func (s *oidcServer) stop() {
	s.ts.Close()
	oidc.Providers.Clear()
}

func (s *oidcServer) wellKnown(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	res := map[string]any{
		"issuer":                 s.url,
		"authorization_endpoint": s.url + "/login/oauth/authorize",
		"token_endpoint":         s.url + "/login/oauth/access_token",
		"jwks_uri":               s.url + "/login/oauth/keys",
		"userinfo_endpoint":      s.url + "/login/oauth/userinfo",
		"response_types_supported": []string{
			"code",
			"id_token",
		},
		"id_token_signing_alg_values_supported": []string{
			"RS256",
		},
		"subject_types_supported": []string{
			"public",
		},
		"scopes_supported": []string{
			"openid",
			"profile",
			"email",
			"groups",
		},
		"claims_supported": []string{
			"aud",
			"exp",
			"iat",
			"iss",
			"sub",
			"name",
			"preferred_username",
			"profile",
			"picture",
			"website",
			"locale",
			"updated_at",
			"email",
			"email_verified",
			"groups",
		},
		"code_challenge_methods_supported": []string{
			"plain",
			"S256",
		},
		"grant_types_supported": []string{
			"authorization_code",
			"refresh_token",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *oidcServer) keys(w http.ResponseWriter, _ *http.Request) {
	res := map[string][]jose.JSONWebKey{
		"keys": {
			s.key.Public(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *oidcServer) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	redir, _ := url.Parse(q.Get("redirect_uri"))
	switch s.clientID {
	case "authorize-error":
		redir.RawQuery = url.Values{
			"error":             {"invalid_client"},
			"error_description": {"client is not valid"},
		}.Encode()
	default:
		redir.RawQuery = url.Values{
			"code":  {"code-ok"},
			"state": q["state"],
			"iss":   {s.url},
		}.Encode()
	}

	s.nonce = q.Get("nonce")
	http.Redirect(w, r, redir.String(), http.StatusFound)
}

func (s *oidcServer) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.Form.Get("client_id") == "" {
		w.WriteHeader(400)
		io.WriteString(w, `{"error": "error"}`)
		return
	}

	sig := jose.SigningKey{Algorithm: jose.SignatureAlgorithm(s.key.Algorithm), Key: s.key.Key}
	signer, err := jose.NewSigner(sig, &jose.SignerOptions{})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	idToken := map[string]any{
		"iss":   s.url,
		"sub":   "user-1",
		"aud":   s.clientID,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
		"nonce": s.nonce,
	}

	switch s.clientID {
	case "id-token":
		maps.Copy(idToken, map[string]any{
			"preferred_username": "oidc-user",
			"email":              "oidc@example.org",
			"groups":             []string{},
		})
	case "custom-claims":
		maps.Copy(idToken, map[string]any{
			"preferred_username": "oidc-user",
			"email":              "oidc@example.org",
			"groups":             []string{},
			"readeck_email":      "readeck@example.org",
			"readeck_user":       "readeck-user",
			"readeck_group":      "admin",
		})
	case "user-update":
		maps.Copy(idToken, map[string]any{
			"preferred_username": "admin",
			"email":              "admin-updated@example.org",
			"groups":             []string{"user"},
		})
	case "token-error":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "expired_token",
		})
		return
	case "token-sig-error":
		_, privKey, _ := generator.NewSigningKey(jose.RS256, 2048)
		sig = jose.SigningKey{Algorithm: jose.RS256, Key: privKey}
		if signer, err = jose.NewSigner(sig, nil); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	data, _ := json.Marshal(idToken)
	payload, err := signer.Sign(data)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	k, err := payload.CompactSerialize()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	res := map[string]string{
		"access_token": "access-token",
		"token_type":   "Bearer",
		"id_token":     k,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *oidcServer) userInfo(w http.ResponseWriter, _ *http.Request) {
	res := map[string]any{
		"preferred_username": "oidc-user",
		"email":              "oidc.userinfo@example.org",
		"groups":             []string{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(res)
}

func TestSigninOk(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	tests := []struct {
		name             string
		clientID         string
		expectedEmail    string
		expectedUsername string
		expectedGroups   []any
	}{
		{
			clientID:         "id-token",
			expectedEmail:    "oidc@example.org",
			expectedUsername: "oidc-user",
			expectedGroups:   []any{"user"},
		},
		{
			clientID:         "user-info",
			expectedEmail:    "oidc.userinfo@example.org",
			expectedUsername: "oidc-user",
			expectedGroups:   []any{"user"},
		},
		{
			clientID:         "custom-claims",
			expectedEmail:    "readeck@example.org",
			expectedUsername: "readeck-user",
			expectedGroups:   []any{"admin"},
		},
		{
			clientID:         "user-update",
			expectedEmail:    "admin-updated@example.org",
			expectedUsername: "admin",
			expectedGroups:   []any{"user"},
		},
	}

	for _, test := range tests {
		t.Run(test.clientID, func(t *testing.T) {
			client := app.Client()

			srv := newOIDCServer()
			t.Cleanup(srv.start(test.clientID))

			client.RT(t,
				WithMethod(http.MethodPost),
				WithTarget("/login/oidc"),
				WithBody(url.Values{"id": {oidc.Providers.Items()[0].ID}}),
				AssertStatus(302),
				AssertRedirect("/login/oauth/authorize"),
			)

			var redir string
			t.Run("authorize", func(t *testing.T) {
				w := httptest.NewRecorder()
				r := httptest.NewRequest("GET", client.History[0].Response.Redirect, nil)
				srv.router.ServeHTTP(w, r)
				require.Equal(t, 302, w.Result().StatusCode)
				redir = w.Result().Header.Get("location")
				require.Regexp(t, "/login/oidc", redir)
			})

			client.RT(t,
				WithName("redir"),
				WithTarget(redir),
				AssertStatus(303),
			)

			client.RT(t,
				WithTarget("/api/profile"),
				WithAssert(func(t *testing.T, rsp *Response) {
					data := rsp.JSON.(map[string]any)
					user := data["user"].(map[string]any)

					assert := assert.New(t)
					assert.Equal(test.expectedEmail, user["email"])
					assert.Equal(test.expectedUsername, user["username"])
					assert.Equal(test.expectedGroups, data["provider"].(map[string]any)["roles"])
				}),
			)
		})
	}
}

func TestSigninKo(t *testing.T) {
	app := NewTestApp(t)
	defer app.Close(t)

	t.Run("unknown provider", func(t *testing.T) {
		client := app.Client()

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/oidc"),
			WithBody(url.Values{"id": {"abcd"}}),
			AssertStatus(404),
		)
	})

	t.Run("authorize error", func(t *testing.T) {
		client := app.Client()

		srv := newOIDCServer()
		t.Cleanup(srv.start("authorize-error"))

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/oidc"),
			WithBody(url.Values{"id": {oidc.Providers.Items()[0].ID}}),
			AssertStatus(302),
			AssertRedirect("/login/oauth/authorize"),
		)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", client.History[0].Response.Redirect, nil)
		srv.router.ServeHTTP(w, r)
		require.Equal(t, 302, w.Result().StatusCode)
		redir := w.Result().Header.Get("location")
		u, _ := url.Parse(redir)

		require.Equal(t, url.Values{
			"error":             {"invalid_client"},
			"error_description": {"client is not valid"},
		}, u.Query())
		t.Log(redir)

		client.RT(t,
			WithName("redir"),
			WithTarget(redir),
			AssertStatus(401),
		)
	})

	t.Run("token error", func(t *testing.T) {
		client := app.Client()

		srv := newOIDCServer()
		t.Cleanup(srv.start("token-error"))

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/oidc"),
			WithBody(url.Values{"id": {oidc.Providers.Items()[0].ID}}),
			AssertStatus(302),
			AssertRedirect("/login/oauth/authorize"),
		)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", client.History[0].Response.Redirect, nil)
		srv.router.ServeHTTP(w, r)
		require.Equal(t, 302, w.Result().StatusCode)
		redir := w.Result().Header.Get("location")

		client.RT(t,
			WithName("redir"),
			WithTarget(redir),
			AssertStatus(500),
		)
	})

	t.Run("token signature error", func(t *testing.T) {
		client := app.Client()

		srv := newOIDCServer()
		t.Cleanup(srv.start("token-sig-error"))

		client.RT(t,
			WithMethod(http.MethodPost),
			WithTarget("/login/oidc"),
			WithBody(url.Values{"id": {oidc.Providers.Items()[0].ID}}),
			AssertStatus(302),
			AssertRedirect("/login/oauth/authorize"),
		)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", client.History[0].Response.Redirect, nil)
		srv.router.ServeHTTP(w, r)
		require.Equal(t, 302, w.Result().StatusCode)
		redir := w.Result().Header.Get("location")

		client.RT(t,
			WithName("redir"),
			WithTarget(redir),
			AssertStatus(500),
		)
	})
}
