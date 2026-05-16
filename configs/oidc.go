// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"os"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
)

// OIDCProvider contains an OIDC provider configuration.
type OIDCProvider struct {
	Name                string      `json:"name"          env:"NAME"`
	URL                 string      `json:"url"           env:"URL"`
	ClientID            string      `json:"client_id"     env:"CLIENT_ID"`
	ClientSecret        string      `json:"client_secret" env:"CLIENT_SECRET"`
	Groups              [][2]string `json:"groups"        env:"GROUPS"`
	ProvisioningEnabled bool        `json:"provisioning"  env:"PROVISIONING"`
}

func (c *config) loadOIDCProviders() error {
	const prefix = "READECK_AUTH_OIDC_"
	funcMap := map[reflect.Type]env.ParserFunc{
		reflect.TypeFor[[][2]string](): func(v string) (any, error) {
			res := [][2]string{}
			for x := range strings.FieldsSeq(v) {
				if k, v, found := strings.Cut(x, "="); found {
					res = append(res, [2]string{k, v})
				}
			}
			return res, nil
		},
	}

	var ids []string
	var found bool

	for _, entry := range os.Environ() {
		// Looking for all the "READECK_AUTH_OIDC_{id}_URL" env vars.
		// They inform us about the existing keys.
		entry = strings.TrimPrefix(entry, "=")
		entry, found = strings.CutPrefix(entry, prefix)
		if !found {
			continue
		}
		entry, _, _ = strings.Cut(entry, "=")
		if entry, found = strings.CutSuffix(entry, "_URL"); found {
			ids = append(ids, entry)
		}
	}

	// Now, we can parse each provider. It's probably not super efficient
	// to parse the env vars so many times but that's a one time thing
	// at startup.
	if c.Auth.OIDC.Providers == nil {
		c.Auth.OIDC.Providers = map[string]OIDCProvider{}
	}

	for _, k := range ids {
		p := c.Auth.OIDC.Providers[k]
		err := env.ParseWithOptions(&p, env.Options{
			Prefix:                prefix + k + "_",
			UseFieldNameByDefault: false,
			FuncMap:               funcMap,
		})
		if err != nil {
			return err
		}
		c.Auth.OIDC.Providers[k] = p
	}
	return nil
}
