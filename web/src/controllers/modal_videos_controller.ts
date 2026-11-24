// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["dialog", "trigger", "template"]
  static override values = {
    selector: {
      type: String,
      default: "",
    },
    baseUrl: {
      type: String,
      default: "",
    },
  }

  declare readonly dialogTarget: HTMLDialogElement
  declare readonly hasDialogTarget: boolean
  declare readonly imgTarget: HTMLImageElement
  declare readonly triggerTarget: HTMLTemplateElement

  declare selectorValue: string
  declare baseUrlValue: string

  override connect() {
    this.#prepareInlineVideos()

    if (!this.selectorValue || !this.hasDialogTarget) {
      return
    }

    this.element
      .querySelectorAll(this.selectorValue)
      .forEach((e) => this.#addTrigger(e as HTMLElement))
    this.dialogTarget.addEventListener("close", () => {
      this.#clearModal()
    })
    this.dialogTarget.addEventListener("click", () => {
      this.dialogTarget.close()
    })
  }

  // turbo:morph-element event handler
  reload(evt: Event) {
    if (evt.target == this.element) {
      this.disconnect()
      this.connect()
    }
  }

  /**
   * Archived videos (downloaded by the server with yt-dlp) are plain <video
   * controls> elements. The native control bar is painted inside the frame, so
   * it covers the bottom of the picture - on a vertical video that is exactly
   * where the subtitles are. It cannot be hidden or moved: it belongs to the
   * player, not to the page.
   *
   * So the bar is removed until the video is actually started: the element
   * shows its poster (a clean cover frame) and a play badge, and the controls
   * are put back on the first click. The stored HTML keeps its controls
   * attribute, so an exported page still plays without JavaScript.
   */
  #prepareInlineVideos() {
    this.element.querySelectorAll("video").forEach((node) => {
      const video = node as HTMLVideoElement
      if (video.dataset.inlineReady == "1" || !video.hasAttribute("controls")) {
        return
      }
      const parent = video.parentNode
      if (!parent) {
        return
      }

      video.dataset.inlineReady = "1"
      video.removeAttribute("controls")
      video.setAttribute("playsinline", "")

      const wrapper = document.createElement("div")
      wrapper.className = "embedded-video"
      parent.insertBefore(wrapper, video)
      wrapper.appendChild(video)

      wrapper.addEventListener("click", (evt) => {
        evt.preventDefault()
        wrapper.classList.add("is-playing")
        video.setAttribute("controls", "")
        const playing = video.play()
        if (playing) {
          playing.catch(() => {})
        }
      })
    })
  }

  get #iframe() {
    return this.dialogTarget.querySelector("iframe")
  }

  #clearModal() {
    const el = this.#iframe
    el?.removeAttribute("src")
    el?.removeAttribute("width")
    el?.removeAttribute("height")
  }

  #addTrigger(e: HTMLElement) {
    const params = new URLSearchParams(e.dataset.iframeParams)
    const w = params.get("w") || 0
    const h = params.get("h") || 0
    if (w == 0 || h == 0) {
      return
    }

    const node = this.triggerTarget.content.cloneNode(true) as DocumentFragment
    const wrapper = node.firstElementChild!

    e.parentNode?.insertBefore(wrapper, e)
    wrapper.appendChild(e)

    wrapper.querySelector("button")?.addEventListener("click", () => {
      const params = new URLSearchParams(e.dataset.iframeParams)

      const w = params.get("w") || "0"
      const h = params.get("h") || "0"

      const src = new URL(this.baseUrlValue, document.location.toString())
      params.forEach((v, k) => {
        src.searchParams.set(k, v)
      })

      const ifr = this.#iframe
      ifr?.setAttribute("src", src.toString())
      ifr?.setAttribute("width", w)
      ifr?.setAttribute("height", h)

      this.dialogTarget.showModal()
      this.dialogTarget.style.aspectRatio = `${w}/${h}`
    })
  }
}
