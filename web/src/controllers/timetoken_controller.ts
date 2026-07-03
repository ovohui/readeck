// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

import type {ActionEvent} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["field", "btn", "template", "value", "unit", "absolute"]
  static classes = ["hidden"]

  declare readonly fieldTarget: HTMLInputElement
  declare readonly btnTarget: HTMLButtonElement
  declare readonly templateTarget: HTMLTemplateElement
  declare readonly valueTarget: HTMLSelectElement
  declare readonly unitTarget: HTMLSelectElement
  declare readonly absoluteTarget: HTMLInputElement

  declare readonly hiddenClass: string

  declare form: HTMLDivElement

  override connect() {
    this.form = this.templateTarget.content.querySelector("div")!
    this.form.classList.add(this.hiddenClass)
    this.element.appendChild(this.form)
    this.loadToken()

    this.btnTarget.addEventListener("click", () => {
      this.toggleForm()
    })
  }

  toggleForm() {
    if (this.isFormHidden()) {
      this.showForm()
    } else {
      this.hideForm()
    }
  }

  isFormHidden() {
    return this.form.classList.contains(this.hiddenClass)
  }

  showForm() {
    if (this.isFormHidden()) {
      this.form.classList.remove(this.hiddenClass)
      document.addEventListener("click", this.onOuterClick)
    }
  }

  hideForm() {
    if (!this.isFormHidden()) {
      this.form.classList.add(this.hiddenClass)
      document.removeEventListener("click", this.onOuterClick)
    }
  }

  onOuterClick = (evt: PointerEvent) => {
    if (!this.element.contains(<HTMLElement>evt.target) || evt.target == this.fieldTarget) {
      this.hideForm()
    }
  }

  loadToken() {
    const token = new timeToken(this.fieldTarget.value)
    this.valueTarget.value = token.value.toString()
    this.unitTarget.value = token.unit
    this.absoluteTarget.value = token.absolute
  }

  update(evt: ActionEvent) {
    if (evt.target == this.valueTarget || evt.target == this.unitTarget) {
      let value = parseInt(this.valueTarget.value || "")
      if (isNaN(value)) {
        value = 1
      }
      const unit = this.unitTarget.value
      if (value == 0) {
        this.fieldTarget.value = "now"
      } else {
        this.fieldTarget.value = `-${value}${unit}`
      }
    } else if (evt.target == this.absoluteTarget) {
      this.fieldTarget.value = this.absoluteTarget.value
      this.valueTarget.value = ""
    }
  }
}

class timeToken {
  tokenRx = RegExp("^([+-])(\\d+)([dwmy])")

  factor: number
  value: number
  unit: string
  absolute: string

  constructor(text: string) {
    text = text.toLowerCase().trim()

    this.factor = 1
    this.value = 0
    this.unit = "d"
    this.absolute = ""

    if (text == "now") {
      // now is value 0 and unit d
      return
    }

    const m = this.tokenRx.exec(text)
    if (m != null) {
      this.value = parseInt(m[2], 10)
      if (m[1] == "-") {
        this.factor = -1
      }
      this.unit = m[3]
      return
    }

    // try to parse the string as a date for absolute values
    const d = new Date(text)
    if (isNaN(d.valueOf())) {
      return
    }

    this.absolute
      = d.getFullYear()
        + "-"
        + ("0" + (d.getMonth() + 1)).slice(-2)
        + "-"
        + ("0" + d.getDate()).slice(-2)
  }
}
