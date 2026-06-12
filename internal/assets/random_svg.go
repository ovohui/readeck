// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package assets

import (
	"encoding/hex"
	"hash"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/http/csp"
)

type random struct {
	*rand.Rand
	letter string
}

func newRandom(data [32]byte) *random {
	return &random{Rand: rand.New(rand.NewChaCha8(data))} //nolint:gosec
}

func (r *random) UpdateEtag(h hash.Hash) {
	io.WriteString(h, strconv.Itoa(r.Int())+r.letter)
}

func (r *random) GetLastModified() []time.Time {
	return []time.Time{}
}

// randomSvg sends an SVG image with a gradient. The gradient's color
// is based on the name.
func randomSvg(w http.ResponseWriter, r *http.Request) {
	seed := chi.URLParam(r, "seed")
	data := [32]byte{}
	_, err := hex.Decode(data[:], []byte(seed))
	if err != nil {
		server.Err(w, r, err)
		return
	}

	rd := newRandom(data)
	if n := r.URL.Query().Get("n"); len(n) > 0 {
		rd.letter = string([]rune(n)[0])
	}

	server.WriteEtag(w, r, rd)
	server.WriteLastModified(w, r, rd)
	csp.Policy{
		"base-uri":    {csp.None},
		"default-src": {csp.None},
	}.Write(w.Header())

	if server.HandleCaching(w, r) {
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	if err := rd.svg().Render(r.Context(), w); err != nil {
		server.Err(w, r, err)
		return
	}
}
