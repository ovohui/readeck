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
  static override targets = ["instruction", "status", "result", "deep", "cover"]
  static override values = {url: String}

  declare readonly instructionTarget: HTMLTextAreaElement
  declare readonly statusTarget: HTMLElement
  declare readonly resultTarget: HTMLElement
  declare readonly deepTarget: HTMLInputElement
  declare readonly coverTarget: HTMLInputElement
  declare readonly urlValue: string

  summary() {
    void this.run("summary")
  }

  clean() {
    void this.run("clean")
  }

  tag() {
    void this.run("tag")
  }

  firstCover() {
    const img = document.querySelector<HTMLImageElement>(".prose img[src]:not([data-iframe-params])")
    if (img) this.coverTarget.value = new URL(img.src, window.location.href).href
  }

  chat() {
    void this.run("chat")
  }

  private async run(mode: string) {
    const instruction = this.instructionTarget.value.trim()
    if (mode === "chat" && /识图|识别图片|图片.{0,6}文字|OCR/i.test(instruction)) {
      mode = "ocr"
    }
    const buttons = Array.from(this.element.querySelectorAll("button"))
    buttons.forEach((b) => (b.disabled = true))
    this.resultTarget.textContent = ""
    const zh = document.documentElement.lang.startsWith("zh")
    const stages = zh
      ? ["读取正文…", "请求 AI…", "等待 AI 生成…", "整理结果…"]
      : ["Reading the article…", "Calling AI…", "Waiting for the model…", "Preparing the result…"]
    let stage = 0
    this.statusTarget.textContent = stages[0]
    const timer = window.setInterval(() => {
      stage = Math.min(stage + 1, stages.length - 1)
      this.statusTarget.textContent = stages[stage]
    }, 2500)

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
          instruction,
          deep: this.deepTarget.checked ? "1" : "0",
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
      if (mode === "summary" || mode === "chat" || mode === "ocr") {
        this.resultTarget.textContent = data.summary || ""
        return
      }
      if (mode === "tag") {
        this.statusTarget.textContent =
          (data.labels || []).join("、") + (document.documentElement.lang.startsWith("zh") ? "，正在刷新…" : ", reloading…")
        window.setTimeout(() => window.location.reload(), 600)
        return
      }
      this.statusTarget.textContent =
        document.documentElement.lang.startsWith("zh") ? "完成，正在刷新…" : "Done, reloading…"
      window.setTimeout(() => window.location.reload(), 600)
    } catch (err) {
      this.statusTarget.textContent = err instanceof Error ? err.message : String(err)
    } finally {
      window.clearInterval(timer)
      buttons.forEach((b) => (b.disabled = false))
    }
  }
}
