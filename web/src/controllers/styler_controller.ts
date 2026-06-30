// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import type {ActionEvent, Context} from "@hotwired/stimulus"

// Controller "styler" is only an outlet for the "styler-option" controller.
export default class extends Controller {
  override initialize() {
    // only add the styler-option controller the first time
    // the controller is initialized.
    if (
      this.application.controllers.find(
        (x) => x.context.controller.identifier == "styler-option",
      ) === undefined
    ) {
      this.application.register("styler-option", stylerOption)
    }
  }
}

class stylerOption extends Controller {
  static override outlets = ["styler"]
  static override values = {
    current: {
      type: String,
      default: "",
    },
    values: {
      type: Array,
      default: [],
    },
  }

  static override targets = ["choices", "value"]

  declare readonly stylerOutletElements: HTMLElement[]

  declare readonly hasChoicesTarget: boolean
  declare readonly choicesTarget: HTMLInputElement
  declare readonly choicesTargets: HTMLInputElement[]
  declare readonly valueTargets: HTMLInputElement[]

  declare currentValue: string
  declare valuesValue: string[]

  declare choices: Record<string, string | undefined>

  constructor(context: Context) {
    super(context)
    this.choices = {}
  }

  override connect() {
    this.valueTargets.forEach((e) => {
      this.currentValue = e.value
    })

    this.choicesTargets.forEach((e) => {
      this.choices[e.value] = e.dataset.choiceValue
    })
  }

  stylerOutletConnected() {
    if (this.currentValue === null) {
      return
    }
    this.applyClass()
    this.updateChoices()
  }

  updateChoices() {
    let found = false
    this.choicesTargets.forEach((e) => {
      if (e.value == this.currentValue) {
        e.setAttribute("data-current", "1")
        found = true
      } else {
        e.removeAttribute("data-current")
      }
    })

    if (!found && this.hasChoicesTarget) {
      // Set current to the first choice
      this.choicesTarget.setAttribute("data-current", "1")
    }
  }

  dispatchEvents() {
    this.valueTargets.forEach((e) => this.dispatch("setValue", {target: e}))
  }

  setChoice(evt: ActionEvent) {
    if (evt.target && "value" in evt.target) {
      this.currentValue = <string>evt.target.value
    }
  }

  increaseValue() {
    const value = parseInt(this.currentValue)
    if (!isNaN(value) && value == this.valuesValue.length) {
      return
    }
    this.currentValue = `${value + 1}`
  }

  decreaseValue() {
    const value = parseInt(this.currentValue)
    if (!isNaN(value) && value == 1) {
      return
    }
    this.currentValue = `${value - 1}`
  }

  currentValueChanged(value: string, prev: string) {
    if (!prev) {
      return
    }
    this.valueTargets.forEach((e) => (e.value = value))
    this.applyClass()
    this.updateChoices()
    this.dispatchEvents()
  }

  getAllClasses() {
    let res
    if (this.valuesValue.length > 0) {
      res = this.valuesValue
    } else {
      res = Object.values(this.choices)
    }
    return res.reduce<string[]>((acc, cur) => {
      if (cur !== undefined) {
        return acc.concat(cur!.split(/\s+/))
      }
      return acc
    }, [])
  }

  getCurrentClasses() {
    const idx = parseInt(this.currentValue)
    let res
    if (this.valuesValue.length > 0 && !isNaN(idx)) {
      res = this.valuesValue[idx - 1]
    } else {
      res = this.choices[this.currentValue]
    }

    if (res) {
      return res.split(/\s+/)
    }
    return []
  }

  applyClass() {
    const classNames = this.getCurrentClasses()
    if (classNames.length == 0) {
      return
    }

    this.stylerOutletElements.forEach((e) => {
      e.classList.remove(...this.getAllClasses())
      e.classList.add(...classNames)
    })
  }
}
