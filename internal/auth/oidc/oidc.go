// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package oidc provides the OIDC routes.
package oidc

import (
	"context"
	"crypto/rand"
	"errors"
	"iter"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sync"

	"golang.org/x/oauth2"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/http/securecookie"
)

const (
	cookieName  = "oidc_tr"
	b32alphabet = "abcdefghijklmnopqrstuvwxyz234567"
)

var cookieHandler *securecookie.Handler

// InitCookieHandler prepares the [securecookie.Handler]
// for the OIDC tracking cookie.
func InitCookieHandler(key []byte) {
	cookieHandler = securecookie.NewHandler(
		securecookie.Key(key),
		securecookie.WithPath(path.Join(urls.Prefix())),
		securecookie.WithMaxAge(3600),
		securecookie.WithName(cookieName),
	)
}

// cookie contains the tracking cookie's payload.
type cookie struct {
	ID           string `json:"p"`
	State        string `json:"s"`
	Nonce        string `json:"n"`
	PKCEVerifier string `json:"v"`
	Redir        string `json:"r"`
}

func randText(l int) string {
	src := make([]byte, l)
	rand.Read(src)
	for i := range src {
		src[i] = b32alphabet[src[i]%32]
	}
	return string(src)
}

// newCookie returns an existing or new [cookie] from
// the request's cookies.
func newCookie(r *http.Request) (*cookie, error) {
	c := &cookie{}

	err := cookieHandler.Load(r, c)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		return nil, err
	}

	return c, nil
}

// Save writes the sentinel cookie to the response.
func (c *cookie) Save(w http.ResponseWriter, r *http.Request) error {
	return cookieHandler.Save(w, r, c)
}

// Delete removes the sentinel cookie.
func (c *cookie) Delete(w http.ResponseWriter, r *http.Request) {
	cookieHandler.Delete(w, r)
}

// providerInfo is wrapper around [configs.OIDCProvider] that contains
// a resolved [oidc.ProviderConfig] and a [oauth2.Config].
type providerInfo struct {
	loaded bool

	ID                  string
	Name                string
	URL                 string
	ClientID            string
	ClientSecret        string
	Groups              [][2]string
	ProvisioningEnabled bool

	Config providerConfig
	OAuth  oauth2.Config
}

type providerConfig struct {
	oidc.ProviderConfig

	// Other claims properties
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported           []string `json:"grant_types_supported"`
	ResponseTypesSupported        []string `json:"response_types_supported"`
}

// providerList contains a list of OIDC providers.
type providerList struct {
	sync.Mutex
	items []providerInfo
}

// Providers is the global [providerList] instance.
var Providers = providerList{
	items: []providerInfo{},
}

// Items returns the provider list.
func (pl *providerList) Items() []providerInfo {
	return pl.items
}

func (pl *providerList) Hosts() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, p := range pl.items {
			u, _ := url.Parse(p.URL)
			if !yield(u.Scheme + "://" + u.Host) {
				return
			}
		}
	}
}

// Add adds a new, unloaded, provider to the list.
func (pl *providerList) Add(name, url, clientID, clientSecret string, groups [][2]string, prov bool) *providerInfo {
	pl.Lock()
	defer pl.Unlock()
	p := providerInfo{
		ID:                  uuid.NewSHA1(uuid.NameSpaceURL, []byte(url)).String(),
		Name:                name,
		URL:                 url,
		ClientID:            clientID,
		ClientSecret:        clientSecret,
		Groups:              groups,
		ProvisioningEnabled: prov,
	}
	pl.items = append(pl.items, p)
	return &p
}

// Clear removes all cached providers. Used by tests.
func (pl *providerList) Clear() {
	pl.Lock()
	defer pl.Unlock()
	pl.items = []providerInfo{}
}

// get returns a [providerInfo] from an ID.
// Providers are loaded from the configuration the first time
// we need them.
func (pl *providerList) get(ctx context.Context, id string) (*providerInfo, error) {
	pl.Lock()
	defer pl.Unlock()

	var pi *providerInfo
	for _, x := range pl.items {
		if x.ID == id {
			pi = &x
			break
		}
	}

	if pi == nil {
		return nil, errNoProvider
	}

	if err := pi.load(ctx); err != nil {
		return nil, err
	}

	return pi, nil
}

func (p *providerInfo) load(ctx context.Context) (err error) {
	var oidcProvider *oidc.Provider
	if !p.loaded {
		// XXX: handle configuration for providers without discovery route
		oidcProvider, err = oidc.NewProvider(ctx, p.URL)
		if err != nil {
			return err
		}

		if err = oidcProvider.Claims(&p.Config); err != nil {
			return err
		}
		p.loaded = true
	} else {
		oidcProvider = p.Config.NewProvider(ctx)
	}

	if len(p.Config.GrantTypesSupported) > 0 && !slices.Contains(p.Config.GrantTypesSupported, "authorization_code") {
		return newError(
			errors.New("provider doesn't support authorization_code grant"),
			withStatus(http.StatusNotImplemented),
			withAttrs(slog.Any("grant_types", p.Config.GrantTypesSupported)),
		)
	}

	if len(p.Config.ResponseTypesSupported) > 0 && !slices.Contains(p.Config.ResponseTypesSupported, "code") {
		return newError(
			errors.New("provider doesn't support code response type"),
			withStatus(http.StatusNotImplemented),
			withAttrs(slog.Any("response_types", p.Config.ResponseTypesSupported)),
		)
	}

	p.OAuth = oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		RedirectURL:  urls.AbsoluteURLContext(ctx, "/login/oidc").String(),
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	return nil
}

// AuthCodeURL returns the auth code URL needed to start the OAuth dance.
func (p *providerInfo) AuthCodeURL(state, nonce, pkceVerifier string) string {
	return p.OAuth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(pkceVerifier))
}

// Exchange exchanges a code for an [oauth2.Token].
// If the token contains an "id_token", it will check that it's
// properly signed and have a matching nonce.
func (p *providerInfo) Exchange(ctx context.Context, code, nonce, pkceVerifier string) (*oauth2.Token, *UserInfo, error) {
	// Fetch token
	var options []oauth2.AuthCodeOption
	if pkceVerifier != "" {
		options = []oauth2.AuthCodeOption{oauth2.VerifierOption(pkceVerifier)}
	}

	token, err := p.OAuth.Exchange(ctx, code, options...)
	if err != nil {
		return nil, nil, err
	}

	userInfo := &UserInfo{Provider: p, Issuer: p.URL}

	// Verify id_token, if any
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return token, userInfo, nil
	}

	verifier := p.Config.NewProvider(ctx).Verifier(&oidc.Config{
		ClientID:                   p.OAuth.ClientID,
		SupportedSigningAlgs:       p.Config.Algorithms,
		InsecureSkipSignatureCheck: false,
	})

	tokenID, err := verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, nil, err
	}

	if tokenID.Nonce != nonce {
		return nil, nil, errors.New("nonce doesn't match")
	}

	// Retrieve the claims, it's ok to fail here.
	_ = tokenID.Claims(userInfo)

	return token, userInfo, nil
}

// GetUser returns a [userInfo] from the user info endpoint.
func (p *providerInfo) GetUser(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	pr := p.Config.NewProvider(ctx)

	info, err := pr.UserInfo(ctx, p.OAuth.TokenSource(ctx, token))
	if err != nil {
		return nil, err
	}

	res := &UserInfo{
		Provider: p,
		Issuer:   p.URL,
		Email:    info.Email,
	}
	if err = info.Claims(res); err != nil {
		return nil, err
	}

	return res, nil
}
