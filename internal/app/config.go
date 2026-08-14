// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"

	"codeberg.org/readeck/readeck/configs"
	"github.com/cristalhq/acmd"
)

func init() {
	commands = append(commands, acmd.Command{
		Name:        "config",
		Description: "Show Readeck configuration",
		ExecFunc:    runConfig,
	})
}

func runConfig(_ context.Context, args []string) error {
	var flags appFlags
	if err := flags.Flags().Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if err := loadConfig(&flags); err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(configs.Config)
}
