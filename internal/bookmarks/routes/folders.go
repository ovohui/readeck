// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
)

func (h *viewsRouter) folderList(w http.ResponseWriter, r *http.Request) {
	tree, err := bookmarks.Folders.Tree(auth.GetUser(r.Context()).ID)
	if err != nil {
		server.Err(w, r, err)
		return
	}
	server.RenderComponent(w, r, http.StatusOK, Views{}.folderList(tree))
}

func (h *viewsRouter) folderManage(w http.ResponseWriter, r *http.Request) {
	tree, err := bookmarks.Folders.Tree(auth.GetUser(r.Context()).ID)
	if err != nil {
		server.Err(w, r, err)
		return
	}
	server.RenderComponent(w, r, http.StatusOK, Views{}.folderManage(tree))
}

func (h *viewsRouter) folderCreate(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUser(r.Context()).ID
	name := strings.TrimSpace(r.FormValue("name"))
	var parent *int
	if v := strings.TrimSpace(r.FormValue("parent_id")); v != "" && v != "0" {
		if id, err := strconv.Atoi(v); err == nil {
			parent = &id
		}
	}
	if _, err := bookmarks.Folders.Create(userID, parent, name); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not create the folder."))
	} else {
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder created."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func (h *viewsRouter) folderRename(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUser(r.Context()).ID
	id, _ := strconv.Atoi(r.FormValue("id"))
	if err := bookmarks.Folders.Rename(userID, id, r.FormValue("name")); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not rename the folder."))
	} else {
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder renamed."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func (h *viewsRouter) folderDelete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUser(r.Context()).ID
	id, _ := strconv.Atoi(r.FormValue("id"))
	if f, err := bookmarks.Folders.GetOne(userID, id); err == nil && f.IsLocked && !folderUnlocked(r.Context(), id) {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Unlock the folder before deleting it."))
		server.Redirect(w, r, "/bookmarks/folders")
		return
	}
	if err := bookmarks.Folders.Delete(userID, id); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not delete the folder."))
	} else {
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder deleted; its bookmarks were moved to the root."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func (h *viewsRouter) folderLock(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUser(r.Context()).ID
	id, _ := strconv.Atoi(r.FormValue("id"))
	if err := bookmarks.Folders.SetLock(userID, id, r.FormValue("password")); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not lock the folder: %s", err.Error()))
	} else {
		removeUnlocked(w, r, id)
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder locked."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func (h *viewsRouter) folderUnlock(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUser(r.Context()).ID
	id, _ := strconv.Atoi(r.FormValue("id"))
	if err := bookmarks.Folders.CheckPassword(userID, id, r.FormValue("password")); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Wrong password."))
	} else {
		sess := server.GetSession(r)
		if sess != nil {
			found := false
			for _, x := range sess.Payload.UnlockedFolders {
				if x == id {
					found = true
				}
			}
			if !found {
				sess.Payload.UnlockedFolders = append(sess.Payload.UnlockedFolders, id)
			}
			_ = sess.Save(w, r)
		}
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder unlocked for this session."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func (h *viewsRouter) folderClearLock(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUser(r.Context()).ID
	id, _ := strconv.Atoi(r.FormValue("id"))
	if err := bookmarks.Folders.CheckPassword(userID, id, r.FormValue("password")); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Wrong password."))
	} else if err := bookmarks.Folders.ClearLock(userID, id); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not remove the folder lock."))
	} else {
		removeUnlocked(w, r, id)
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder lock removed."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func (h *viewsRouter) folderRelock(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	removeUnlocked(w, r, id)
	server.Redirect(w, r, "/bookmarks/folders")
}

func removeUnlocked(w http.ResponseWriter, r *http.Request, id int) {
	sess := server.GetSession(r)
	if sess == nil {
		return
	}
	out := sess.Payload.UnlockedFolders[:0]
	for _, x := range sess.Payload.UnlockedFolders {
		if x != id {
			out = append(out, x)
		}
	}
	sess.Payload.UnlockedFolders = out
	_ = sess.Save(w, r)
}

func folderUnlocked(ctx context.Context, id int) bool {
	u := auth.GetUser(ctx)
	if u == nil {
		return false
	}
	sess := server.GetSessionFromContext(ctx)
	if sess == nil {
		return false
	}
	unlocked := map[int]bool{}
	for _, x := range sess.Payload.UnlockedFolders {
		unlocked[x] = true
	}
	folders, err := bookmarks.Folders.List(u.ID)
	if err != nil {
		return false
	}
	byID := map[int]*bookmarks.Folder{}
	for _, f := range folders {
		byID[f.ID] = f
	}
	cur := byID[id]
	for cur != nil {
		if unlocked[cur.ID] {
			return true
		}
		if cur.ParentID == nil {
			break
		}
		cur = byID[*cur.ParentID]
	}
	return false
}

func folderSelected(b *bookmarks.Bookmark, id int) bool {
	return b.FolderID != nil && *b.FolderID == id
}

func unlockedFolderSet(r *http.Request) map[int]bool {
	res := map[int]bool{}
	sess := server.GetSession(r)
	if sess == nil {
		return res
	}
	for _, id := range sess.Payload.UnlockedFolders {
		res[id] = true
	}
	return res
}

// blockedFolderIDs returns locked folders that are not covered by an unlocked
// ancestor in this session.
func blockedFolderIDs(r *http.Request) []int {
	u := auth.GetUser(r.Context())
	if u == nil {
		return nil
	}
	folders, err := bookmarks.Folders.List(u.ID)
	if err != nil {
		return nil
	}
	byID := map[int]*bookmarks.Folder{}
	for _, f := range folders {
		byID[f.ID] = f
	}
	unlocked := unlockedFolderSet(r)
	blocked := []int{}
	for _, f := range folders {
		if !f.IsLocked {
			continue
		}
		allowed := false
		for cur := f; cur != nil; {
			if unlocked[cur.ID] {
				allowed = true
				break
			}
			if cur.ParentID == nil {
				break
			}
			cur = byID[*cur.ParentID]
		}
		if !allowed {
			blocked = append(blocked, f.ID)
		}
	}
	return blocked
}

// isFolderBlocked reports whether one folder is locked and not unlocked.
func isFolderBlocked(r *http.Request, folderID *int) bool {
	if folderID == nil {
		return false
	}
	for _, id := range blockedFolderIDs(r) {
		if id == *folderID {
			return true
		}
	}
	return false
}

// folderOptions returns a flat tree for the bookmark edit dialog.
func folderOptions(ctx context.Context) []*bookmarks.FolderNode {
	u := auth.GetUser(ctx)
	if u == nil {
		return nil
	}
	tree, err := bookmarks.Folders.Tree(u.ID)
	if err != nil {
		return nil
	}
	var res []*bookmarks.FolderNode
	var walk func([]*bookmarks.FolderNode)
	walk = func(nodes []*bookmarks.FolderNode) {
		for _, n := range nodes {
			res = append(res, n)
			walk(n.Children)
		}
	}
	walk(tree)
	return res
}
