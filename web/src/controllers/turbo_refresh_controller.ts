// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import type {FrameElement} from "@hotwired/turbo"

// This controller reload a given turbo-frame at a given interval
// until it find a target named "loaded" in its content.
// This replaces a meta refresh of the full page and can be used on
// several frames on the same page.
export default class extends Controller<FrameElement> {
  static override values = {
    // A CSS selector that triggers the refresh when present
    on: String,
    // The page source to load, uses window.location if none
    src: String,
    // Refresh every given seconds
    interval: Number,
  }

  declare hasSrcValue: boolean
  declare onValue: string
  declare srcValue: string
  declare intervalValue: number

  declare observer: MutationObserver | null
  declare timeout: number | null

  override connect() {
    if (!this.onValue) {
      throw new Error(
        `you must set data-${this.identifier}-on-value on the component`,
      )
    }

    if (!this.hasSrcValue) {
      this.srcValue = window.location.href
    }

    // We need this to check the selector on every mutation
    this.timeout = null

    if (!this.isLoaded()) {
      this.check()
    }

    this.observer = new MutationObserver(() => this.check())
    this.observer.observe(this.element, {
      attributes: true,
      childList: true,
      subtree: true,
    })
  }

  override disconnect() {
    this.observer?.disconnect()
  }

  async check() {
    await this.element.loaded
    if (this.isLoaded()) {
      return
    }
    if (this.timeout !== null) {
      return
    }

    this.timeout = window.setTimeout(async () => {
      this.element.src = this.srcValue
      await this.element.loaded

      this.element.src = ""
      this.timeout = null
    }, this.intervalValue * 1000)
  }

  isLoaded() {
    return this.element.querySelector(this.onValue) === null
  }
}
