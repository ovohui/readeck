// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import {FrameElement} from "@hotwired/turbo"

import {request} from "../lib/request"

interface selectionPayload {
  start_selector: string | null
  start_offset: number | null
  end_selector: string | null
  end_offset: number | null
}

export default class extends Controller {
  static override targets = [
    "root",
    "controls",
    "color",
    "note",
    "arrow",
    "whenNew",
    "whenOne",
    "whenMany",
  ]

  static classes = ["hidden"]
  static override values = {
    apiUrl: String,
    color: String,
  }

  declare readonly hasNoteTarget: boolean
  declare readonly hasArrowTarget: boolean

  declare readonly rootTarget: HTMLElement
  declare readonly controlsTarget: HTMLElement
  declare readonly colorTargets: HTMLInputElement[]
  declare readonly noteTarget: HTMLTextAreaElement
  declare readonly arrowTarget: HTMLElement
  declare readonly whenNewTargets: HTMLElement[]
  declare readonly whenOneTargets: HTMLElement[]
  declare readonly whenManyTargets: HTMLElement[]

  declare apiUrlValue: string
  declare colorValue: string

  declare hiddenClass: string

  declare annotation: Annotation | null
  declare payload: selectionPayload | null
  declare currentIDs: string[]

  override connect() {
    this.annotation = null
    this.payload = null
    this.currentIDs = []

    this.colorTargets.forEach((e) => {
      e.addEventListener("click", (evt) => {
        this.colorValue = (evt.target as HTMLButtonElement).value
      })
    })

    selectionendObserver(document, () => {
      this.onSelectText()
    })

    // Hide the controls when a click took place outside the controls.
    document.addEventListener("click", (evt) => {
      if (
        this.controlsTarget.classList.contains(this.hiddenClass)
        || ((evt.target instanceof Element) && this.controlsTarget.contains(evt.target))
      ) {
        return
      }
      setTimeout(() => {
        if (document.getSelection()?.isCollapsed) {
          this.controlsTarget.classList.add(this.hiddenClass)
        }
      }, 0)
    })
  }

  colorValueChanged(value: string, previous: string) {
    // Initial value
    if (value === "" && previous === undefined) {
      this.colorTargets[0].checked = true
      this.colorValue = this.colorTargets[0].value
      return
    }

    // When the frame is restored
    if (value === "" && !!previous) {
      this.colorTargets.forEach((e) => {
        if (e.value == previous) {
          e.checked = true
          return
        }
      })
      this.colorValue = previous
      return
    }
  }

  /**
   * onSelectText is the listener for "selectionchange".
   */
  onSelectText() {
    const selection = document.getSelection()
    if (!selection) {
      return
    }

    this.annotation = new Annotation(this.rootTarget, selection)

    if (
      this.annotation.isValid()
      || this.annotation.coveredAnnotations().length > 0
    ) {
      this.showControls()
    }

    if (this.annotation.isValid()) {
      /* eslint-disable camelcase -- request JSON payload */
      this.payload = {
        start_selector: this.annotation.startSelector,
        start_offset: this.annotation.startOffset,
        end_selector: this.annotation.endSelector,
        end_offset: this.annotation.endOffset,
      }
      /* eslint-enable */
    }
  }

  showControls() {
    this.currentIDs = []
    this.iterCoveredAnnotations((id: string) => {
      this.currentIDs.push(id)
    })

    // Show controls
    this.controlsTarget.classList.remove(this.hiddenClass)

    /**
     * Hide the controls on Escape.
     *
     * @param {KeyboardEvent} evt
     */
    const onEsc = (evt: KeyboardEvent) => {
      if (evt.key == "Escape") {
        this.controlsTarget.classList.add(this.hiddenClass)
        document.getSelection()?.removeAllRanges()
        document.removeEventListener("keyup", onEsc)
      }
    }

    document.addEventListener("keyup", onEsc)

    const covered = this.annotation?.coveredAnnotations()
    const selected = [
      ...new Set(covered?.map((e) => e!.dataset.annotationIdValue)),
    ]

    // Show/hide sub controls
    switch (selected.length) {
      case 0:
        this.#hide(this.whenOneTargets)
        this.#hide(this.whenManyTargets)
        this.#show(this.whenNewTargets)
        break
      case 1:
        this.#hide(this.whenNewTargets)
        this.#hide(this.whenManyTargets)
        this.#show(this.whenOneTargets)
        break
      default:
        this.#hide(this.whenNewTargets)
        this.#hide(this.whenOneTargets)
        this.#show(this.whenManyTargets)
    }

    // Set color and text when annotation exists
    switch (selected.length) {
      case 0:
        this.noteTarget.value = ""
        break
      // @ts-expect-error: we need to fall through
      case 1: {
        // One selected annotation, check if it has a note
        const e = Array.from(
          this.rootTarget.querySelectorAll<HTMLElement>(
            "rd-annotation[data-annotation-note][title]",
          ),
        ).find((e) => e.dataset.annotationIdValue == selected[0])

        // set the note and fall through the next case (sets color)
        this.noteTarget.value = e?.getAttribute("title") || ""
      }
      // eslint-disable-next-line no-fallthrough
      default:
        this.colorTargets.forEach((e) => {
          if (e.value == covered?.[0].dataset.annotationColor) {
            e.checked = true
            this.colorValue = e.value
            return
          }
        })
    }

    this.#positionControls()
  }

  #positionControls() {
    // Get root, range and controlls coordinates
    const rangeRect = this.annotation?.range?.getBoundingClientRect()
    const rootRect = this.findRelativeRoot().getBoundingClientRect()

    if (!rangeRect) {
      return
    }

    switch (getComputedStyle(this.controlsTarget).position) {
      case "absolute": {
        // Controlls dimension
        const h = this.controlsTarget.clientHeight
        const w = this.controlsTarget.clientWidth

        // Default position
        let position = "top"
        if (rangeRect.top - 20 < h) {
          position = "bottom"
        }
        this.controlsTarget.dataset.position = position

        // Range position relative to its root element
        const rangeTop
          = position == "top"
            ? Math.round(rangeRect.top - rootRect.top)
            : Math.round(rangeRect.top + rangeRect.height - rootRect.top)
        const rangeLeft = Math.round(rangeRect.left - rootRect.left)
        const rangeCenter = Math.round(rangeLeft + rangeRect.width / 2)

        // Set controlls position
        const y = position == "top" ? Math.round(rangeTop - h) : rangeTop
        // prettier-ignore
        const x = Math.floor(
          Math.max(
            0,
            Math.min(
              rangeCenter - w / 2,
              rootRect.width - w - 1,
            ),
          ),
        )

        this.controlsTarget.style.top = `${position == "top" ? y - 4 : y + 4}px`
        this.controlsTarget.style.left = `${x}px`

        // Set arrow position
        if (!this.hasArrowTarget) {
          return
        }
        const arrowWidth = this.arrowTarget.offsetWidth
        // prettier-ignore
        const arrowX = Math.max(
          arrowWidth / 2,
          Math.min(
            rangeCenter - x - arrowWidth / 2,
            w - arrowWidth - arrowWidth / 2,
          ),
        )
        this.arrowTarget.style.marginLeft = `${arrowX}px`
        break
      }
      case "fixed":
        // On a fixed position, try not to cover the selection
        this.controlsTarget.style.bottom = "0"
        this.controlsTarget.style.top = "unset"
        if (rangeRect.top > this.controlsTarget.getBoundingClientRect().top) {
          this.controlsTarget.style.bottom = "unset"
          this.controlsTarget.style.top = "0"
        }
    }
  }

  #hide(nodes: Element[]) {
    nodes.forEach((e) => {
      e.classList.add(this.hiddenClass)
    })
  }

  #show(nodes: Element[]) {
    nodes.forEach((e) => {
      e.classList.remove(this.hiddenClass)
    })
  }

  #getNote() {
    if (!this.hasNoteTarget) {
      return
    }
    return this.noteTarget.value
  }

  findRelativeRoot() {
    let p = this.rootTarget
    while (p.parentElement) {
      if (getComputedStyle(p).position == "relative") {
        return p
      }
      p = p.parentElement
    }
    return p
  }

  /**
   * reload loads and replace the turbo frame content
   */
  reload = async () => {
    if (!(this.element instanceof FrameElement)) {
      throw new Error("controller element must be a turbo-frame")
    }

    // Enable turbo frame and wait for it to be reloaded.
    this.element.disabled = false
    window.getSelection()?.removeAllRanges()
    await this.element.loaded
  }

  /**
   * create creates a new annotation on the document
   */
  async create() {
    if (!this.payload) {
      return
    }

    const body = {
      ...this.payload,
      color: this.colorValue,
      note: this.#getNote(),
    }

    await request(this.apiUrlValue, {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify(body),
    })
    await this.reload()
  }

  /**
   * update updates the selected annotations
   */
  async update() {
    if (this.currentIDs.length == 0) {
      return
    }

    // Common body
    const body: {
      color: string
      note?: string
    } = {color: this.colorValue}

    // Can only change the note when there's one updated item
    if (this.currentIDs.length == 1) {
      body.note = this.#getNote()
    }

    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)

    for (const id of this.currentIDs) {
      await request(new URL(id, baseURL).toString(), {
        method: "PATCH",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body),
      })
    }
    await this.reload()
  }

  /**
   * removes the selected annotations
   */
  async delete() {
    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)

    for (const id of this.currentIDs) {
      await request(new URL(id, baseURL).toString(), {method: "DELETE"})
    }
    await this.reload()
  }

  /**
   * iterCoveredAnnotations executes a function on each annotation in the
   * current selection.
   */
  iterCoveredAnnotations(fn: (s: string) => void) {
    const ids: Set<string> = new Set()
    this.annotation?.coveredAnnotations().forEach((n) => {
      if (n && n.dataset.annotationIdValue) {
        ids.add(n.dataset.annotationIdValue)
      }
    })

    for (const id of ids) {
      fn(id)
    }
  }
}

class Annotation {
  /**
   * Annotation holds raw information about an annotation. It contains only text with
   * selectors and offsets providing needed information to find an annotation.
   */
  root: Node
  selection: Selection
  range: Range | null
  textNodes: Node[]
  ancestor: Node | null
  startSelector: string | null
  startOffset: number | null
  endSelector: string | null
  endOffset: number | null
  color: string | null

  constructor(root: Node, selection: Selection) {
    this.root = root
    this.selection = selection
    this.range = null
    this.textNodes = []
    this.ancestor = null
    this.startSelector = null
    this.startOffset = null
    this.endSelector = null
    this.endOffset = null
    this.color = null

    this.init()
  }

  init() {
    // Selection must be a range and contains something
    if (
      this.selection.type.toLowerCase() != "range"
      || !this.selection.toString().trim()
    ) {
      return
    }

    // Only one range
    if (this.selection.rangeCount != 1) {
      return
    }

    const range = this.selection.getRangeAt(0)
    if (range.collapsed || !this.root.contains(range.startContainer)) {
      return
    }

    // After triple-clicking an element (as opposed to dragging the pointer) to select text, the
    // start and/or end container nodes may be elements. We normalize the range to always start and
    // stop on text nodes.
    if (
      range.startContainer.nodeType == Node.ELEMENT_NODE
      && range.endContainer.nodeType == Node.ELEMENT_NODE
      && range.endOffset > 0
    ) {
      // This is meant to detect behavior seemingly exclusive to Firefox/Gecko where the start and
      // end container are both elements and startOffset/endOffset values count child nodes
      // instead of characters. https://bugzilla.mozilla.org/show_bug.cgi?id=516782
      const endNode = range.endContainer.childNodes[range.endOffset - 1]
      const tw = textNodeWalker(endNode)
      while (tw.nextNode()) {
        //
      }
      if (tw.currentNode.nodeType == Node.TEXT_NODE) {
        range.setEnd(tw.currentNode, tw.currentNode?.textContent?.length || 0)
      } else {
        // No text nodes were found at the node where the selection ends.
        range.setEnd(endNode, 0)
      }
    }

    // In Gecko, startContainer may be an element, in which case we advance the start of the
    // selection to the first text node it contains.
    if (range.startContainer.nodeType != Node.TEXT_NODE) {
      const n = textNodeWalker(
        range.commonAncestorContainer,
        range.startContainer,
      ).nextNode()
      if (n) {
        range.setStart(n, 0)
      }
    }

    // In Chromium and WebKit, triple-clicking the final element in the article creates a selection
    // where endContainer is is the next interactive element in DOM order, which might be outside of
    // the article root element. This normalizes the selection to end at the last text node that is
    // contained in root.
    if (
      range.endContainer.nodeType != Node.TEXT_NODE
      || !this.root.contains(range.endContainer)
    ) {
      const tw = textNodeWalker(
        range.commonAncestorContainer,
        range.endContainer,
      )
      let n = tw.previousNode()
      while (n && !this.root.contains(n)) {
        n = tw.previousNode()
      }
      if (n) {
        range.setEnd(n, n.textContent?.length || 0)
      }
    }

    this.range = range
    const s = getSelector(
      this.root,
      this.range.startContainer,
      this.range.startOffset,
    )
    const e = getSelector(
      this.root,
      this.range.endContainer,
      this.range.endOffset,
    )
    this.startSelector = s.selector
    this.startOffset = s.offset
    this.endSelector = e.selector
    this.endOffset = e.offset

    // Get ancestor
    if (this.range.commonAncestorContainer.nodeType == Node.TEXT_NODE) {
      this.ancestor = this.range.commonAncestorContainer.parentElement
    } else {
      this.ancestor = this.range.commonAncestorContainer
    }

    // Collect included text nodes
    let started = false
    let done = false
    if (this.ancestor) {
      walkTextNodes(this.ancestor, (n) => {
        if (done) {
          return
        }
        if (n == this.range?.startContainer) {
          started = true
        }
        if (started) {
          this.textNodes.push(n)
        }
        if (n == this.range?.endContainer) {
          done = true
        }
      })
    }
  }

  coveredAnnotations(): HTMLElement[] {
    return this.textNodes
      .filter((n) => n.parentElement && n.parentElement.tagName.toLowerCase() == "rd-annotation")
      .map((n) => n.parentElement!)
  }

  /**
   * @returns {Boolean}
   */
  isValid() {
    return this.range != null && this.coveredAnnotations().length == 0
  }
}

/**
 * getSelector returns a CSS selector for a text node at the given offset.
 */
function getSelector(root: Node, node: Node, offset: number): {selector: string, offset: number} {
  let p = node.parentElement
  const names = []

  // Get selector
  while (p?.parentElement && p != root) {
    let i = 1
    let s = p as Element
    while (s?.previousElementSibling) {
      s = s.previousElementSibling
      if (s.tagName.toLowerCase() == p.tagName.toLowerCase()) {
        i++
      }
    }
    names.unshift(`${p.tagName.toLowerCase()}[${i}]`)

    p = p.parentElement
  }

  // Get offset
  let done = false
  let newOffset = 0
  walkTextNodes(node.parentElement, (n) => {
    if (done) {
      return
    }
    if (n == node) {
      done = true
    }

    // We need to find the rune-based offset because that's how
    // Readeck saves and renders highlights.
    // https://codeberg.org/readeck/readeck/issues/1205
    if (!done) {
      newOffset += [...n.textContent || ""].length
    } else if (n.textContent) {
      newOffset += [...n.textContent.substring(0, offset)].length
    } else {
      newOffset += offset
    }
  })

  return {selector: names.join("/"), offset: newOffset}
}

/**
 * walkTextNodes calls a callback function on each text node
 * found in a given node and its descendants.
 */
function walkTextNodes(node: Node | null, callback: (n: Node, i: number) => void, index?: number) {
  if (node === null) {
    return
  }

  index = index === undefined ? 0 : index
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.nodeType == Node.TEXT_NODE) {
      callback(child, index)
      index++
    }
    walkTextNodes(child, callback, index)
  }
}

/**
 * textNodeWalker creates a TreeWalker scoped to root, starting at currentNode, and configured to
 * yield only text nodes that aren't whitespace-only.
 */
function textNodeWalker(root: Node, currentNode: Node | null = null): TreeWalker {
  const tw = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, (node) =>
    (node.textContent?.trim().length || 0) > 0
      ? NodeFilter.FILTER_ACCEPT
      : NodeFilter.FILTER_SKIP,
  )
  if (currentNode) {
    tw.currentNode = currentNode
  }
  return tw
}

/**
 * selectionendObserver reacts to the "selectionchange" event by triggering the
 * callback function only after the pointer device is not pressed anymore, at
 * which point the user has likely completed their text selection.
 */
function selectionendObserver(node: Node, callback: () => void) {
  /**
   * @type {function(any): void | null}
   */
  let selectionResolve: ((value?: unknown) => void) | null
  let pointerIsPressed = false

  node.addEventListener("pointerdown", () => {
    pointerIsPressed = true
  })

  node.addEventListener("pointerup", () => {
    pointerIsPressed = false
    if (selectionResolve) {
      selectionResolve()
      selectionResolve = null
    }
  })

  node.addEventListener("pointercancel", () => {
    pointerIsPressed = false
    if (selectionResolve) {
      selectionResolve()
      selectionResolve = null
    }
  })

  node.addEventListener("selectionchange", (evt) => {
    if (
      evt.target instanceof HTMLTextAreaElement
      || evt.target instanceof HTMLInputElement
    ) {
      return
    }
    if (!pointerIsPressed) {
      Promise.resolve().then(() => {
        callback()
      })
      return
    }
    if (selectionResolve != null) {
      return
    }
    new Promise((resolve) => {
      selectionResolve = resolve
    }).then(() => {
      callback()
    })
  })
}
