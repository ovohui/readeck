// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * Bilibili video pages.
 *
 * A video page renders its player with JavaScript, so the archived document
 * holds a title and nothing else: readability finds no paragraph and the
 * bookmark ends up as a player with no description. The page does ship its
 * whole state in `window.__INITIAL_STATE__`, including the uploader, the
 * publication date, the description and the cover.
 *
 * This script rebuilds the page as an article around that state: a meta line,
 * the description, the cover, and a poster placeholder carrying the page URL.
 * The archiver turns that placeholder into a real <video> with yt-dlp, which
 * knows how to resolve a Bilibili page.
 *
 * @returns {boolean}
 */
exports.isActive = function () {
  return /(^|\.)bilibili\.com$/.test($.domain)
}

/**
 * Decodes the escapes a JSON string can carry. Bilibili writes URLs as
 * `http:\u002F\u002F...`, so a plain read of the raw text yields a broken link.
 *
 * @param {string} s
 * @returns {string}
 */
function unescapeJSON(s) {
  return String(s)
    .replace(/\\u([0-9a-fA-F]{4})/g, function (_, hex) {
      return String.fromCharCode(parseInt(hex, 16))
    })
    .replace(/\\n/g, "\n")
    .replace(/\\r/g, "")
    .replace(/\\t/g, " ")
    .replace(/\\"/g, '"')
    .replace(/\\\//g, "/")
    .replace(/\\\\/g, "\\")
}

/**
 * Reads the first capture group of rx in body, or "".
 *
 * @param {string} body
 * @param {RegExp} rx
 * @returns {string}
 */
function pick(body, rx) {
  var m = body.match(rx)
  return m && m[1] ? unescapeJSON(m[1]) : ""
}

/**
 * Returns the `videoData` block of the page state, or null.
 *
 * The state is not strict JSON (Bilibili writes `undefined` in places), so it
 * is sliced and read with targeted patterns instead of being parsed.
 *
 * @param {Node} doc
 * @returns {any|null}
 */
function readVideoData(doc) {
  var scripts = doc.querySelectorAll("script")
  for (var i = 0; i < scripts.length; i++) {
    var text = scripts[i].textContent || ""
    var at = text.indexOf("__INITIAL_STATE__")
    if (at < 0) {
      continue
    }
    var open = text.indexOf("{", at)
    if (open < 0) {
      continue
    }
    var slice = text.substring(open, open + 40000)
    var vd = slice.indexOf('"videoData":{')
    if (vd < 0) {
      continue
    }
    // Stop at the next top level key so that the recommendation list, which
    // repeats title/desc/pic, cannot be mistaken for the video itself.
    var body = slice.substring(vd, vd + 16000)
    var end = body.indexOf('"upData":')
    if (end < 0) {
      end = body.indexOf('"videoStatus":')
    }
    if (end > 0) {
      body = body.substring(0, end)
    }

    var data = {
      bvid: pick(body, /"bvid":"((?:[^"\\]|\\.)*)"/),
      title: pick(body, /"title":"((?:[^"\\]|\\.)*)"/),
      desc: pick(body, /"desc":"((?:[^"\\]|\\.)*)"/),
      // Bilibili still hands out http:// covers; an https page must not embed one.
      pic: pick(body, /"pic":"((?:[^"\\]|\\.)*)"/).replace(/^http:/, "https:"),
      owner: pick(body, /"owner":\{"mid":\d+,"name":"((?:[^"\\]|\\.)*)"/),
      category: pick(body, /"tname":"((?:[^"\\]|\\.)*)"/),
      pubdate: parseInt(pick(body, /"pubdate":(\d+)/), 10) || 0,
      duration: parseInt(pick(body, /"duration":(\d+)/), 10) || 0,
      views: parseInt(pick(body, /"stat":\{"aid":\d+,"view":(\d+)/), 10) || 0,
    }
    if (data.bvid || data.title) {
      return data
    }
  }
  return null
}

/**
 * @param {number} n
 * @returns {string}
 */
function pad2(n) {
  return n < 10 ? "0" + n : String(n)
}

/**
 * Renders a Unix timestamp in UTC+8, so the archive shows the same date
 * whatever timezone the server runs in.
 *
 * @param {number} seconds
 * @returns {string}
 */
function formatDate(seconds) {
  if (!seconds || !isFinite(seconds)) {
    return ""
  }
  var d = new Date(seconds * 1000 + 8 * 3600 * 1000)
  if (isNaN(d.getTime()) || !d.getUTCFullYear) {
    return ""
  }
  return d.getUTCFullYear() + "-" + pad2(d.getUTCMonth() + 1) + "-" + pad2(d.getUTCDate())
}

/**
 * @param {number} seconds
 * @returns {string}
 */
function formatDuration(seconds) {
  if (!seconds || !isFinite(seconds)) {
    return ""
  }
  var h = Math.floor(seconds / 3600)
  var m = Math.floor((seconds % 3600) / 60)
  var s = Math.floor(seconds % 60)
  return h > 0 ? h + ":" + pad2(m) + ":" + pad2(s) : m + ":" + pad2(s)
}

/**
 * @param {number} n
 * @returns {string}
 */
function formatCount(n) {
  if (!n || !isFinite(n)) {
    return ""
  }
  if (n >= 100000000) {
    return (n / 100000000).toFixed(1) + " 亿"
  }
  if (n >= 10000) {
    return (n / 10000).toFixed(1) + " 万"
  }
  return String(n)
}

/**
 * @param {string} s
 * @returns {string}
 */
function escapeHTML(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
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
 * Collects the topic tags shown under the player.
 *
 * @param {Node} doc
 * @returns {Array<string>}
 */
function readTags(doc) {
  var out = []
  var nodes = doc.querySelectorAll("a.tag-link, .tag-link")
  for (var i = 0; i < nodes.length; i++) {
    var t = nodes[i].getAttribute("title") || nodes[i].textContent || ""
    t = String(t).replace(/^\s+|\s+$/g, "")
    if (t && out.indexOf(t) < 0) {
      out.push(t)
    }
  }
  return out
}

/**
 * Cuts the statistics block Bilibili appends to the description. The rest of
 * the text is the uploader's own writing and is worth keeping.
 *
 * @param {string} s
 * @returns {string}
 */
function cleanDescription(s) {
  // Keep line breaks: the article turns each one into a paragraph.
  s = String(s || "")
    .replace(/\r\n/g, "\n")
    .replace(/[ \t\f\v]+/g, " ")
    .replace(/\n{2,}/g, "\n")
    .trim()
  var markers = ["视频播放量", "弹幕量", "视频作者", "作者简介", "相关视频：", "相关视频:"]
  var cut = -1
  for (var i = 0; i < markers.length; i++) {
    var at = s.indexOf(markers[i])
    if (at > 20 && (cut < 0 || at < cut)) {
      cut = at
    }
  }
  if (cut > 0) {
    s = s.substring(0, cut)
  }
  return s.replace(/[\s,，、;；:：–—-]+$/, "")
}

/**
 * Builds the video data from the DOM when the state payload is missing or has
 * moved. The page still has its title, description and cover in meta tags, and
 * that is enough for a readable bookmark; without this fallback the script
 * returns early, the extractor sees a media page with no article, and the
 * reader shows an empty page next to the downloaded video.
 *
 * @param {Node} doc
 * @returns {any}
 */
function readFallback(doc) {
  var meta = function (sel) {
    var el = doc.querySelector(sel)
    return el ? String(el.getAttribute("content") || "") : ""
  }
  var text = function (sel) {
    var el = doc.querySelector(sel)
    return el ? String(el.textContent || "").replace(/^\s+|\s+$/g, "") : ""
  }

  var title = cleanTitle(
    meta('meta[property="og:title"]') ||
      text("h1.video-title") ||
      (doc.title || ""),
  )
  if (!title) {
    return null
  }

  return {
    title: title,
    desc: cleanDescription(
      meta('meta[property="og:description"]') ||
        meta('meta[name="description"]') ||
        text("#v_desc") ||
        text(".desc-info"),
    ),
    pic: (meta('meta[property="og:image"]') || "").replace(/^http:/, "https:"),
    owner: text(".up-name") || text(".username"),
    category: "",
    pubdate: 0,
    duration: 0,
    views: 0,
  }
}

/**
 * Removes the site suffix Bilibili puts on document.title.
 *
 * @param {string} s
 * @returns {string}
 */
function cleanTitle(s) {
  return String(s || "")
    .replace(/\s*[-_|]\s*哔哩哔哩\s*[-_|]?\s*bilibili\s*$/i, "")
    .replace(/\s*[-_|]\s*哔哩哔哩\s*$/i, "")
    .replace(/\s*[-_|]\s*bilibili\s*$/i, "")
    .replace(/^\s+|\s+$/g, "")
}

/**
 * @param {Node} document
 */
exports.documentLoaded = function (document) {
  // A selection submitted from the browser extension is left as is.
  if ($.readability === false) {
    return
  }

  var path = String($.url).replace(/^https?:\/\/[^/]+/, "")
  if (!/^\/video\/(BV[0-9A-Za-z]+|av\d+)/i.test(path)) {
    return
  }

  var data = readVideoData(document) || readFallback(document)
  if (!data) {
    return
  }

  var title = cleanTitle(data.title)
  if (!title) {
    var h1 = document.querySelector("h1.video-title")
    title = h1 ? cleanTitle(h1.textContent || "") : ""
  }
  if (!title) {
    return
  }

  var desc = cleanDescription(data.desc)

  var meta = []
  if (data.owner) {
    meta.push("UP：" + data.owner)
  }
  var when = formatDate(data.pubdate)
  if (when) {
    meta.push(when)
  }
  var length = formatDuration(data.duration)
  if (length) {
    meta.push(length)
  }
  if (data.category) {
    meta.push(data.category)
  }
  var views = formatCount(data.views)
  if (views) {
    meta.push(views + "播放")
  }

  var parts = ['<article class="bili-video">']
  if (meta.length > 0) {
    parts.push('<p class="bili-meta">' + escapeHTML(meta.join(" · ")) + "</p>")
  }

  var lines = desc.replace(/\r\n/g, "\n").split(/\n+/)
  for (var i = 0; i < lines.length; i++) {
    var line = lines[i].replace(/^\s+|\s+$/g, "")
    if (line) {
      parts.push("<p>" + escapeHTML(line) + "</p>")
    }
  }

  // The placeholder carries the page URL, not the player URL: yt-dlp resolves
  // https://www.bilibili.com/video/<id> on its own and the archiver swaps this
  // <img> for the downloaded <video>.
  var pageURL = String($.url)
  var poster = data.pic ? ' src="' + escapeHTML(data.pic) + '"' : ""
  parts.push(
    '<figure><img' +
      poster +
      ' alt="' +
      escapeHTML(title) +
      '" data-iframe-params="src=' +
      escapeHTML(encodeURIComponent(pageURL)) +
      '"></figure>',
  )

  var tags = readTags(document)
  if (tags.length > 0) {
    var line2 = []
    for (var k = 0; k < tags.length; k++) {
      line2.push("#" + tags[k])
    }
    parts.push('<p class="bili-tags">' + escapeHTML(line2.join(" ")) + "</p>")
  }

  parts.push("</article>")

  var body = document.body
  if (!body) {
    return
  }
  body.innerHTML = parts.join("")

  // The extension and the mobile share sheet read document.title, and the
  // <title> tag still carries "_哔哩哔哩_bilibili" when the state payload was
  // missing. Fix it here as well as in the metadata.
  setMeta(document, "property", "og:title", title)
  setMeta(document, "property", "og:type", "video.other")
  setMeta(document, "property", "og:description", desc.substring(0, 300))
  setMeta(document, "name", "description", desc.substring(0, 300))
  if (data.pic) {
    setMeta(document, "property", "og:image", data.pic)
  }
  // A video page is a media document, and the extractor empties those before
  // readability runs. Ask to keep the article that was just built.
  setMeta(document, "name", "readeck:keep-content", "1")

  $.title = title
  if (data.owner) {
    $.authors = [data.owner]
  }
}
