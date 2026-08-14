// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["dialog", "img", "trigger"]
  static override values = {
    selector: {
      type: String,
      default: "img[src]",
    },
  }

  declare readonly dialogTarget: HTMLDialogElement
  declare readonly imgTarget: HTMLImageElement
  declare readonly triggerTarget: HTMLTemplateElement

  declare selectorValue: string

  override connect() {
    this.element
      .querySelectorAll<HTMLImageElement>(this.selectorValue)
      .forEach((e) => this.#addTrigger(e))
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

  #addTrigger(e: HTMLElement) {
    if (e == this.imgTarget || !(e instanceof HTMLImageElement)) {
      return
    }
    const src = e.src
    if (!src) {
      return
    }

    // Disable on small images (icons, etc.)
    if (e.width < 48 || e.height < 48 || (e.width < 128 && e.height < 128)) {
      return
    }

    const node = this.triggerTarget.content.cloneNode(true) as DocumentFragment
    const wrapper = node.firstElementChild!

    // If the image is the only child of a link, we used the link
    // as our wrapper's child node.
    // This is to avoid inserting a button inside a link element.
    if (e.parentNode?.nodeName == "A" && e.parentNode.children.length == 1) {
      e = e.parentNode as HTMLElement
    }

    e.parentNode?.insertBefore(wrapper, e)
    wrapper.appendChild(e)

    wrapper.querySelector("button")?.addEventListener("click", (evt) => {
      evt.preventDefault()
      this.imgTarget.setAttribute("src", src)
      this.dialogTarget.showModal()
    })
  }
}
