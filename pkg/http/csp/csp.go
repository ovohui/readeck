// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package csp provides simple tools to create and modify a Content Security Policy.
package csp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"iter"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// CSP source values.
const (
	HeaderName = "Content-Security-Policy"

	None          = "'none'"
	Self          = "'self'"
	Data          = "data:"
	ReportSample  = "'report-sample'"
	StrictDynamic = "'strict-dynamic'"
	UnsafeEval    = "'unsafe-eval'"
	UnsafeHashes  = "'unsafe-hashes'"
	UnsafeInline  = "'unsafe-inline'"
)

// Policy is a map of CSP directives.
// It's the same data structure as http.Header, with a
// different serialization.
type Policy map[string][]string

// Add adds values to an existing directive, or creates it
// if it does not exist.
func (p Policy) Add(name string, values ...string) {
	p[name] = append(p[name], values...)
}

// Set creates or replaces a directive.
func (p Policy) Set(name string, values ...string) {
	p[name] = values
}

// Del removes a directive from the policy.
func (p Policy) Del(name string) {
	delete(p, name)
}

// Clone returns a copy of the policy.
func (p Policy) Clone() Policy {
	return maps.Clone(p)
}

func (p Policy) directives() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		keys := make([]string, 0, len(p))
		for k := range p {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if !yield(k, p[k]) {
				return
			}
		}
	}
}

// String returns the policy suitable for an http.Header value.
func (p Policy) String() string {
	w := &strings.Builder{}
	for k, v := range p.directives() {
		fmt.Fprintf(w, "%s %s; ", k, strings.Join(v, " "))
	}

	return strings.TrimRight(w.String(), "; ")
}

// Write sets the CSP header to an http.Header.
func (p Policy) Write(h http.Header) {
	h.Set(HeaderName, p.String())
}

// MakeNonce returns a random nonce value.
// It's an hex encoded 128-bit random value.
func MakeNonce() string {
	n := make([]byte, 16)
	rand.Read(n)
	return hex.EncodeToString(n)
}
