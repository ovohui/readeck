// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/**
 * The sidebar AI panel. All requests are AJAX so the user sees "working…" and,
 * most importantly, the provider's error (a wrong key used to look like a
 * button that did nothing).
 */
export default class extends Controller<HTMLElement> {
  static override targets = ["instruction", "status", "result"]
  static override values = {url: String}

  declare readonly instructionTarget: HTMLTextAreaElement
  declare readonly statusTarget: HTMLElement
  declare readonly resultTarget: HTMLElement
  declare readonly urlValue: string

  summary() {
    void this.run("summary")
  }

  clean() {
    void this.run("clean")
  }

  links() {
    void this.run("links")
  }

  private async run(mode: string) {
    const buttons = Array.from(this.element.querySelectorAll("button"))
    buttons.forEach((b) => (b.disabled = true))
    this.resultTarget.textContent = ""
    this.statusTarget.textContent =
      document.documentElement.lang.startsWith("zh")
        ? "AI 正在处理，通常需要 10～60 秒…"
        : "AI is working, usually 10–60 seconds…"

    try {
      const rsp = await fetch(this.urlValue, {
        method: "POST",
        headers: {
          "Accept": "application/json",
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "XMLHttpRequest",
        },
        body: new URLSearchParams({
          mode,
          instruction: this.instructionTarget.value.trim(),
        }),
        credentials: "same-origin",
      })
      const data = await rsp.json().catch(() => null)
      if (!rsp.ok || !data || !data.ok) {
        throw new Error(
          (data && data.error) ||
            (document.documentElement.lang.startsWith("zh")
              ? "AI 请求失败（HTTP " + rsp.status + "）"
              : "AI request failed (HTTP " + rsp.status + ")"),
        )
      }

      this.statusTarget.textContent = ""
      if (mode === "summary") {
        this.resultTarget.textContent = data.summary || ""
        return
      }
      this.statusTarget.textContent =
        document.documentElement.lang.startsWith("zh") ? "完成，正在刷新…" : "Done, reloading…"
      window.setTimeout(() => window.location.reload(), 600)
    } catch (err) {
      this.statusTarget.textContent = (err instanceof Error ? err.message : String(err))
    } finally {
      buttons.forEach((b) => (b.disabled = false))
    }
  }
}
