// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

type BookmarkItem = {
  id: string
  title?: string
  site_name?: string
  description?: string
  url?: string
  labels?: string[]
  resources?: {thumbnail?: {src?: string}; image?: {src?: string}}
}

/** Bilibili-style folder page: the left tree selects a folder, the right grid
 * loads that folder's bookmarks from the API without a full page reload. */
export default class extends Controller<HTMLElement> {
  static override targets = ["list", "title", "count"]
  static override values = {url: String}

  declare readonly listTarget: HTMLElement
  declare readonly titleTarget: HTMLElement
  declare readonly countTarget: HTMLElement
  declare readonly urlValue: string

  private id = "0"
  private name = ""

  override connect() {
    const params = new URLSearchParams(window.location.search)
    this.id = params.get("folder") || "0"
    const active = document.querySelector<HTMLElement>(`[data-folder-browser-id-param="${this.id}"]`)
    this.name = active?.dataset.folderBrowserNameParam || this.titleTarget.textContent || ""
    this.markActive()
    void this.load()
  }

  select(event: Event) {
    event.preventDefault()
    const p = (event as unknown as {params?: {id?: string; name?: string}}).params
    this.id = p?.id || "0"
    this.name = p?.name || ""
    const url = new URL(window.location.href)
    url.searchParams.set("folder", this.id)
    window.history.replaceState({}, "", url)
    this.markActive()
    void this.load()
  }

  private markActive() {
    document.querySelectorAll<HTMLElement>(".folder-nav--item").forEach((el) => {
      el.classList.toggle("is-active", el.dataset.folderBrowserIdParam === this.id)
    })
  }

  private async load() {
    this.titleTarget.textContent = this.name || this.titleTarget.textContent
    this.countTarget.textContent = ""
    this.listTarget.innerHTML = '<div class="folder-loading">Loading bookmarks…</div>'
    try {
      const query = this.id === "0" ? "?limit=24" : `?folder=${encodeURIComponent(this.id)}&limit=24`
      const url = this.urlValue + query
      const rsp = await fetch(url, {
        headers: {"Accept": "application/json"},
        credentials: "same-origin",
      })
      if (!rsp.ok) throw new Error(`HTTP ${rsp.status}`)
      const items = (await rsp.json()) as BookmarkItem[]
      this.countTarget.textContent = document.documentElement.lang.startsWith("zh")
        ? `${items.length} 篇书签`
        : `${items.length} bookmarks`
      if (items.length === 0) {
        this.listTarget.innerHTML = ""
        const empty = document.createElement("div")
        empty.className = "folder-empty"
        empty.textContent = document.documentElement.lang.startsWith("zh")
          ? "这个文件夹还没有书签。"
          : "No bookmarks in this folder yet."
        this.listTarget.appendChild(empty)
        return
      }
      this.listTarget.innerHTML = ""
      for (const item of items) {
        this.listTarget.appendChild(this.card(item))
      }
    } catch (err) {
      this.listTarget.innerHTML = ""
      const box = document.createElement("div")
      box.className = "folder-empty"
      box.textContent = err instanceof Error ? err.message : String(err)
      this.listTarget.appendChild(box)
    }
  }

  private card(item: BookmarkItem): HTMLElement {
    const a = document.createElement("a")
    a.className = "folder-card"
    a.href = `/bookmarks/${item.id}`

    const image = document.createElement("div")
    image.className = "folder-card-image"
    const src = item.resources?.thumbnail?.src || item.resources?.image?.src
    if (src) {
      const img = document.createElement("img")
      img.src = src
      img.alt = ""
      img.loading = "lazy"
      image.appendChild(img)
    }
    a.appendChild(image)

    const body = document.createElement("div")
    body.className = "folder-card-body"
    const title = document.createElement("strong")
    title.textContent = item.title || item.id
    body.appendChild(title)
    if (item.site_name) {
      const site = document.createElement("span")
      site.textContent = item.site_name
      body.appendChild(site)
    }
    if (item.labels?.length) {
      const labels = document.createElement("div")
      labels.className = "folder-card-labels"
      for (const label of item.labels.slice(0, 3)) {
        const tag = document.createElement("span")
        tag.textContent = label
        labels.appendChild(tag)
      }
      body.appendChild(labels)
    }
    a.appendChild(body)
    return a
  }
}
