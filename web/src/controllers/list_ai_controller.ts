// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * Assistant for the bookmark list: summarize the filtered list, tag selected
 * cards, or ask a question and get the matching bookmarks back.
 */
export default class extends Controller<HTMLElement> {
  static override targets = ["question", "status", "result"]
  static override values = {url: String}

  declare readonly questionTarget: HTMLTextAreaElement
  declare readonly statusTarget: HTMLElement
  declare readonly resultTarget: HTMLElement
  declare readonly urlValue: string

  summary() {
    void this.run("summary")
  }

  tag() {
    void this.run("tag")
  }

  ask() {
    void this.run("ask")
  }

  private selected(): string[] {
    return Array.from(
      document.querySelectorAll<HTMLInputElement>(".bookmark-card--select:checked"),
    )
      .map((cb) => cb.dataset.bulkId || "")
      .filter(Boolean)
  }

  private async run(action: string) {
    const ids = this.selected()
    if (action === "tag" && ids.length === 0) {
      this.statusTarget.textContent = document.documentElement.lang.startsWith("zh")
        ? "请先勾选要打标签的书签。"
        : "Select bookmarks first."
      return
    }

    const buttons = Array.from(this.element.querySelectorAll("button"))
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

      const answer = document.createElement("p")
      answer.className = "whitespace-pre-wrap"
      answer.textContent = data.answer || ""
      this.resultTarget.appendChild(answer)

      if (data.matches && data.matches.length > 0) {
        const list = document.createElement("ul")
        list.className = "mt-3 space-y-1"
        for (const item of data.matches) {
          const li = document.createElement("li")
          const a = document.createElement("a")
          a.className = "link"
          a.href = `/bookmarks/${item.id}`
          a.textContent = item.title || item.id
          li.appendChild(a)
          list.appendChild(li)
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
