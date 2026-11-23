// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * Fetches the markdown rendition of a bookmark and copies it to the
 * clipboard, so it can be pasted straight into Obsidian.
 *
 * The URL is provided by the button itself (`data-markdowncopy-url-value`),
 * which keeps the controller independent from the API layout.
 */
export default class extends Controller {
  static override values = {
    url: String,
  }

  declare readonly urlValue: string

  async copy(evt: Event) {
    const btn = evt.currentTarget
    if (!(btn instanceof HTMLElement) || btn.getAttribute("aria-busy") == "true") {
      return
    }

    btn.setAttribute("aria-busy", "true")
    try {
      const rsp = await fetch(this.urlValue, {
        headers: {Accept: "text/markdown"},
        credentials: "same-origin",
      })
      if (!rsp.ok) {
        throw new Error(`HTTP ${rsp.status}`)
      }
      await navigator.clipboard.writeText(await rsp.text())
      this.flash(btn, "✓")
    } catch (_err) {
      this.flash(btn, "!")
    } finally {
      btn.removeAttribute("aria-busy")
    }
  }

  /** Briefly replaces the button label to acknowledge the action. */
  flash(btn: HTMLElement, text: string) {
    const label = btn.querySelector("[data-markdowncopy-label]")
    if (!(label instanceof HTMLElement)) {
      return
    }
    const initial = label.textContent ?? ""
    label.textContent = text
    window.setTimeout(() => {
      label.textContent = initial
    }, 1200)
  }
}
