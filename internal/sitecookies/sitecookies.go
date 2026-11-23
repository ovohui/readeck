// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package sitecookies stores per-site cookies and HTTP headers used by the
// extractor. It lets Readeck fetch pages that require a logged-in session
// (Xiaohongshu, Zhihu, ...) and can import cookies from CookieCloud.
//
// The store is a JSON file in the data directory, guarded by a mutex. This
// keeps the feature self-contained: no database migration is required and
// cookies survive application updates.
package sitecookies

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" //nolint:gosec // required by the CookieCloud protocol
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/base58"
)

// SiteCookie is a set of cookies and headers attached to a domain.
// Domain matches the host and all its subdomains (a leading dot is optional).
type SiteCookie struct {
	UID     string            `json:"uid"`
	Domain  string            `json:"domain"`
	Name    string            `json:"name"`
	Cookies string            `json:"cookies"`
	Headers map[string]string `json:"headers,omitempty"`
	Source  string            `json:"source,omitempty"`
	Created time.Time         `json:"created"`
	Updated time.Time         `json:"updated"`
}

// CookieCloudConfig holds the connection settings for a CookieCloud server.
type CookieCloudConfig struct {
	Server   string `json:"server"`
	UUID     string `json:"uuid"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
}

// store is the on-disk representation of the cookie database.
type store struct {
	Cookies     []*SiteCookie    `json:"cookies"`
	CookieCloud CookieCloudConfig `json:"cookiecloud"`
}

var (
	mu      sync.RWMutex
	loaded  bool
	data    = &store{CookieCloud: CookieCloudConfig{Server: "https://www.cookiecloud.com"}}
	dataFile string
)

func fileName() string {
	if dataFile != "" {
		return dataFile
	}
	return filepath.Join(configs.Config.Main.DataDirectory, "site-cookies.json")
}

// SetFileName overrides the store location (used by tests).
func SetFileName(name string) {
	mu.Lock()
	defer mu.Unlock()
	dataFile = name
	loaded = false
}

// load reads the store from disk once.
func load() error {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return nil
	}
	fd, err := os.Open(fileName())
	if err != nil {
		if os.IsNotExist(err) {
			loaded = true
			return nil
		}
		return err
	}
	defer fd.Close()
	if err := json.NewDecoder(fd).Decode(data); err != nil {
		return err
	}
	if data.CookieCloud.Server == "" {
		data.CookieCloud.Server = "https://www.cookiecloud.com"
	}
	loaded = true
	return nil
}

// save writes the current store to disk. The caller must hold the write lock.
func save() error {
	path := fileName()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer fd.Close()
	return json.NewEncoder(fd).Encode(data)
}

// All returns a copy of all stored site cookies.
func All() ([]*SiteCookie, error) {
	if err := load(); err != nil {
		return nil, err
	}
	mu.RLock()
	defer mu.RUnlock()
	res := make([]*SiteCookie, 0, len(data.Cookies))
	for _, c := range data.Cookies {
		clone := *c
		clone.Headers = make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			clone.Headers[k] = v
		}
		res = append(res, &clone)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Domain == res[j].Domain {
			return res[i].Name < res[j].Name
		}
		return res[i].Domain < res[j].Domain
	})
	return res, nil
}

// Get returns a site cookie by uid.
func Get(uid string) (*SiteCookie, bool, error) {
	if err := load(); err != nil {
		return nil, false, err
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, c := range data.Cookies {
		if c.UID == uid {
			clone := *c
			return &clone, true, nil
		}
	}
	return nil, false, nil
}

// SaveCookie creates or updates a site cookie.
func SaveCookie(sc *SiteCookie) error {
	if err := load(); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	now := time.Now().UTC()
	sc.Updated = now
	if sc.Headers == nil {
		sc.Headers = map[string]string{}
	}
	if sc.UID == "" {
		sc.UID = base58.NewUUID()
		sc.Created = now
		if sc.Source == "" {
			sc.Source = "manual"
		}
		data.Cookies = append(data.Cookies, sc)
	} else {
		for i, c := range data.Cookies {
			if c.UID == sc.UID {
				sc.Created = c.Created
				if sc.Source == "" {
					sc.Source = c.Source
				}
				data.Cookies[i] = sc
				break
			}
		}
	}
	return save()
}

// DeleteCookie removes a site cookie by uid.
func DeleteCookie(uid string) error {
	if err := load(); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for i, c := range data.Cookies {
		if c.UID == uid {
			data.Cookies = append(data.Cookies[:i], data.Cookies[i+1:]...)
			return save()
		}
	}
	return nil
}

// Matching returns the cookie header and extra headers that apply to the
// given URL. More specific domains take precedence over broader ones.
func Matching(u *url.URL) (cookieHeader string, headers map[string]string, err error) {
	items, err := All()
	if err != nil {
		return "", nil, err
	}

	host := strings.ToLower(u.Hostname())
	headers = map[string]string{}

	type match struct {
		sc     *SiteCookie
		length int
	}
	var matches []match
	for _, sc := range items {
		d := normalizeDomain(sc.Domain)
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			matches = append(matches, match{sc: sc, length: len(d)})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].length > matches[j].length
	})

	var parts []string
	for _, mt := range matches {
		if c := strings.TrimSpace(mt.sc.Cookies); c != "" {
			parts = append(parts, strings.TrimSuffix(c, ";"))
		}
		for k, v := range mt.sc.Headers {
			if _, exists := headers[k]; !exists {
				headers[k] = v
			}
		}
	}
	return strings.Join(parts, "; "), headers, nil
}

// normalizeDomain cleans a domain string, accepting full URLs as input.
func normalizeDomain(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	d = strings.TrimPrefix(d, ".")
	if strings.Contains(d, "://") {
		if u, err := url.Parse(d); err == nil {
			d = u.Hostname()
		}
	}
	if i := strings.IndexAny(d, "/:"); i >= 0 {
		d = d[:i]
	}
	return strings.Trim(d, ".")
}

// CookieCloudConfig returns a copy of the CookieCloud settings.
func GetCookieCloudConfig() (CookieCloudConfig, error) {
	if err := load(); err != nil {
		return CookieCloudConfig{}, err
	}
	mu.RLock()
	defer mu.RUnlock()
	return data.CookieCloud, nil
}

// UpdateCookieCloudConfig saves the CookieCloud settings.
func UpdateCookieCloudConfig(cfg CookieCloudConfig) error {
	if err := load(); err != nil {
		return err
	}
	if cfg.Server == "" {
		cfg.Server = "https://www.cookiecloud.com"
	}
	mu.Lock()
	defer mu.Unlock()
	data.CookieCloud = cfg
	return save()
}

// ---------------------------------------------------------------------------
// CookieCloud end-to-end encryption support
// ---------------------------------------------------------------------------

type cookieCloudResponse struct {
	Encrypted string `json:"encrypted"`
}

type cookieCloudEntry struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

type cookieCloudData struct {
	Cookies map[string][]cookieCloudEntry `json:"cookies"`
}

// cookieCloudKey derives the AES key from UUID and password, as specified by
// the CookieCloud end-to-end encryption protocol.
func cookieCloudKey(uuid, password string) []byte {
	sum := md5.Sum([]byte(uuid + "-" + password)) //nolint:gosec
	return sum[:]
}

// decryptCookieCloud decrypts the payload returned by the CookieCloud server.
func decryptCookieCloud(encrypted, uuid, password string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}
	if len(raw) < aes.BlockSize || (len(raw)-aes.BlockSize)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid encrypted payload length")
	}
	block, err := aes.NewCipher(cookieCloudKey(uuid, password))
	if err != nil {
		return nil, err
	}
	iv := raw[:aes.BlockSize]
	payload := append([]byte(nil), raw[aes.BlockSize:]...)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(payload, payload)
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty decrypted payload")
	}
	padding := int(payload[len(payload)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(payload) {
		return nil, fmt.Errorf("invalid padding")
	}
	return payload[:len(payload)-padding], nil
}

// SyncCookieCloud fetches cookies from the configured server, decrypts them
// and replaces all entries previously imported from CookieCloud.
func SyncCookieCloud() (int, error) {
	cfg, err := GetCookieCloudConfig()
	if err != nil {
		return 0, err
	}
	if cfg.UUID == "" || cfg.Password == "" {
		return 0, fmt.Errorf("CookieCloud UUID and password are required")
	}
	endpoint := fmt.Sprintf("%s/get/%s", strings.TrimRight(cfg.Server, "/"), url.PathEscape(cfg.UUID))

	client := &http.Client{Timeout: 30 * time.Second}
	rsp, err := client.Get(endpoint) //nolint:gosec // server URL is admin-provided
	if err != nil {
		return 0, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("CookieCloud server returned %s", rsp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(rsp.Body, 8<<20))
	if err != nil {
		return 0, err
	}
	var resp cookieCloudResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}
	plain, err := decryptCookieCloud(resp.Encrypted, cfg.UUID, cfg.Password)
	if err != nil {
		return 0, fmt.Errorf("decrypting CookieCloud payload: %w", err)
	}
	var payload cookieCloudData
	if err := json.Unmarshal(plain, &payload); err != nil {
		return 0, err
	}

	// CookieCloud cookie map may be keyed by domain with an array of entries.
	byDomain := map[string][]string{}
	for domain, entries := range payload.Cookies {
		d := normalizeDomain(domain)
		if d == "" {
			continue
		}
		for _, c := range entries {
			if c.Name == "" {
				continue
			}
			byDomain[d] = append(byDomain[d], c.Name+"="+c.Value)
		}
	}
	// Some servers return a flat list under "cookies".
	if len(byDomain) == 0 {
		var flat []cookieCloudEntry
		if err := json.Unmarshal(plain, &struct {
			Cookies *[]cookieCloudEntry `json:"cookies"`
		}{Cookies: &flat}); err == nil {
			for _, c := range flat {
				if c.Domain == "" || c.Name == "" {
					continue
				}
				d := normalizeDomain(c.Domain)
				byDomain[d] = append(byDomain[d], c.Name+"="+c.Value)
			}
		}
	}

	if err := load(); err != nil {
		return 0, err
	}
	mu.Lock()
	defer mu.Unlock()
	filtered := data.Cookies[:0]
	for _, c := range data.Cookies {
		if c.Source != "cookiecloud" {
			filtered = append(filtered, c)
		}
	}
	data.Cookies = filtered

	now := time.Now().UTC()
	count := 0
	for domain, values := range byDomain {
		sort.Strings(values)
		data.Cookies = append(data.Cookies, &SiteCookie{
			UID:     base58.NewUUID(),
			Domain:  domain,
			Name:    "cookiecloud",
			Cookies: strings.Join(values, "; "),
			Headers: map[string]string{},
			Source:  "cookiecloud",
			Created: now,
			Updated: now,
		})
		count++
	}
	if err := save(); err != nil {
		return 0, err
	}
	slog.Info("CookieCloud sync complete", slog.Int("domains", count))
	return count, nil
}
