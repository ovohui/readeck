// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override values = {
    action: String,
    method: String,
  }

  declare hasActionValue: boolean
  declare hasMethodValue: boolean
  declare actionValue: string
  declare methodValue: string

  override connect() {
    const tagName = this.element.tagName.toLowerCase()
    switch (tagName) {
      case "form":
        this.element.setAttribute("data-turbo", "true")
        this.conditionnalAttr("action", this.actionValue, this.hasActionValue)
        this.conditionnalAttr("method", this.methodValue, this.hasMethodValue)
        break
      case "button":
        this.element.closest("form")?.setAttribute("data-turbo", "true")
        this.conditionnalAttr("formaction", this.actionValue, this.hasActionValue)
        this.conditionnalAttr("formmethod", this.methodValue, this.hasMethodValue)
        break
      default:
        throw new Error(
          "turbo-form can only be used on form and button elements",
        )
    }
  }

  conditionnalAttr(name: string, value: string, condition: boolean) {
    if (condition) {
      this.element.setAttribute(name, value)
    }
  }

  stopSubmit(evt: Event) {
    evt.stopPropagation()
  }
}
