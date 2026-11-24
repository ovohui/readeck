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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
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
	Cookies     []*SiteCookie     `json:"cookies"`
	CookieCloud CookieCloudConfig `json:"cookiecloud"`
}

var (
	mu       sync.RWMutex
	loaded   bool
	data     = &store{CookieCloud: CookieCloudConfig{Server: "https://www.cookiecloud.com"}}
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

	// Sites commonly serve assets (especially images) from a separate CDN
	// domain. Map those asset hosts back to the site domain so that cookies,
	// Referer and headers configured for the site are also applied when the
	// archiver downloads its resources. Without this, the page saves fine but
	// all images fail (login/anti-hotlink check on the CDN).
	assetHosts := map[string]string{
		"xhscdn.com":                     "xiaohongshu.com",
		"xhsimg.com":                     "xiaohongshu.com",
		"picasso-static.xiaohongshu.com": "xiaohongshu.com",
		"zhimg.com":                      "zhihu.com",
		"picx.zhimg.com":                 "zhihu.com",
		"vdn.zhimg.com":                  "zhihu.com",
		"bilivideo.com":                  "bilibili.com",
		"hdslb.com":                      "bilibili.com",
	}
	effectiveHost := host
	for asset, site := range assetHosts {
		if host == asset || strings.HasSuffix(host, "."+asset) {
			effectiveHost = site
			break
		}
	}

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
		// Match against both the real host and the mapped site host. This lets
		// a xiaohongshu.com cookie apply to xhscdn.com image requests too.
		if effectiveHost == d || strings.HasSuffix(effectiveHost, "."+d) || host == d || strings.HasSuffix(host, "."+d) {
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

// HasCookieFor reports whether at least one cookie string is stored for host.
// Used to tell "you never saved a cookie" from "the cookie you saved was
// refused" when a site answers 401/403.
func HasCookieFor(host string) bool {
	cookie, _, err := Matching(&url.URL{Host: host})
	return err == nil && strings.TrimSpace(cookie) != ""
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

// cookieCloudEntry is the Playwright-style cookie object used by CookieCloud.
type cookieCloudEntry struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

// cookieCloudData covers both the current ({cookie_data:{...}}) and legacy
// ({cookies:{...}}) payload shapes returned by CookieCloud.
type cookieCloudData struct {
	CookieData map[string][]cookieCloudEntry `json:"cookie_data"`
	Cookies    map[string][]cookieCloudEntry `json:"cookies"`
}

// cookieCloudPassword implements the key derivation shared by both
// CookieCloud encryption modes: the first 16 hex characters of
// MD5(uuid + "-" + password). This string is used as the CryptoJS passphrase
// (legacy mode) or, re-encoded as bytes, as the AES-128 key (fixed-IV mode).
func cookieCloudPassword(uuid, password string) []byte {
	sum := md5.Sum([]byte(uuid + "-" + password)) //nolint:gosec // required by CookieCloud protocol
	return []byte(hex.EncodeToString(sum[:])[:16])
}

// bytesToKey implements OpenSSL's EVP_BytesToKey, which CryptoJS uses to turn
// a passphrase + salt into a key and IV. CookieCloud's legacy encryption uses
// MD5 as the digest with a 32-byte key and 16-byte IV.
func bytesToKey(salt, data []byte, h hash.Hash, keyLen, blockLen int) (key, iv []byte) {
	var concat, last []byte
	total := keyLen + blockLen
	for len(concat) < total {
		h.Reset()
		h.Write(last)
		h.Write(data)
		h.Write(salt)
		last = h.Sum(nil)
		concat = append(concat, last...)
	}
	return concat[:keyLen], concat[keyLen:total]
}

// pkcs7Strip validates and removes PKCS#7 padding.
func pkcs7Strip(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 || length%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded data length")
	}
	padLen := int(data[length-1])
	if padLen < 1 || padLen > blockSize || padLen > length {
		return nil, fmt.Errorf("invalid padding")
	}
	for _, b := range data[length-padLen:] {
		if int(b) != padLen {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return data[:length-padLen], nil
}

// decryptCookieCloud decrypts a CookieCloud payload. Two formats exist:
//
//   - Legacy (CryptoJS AES.encrypt with a passphrase): base64 of
//     "Salted__" + 8-byte salt + ciphertext. Key/IV are derived with
//     EVP_BytesToKey(MD5) into 32-byte AES key + 16-byte IV (AES-256-CBC).
//   - Fixed IV (CookieCloud >= 0.3.0): base64 of raw ciphertext, AES-128-CBC
//     with the 16-byte key = cookieCloudPassword() and an all-zero IV.
//
// Both use PKCS#7 padding. The format is auto-detected from the "Salted__"
// marker, so the same code handles old and new CookieCloud servers.
func decryptCookieCloud(encrypted, uuid, password string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	pass := cookieCloudPassword(uuid, password)
	var block cipher.Block
	var payload []byte
	var iv []byte

	if len(raw) >= 16 && string(raw[:8]) == "Salted__" {
		// Legacy CryptoJS format: salt is embedded in the payload.
		salt := raw[8:16]
		payload = append([]byte(nil), raw[16:]...)
		var key []byte
		key, iv = bytesToKey(salt, pass, md5.New(), 32, aes.BlockSize) //nolint:gosec
		block, err = aes.NewCipher(key)
	} else {
		// Fixed-IV format: all-zero IV, key is the 16-byte password bytes.
		payload = append([]byte(nil), raw...)
		block, err = aes.NewCipher(pass)
		iv = make([]byte, aes.BlockSize)
	}
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 || len(payload)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid encrypted payload length")
	}
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(payload, payload)
	return pkcs7Strip(payload, aes.BlockSize)
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

	// Current CookieCloud format: {cookie_data: {domain: [cookies]}}.
	// Legacy format: {cookies: {domain: [cookies]}}.
	byDomain := map[string][]string{}
	collect := func(groups map[string][]cookieCloudEntry) {
		for domain, entries := range groups {
			for _, c := range entries {
				if c.Name == "" {
					continue
				}
				// Prefer the cookie's own domain; fall back to the map key.
				d := normalizeDomain(c.Domain)
				if d == "" {
					d = normalizeDomain(domain)
				}
				if d == "" {
					continue
				}
				byDomain[d] = append(byDomain[d], c.Name+"="+c.Value)
			}
		}
	}
	collect(payload.CookieData)
	collect(payload.Cookies)

	// Some servers return a flat list under either key.
	if len(byDomain) == 0 {
		var alt struct {
			CookieData *[]cookieCloudEntry `json:"cookie_data"`
			Cookies    *[]cookieCloudEntry `json:"cookies"`
		}
		if err := json.Unmarshal(plain, &alt); err == nil {
			var entries []cookieCloudEntry
			if alt.CookieData != nil {
				entries = *alt.CookieData
			} else if alt.Cookies != nil {
				entries = *alt.Cookies
			}
			for _, c := range entries {
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
