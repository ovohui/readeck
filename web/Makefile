#!/usr/bin/make

# SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

# Build the bundles and copy all assets
.PHONY: build
build: export NODE_ENV=production
build:
	npm run build

# Clean built files
.PHONY: clean
clean:
	npm run clean

# Run linter
.PHONY: lint
lint: export NODE_ENV=production
lint:
	npm run check
	npm run lint

# Start watching files for changes
.PHONY: watch
watch:
	npm run watch

# Setup the development env
.PHONY: setup
setup:
	npm ci --silent
