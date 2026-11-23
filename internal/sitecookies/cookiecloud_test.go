// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package sitecookies

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" //nolint:gosec
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func passKey(uuid, password string) []byte {
	s := md5.Sum([]byte(uuid + "-" + password)) //nolint:gosec
	return []byte(hex.EncodeToString(s[:])[:16])
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	p := blockSize - len(data)%blockSize
	for i := 0; i < p; i++ {
		data = append(data, byte(p))
	}
	return data
}

// encryptLegacy builds a CryptoJS-style "Salted__"+salt+ciphertext payload.
func encryptLegacy(plain, uuid, password string, salt []byte) string {
	pass := passKey(uuid, password)
	key, iv := bytesToKey(salt, pass, md5.New(), 32, aes.BlockSize) //nolint:gosec
	block, _ := aes.NewCipher(key)
	ct := pkcs7Pad([]byte(plain), aes.BlockSize)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, ct)
	raw := append([]byte("Salted__"), salt...)
	raw = append(raw, ct...)
	return base64.StdEncoding.EncodeToString(raw)
}

// encryptFixed builds a fixed-IV (AES-128-CBC, zero IV) payload.
func encryptFixed(plain, uuid, password string) string {
	pass := passKey(uuid, password)
	block, _ := aes.NewCipher(pass)
	ct := pkcs7Pad([]byte(plain), aes.BlockSize)
	iv := make([]byte, aes.BlockSize)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, ct)
	return base64.StdEncoding.EncodeToString(ct)
}

func TestDecryptCookieCloudBothFormats(t *testing.T) {
	uuid := "test-uuid-1234"
	password := "test-password"
	plain := `{"cookie_data":{"www.xiaohongshu.com":[{"name":"a1","value":"abc","domain":".xiaohongshu.com"}]}}`
	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	cases := map[string]string{
		"legacy": encryptLegacy(plain, uuid, password, salt),
		"fixed":  encryptFixed(plain, uuid, password),
	}
	for name, enc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decryptCookieCloud(enc, uuid, password)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}
			if string(got) != plain {
				t.Fatalf("mismatch:\n got: %s\nwant: %s", got, plain)
			}
		})
	}
}

func TestDecryptCookieCloudWrongPassword(t *testing.T) {
	enc := encryptFixed("hello", "u", "p")
	if _, err := decryptCookieCloud(enc, "u", "wrong"); err == nil {
		t.Fatal("expected error with wrong password")
	}
}
