// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override values = {
    select: String,
  }

  declare hasSelectValue: boolean
  declare selectValue: string

  open() {
    if (!this.hasSelectValue) {
      return
    }

    const el = document.querySelector(this.selectValue)
    if (el instanceof HTMLDialogElement) {
      el.showModal()
    }
  }
}
