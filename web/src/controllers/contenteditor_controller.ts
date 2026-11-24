// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

type CommandParams = {command?: string; value?: string}

/**
 * In-place editor for the archived article body.
 *
 * The article is shown as it reads (same `prose` typography as the reader) in a
 * `contenteditable` block. What gets submitted is still plain HTML: the form
 * posts the hidden textarea, which is refreshed from the editable block on
 * submit. A source mode exposes the raw markup for the rare case where a tag
 * has to be edited by hand.
 */
export default class extends Controller {
  static override targets = ["surface", "source", "sourceButton"]
  static classes = ["hidden"]

  declare readonly surfaceTarget: HTMLElement
  declare readonly sourceTarget: HTMLTextAreaElement
  declare readonly sourceButtonTarget: HTMLElement
  declare readonly hiddenClasses: string[]

  private sourceVisible = false
  private toolbarButtons: HTMLElement[] = []
  private savedRange: Range | null = null

  override connect() {
    this.sourceTarget.classList.add(...this.hiddenClasses)
    // Keep the archive clean: pasted rich text brings inline styles, font
    // families and tracking attributes that make the article unreadable on
    // another device. Plain text keeps the structure of what is being read.
    this.surfaceTarget.addEventListener("paste", this.onPaste)

    // A toolbar button must not take the focus away from the editable block:
    // the selection would collapse and execCommand would apply to nothing.
    this.toolbarButtons = Array.from(this.element.querySelectorAll(".editor-bar button"))
    for (const btn of this.toolbarButtons) {
      btn.addEventListener("mousedown", keepFocus)
    }
    this.sync()
  }

  override disconnect() {
    this.surfaceTarget.removeEventListener("paste", this.onPaste)
    for (const btn of this.toolbarButtons) {
      btn.removeEventListener("mousedown", keepFocus)
    }
  }

  private onPaste = (evt: ClipboardEvent) => {
    const text = evt.clipboardData?.getData("text/plain")
    if (text == null) {
      return
    }
    evt.preventDefault()
    document.execCommand("insertText", false, text ?? "")
    this.sync()
  }

  /** Runs a `document.execCommand` given by the toolbar button. */
  command(evt: Event) {
    const {command, value} = (evt as Event & {params: CommandParams}).params
    if (!command) {
      return
    }
    this.surfaceTarget.focus()
    document.execCommand(command, false, value)
    this.sync()
  }

  /** Wraps the selection in a link, or inserts one when nothing is selected. */
  link(evt: Event) {
    const {value} = (evt as Event & {params: CommandParams}).params
    this.saveSelection()
    const url = window.prompt("链接地址", value ?? "https://")
    if (!url) {
      return
    }
    this.restoreSelection()
    const selection = window.getSelection()
    const hasText = selection != null && !selection.isCollapsed
    this.surfaceTarget.focus()
    if (hasText) {
      document.execCommand("createLink", false, url)
    } else {
      document.execCommand("insertHTML", false, `<a href="${escapeAttr(url)}">${escapeHTML(url)}</a>`)
    }
    this.sync()
  }

  /** Inserts an image, either by URL or by one of the archive's resources. */
  image(evt: Event) {
    const {value} = (evt as Event & {params: CommandParams}).params
    this.saveSelection()
    const url = window.prompt("图片地址（存档里的图片是 ./_resources/…）", value ?? "./_resources/")
    if (!url) {
      return
    }
    this.restoreSelection()
    this.surfaceTarget.focus()
    document.execCommand("insertHTML", false, `<img src="${escapeAttr(url)}" alt="">`)
    this.sync()
  }

  /** Inserts an <hr> separator. */
  rule() {
    this.surfaceTarget.focus()
    document.execCommand("insertHorizontalRule")
    this.sync()
  }

  /** Switches between the readable view and the raw HTML source. */
  toggleSource() {
    this.sourceVisible = !this.sourceVisible
    if (this.sourceVisible) {
      this.sourceTarget.value = this.surfaceTarget.innerHTML
      this.sourceTarget.classList.remove(...this.hiddenClasses)
      this.surfaceTarget.classList.add(...this.hiddenClasses)
      this.sourceButtonTarget.setAttribute("aria-pressed", "true")
      this.sourceTarget.focus()
    } else {
      this.surfaceTarget.innerHTML = this.sourceTarget.value
      this.sourceTarget.classList.add(...this.hiddenClasses)
      this.surfaceTarget.classList.remove(...this.hiddenClasses)
      this.sourceButtonTarget.setAttribute("aria-pressed", "false")
      this.sync()
    }
  }

  /** Keeps the submitted field in sync with the visible block. */
  sync() {
    if (!this.sourceVisible) {
      this.sourceTarget.value = this.surfaceTarget.innerHTML
    }
  }

  /** Remembers the caret position, which window.prompt() would lose. */
  private saveSelection() {
    const selection = window.getSelection()
    if (selection && selection.rangeCount > 0 && this.surfaceTarget.contains(selection.anchorNode)) {
      this.savedRange = selection.getRangeAt(0).cloneRange()
    }
  }

  private restoreSelection() {
    const selection = window.getSelection()
    if (!selection || !this.savedRange) {
      return
    }
    selection.removeAllRanges()
    selection.addRange(this.savedRange)
  }

  /** Ctrl/⌘+S saves without reaching for the button. */
  keydown(evt: KeyboardEvent) {
    if (!(evt.ctrlKey || evt.metaKey) || evt.key.toLowerCase() != "s") {
      return
    }
    evt.preventDefault()
    this.sync()
    if (this.element instanceof HTMLFormElement) {
      this.element.requestSubmit()
    }
  }
}

function escapeHTML(s: string) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
}

function escapeAttr(s: string) {
  return escapeHTML(s).replace(/"/g, "&quot;")
}

function keepFocus(evt: MouseEvent) {
  evt.preventDefault()
}
