// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/** Remember how many archived/AI images to show per row. */
export default class GridColsController extends Controller<HTMLElement> {
  static override targets = ["select"]
  declare readonly selectTarget: HTMLSelectElement

  private static readonly KEY = "readeck.ai.image.columns"

  override connect() {
    const saved = window.localStorage.getItem(GridColsController.KEY) || "2"
    if (["1", "2", "3"].includes(saved)) {
      this.selectTarget.value = saved
    }
    this.apply(this.selectTarget.value)
  }

  set() {
    const value = this.selectTarget.value
    window.localStorage.setItem(GridColsController.KEY, value)
    this.apply(value)
  }

  private apply(value: string) {
    document.querySelectorAll<HTMLElement>(".ai-image-grid").forEach((grid) => {
      grid.style.setProperty("--ai-cols", value)
    })
  }
}
