// SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller<HTMLButtonElement> {
  static classes = ["hidden"]

  declare readonly triggerTargets: HTMLTemplateElement[]

  declare hiddenClass: string

  override connect() {
    if (!this.element.requestFullscreen) {
      return
    }

    this.element.classList.remove(this.hiddenClass)
  }

  async toggle() {
    if (!document.fullscreenElement) {
      await document.documentElement.requestFullscreen()
    } else {
      await document.exitFullscreen()
    }
  }
}
