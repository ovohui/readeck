// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

/** One folder picker dialog reused by every card. */
export default class extends Controller<HTMLElement> {
  static override targets = ["dialog", "form", "to", "select"]
  declare readonly dialogTarget: HTMLDialogElement
  declare readonly formTarget: HTMLFormElement
  declare readonly toTarget: HTMLInputElement
  declare readonly selectTarget: HTMLSelectElement

  open(event: Event) {
    const id = (event as unknown as {params?: {id?: string}}).params?.id || ""
    if (!id) {
      return
    }
    this.formTarget.action = `/bookmarks/${id}/update`
    this.toTarget.value = window.location.pathname + window.location.search
    this.dialogTarget.showModal()
  }

  close() {
    this.dialogTarget.close()
  }
}
