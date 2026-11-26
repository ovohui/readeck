// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * Tiny trigger controller. The AI button lives in the page head (next to
 * Filters) while the drawer lives inside the bookmark list, so they cannot
 * share a Stimulus target scope. This controller toggles the one drawer by
 * selector; the drawer's own list-ai controller handles its actions.
 */
export default class extends Controller<HTMLElement> {
  private drawer(): HTMLElement | null {
    return document.querySelector<HTMLElement>('[data-list-ai-target="drawer"]')
  }

  toggle() {
    const drawer = this.drawer()
    if (!drawer) return
    if (drawer.hidden) {
      drawer.hidden = false
      void drawer.offsetWidth
      drawer.classList.add("is-open")
      this.element.setAttribute("aria-expanded", "true")
      document.querySelector<HTMLTextAreaElement>('[data-list-ai-target="question"]')?.focus()
    } else {
      drawer.classList.remove("is-open")
      this.element.setAttribute("aria-expanded", "false")
      window.setTimeout(() => {
        if (!drawer.classList.contains("is-open")) {
          drawer.hidden = true
        }
      }, 220)
    }
  }
}
