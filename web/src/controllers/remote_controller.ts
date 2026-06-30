// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import type {ActionEvent} from "@hotwired/stimulus"

export default class extends Controller {
  toggleDetails(evt: ActionEvent) {
    if (!(evt.params.selector)) {
      return
    }

    document.querySelectorAll<HTMLDetailsElement>(evt.params.selector).forEach((e) => {
      e.open = !e.open
    })
  }
}
