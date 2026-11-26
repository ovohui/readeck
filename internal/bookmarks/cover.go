// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"time"
)

// CoverFile is one picture of a custom cover. The map passed to [SetCover]
// uses the resource keys "image" and "thumbnail".
type CoverFile struct {
	Name string
	Type string
	Data []byte
	Size [2]int
}

// SetCover replaces the bookmark's cover. Passing an empty map removes it.
//
// The cover lives inside the bookmark archive, under img/, exactly like a
// cover found by the extractor. Keeping it there means the media route, the
// card, exports and backups need no special case, and deleting the bookmark
// deletes the picture with it.
func (b *Bookmark) SetCover(files map[string]CoverFile) error {
	p := b.GetFilePath()
	if p == "" {
		return os.ErrNotExist
	}
	if b.Files == nil {
		b.Files = BookmarkFiles{}
	}

	zr, err := zip.OpenReader(p)
	if err != nil {
		return err
	}

	oldNames := map[string]bool{}
	for _, key := range []string{"image", "thumbnail"} {
		if f, ok := b.Files[key]; ok && f != nil {
			oldNames[f.Name] = true
		}
	}
	newNames := map[string]bool{}
	for _, f := range files {
		newNames[f.Name] = true
	}

	tmpName, err := b.writeCoverArchive(zr, oldNames, newNames, files)
	// The original archive must be closed before it can be replaced (Windows
	// keeps the file locked while a handle is open).
	if cErr := zr.Close(); err == nil {
		err = cErr
	}
	if err != nil {
		if tmpName != "" {
			os.Remove(tmpName) //nolint:errcheck
		}
		return err
	}

	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName) //nolint:errcheck
		return err
	}

	// Only update the model once the new archive is in place.
	for _, key := range []string{"image", "thumbnail"} {
		delete(b.Files, key)
	}
	for key, f := range files {
		b.Files[key] = &BookmarkFile{
			Name: f.Name,
			Type: f.Type,
			Size: f.Size,
		}
	}

	return b.Update(map[string]any{"files": b.Files})
}

// RemoveCover deletes the bookmark's cover pictures.
func (b *Bookmark) RemoveCover() error {
	return b.SetCover(nil)
}

// writeCoverArchive copies the archive to a temporary file, dropping the old
// cover entries and writing the new ones.
func (b *Bookmark) writeCoverArchive(zr *zip.ReadCloser, oldNames, newNames map[string]bool, files map[string]CoverFile) (string, error) {
	p := b.GetFilePath()

	tmp, err := os.CreateTemp(filepath.Dir(p), "cover-*.zip")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()

	zw := zip.NewWriter(tmp)
	for _, entry := range zr.File {
		if oldNames[entry.Name] || newNames[entry.Name] {
			continue
		}

		// Copy the entry without recompressing it.
		rc, err := entry.OpenRaw()
		if err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
		hdr := entry.FileHeader
		w, err := zw.CreateRaw(&hdr)
		if err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
		if _, err = io.Copy(w, rc); err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
	}

	for _, f := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     f.Name,
			Method:   zip.Deflate,
			Modified: time.Now(),
		})
		if err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
		if _, err = w.Write(f.Data); err != nil {
			tmp.Close() //nolint:errcheck
			return tmpName, err
		}
	}

	if err := zw.Close(); err != nil {
		tmp.Close() //nolint:errcheck
		return tmpName, err
	}
	if err := tmp.Close(); err != nil {
		return tmpName, err
	}

	return tmpName, nil
}
