// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * Xiaohongshu (小红书) renders its note slider with JavaScript: the HTML we
 * fetch contains the note text but no usable <img> (the counter "1/15" is
 * there, the pictures are not). The real URLs only exist in the
 * `window.__INITIAL_STATE__` payload embedded in a <script> tag.
 *
 * This script extracts those URLs and injects real <img> elements, plus a
 * <video> for live photos (动图), before the DOM is handed over to
 * go-readability.
 *
 * @returns {boolean}
 */
exports.isActive = function () {
  return /xiaohongshu\.com$/.test($.domain)
}

const MEDIA_HOST = /(xhscdn\.com|sns-webpic|xhsimg\.com)/

/**
 * Removes the JSON escaping that appears in inline scripts: "\/" -> "/",
 * "\u002F" -> "/", "\u0026" -> "&".
 *
 * @param {string} s
 * @returns {string}
 */
function unescapeJSON(s) {
  return s
    .replace(/\\u002[fF]/g, "/")
    .replace(/\\u0026/g, "&")
    .replace(/\\\//g, "/")
    .replace(/\\u003[dD]/g, "=")
}

/**
 * Returns the text of every inline script of the document.
 *
 * @param {Node} document
 * @returns {string[]}
 */
function scriptTexts(document) {
  const out = []
  document.querySelectorAll("script").forEach((s) => {
    const txt = s.textContent
    if (txt && txt.length > 1000) {
      out.push(txt)
    }
  })
  return out
}

/**
 * Walks any parsed value and collects image/video URLs.
 *
 * @param {any} node
 * @param {{images: string[], videos: string[]}} acc
 * @param {number} depth
 */
function walk(node, acc, depth) {
  if (node == null || depth > 12) {
    return
  }
  if (Array.isArray(node)) {
    node.forEach((n) => walk(n, acc, depth + 1))
    return
  }
  if (typeof node != "object") {
    return
  }
  Object.keys(node).forEach((k) => {
    const v = node[k]
    if (typeof v == "string") {
      const url = unescapeJSON(v)
      if (!/^https?:\/\//.test(url) || !MEDIA_HOST.test(url)) {
        return
      }
      if (/\.(mp4|m3u8)(\?|$)/.test(url) || k == "masterUrl" || k == "originVideoKey") {
        acc.videos.push(url)
      } else {
        acc.images.push(url)
      }
      return
    }
    walk(v, acc, depth + 1)
  })
}

/**
 * Extracts the `noteDetailMap` JSON object from the page scripts.
 *
 * @param {string[]} texts
 * @returns {any|null}
 */
function parseNoteDetail(texts) {
  for (let t = 0; t < texts.length; t++) {
    const txt = texts[t]
    const idx = txt.indexOf('"noteDetailMap"')
    if (idx < 0) {
      continue
    }
    const start = txt.indexOf("{", idx)
    if (start < 0) {
      continue
    }
    // Brace matching, ignoring braces inside strings.
    let depth = 0
    let inStr = false
    let esc = false
    for (let i = start; i < txt.length; i++) {
      const c = txt[i]
      if (inStr) {
        if (esc) {
          esc = false
        } else if (c == "\\") {
          esc = true
        } else if (c == '"') {
          inStr = false
        }
        continue
      }
      if (c == '"') {
        inStr = true
      } else if (c == "{") {
        depth++
      } else if (c == "}") {
        depth--
        if (depth == 0) {
          const raw = txt.slice(start, i + 1)
          try {
            return JSON.parse(raw)
          } catch (e) {
            return null
          }
        }
      }
    }
  }
  return null
}

/**
 * Last resort: scrape every media URL from the inline scripts.
 *
 * @param {string[]} texts
 * @param {{images: string[], videos: string[]}} acc
 */
function scrapeURLs(texts, acc) {
  const rx = /https?:[^"'\\\s]{6,}/g
  texts.forEach((txt) => {
    const clean = unescapeJSON(txt)
    let m
    while ((m = rx.exec(clean)) != null) {
      const url = m[0]
      if (!MEDIA_HOST.test(url)) {
        continue
      }
      if (/\.(mp4|m3u8)(\?|$)/.test(url)) {
        acc.videos.push(url)
      } else if (/\.(jpe?g|png|webp|gif)(\?|!|$)/i.test(url)) {
        acc.images.push(url)
      }
    }
  })
}

/**
 * @param {string[]} urls
 * @param {number} limit
 * @returns {string[]}
 */
function unique(urls, limit) {
  const seen = {}
  const out = []
  urls.forEach((u) => {
    if (seen[u] || /avatar|emoji|fe-platform|logo/i.test(u)) {
      return
    }
    seen[u] = true
    out.push(u)
  })
  return out.slice(0, limit)
}

/**
 *  @param {Node} document
 */
exports.documentReady = function (document) {
  const texts = scriptTexts(document)
  if (texts.length == 0) {
    return
  }

  const acc = {images: [], videos: []}
  let state = null
  try {
    state = parseNoteDetail(texts)
  } catch (e) {
    state = null
  }
  if (state != null) {
    walk(state, acc, 0)
  }
  if (acc.images.length == 0) {
    scrapeURLs(texts, acc)
  }

  const images = unique(acc.images, 40)
  const videos = unique(acc.videos, 5)
  if (images.length == 0 && videos.length == 0) {
    console.debug("xiaohongshu: no media found in initial state")
    return
  }

  // The media must land inside the element go-readability will keep. Note
  // pages nest the text in .note-content (or #detail-desc), which is a
  // *child* of #noteContainer: appending to the wrapper would leave the
  // figures outside the extracted body, which is why they disappeared.
  const candidates = [
    ".note-content",
    "#detail-desc",
    ".note-scroller",
    "#noteContainer",
    "article",
  ]
  let container = null
  for (let i = 0; i < candidates.length; i++) {
    container = document.querySelector(candidates[i])
    if (container) {
      break
    }
  }
  if (!container) {
    container = document.body
  }

  const media = document.createElement("div")
  media.setAttribute("class", "xhs-media")

  const addVideo = (url) => {
    const figure = document.createElement("figure")
    figure.setAttribute("class", "content")
    const video = document.createElement("video")
    video.setAttribute("src", url)
    video.setAttribute("controls", "")
    video.setAttribute("playsinline", "")
    video.setAttribute("preload", "metadata")
    figure.appendChild(video)
    media.appendChild(figure)
  }

  images.forEach((url) => {
    const figure = document.createElement("figure")
    figure.setAttribute("class", "content")
    const img = document.createElement("img")
    img.setAttribute("src", url)
    img.setAttribute("alt", $.title || "")
    figure.appendChild(img)
    media.appendChild(figure)
  })

  videos.forEach(addVideo)
  container.appendChild(media)

  console.debug("xiaohongshu: injected media", {
    images: images.length,
    videos: videos.length,
  })
}
