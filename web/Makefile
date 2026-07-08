#!/usr/bin/make

# SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

NPM ?= npm

# Build the bundles and copy all assets
.PHONY: build
build: export NODE_ENV=production
build:
	${NPM} run build

# Clean built files
.PHONY: clean
clean:
	${NPM} run clean

# Run linter
.PHONY: lint
lint: export NODE_ENV=production
lint:
	${NPM} run check
	${NPM} run lint

# Start watching files for changes
.PHONY: watch
watch:
	${NPM} run watch

# Setup the development env
.PHONY: setup
setup:
	${NPM} ci --silent
