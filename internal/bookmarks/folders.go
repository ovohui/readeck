// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"errors"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"golang.org/x/crypto/bcrypt"

	"codeberg.org/readeck/readeck/internal/db"
)

// Folder is one node in the user's folder tree.
type Folder struct {
	ID           int       `db:"id"       goqu:"skipinsert"`
	UserID       int       `db:"user_id"`
	ParentID     *int      `db:"parent_id"`
	Name         string    `db:"name"`
	PasswordHash string    `db:"password_hash" json:"-"`
	IsLocked     bool      `db:"is_locked" json:"is_locked"`
	Created      time.Time `db:"created"`
	Updated      time.Time `db:"updated"`
}

// FolderNode is a folder with its children.
type FolderNode struct {
	*Folder
	Depth    int
	Children []*FolderNode
}

// FolderManager handles folder queries.
type FolderManager struct{}

// Folders is the folder manager.
var Folders = &FolderManager{}

// Query returns a base folder query.
func (m *FolderManager) Query() *goqu.SelectDataset {
	return db.Q().From(goqu.T(db.TableFolder).As("f")).Prepared(true)
}

// List returns all folders of a user ordered by name.
func (m *FolderManager) List(userID int) ([]*Folder, error) {
	res := []*Folder{}
	err := m.Query().Where(goqu.C("user_id").Eq(userID)).Order(goqu.C("name").Asc()).ScanStructs(&res)
	return res, err
}

// GetOne returns one folder owned by the user.
func (m *FolderManager) GetOne(userID, id int) (*Folder, error) {
	f := new(Folder)
	found, err := m.Query().Where(
		goqu.C("user_id").Eq(userID),
		goqu.C("id").Eq(id),
	).ScanStruct(f)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("folder not found")
	}
	return f, nil
}

// Create adds a folder, optionally under a parent.
func (m *FolderManager) Create(userID int, parentID *int, name string) (*Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 128 {
		return nil, errors.New("invalid folder name")
	}
	if parentID != nil {
		if _, err := m.GetOne(userID, *parentID); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	f := &Folder{
		UserID:   userID,
		ParentID: parentID,
		Name:     name,
		Created:  now,
		Updated:  now,
	}
	id, err := db.InsertWithID(db.Q().Insert(db.TableFolder).Rows(f).Prepared(true), "id")
	if err != nil {
		return nil, err
	}
	f.ID = id
	return f, nil
}

// Rename changes a folder's name.
func (m *FolderManager) Rename(userID, id int, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 128 {
		return errors.New("invalid folder name")
	}
	if _, err := m.GetOne(userID, id); err != nil {
		return err
	}
	_, err := db.Q().Update(db.TableFolder).Prepared(true).
		Set(goqu.Record{"name": name, "updated": time.Now().UTC()}).
		Where(goqu.C("user_id").Eq(userID), goqu.C("id").Eq(id)).
		Executor().Exec()
	return err
}

// Delete removes a folder and its descendants. Bookmarks are moved to the
// root, never deleted with the folder.
func (m *FolderManager) Delete(userID, id int) error {
	folders, err := m.List(userID)
	if err != nil {
		return err
	}
	children := map[int][]int{}
	for _, f := range folders {
		if f.ParentID != nil {
			children[*f.ParentID] = append(children[*f.ParentID], f.ID)
		}
	}

	ids := []int{}
	var walk func(int)
	walk = func(x int) {
		ids = append(ids, x)
		for _, c := range children[x] {
			walk(c)
		}
	}
	walk(id)
	if len(ids) == 0 {
		return errors.New("folder not found")
	}

	_, err = db.Q().Update(db.TableBookmark).Prepared(true).
		Set(goqu.Record{"folder_id": nil}).
		Where(goqu.C("user_id").Eq(userID), goqu.C("folder_id").In(ids)).
		Executor().Exec()
	if err != nil {
		return err
	}
	_, err = db.Q().Delete(db.TableFolder).Prepared(true).
		Where(goqu.C("user_id").Eq(userID), goqu.C("id").In(ids)).
		Executor().Exec()
	return err
}

// Tree returns the folder list as a tree.
func (m *FolderManager) Tree(userID int) ([]*FolderNode, error) {
	folders, err := m.List(userID)
	if err != nil {
		return nil, err
	}
	nodes := map[int]*FolderNode{}
	for _, f := range folders {
		nodes[f.ID] = &FolderNode{Folder: f}
	}
	var roots []*FolderNode
	for _, f := range folders {
		n := nodes[f.ID]
		if f.ParentID != nil {
			if p, ok := nodes[*f.ParentID]; ok {
				p.Children = append(p.Children, n)
				continue
			}
		}
		roots = append(roots, n)
	}
	var setDepth func(*FolderNode, int)
	setDepth = func(n *FolderNode, d int) {
		n.Depth = d
		for _, c := range n.Children {
			setDepth(c, d+1)
		}
	}
	for _, n := range roots {
		setDepth(n, 0)
	}
	return roots, nil
}

// SetLock stores a bcrypt password and locks the folder.
func (m *FolderManager) SetLock(userID, id int, password string) error {
	password = strings.TrimSpace(password)
	if len(password) < 4 {
		return errors.New("password too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Q().Update(db.TableFolder).Prepared(true).
		Set(goqu.Record{
			"password_hash": string(hash),
			"is_locked":     true,
			"updated":       time.Now().UTC(),
		}).
		Where(goqu.C("user_id").Eq(userID), goqu.C("id").Eq(id)).
		Executor().Exec()
	return err
}

// ClearLock removes the password and unlocks the folder.
func (m *FolderManager) ClearLock(userID, id int) error {
	_, err := db.Q().Update(db.TableFolder).Prepared(true).
		Set(goqu.Record{
			"password_hash": "",
			"is_locked":     false,
			"updated":       time.Now().UTC(),
		}).
		Where(goqu.C("user_id").Eq(userID), goqu.C("id").Eq(id)).
		Executor().Exec()
	return err
}

// CheckPassword verifies a folder password.
func (m *FolderManager) CheckPassword(userID, id int, password string) error {
	f, err := m.GetOne(userID, id)
	if err != nil {
		return err
	}
	if !f.IsLocked || f.PasswordHash == "" {
		return nil
	}
	return bcrypt.CompareHashAndPassword([]byte(f.PasswordHash), []byte(password))
}

// LockedFolderIDs returns locked folders and all their descendants.
func (m *FolderManager) LockedFolderIDs(userID int) ([]int, error) {
	folders, err := m.List(userID)
	if err != nil {
		return nil, err
	}
	children := map[int][]int{}
	locked := []int{}
	for _, f := range folders {
		if f.ParentID != nil {
			children[*f.ParentID] = append(children[*f.ParentID], f.ID)
		}
		if f.IsLocked {
			locked = append(locked, f.ID)
		}
	}
	var res []int
	seen := map[int]bool{}
	var walk func(int)
	walk = func(id int) {
		if seen[id] {
			return
		}
		seen[id] = true
		res = append(res, id)
		for _, c := range children[id] {
			walk(c)
		}
	}
	for _, id := range locked {
		walk(id)
	}
	return res, nil
}

// SetBookmarkFolder moves one bookmark to a folder (nil means root).
func (m *FolderManager) SetBookmarkFolder(userID, bookmarkID int, folderID *int) error {
	if folderID != nil {
		if _, err := m.GetOne(userID, *folderID); err != nil {
			return err
		}
	}
	_, err := db.Q().Update(db.TableBookmark).Prepared(true).
		Set(goqu.Record{"folder_id": folderID, "updated": time.Now().UTC()}).
		Where(goqu.C("user_id").Eq(userID), goqu.C("id").Eq(bookmarkID)).
		Executor().Exec()
	return err
}
