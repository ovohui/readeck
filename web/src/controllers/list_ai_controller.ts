// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * Right-hand AI drawer for the bookmark list. The trigger sits next to the
 * filter button; the panel is a proper dialog-like drawer with a backdrop,
 * Escape handling, a context line and a composer.
 */
export default class extends Controller<HTMLElement> {
  static override targets = ["drawer", "toggle", "question", "status", "result", "context"]
  static override values = {url: String}

  declare readonly drawerTarget: HTMLElement
  declare readonly toggleTarget: HTMLButtonElement
  declare readonly hasToggleTarget: boolean
  declare readonly questionTarget: HTMLTextAreaElement
  declare readonly statusTarget: HTMLElement
  declare readonly resultTarget: HTMLElement
  declare readonly contextTarget: HTMLElement
  declare readonly urlValue: string

  private keydown = (event: KeyboardEvent) => {
    if (event.key === "Escape" && this.isOpen()) {
      this.close()
    }
  }

  override connect() {
    document.addEventListener("keydown", this.keydown)
    document.addEventListener("change", this.refreshContext)
    this.refreshContext()
  }

  override disconnect() {
    document.removeEventListener("keydown", this.keydown)
    document.removeEventListener("change", this.refreshContext)
  }

  private isOpen() {
    return !this.drawerTarget.hidden
  }

  toggle() {
    this.isOpen() ? this.close() : this.open()
  }

  open() {
    this.drawerTarget.hidden = false
    if (this.hasToggleTarget) this.toggleTarget.setAttribute("aria-expanded", "true")
    void this.drawerTarget.offsetWidth
    this.drawerTarget.classList.add("is-open")
    this.questionTarget.focus()
  }

  close() {
    this.drawerTarget.classList.remove("is-open")
    if (this.hasToggleTarget) this.toggleTarget.setAttribute("aria-expanded", "false")
    window.setTimeout(() => {
      if (!this.drawerTarget.classList.contains("is-open")) {
        this.drawerTarget.hidden = true
      }
    }, 220)
  }

  private selected(): string[] {
    return Array.from(
      document.querySelectorAll<HTMLInputElement>(".bookmark-card--select:checked"),
    )
      .map((cb) => cb.dataset.bulkId || "")
      .filter(Boolean)
  }

  refreshContext = () => {
    const ids = this.selected()
    this.contextTarget.textContent =
      ids.length > 0
        ? document.documentElement.lang.startsWith("zh")
          ? `已选 ${ids.length} 篇`
          : `${ids.length} selected`
        : document.documentElement.lang.startsWith("zh")
          ? "当前筛选结果"
          : "current filter"
  }

  summary() {
    this.open()
    void this.run("summary")
  }

  tag() {
    this.open()
    void this.run("tag")
  }

  ask() {
    this.open()
    void this.run("ask")
  }

  private async run(action: string) {
    const ids = this.selected()
    if (action === "tag" && ids.length === 0) {
      this.statusTarget.textContent = document.documentElement.lang.startsWith("zh")
        ? "请先勾选要打标签的书签。"
        : "Select bookmarks first."
      return
    }
    if (action === "ask" && !this.questionTarget.value.trim()) {
      this.statusTarget.textContent = document.documentElement.lang.startsWith("zh")
        ? "先输入一个问题，或点下面的快捷操作。"
        : "Type a question or use a shortcut."
      return
    }

    this.element.querySelector(".ai-empty")?.remove()
    const buttons = Array.from(this.drawerTarget.querySelectorAll<HTMLButtonElement>("button"))
    buttons.forEach((b) => (b.disabled = true))
    this.resultTarget.textContent = ""
    this.statusTarget.textContent = document.documentElement.lang.startsWith("zh")
      ? "AI 正在阅读收藏，通常需要 10～60 秒…"
      : "AI is reading, usually 10–60 seconds…"

    try {
      const body = new URLSearchParams({
        action,
        question: this.questionTarget.value.trim(),
        ids: ids.join(","),
      })
      const rsp = await fetch(this.urlValue + window.location.search, {
        method: "POST",
        headers: {
          "Accept": "application/json",
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "XMLHttpRequest",
        },
        body,
        credentials: "same-origin",
      })
      const data = await rsp.json().catch(() => null)
      if (!rsp.ok || !data || !data.ok) {
        throw new Error((data && data.error) || `HTTP ${rsp.status}`)
      }

      this.statusTarget.textContent = ""
      if (action === "tag") {
        this.statusTarget.textContent = document.documentElement.lang.startsWith("zh")
          ? `已添加标签：${(data.labels || []).join("、")}，正在刷新…`
          : `Labels added: ${(data.labels || []).join(", ")}, reloading…`
        window.setTimeout(() => window.location.reload(), 600)
        return
      }

      const answer = document.createElement("div")
      answer.className = "ai-answer"
      answer.textContent = data.answer || ""
      this.resultTarget.appendChild(answer)

      if (data.matches && data.matches.length > 0) {
        const label = document.createElement("p")
        label.className = "ai-matches-label"
        label.textContent = document.documentElement.lang.startsWith("zh")
          ? "相关书签"
          : "Matching bookmarks"
        this.resultTarget.appendChild(label)

        const list = document.createElement("div")
        list.className = "ai-matches"
        for (const item of data.matches) {
          const a = document.createElement("a")
          a.className = "ai-match"
          a.href = `/bookmarks/${item.id}`
          a.textContent = item.title || item.id
          list.appendChild(a)
        }
        this.resultTarget.appendChild(list)
      }
    } catch (err) {
      this.statusTarget.textContent = (err instanceof Error ? err.message : String(err))
    } finally {
      buttons.forEach((b) => (b.disabled = false))
    }
  }
}
