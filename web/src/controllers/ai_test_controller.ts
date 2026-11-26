// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * Test the AI provider with the values currently typed in the profile form,
 * without saving them first. A wrong key used to fail silently.
 */
export default class extends Controller<HTMLElement> {
  static override targets = ["result"]
  declare readonly resultTarget: HTMLElement

  async test() {
    const form = this.element.closest("form")
    if (!form) {
      return
    }
    const body = new URLSearchParams()
    for (const name of [
      "settings.ai_provider",
      "settings.ai_base_url",
      "settings.ai_model",
      "settings.ai_api_key",
    ]) {
      const input = form.querySelector<HTMLInputElement>(`[name="${name}"]`)
      if (input) {
        body.set(name, input.value)
      }
    }

    this.resultTarget.textContent = document.documentElement.lang.startsWith("zh")
      ? "正在连接…"
      : "Connecting…"
    try {
      const rsp = await fetch("/profile/ai/test", {
        method: "POST",
        headers: {
          "Accept": "application/json",
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body,
        credentials: "same-origin",
      })
      const data = await rsp.json().catch(() => null)
      if (rsp.ok && data && data.ok) {
        this.resultTarget.textContent = document.documentElement.lang.startsWith("zh")
          ? "✅ 连接成功"
          : "✅ Connected"
      } else {
        this.resultTarget.textContent =
          "❌ " + ((data && data.error) || `HTTP ${rsp.status}`)
      }
    } catch (err) {
      this.resultTarget.textContent = "❌ " + (err instanceof Error ? err.message : String(err))
    }
  }
}
