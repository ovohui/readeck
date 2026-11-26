// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * Multi-select for the bookmark list: select all / invert / label / archive /
 * delete. The update route is the same one the card buttons use; each request
 * only carries the single field being changed.
 */
export default class extends Controller<HTMLElement> {
  static override targets = ["checkbox", "toolbar", "count"]

  declare readonly checkboxTargets: HTMLInputElement[]
  declare readonly toolbarTarget: HTMLElement
  declare readonly countTarget: HTMLElement

  override connect() {
    this.update()
  }

  toggle() {
    this.update()
  }

  selectAll() {
    this.checkboxTargets.forEach((cb) => (cb.checked = true))
    this.update()
  }

  invert() {
    this.checkboxTargets.forEach((cb) => (cb.checked = !cb.checked))
    this.update()
  }

  async archive() {
    await this.batch("is_archived", "1")
  }

  async label() {
    const label = window.prompt(
      document.documentElement.lang.startsWith("zh")
        ? "给选中的书签加一个标签/文件夹名："
        : "Label or folder for the selected bookmarks:",
    )
    if (!label || !label.trim()) {
      return
    }
    await this.batch("add_labels", label.trim())
  }

  async delete() {
    const ok = window.confirm(
      document.documentElement.lang.startsWith("zh")
        ? "确定删除选中的书签？"
        : "Delete the selected bookmarks?",
    )
    if (!ok) {
      return
    }
    await this.batch("is_deleted", "1")
  }

  private selected(): string[] {
    return this.checkboxTargets
      .filter((cb) => cb.checked)
      .map((cb) => cb.dataset.bulkId || "")
      .filter((id) => id !== "")
  }

  private async batch(name: string, value: string) {
    const ids = this.selected()
    if (ids.length === 0) {
      return
    }

    for (const id of ids) {
      await fetch(`/bookmarks/${id}/update`, {
        method: "POST",
        headers: {"Content-Type": "application/x-www-form-urlencoded"},
        body: new URLSearchParams({[name]: value}),
        credentials: "same-origin",
      })
    }

    window.location.reload()
  }

  private update() {
    const n = this.selected().length
    this.countTarget.textContent = String(n)
    this.toolbarTarget.hidden = n === 0
    this.element.classList.toggle("bulk-active", n > 0)
  }
}
