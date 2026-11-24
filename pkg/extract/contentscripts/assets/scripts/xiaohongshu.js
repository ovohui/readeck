// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * Xiaohongshu (小红书) renders its note with JavaScript: the server side HTML
 * carries the note text and an empty slider ("1/3" counter), but the pictures
 * only exist as URLs in the `window.__INITIAL_STATE__` payload embedded in a
 * <script> tag of the document head.
 *
 * This script runs on `documentLoaded`, i.e. BEFORE the article body is
 * extracted. That matters: `documentReady` runs after ExtractBody, when the
 * <head> (and therefore the state payload) has already been dropped, which is
 * why an earlier version of this script never found anything.
 *
 * It reads the state, then appends a flat list of <figure><img> and
 * <figure><video> elements inside the note container, so the archived note
 * keeps every image (and live photos) whatever the original layout was.
 *
 * @returns {boolean}
 */
exports.isActive = function () {
  return /xiaohongshu\.com$/.test($.domain)
}

var MEDIA_HOST = /(xhscdn\.com|xhsimg\.com)/

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
 * @param {Node} doc
 * @returns {string[]}
 */
function scriptTexts(doc) {
  var out = []
  var nodes = doc.querySelectorAll("script")
  for (var i = 0; i < nodes.length; i++) {
    var txt = nodes[i].textContent
    if (txt && txt.length > 200) {
      out.push(txt)
    }
  }
  return out
}

/**
 * Extracts the `noteDetailMap` JSON object from the page scripts.
 *
 * @param {string[]} texts
 * @returns {any|null}
 */
function parseNoteDetail(texts) {
  for (var t = 0; t < texts.length; t++) {
    var txt = texts[t]
    var idx = txt.indexOf('"noteDetailMap"')
    if (idx < 0) {
      continue
    }
    var start = txt.indexOf("{", idx)
    if (start < 0) {
      continue
    }
    // Brace matching, ignoring braces inside strings.
    var depth = 0
    var inStr = false
    var esc = false
    for (var i = start; i < txt.length; i++) {
      var c = txt.charAt(i)
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
          var raw = txt.slice(start, i + 1)
          // The state is serialized by React: it can contain bare
          // `undefined`, which is not valid JSON.
          raw = raw.replace(/:\s*undefined/g, ":null")
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
 * Walks any parsed value and returns every object that carries an imageList
 * (i.e. a note).
 *
 * @param {any} node
 * @param {any[]} out
 * @param {number} depth
 */
function findNotes(node, out, depth) {
  if (node == null || typeof node != "object" || depth > 10) {
    return
  }
  if (Array.isArray(node)) {
    for (var i = 0; i < node.length; i++) {
      findNotes(node[i], out, depth + 1)
    }
    return
  }
  if (Array.isArray(node.imageList) && node.imageList.length > 0) {
    out.push(node)
  }
  var keys = Object.keys(node)
  for (var k = 0; k < keys.length; k++) {
    findNotes(node[keys[k]], out, depth + 1)
  }
}

/**
 * Picks the best image URL of an imageList entry.
 *
 * @param {any} item
 * @returns {string}
 */
function imageURL(item) {
  if (item.urlDefault) {
    return unescapeJSON(item.urlDefault)
  }
  if (item.urlPre) {
    return unescapeJSON(item.urlPre)
  }
  var list = item.infoList || []
  for (var i = 0; i < list.length; i++) {
    if (list[i].url) {
      return unescapeJSON(list[i].url)
    }
  }
  return ""
}

/**
 * Returns the master URL of a live photo (动图) or of a video note.
 *
 * @param {any} node
 * @returns {string}
 */
function streamURL(node) {
  if (!node) {
    return ""
  }
  var stream = node.media ? node.media.stream : node.stream
  if (!stream) {
    return ""
  }
  var codecs = ["h264", "h265", "av1"]
  for (var i = 0; i < codecs.length; i++) {
    var list = stream[codecs[i]] || []
    for (var j = 0; j < list.length; j++) {
      if (list[j].masterUrl) {
        return unescapeJSON(list[j].masterUrl)
      }
    }
  }
  return ""
}

/**
 * Last resort: scrape every media URL from the inline scripts.
 *
 * @param {string[]} texts
 * @param {{images: string[], videos: string[]}} acc
 */
function scrapeURLs(texts, acc) {
  var rx = /https?:[^"'\\\s]{6,}/g
  for (var t = 0; t < texts.length; t++) {
    var clean = unescapeJSON(texts[t])
    var m
    while ((m = rx.exec(clean)) != null) {
      var url = m[0]
      if (!MEDIA_HOST.test(url)) {
        continue
      }
      if (/\.(mp4|m3u8)(\?|$)/.test(url)) {
        acc.videos.push(url)
      } else if (/\.(jpe?g|png|webp|gif)(\?|!|$)/i.test(url)) {
        acc.images.push(url)
      }
    }
  }
}

/**
 * @param {string[]} urls
 * @param {number} limit
 * @returns {string[]}
 */
function unique(urls, limit) {
  var seen = {}
  var out = []
  for (var i = 0; i < urls.length; i++) {
    var u = urls[i]
    if (!/^https?:\/\//.test(u) || seen[u] || /avatar|emoji|logo|fe-platform/i.test(u)) {
      continue
    }
    seen[u] = true
    out.push(u)
  }
  return out.slice(0, limit)
}

/**
 * Detaches the empty slider placeholder produced by the server side render,
 * so the flat media list is not preceded by a "1/3" skeleton.
 *
 * @param {Node} container
 */
function removePlaceholders(container) {
  var selectors = ['[elementtiming="note-cover"]', ".swiper", ".note-slider", ".slider-container"]
  for (var i = 0; i < selectors.length; i++) {
    var nodes = container.querySelectorAll(selectors[i])
    for (var j = nodes.length - 1; j >= 0; j--) {
      var parent = nodes[j].parentNode
      if (parent) {
        parent.removeChild(nodes[j])
      }
    }
  }
}

/**
 * @param {Node} doc
 * @param {string} selector
 * @param {string} tag
 * @param {Object} attrs
 * @returns {Node}
 */
function element(doc, selector, tag, attrs) {
  var el = doc.createElement(tag)
  Object.keys(attrs).forEach(function (k) {
    el.setAttribute(k, attrs[k])
  })
  return el
}

/**
 *  @param {Node} document
 */
exports.documentLoaded = function (document) {
  var texts = scriptTexts(document)
  var acc = {images: [], videos: []}

  if (texts.length > 0) {
    var state = null
    try {
      state = parseNoteDetail(texts)
    } catch (e) {
      state = null
    }

    if (state != null) {
      var notes = []
      findNotes(state, notes, 0)
      for (var n = 0; n < notes.length; n++) {
        var note = notes[n]
        var list = note.imageList
        for (var i = 0; i < list.length; i++) {
          var url = imageURL(list[i])
          if (url) {
            acc.images.push(url)
          }
          // Live photo: the picture has a short video attached.
          var live = streamURL(list[i])
          if (live) {
            acc.videos.push(live)
          }
        }
        var video = streamURL(note.video)
        if (video) {
          acc.videos.push(video)
        }
      }
    }

    if (acc.images.length == 0) {
      scrapeURLs(texts, acc)
    }
  }

  var images = unique(acc.images, 40)
  var videos = unique(acc.videos, 6)

  if (images.length == 0 && videos.length == 0) {
    console.debug("xiaohongshu: no media in server side HTML", {
      scripts: texts.length,
      hasState: texts.join("").indexOf("noteDetailMap") >= 0,
    })
    return
  }

  // The media must live inside the element the site configuration extracts
  // (//div[@id='noteContainer']).
  var candidates = ["#noteContainer", ".note-scroller", "article", "body"]
  var container = null
  for (var c = 0; c < candidates.length; c++) {
    container = document.querySelector(candidates[c])
    if (container) {
      break
    }
  }
  if (!container) {
    container = document.body
  }

  removePlaceholders(container)

  var media = element(document, null, "div", {"class": "xhs-media"})

  for (var v = 0; v < videos.length; v++) {
    var figure = element(document, null, "figure", {"class": "content"})
    figure.appendChild(
      element(document, null, "video", {
        src: videos[v],
        controls: "",
        playsinline: "",
        preload: "metadata",
      }),
    )
    media.appendChild(figure)
  }

  for (var p = 0; p < images.length; p++) {
    var fig = element(document, null, "figure", {"class": "content"})
    fig.appendChild(
      element(document, null, "img", {
        src: images[p],
        alt: $.title || "",
      }),
    )
    media.appendChild(fig)
  }

  container.appendChild(media)

  console.debug("xiaohongshu: injected media", {
    images: images.length,
    videos: videos.length,
  })
}
