// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import {visit} from "@hotwired/turbo"

import type {TurboSubmitEndEvent, VisitOptions} from "@hotwired/turbo"

// This controller listens for turbo:submit-end events
// and reload the frame with the ID given by data-turbo-reload-frame-id-value
// attribute by reloading the current page when the form is submited.
// It obviously applies only to form elements.
export default class extends Controller<HTMLFormElement> {
  static override values = {
    frameId: String,
  }

  declare hasFrameIdValue: boolean
  declare frameIdValue: string

  override connect() {
    document.addEventListener("turbo:submit-end", this.reloadFrame.bind(this))
  }

  override disconnect() {
    document.removeEventListener(
      "turbo:submit-end",
      this.reloadFrame.bind(this),
    )
  }

  reloadFrame(evt: TurboSubmitEndEvent) {
    if (evt.target != this.element) {
      return
    }

    const options: VisitOptions = {action: "replace"}
    if (this.hasFrameIdValue) {
      options.frame = this.frameIdValue
    }

    visit(document.location.href, options)
  }
}
