// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/readeck/readeck/internal/ai"
	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/server"
)

type aiTestResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	KeyHint string `json:"key_hint,omitempty"`
}

func (v *profileViews) aiTest(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	key := strings.TrimSpace(r.FormValue("settings.ai_api_key"))
	if key == "" {
		key = user.Settings.AI.APIKey
	}
	cfg := ai.Config{
		Provider: r.FormValue("settings.ai_provider"),
		BaseURL:  r.FormValue("settings.ai_base_url"),
		APIKey:   key,
		Model:    r.FormValue("settings.ai_model"),
	}
	client := ai.New(cfg)
	if !client.Configured() {
		writeAITest(w, http.StatusBadRequest, aiTestResponse{Error: "AI is not configured."})
		return
	}

	if err := client.Test(r.Context()); err != nil {
		server.Log(r).Warn("ai test", "err", err, "key_len", len(key))
		writeAITest(w, http.StatusBadGateway, aiTestResponse{Error: err.Error(), KeyHint: keyHint(key)})
		return
	}
	writeAITest(w, http.StatusOK, aiTestResponse{OK: true})
}

func writeAITest(w http.ResponseWriter, status int, res aiTestResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(res)
}

func keyHint(key string) string {
	if len(key) <= 10 {
		return "len=" + strconv.Itoa(len(key))
	}
	return key[:6] + "…" + key[len(key)-4:] + " (len=" + strconv.Itoa(len(key)) + ")"
}
