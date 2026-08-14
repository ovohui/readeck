// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/* global Turbo */

import {Controller} from "@hotwired/stimulus"
import {request} from "../lib/request"
import {cspNonce, markStaleRefresh} from "../lib/turbo"

import type {ActionEvent} from "@hotwired/stimulus"
import type {requestOptions} from "../lib/request"

export default class extends Controller {
  static override values = {
    url: {type: String, default: ""},
    method: {type: String, default: ""},
    turbo: {type: Boolean, default: false},
  }

  declare urlValue: string
  declare methodValue: string
  declare turboValue: boolean

  async fetch(event: ActionEvent) {
    const src = event.params.url ? event.params.url : this.urlValue
    if (!src) {
      throw new Error("url param is not set")
    }

    const method = event.params.method ? event.params.method : this.methodValue
    const options: requestOptions = {
      method: method || "get",
      headers: {},
    }

    if (this.turboValue) {
      options.headers!["X-Turbo"] = "1"
      options.headers = {
        ...options.headers,
        "X-Turbo": "1",
        "X-Turbo-Nonce": cspNonce,
      }
    }

    if (event.currentTarget instanceof HTMLFormElement) {
      // Set the body based on the form elements when the event receiver
      // is a form.
      const form = event.currentTarget
      options.body = new FormData(form)
    } else if (
      event.currentTarget
      && "name" in event.currentTarget
      && "value" in event.currentTarget
      && event.currentTarget.name
      && event.currentTarget.value
    ) {
      // Otherwise, anything with a name and value attribute does the job.
      options.body = new FormData()
      options.body.set(event.currentTarget.name as string, event.currentTarget.value as string)
    }

    const rsp = await request(src, options)

    // Mark any unsafe request for refresh on back/forward navigation.
    if (
      rsp.ok
      && !["GET", "HEAD", "OPTIONS"].includes(options.method!.toUpperCase())
    ) {
      markStaleRefresh()
    }

    if (this.turboValue) {
      Turbo.renderStreamMessage(await rsp.text())
    }

    this.dispatch(event.params.eventName || "done")
  }
}
