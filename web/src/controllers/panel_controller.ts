// SPDX-FileCopyrightText: © 2023 Joachim Robert <joachim.robert@protonmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["panel", "button"]
  static classes = ["hidden", "forceHidden", "body"]

  declare readonly panelTarget: HTMLElement
  declare readonly buttonTarget: HTMLButtonElement

  declare readonly hiddenClasses: string[]
  declare readonly forceHiddenClasses: string[]
  declare readonly bodyClasses: string[]

  toggle() {
    if (this.isVisible()) {
      this.close()
    } else {
      this.open()
    }
  }

  open() {
    document.body.classList.add(...this.bodyClasses)
    this.panelTarget.classList.remove(...this.hiddenClasses)
    this.panelTarget.focus()
    this.buttonTarget.setAttribute("aria-expanded", String(this.isVisible()))
  }

  close(evt?: Event) {
    document.body.classList.remove(...this.bodyClasses)
    if (evt === undefined) {
      this.panelTarget.classList.add(...this.hiddenClasses)
      this.buttonTarget.focus()
    } else {
      this.panelTarget.classList.add(...this.forceHiddenClasses)
    }
    this.buttonTarget.setAttribute("aria-expanded", String(this.isVisible()))
  }

  /**
   * isVisible returns true when the menu/panel is visible
   */
  isVisible() {
    const cs = getComputedStyle(this.panelTarget)
    return !(cs.visibility == "hidden" || cs.display == "none")
  }
}
