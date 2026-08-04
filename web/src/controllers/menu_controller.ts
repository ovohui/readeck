// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller<HTMLElement> {
  toggle = () => {
    if (this.element.popover) {
      this.element.togglePopover()
    }
  }
}
