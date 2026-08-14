// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller<HTMLElement> {
  static override targets = ["embed", "play"]

  declare readonly embedTarget: HTMLTemplateElement
  declare readonly playTarget: HTMLTemplateElement

  declare ifr: HTMLIFrameElement | null
  declare playBtn: HTMLButtonElement | null

  override connect() {
    this.ifr = this.embedTarget.content.querySelector("iframe")
    if (!this.ifr) {
      return
    }

    this.ifr.setAttribute("sandbox", "allow-scripts allow-same-origin")

    const w = parseInt(this.ifr.getAttribute("width") || "") || 0
    const h = parseInt(this.ifr.getAttribute("height") || "") || 0

    if (w > 0 && h > 0) {
      this.element.style.aspectRatio = `${w} / ${h}`
    }

    this.playBtn = this.playTarget.content.querySelector("button")
    if (!this.playBtn) {
      return
    }
    this.element.appendChild(this.playBtn)
  }

  play() {
    if (!this.ifr || !this.playBtn) {
      return
    }

    this.playBtn.remove()
    this.element.appendChild(this.ifr)
  }
}
