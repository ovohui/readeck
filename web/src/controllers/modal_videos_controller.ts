// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["dialog", "trigger", "template"]
  static override values = {
    selector: {
      type: String,
      default: "",
    },
    baseUrl: {
      type: String,
      default: "",
    },
  }

  declare readonly dialogTarget: HTMLDialogElement
  declare readonly imgTarget: HTMLImageElement
  declare readonly triggerTarget: HTMLTemplateElement

  declare selectorValue: string
  declare baseUrlValue: string

  override connect() {
    if (!this.selectorValue) {
      return
    }

    this.element
      .querySelectorAll(this.selectorValue)
      .forEach((e) => this.#addTrigger(e as HTMLElement))
    this.dialogTarget.addEventListener("close", () => {
      this.#clearModal()
    })
    this.dialogTarget.addEventListener("click", () => {
      this.dialogTarget.close()
    })
  }

  // turbo:morph-element event handler
  reload(evt: Event) {
    if (evt.target == this.element) {
      this.disconnect()
      this.connect()
    }
  }

  get #iframe() {
    return this.dialogTarget.querySelector("iframe")
  }

  #clearModal() {
    const el = this.#iframe
    el?.removeAttribute("src")
    el?.removeAttribute("width")
    el?.removeAttribute("height")
  }

  #addTrigger(e: HTMLElement) {
    const params = new URLSearchParams(e.dataset.iframeParams)
    const w = params.get("w") || 0
    const h = params.get("h") || 0
    if (w == 0 || h == 0) {
      return
    }

    const node = this.triggerTarget.content.cloneNode(true) as DocumentFragment
    const wrapper = node.firstElementChild!

    e.parentNode?.insertBefore(wrapper, e)
    wrapper.appendChild(e)

    wrapper.querySelector("button")?.addEventListener("click", () => {
      const params = new URLSearchParams(e.dataset.iframeParams)

      const w = params.get("w") || "0"
      const h = params.get("h") || "0"

      const src = new URL(this.baseUrlValue, document.location.toString())
      params.forEach((v, k) => {
        src.searchParams.set(k, v)
      })

      const ifr = this.#iframe
      ifr?.setAttribute("src", src.toString())
      ifr?.setAttribute("width", w)
      ifr?.setAttribute("height", h)

      this.dialogTarget.showModal()
      this.dialogTarget.style.aspectRatio = `${w}/${h}`
    })
  }
}
