// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package oidc provides the OIDC routes.
package oidc

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"slices"
	"sync"

	"golang.org/x/oauth2"

	"github.com/coreos/go-oidc/v3/oidc"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/server/urls"
	"codeberg.org/readeck/readeck/pkg/http/securecookie"
)

const (
	cookieName  = "oidc_tr"
	b32alphabet = "abcdefghijklmnopqrstuvwxyz234567"
)

var cookieHandler *securecookie.Handler

// initCookieHandler prepares the [securecookie.Handler]
// for the OIDC tracking cookie.
func initCookieHandler() {
	cookieHandler = securecookie.NewHandler(
		securecookie.Key(configs.Keys.OIDCKey()),
		securecookie.WithPath(path.Join(urls.Prefix())),
		securecookie.WithMaxAge(configs.Config.Server.Session.MaxAge),
		securecookie.WithName(cookieName),
	)
}

// cookie contains the tracking cookie's payload.
type cookie struct {
	ID           string `json:"p"`
	State        string `json:"s"`
	Nonce        string `json:"n"`
	PKCEVerifier string `json:"v"`
}

// userInfo contains the information gathered after the token exchange.
type userInfo struct {
	Email  string   `json:"email,omitempty"`
	Groups []string `json:"groups,omitempty"`
	Group  string   `json:"-"`
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
	configs.OIDCProvider

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
	items map[string]providerInfo
}

// Providers is the global [providerList] instance.
var Providers = providerList{
	items: map[string]providerInfo{},
}

func newProvider(ctx context.Context, cf *configs.OIDCProvider) (*providerInfo, error) {
	// XXX: handle configuration for providers without discovery route
	p, err := oidc.NewProvider(ctx, cf.URL)
	if err != nil {
		return nil, err
	}

	res := &providerInfo{OIDCProvider: *cf}
	if err = p.Claims(&res.Config); err != nil {
		return nil, err
	}

	if len(res.Config.GrantTypesSupported) > 0 && !slices.Contains(res.Config.GrantTypesSupported, "authorization_code") {
		return nil, newError(
			errors.New("provider doesn't support authorization_code grant"),
			withStatus(http.StatusNotImplemented),
			withAttrs(slog.Any("grant_types", res.Config.GrantTypesSupported)),
		)
	}

	if len(res.Config.ResponseTypesSupported) > 0 && !slices.Contains(res.Config.ResponseTypesSupported, "code") {
		return nil, newError(
			errors.New("provider doesn't support code response type"),
			withStatus(http.StatusNotImplemented),
			withAttrs(slog.Any("response_types", res.Config.ResponseTypesSupported)),
		)
	}

	res.OAuth = oauth2.Config{
		ClientID:     cf.ClientID,
		ClientSecret: cf.ClientSecret,
		RedirectURL:  urls.AbsoluteURLContext(ctx, "/login/oidc").String(),
		Endpoint:     p.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	return res, nil
}

// get returns a [providerInfo] from an ID.
// Providers are loaded from the configuration the first time
// we need them.
func (pl *providerList) get(ctx context.Context, id string) (*providerInfo, error) {
	pl.Lock()
	defer pl.Unlock()

	if cf, ok := pl.items[id]; ok {
		return &cf, nil
	}

	if cf, ok := configs.Config.Auth.OIDC.Providers[id]; ok {
		provider, err := newProvider(ctx, &cf)
		if err != nil {
			return nil, err
		}
		provider.Groups = cf.Groups
		pl.items[id] = *provider
		return provider, nil

	}

	return nil, errNoProvider
}

// AuthCodeURL returns the auth code URL needed to start the OAuth dance.
func (p *providerInfo) AuthCodeURL(state, nonce, pkceVerifier string) string {
	return p.OAuth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(pkceVerifier))
}

// Exchange exchanges a code for an [oauth2.Token].
// If the token contains an "id_token", it will check that it's
// properly signed and have a matching nonce.
func (p *providerInfo) Exchange(ctx context.Context, code, nonce, pkceVerifier string) (*oauth2.Token, *userInfo, error) {
	// Fetch token
	var options []oauth2.AuthCodeOption
	if pkceVerifier != "" {
		options = []oauth2.AuthCodeOption{oauth2.VerifierOption(pkceVerifier)}
	}

	token, err := p.OAuth.Exchange(ctx, code, options...)
	if err != nil {
		return nil, nil, err
	}

	// Verify id_token, if any
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return token, &userInfo{}, nil
	}

	verifier := p.Config.NewProvider(ctx).Verifier(&oidc.Config{
		ClientID:                   p.OAuth.ClientID,
		SupportedSigningAlgs:       p.Config.Algorithms,
		InsecureSkipSignatureCheck: true,
	})

	tokenID, err := verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, nil, err
	}

	if tokenID.Nonce != nonce {
		return nil, nil, errors.New("nonce doesn't match")
	}

	// Retrieve the claims, it's ok to fail here.
	userInfo := &userInfo{}
	_ = tokenID.Claims(userInfo)

	return token, userInfo, nil
}

// GetUser returns a [userInfo] from the user info endpoint.
func (p *providerInfo) GetUser(ctx context.Context, token *oauth2.Token) (*userInfo, error) {
	pr := p.Config.NewProvider(ctx)

	info, err := pr.UserInfo(ctx, p.OAuth.TokenSource(ctx, token))
	if err != nil {
		return nil, err
	}

	res := &userInfo{
		Email: info.Email,
	}
	if err = info.Claims(res); err != nil {
		return nil, err
	}

	return res, nil
}
