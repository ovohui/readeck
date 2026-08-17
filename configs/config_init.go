// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"text/template"
)

const initialConfiguration = `
[main]
log_level = "{{ .Main.LogLevel }}"
secret_key = "{{ .Main.SecretKey }}"
data_directory = "{{ .Main.DataDirectory }}"

[server]
host = "{{ .Server.Host }}"
port = {{ .Server.Port }}

[database]
source = "{{ .Database.Source }}"
`

// CreateFile writes an initial configuration to a file.
func CreateFile(filename string) error {
	if Config.Main.SecretKey != "" {
		return errors.New("secret key is not empty")
	}

	tmpl, err := template.New("cfg").Parse(initialConfiguration)
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	Config.Main.SecretKey = generateKey()
	loadKeys() // we must reload the IKM with the new key

	if err = tmpl.Execute(fd, Config); err != nil {
		defer fd.Close() //nolint:errcheck
		return err
	}

	return fd.Close()
}

// generateKey returns a random key.
func generateKey() string {
	// 48 bytes = 384 bits
	d := make([]byte, 48)
	rand.Read(d)

	// 384/6 = 64 base64 characters
	return base64.StdEncoding.EncodeToString(d)
}
