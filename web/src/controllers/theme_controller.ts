// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["icon", "iconLight", "iconDark", "iconSystem"]

  declare readonly iconTarget: HTMLTemplateElement
  declare readonly iconLightTarget: HTMLTemplateElement
  declare readonly iconDarkTarget: HTMLTemplateElement
  declare readonly iconSystemTarget: HTMLTemplateElement

  override connect() {
    this.setIcon()

    // Listen to system theme change
    window
      .matchMedia("(prefers-color-scheme: dark)")
      .addEventListener("change", () => {
        if (this.currentTheme == "system") {
          this.setTheme()
        }
      })

    // Listen to user theme change
    window.addEventListener("storage", () => {
      this.setTheme()
      this.setIcon()
    })
  }

  get currentTheme(): string {
    if (localStorage.getItem("theme") !== null) {
      return localStorage.theme as string
    }
    return "system"
  }

  toggleTheme() {
    switch (this.currentTheme) {
      case "system":
        localStorage.theme = "dark"
        break
      case "dark":
        localStorage.theme = "light"
        break
      case "light":
        localStorage.removeItem("theme")
        break
    }
    window.dispatchEvent(new Event("storage"))
  }

  setTheme() {
    let theme = "light"
    switch (this.currentTheme) {
      case "light":
        break
      case "dark":
        theme = "dark"
        break
      case "system":
        if (window.matchMedia("(prefers-color-scheme: dark)").matches) {
          theme = "dark"
        }
        break
    }

    if (theme == "dark") {
      document.documentElement.classList.add("dark")
    } else {
      document.documentElement.classList.remove("dark")
    }
  }

  changeIcon(tpl: HTMLTemplateElement) {
    this.iconTarget.replaceChildren(tpl.content.cloneNode(true))
  }

  setIcon() {
    switch (this.currentTheme) {
      case "light":
        this.changeIcon(this.iconLightTarget)
        break
      case "dark":
        this.changeIcon(this.iconDarkTarget)
        break
      case "system":
        this.changeIcon(this.iconSystemTarget)
        break
    }
  }
}
