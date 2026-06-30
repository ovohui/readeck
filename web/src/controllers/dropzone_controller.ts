// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["zone", "placeholder", "fileinfo", "clearbtn"]
  static classes = ["hidden", "focus"]

  declare readonly zoneTargets: HTMLElement[]
  declare readonly placeholderTargets: HTMLElement[]
  declare readonly fileinfoTargets: HTMLElement[]
  declare readonly clearbtnTargets: HTMLButtonElement[]

  declare readonly hiddenClass: string
  declare readonly focusClass: string

  declare actions: [Node, string, (e: Event) => void][]

  override connect() {
    this.actions = []

    if (!this.input) {
      return
    }

    this.input.removeAttribute("required")
    this.actions = [
      [this.input, "change", () => this.setFileInfo()],
      [this.element, "dragenter", (e) => this.enter(e as DragEvent)],
      [this.element, "dragleave", (e) => this.leave(e as DragEvent)],
      [this.element, "drop", (e) => this.drop(e as DragEvent)],
      [document, "dragover", (e) => this.noop(e as DragEvent)],
      [document, "dragleave", (e) => this.noop(e as DragEvent)],
      [document, "drop", (e) => this.noop(e as DragEvent)],
    ]

    this.actions.forEach((a) => {
      a[0].addEventListener(a[1], a[2])
    })
    this.setFileInfo()
  }

  override disconnect() {
    this.actions.forEach((a) => {
      a[0].removeEventListener(a[1], a[2])
    })
  }

  get input() {
    return this.scope.findElement("input[type=file]") as HTMLInputElement
  }

  setFileInfo() {
    this.zoneTargets.forEach((e) => {
      e.classList.remove(this.focusClass)
    })

    const files = this.input.files
    if (files?.length == 1) {
      this.placeholderTargets.forEach((e) => e.classList.add(this.hiddenClass))
      this.clearbtnTargets.forEach((e) => e.classList.remove(this.hiddenClass))

      this.fileinfoTargets.forEach((e) => {
        e.innerText = `${files[0].name} (${formatBytes(files[0].size)})`
      })
      return
    }

    this.placeholderTargets.forEach((e) => e.classList.remove(this.hiddenClass))
    this.clearbtnTargets.forEach((e) => e.classList.add(this.hiddenClass))
    this.fileinfoTargets.forEach((e) => {
      e.innerText = ""
    })
  }

  clear() {
    this.input.value = ""
    this.input.dispatchEvent(
      new Event("input", {bubbles: true, cancelable: true}),
    )
    this.input.dispatchEvent(
      new Event("change", {bubbles: true, cancelable: true}),
    )
  }

  select() {
    this.input.click()
  }

  enter(evt: DragEvent) {
    evt.preventDefault()
    this.zoneTargets.forEach((e) => {
      e.classList.add(this.focusClass)
    })
  }

  leave(evt: DragEvent) {
    evt.preventDefault()
    this.zoneTargets.forEach((e) => {
      if (evt.relatedTarget instanceof Node && !e.contains(evt.relatedTarget)) {
        e.classList.remove(this.focusClass)
      }
    })
  }

  drop(evt: DragEvent) {
    evt.preventDefault()
    if (evt.dataTransfer?.files.length != 1) {
      return
    }
    const input = this.input
    if (input) {
      input.files = evt.dataTransfer.files
      input.dispatchEvent(new Event("input", {bubbles: true, cancelable: true}))
      input.dispatchEvent(
        new Event("change", {bubbles: true, cancelable: true}),
      )
    }
  }

  noop(evt: Event) {
    evt.preventDefault()
  }
}

function formatBytes(bytes: number, decimals = 2) {
  if (!+bytes) return "0 Bytes"

  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = [
    "Bytes",
    "KiB",
    "MiB",
    "GiB",
    "TiB",
    "PiB",
    "EiB",
    "ZiB",
    "YiB",
  ]

  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}
