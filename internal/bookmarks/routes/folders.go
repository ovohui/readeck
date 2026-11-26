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
	if err := bookmarks.Folders.Delete(userID, id); err != nil {
		_ = server.AddFlash(w, r, "error", server.Locale(r.Context()).Gettext("Could not delete the folder."))
	} else {
		_ = server.AddFlash(w, r, "success", server.Locale(r.Context()).Gettext("Folder deleted; its bookmarks were moved to the root."))
	}
	server.Redirect(w, r, "/bookmarks/folders")
}

func folderSelected(b *bookmarks.Bookmark, id int) bool {
	return b.FolderID != nil && *b.FolderID == id
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
