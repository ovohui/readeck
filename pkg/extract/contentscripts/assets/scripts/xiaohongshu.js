// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * Xiaohongshu (小红书) renders its note with JavaScript. The server side HTML
 * carries the note text and a skeleton container, but the pictures only exist
 * as URLs in the `window.__INITIAL_STATE__` payload embedded in a <script> tag
 * of the document head. Readability then keeps the *skeleton* - a slider
 * placeholder, the author row, the interaction bar - and throws the note
 * itself away.
 *
 * This script runs on `documentLoaded`, i.e. BEFORE the article body is
 * extracted. That matters: `documentReady` runs after ExtractBody, when the
 * <head> (and therefore the state payload) has already been dropped, which is
 * why an earlier version of this script never found anything.
 *
 * Rather than injecting a media block next to whatever readability left
 * behind, it rebuilds the note as a small, clean article - title line,
 * author/date/location line, the description as paragraphs, then the pictures
 * - and replaces the body with it. The archive then reads like the note page
 * does, and the same structure is what the browser extension produces, so a
 * note archived from either side looks the same.
 *
 * Video notes have no playable stream in the payload (the player asks a
 * signed API for it at runtime, and that URL expires). A player element would
 * be stripped from the extracted content anyway, so the note cover doubles as
 * a placeholder: it carries `data-iframe-params` and the archiver replaces it
 * with a locally stored <video> downloaded from the note URL.
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
 * The state stores some pictures with an http:// URL. Normalizing to https
 * avoids a mixed-content block when the archive is viewed over TLS.
 *
 * @param {string} url
 * @returns {string}
 */
function secureURL(url) {
  return url.replace(/^http:\/\//i, "https://")
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
 * Returns the note id of the current URL.
 *
 * @param {string} url
 * @returns {string}
 */
function noteID(url) {
  var m = /\/(?:explore|discovery\/item)\/([0-9a-zA-Z]+)/.exec(url || "")
  return m ? m[1] : ""
}

/**
 * The detail map can hold more than one note (the feed caches the notes it
 * visited). Pick the one the reader opened.
 *
 * @param {any[]} notes
 * @returns {any|null}
 */
function pickNote(notes) {
  if (!notes || notes.length == 0) {
    return null
  }
  var id = noteID($.url)
  if (id) {
    for (var i = 0; i < notes.length; i++) {
      var n = notes[i]
      if (n.noteId == id || n.id == id) {
        return n
      }
    }
  }
  return notes[0]
}

/**
 * Picks the best image URL of an imageList entry.
 *
 * @param {any} item
 * @returns {string}
 */
function imageURL(item) {
  if (item.urlDefault) {
    return secureURL(unescapeJSON(item.urlDefault))
  }
  if (item.urlPre) {
    return secureURL(unescapeJSON(item.urlPre))
  }
  var list = item.infoList || []
  for (var i = 0; i < list.length; i++) {
    if (list[i].url) {
      return secureURL(unescapeJSON(list[i].url))
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
        return secureURL(unescapeJSON(list[j].masterUrl))
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
 * The published description carries "[话题]" before every hashtag.
 *
 * @param {string} desc
 * @returns {string}
 */
function cleanDesc(desc) {
  return String(desc || "")
    .replace(/\[话题\]#/g, "")
    .replace(/\[话题\]/g, "")
    .replace(/\r\n/g, "\n")
}

function pad2(n) {
  return n < 10 ? "0" + n : String(n)
}

/**
 * Renders the publication time in UTC+8, so the archive shows the same clock
 * reading whatever timezone the server runs in.
 *
 * @param {number} ms
 * @returns {string}
 */
function formatTime(ms) {
  var t = Number(ms)
  if (!t || !isFinite(t)) {
    return ""
  }
  var d = new Date(t + 8 * 3600 * 1000)
  if (isNaN(d.getTime()) || !d.getUTCFullYear) {
    return ""
  }
  return (
    d.getUTCFullYear() +
    "-" +
    pad2(d.getUTCMonth() + 1) +
    "-" +
    pad2(d.getUTCDate()) +
    " " +
    pad2(d.getUTCHours()) +
    ":" +
    pad2(d.getUTCMinutes())
  )
}

/**
 * @param {string} url
 * @param {number} w
 * @param {number} h
 * @returns {string}
 */
function iframeParams(url, w, h) {
  return "src=" + encodeURIComponent(url) + "&w=" + (w || 0) + "&h=" + (h || 0)
}

/**
 * Fallback when the state payload is missing: the note container still holds
 * the text, the author and the date (but no pictures, they are a CSS
 * background on the server side render).
 *
 * @param {Node} doc
 * @returns {any|null}
 */
function noteFromDOM(doc) {
  var box = doc.querySelector("#noteContainer") || doc.querySelector(".note-container")
  if (!box) {
    return null
  }
  var text = function (sel) {
    var el = box.querySelector(sel)
    return el ? String(el.textContent || "").replace(/^\s+|\s+$/g, "") : ""
  }
  var desc = text("#detail-desc")
  var user = text(".author-wrapper .name") || text(".author-container .name")
  var where = text(".bottom-container .date")
  if (!desc && !user) {
    return null
  }
  return {
    type: "",
    title: "",
    desc: desc,
    time: 0,
    ipLocation: where,
    user: user ? {nickname: user} : null,
    imageList: [],
    video: null,
  }
}

/**
 * @param {string[]} parts
 * @param {any} item
 * @param {string} title
 */
function pushPicture(parts, item, title) {
  if (item.url) {
    var attrs = ' src="' + escapeHTML(item.url) + '" alt="' + escapeHTML(title) + '"'
    if (item.width) {
      attrs += ' width="' + item.width + '" height="' + item.height + '"'
    }
    parts.push("<figure><img" + attrs + "></figure>")
  }
}

/**
 * Writes (or creates) a <meta> tag.
 *
 * @param {Node} doc
 * @param {string} attr "property" or "name"
 * @param {string} key
 * @param {string} content
 */
function setMeta(doc, attr, key, content) {
  var nodes = doc.querySelectorAll("meta[" + attr + '="' + key + '"]')
  if (nodes && nodes.length > 0) {
    for (var i = 0; i < nodes.length; i++) {
      nodes[i].setAttribute("content", content)
    }
    return
  }
  var head = doc.querySelector("head")
  if (!head) {
    return
  }
  var meta = doc.createElement("meta")
  meta.setAttribute(attr, key)
  meta.setAttribute("content", content)
  head.appendChild(meta)
}

/**
 * @param {Node} document
 */
exports.documentLoaded = function (document) {
  // Only a note page carries a single note. A profile or feed page keeps the
  // notes it visited in the same payload, and rebuilding one of them over the
  // whole page would archive the wrong thing.
  var notePath = noteID($.url)
  if (!notePath) {
    return
  }

  var texts = scriptTexts(document)
  var state = null
  if (texts.length > 0) {
    try {
      state = parseNoteDetail(texts)
    } catch (e) {
      state = null
    }
  }

  var note = null
  var items = []
  var video = ""

  if (state != null) {
    var notes = []
    findNotes(state, notes, 0)
    note = pickNote(notes)
    if (note) {
      video = streamURL(note.video)
      var list = note.imageList || []
      for (var i = 0; i < list.length; i++) {
        var url = imageURL(list[i])
        if (!url) {
          continue
        }
        items.push({
          url: url,
          live: streamURL(list[i]),
          width: list[i].width || 0,
          height: list[i].height || 0,
        })
      }
    }
  }

  if (!note) {
    note = noteFromDOM(document)
  }

  // No note at all: the page is a feed or a login wall, leave it alone.
  if (!note && items.length == 0) {
    return
  }

  // The state was there but held no picture (deleted note, video without a
  // cover). The inline scripts still quote the CDN URLs.
  if (items.length == 0 && state != null) {
    var acc = {images: [], videos: [], sizes: {}}
    scrapeURLs(texts, acc)
    var scraped = unique(acc.images, 40)
    for (var s = 0; s < scraped.length; s++) {
      items.push({url: scraped[s], live: "", width: 0, height: 0})
    }
    if (!video) {
      video = unique(acc.videos, 1)[0] || ""
    }
  }

  var title = note && note.title ? String(note.title).replace(/^\s+|\s+$/g, "") : ""
  if (!title) {
    var titleNode = document.querySelector("title")
    title = titleNode ? String(titleNode.textContent || "") : ""
    title = title.replace(/\s*[-–|]\s*小红书\s*$/, "").replace(/^\s+|\s+$/g, "")
  }

  var desc = cleanDesc(note ? note.desc : "")
  var author = note && note.user && note.user.nickname ? String(note.user.nickname) : ""
  var when = note ? formatTime(note.time) : ""
  var where = note ? String(note.ipLocation || "") : ""
  var isVideoNote = video != "" || (note && /^video/i.test(String(note.type || "")))

  var parts = ['<article class="xhs-note">']

  var meta = []
  if (author) {
    meta.push("作者：" + author)
  }
  if (when) {
    meta.push(when)
  }
  if (where) {
    meta.push(where)
  }
  if (meta.length > 0) {
    parts.push('<p class="xhs-meta">' + escapeHTML(meta.join(" · ")) + "</p>")
  }

  var lines = desc.split(/\n+/)
  for (var l = 0; l < lines.length; l++) {
    var line = lines[l].replace(/^\s+|\s+$/g, "")
    if (line) {
      parts.push("<p>" + escapeHTML(line) + "</p>")
    }
  }

  if (isVideoNote && items.length > 0) {
    // One picture, the cover, that the archiver turns into the player. The
    // signed stream is not in the payload and would expire anyway.
    var cover = items[0]
    var attrs = ' src="' + escapeHTML(cover.url) + '" alt="' + escapeHTML(title) + '"'
    if (cover.width) {
      attrs += ' width="' + cover.width + '" height="' + cover.height + '"'
    }
    attrs += ' poster="' + escapeHTML(cover.url) + '"'
    attrs += ' data-iframe-params="' + escapeHTML(iframeParams($.url, cover.width, cover.height)) + '"'
    parts.push("<figure><img" + attrs + "></figure>")
    for (var r = 1; r < items.length; r++) {
      pushPicture(parts, items[r], title)
    }
  } else {
    for (var p = 0; p < items.length; p++) {
      pushPicture(parts, items[p], title)
    }
  }

  parts.push("</article>")

  var body = document.body
  if (!body) {
    return
  }
  body.innerHTML = parts.join("")

  // The reader takes the title from og:title ("... - 小红书" is the page
  // title, not the note title) and the excerpt from og:description.
  if (title) {
    setMeta(document, "property", "og:title", title)
    $.title = title
  }
  if (desc) {
    setMeta(document, "property", "og:description", desc.replace(/\n+/g, " "))
  }
  // A video note is a media document, and the readability pass throws media
  // content away before it can be archived. Ask it to keep what we just built.
  setMeta(document, "name", "readeck:keep-content", "1")
}
