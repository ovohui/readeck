// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * ZCOOL (站酷) work pages.
 *
 * The desktop host sits behind an Aliyun WAF JavaScript challenge that a plain
 * HTTP client can never answer (see internal/wafshield, which replays the
 * request against m.zcool.com.cn). The mobile front end is a Next.js app: it
 * ships the whole work in a `__NEXT_DATA__` JSON payload and paints the page
 * afterwards. So the server receives a document whose body is empty except for
 * a container, and readability finds nothing but the navigation bar.
 *
 * This script reads the payload and rebuilds the work as an article: the meta
 * line (author, category, date), the description when there is one, then every
 * picture of the gallery in order. That is what the page shows, and it is what
 * the archive keeps.
 *
 * @returns {boolean}
 */
exports.isActive = function () {
  return /(^|\.)zcool\.com\.cn$/.test($.domain)
}

/**
 * @param {number} n
 * @returns {string}
 */
function pad2(n) {
  return n < 10 ? "0" + n : String(n)
}

/**
 * Renders a publication date in UTC+8, so the archive shows the same clock
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
    pad2(d.getUTCDate())
  )
}

/**
 * Returns the `workData` object of the Next.js payload, or null.
 *
 * @param {Node} doc
 * @returns {any|null}
 */
function readWorkData(doc) {
  var node = doc.querySelector("script#__NEXT_DATA__")
  var texts = []
  if (node) {
    texts.push(node.textContent)
  } else {
    var scripts = doc.querySelectorAll("script")
    for (var i = 0; i < scripts.length; i++) {
      var t = scripts[i].textContent
      if (t && t.length > 2000 && t.charAt(0) == "{") {
        texts.push(t)
      }
    }
  }

  for (var j = 0; j < texts.length; j++) {
    var data = null
    try {
      data = JSON.parse(texts[j])
    } catch (e) {
      data = null
    }
    if (!data || !data.props || !data.props.pageProps) {
      continue
    }
    var w = data.props.pageProps.workData
    if (w && (w.title || (w.productImages && w.productImages.length > 0))) {
      return w
    }
  }
  return null
}

/**
 * @param {any} item
 * @returns {string}
 */
function imageURL(item) {
  if (!item) {
    return ""
  }
  var u = item.urlBig || item.url || ""
  // Some payloads carry protocol relative URLs.
  if (u.indexOf("//") == 0) {
    u = "https:" + u
  }
  return u
}

/**
 * @param {any} item
 * @returns {string}
 */
function videoURL(item) {
  if (!item) {
    return ""
  }
  var u = item.url || item.playUrl || item.src || ""
  if (u.indexOf("//") == 0) {
    u = "https:" + u
  }
  if (!/^https?:\/\//.test(u) || !/\.(mp4|webm|m3u8)/i.test(u)) {
    return ""
  }
  return u
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
  // A work page is a single piece; user, list and search pages carry the same
  // payload shape but no work of their own.
  if (!/\/work\//.test($.url)) {
    return
  }

  var work = readWorkData(document)
  if (!work) {
    return
  }

  var title = String(work.title || "").replace(/^\s+|\s+$/g, "")
  if (!title) {
    var t = document.querySelector("title")
    title = t ? String(t.textContent || "").replace(/^\s+|\s+$/g, "") : ""
  }

  var author =
    work.creatorObj && work.creatorObj.username ? String(work.creatorObj.username) : ""
  var when = formatTime(work.publishTime || work.createTime)

  var kind = []
  if (work.typeStr) {
    kind.push(String(work.typeStr))
  }
  var cate = []
  if (work.cateName) {
    cate.push(String(work.cateName))
  }
  if (work.subcateName) {
    cate.push(String(work.subcateName))
  }
  if (cate.length > 0) {
    kind.push(cate.join(" / "))
  }

  var meta = []
  if (author) {
    meta.push("作者：" + author)
  }
  if (kind.length > 0) {
    meta.push(kind.join(" · "))
  }
  if (when) {
    meta.push(when)
  }

  var parts = ['<article class="zcool-work">']

  if (meta.length > 0) {
    parts.push('<p class="zcool-meta">' + escapeHTML(meta.join(" · ")) + "</p>")
  }

  var desc = String(work.description || "").replace(/\r\n/g, "\n")
  var lines = desc.split(/\n+/)
  for (var l = 0; l < lines.length; l++) {
    var line = lines[l].replace(/^\s+|\s+$/g, "")
    if (line) {
      parts.push("<p>" + escapeHTML(line) + "</p>")
    }
  }

  var videos = work.productVideos || []
  for (var v = 0; v < videos.length; v++) {
    var vurl = videoURL(videos[v])
    if (!vurl) {
      continue
    }
    var poster = videos[v] && videos[v].cover ? String(videos[v].cover) : ""
    parts.push(
      "<figure><video controls playsinline preload=\"metadata\" src=\"" +
        escapeHTML(vurl) +
        '"' +
        (poster ? ' poster="' + escapeHTML(poster) + '"' : "") +
        "></video></figure>",
    )
  }

  var images = work.productImages || []
  for (var i = 0; i < images.length; i++) {
    var url = imageURL(images[i])
    if (!url) {
      continue
    }
    var attrs = ' src="' + escapeHTML(url) + '" alt="' + escapeHTML(title) + '"'
    if (images[i].width && images[i].height) {
      attrs += ' width="' + images[i].width + '" height="' + images[i].height + '"'
    }
    parts.push("<figure><img" + attrs + "></figure>")
  }

  var tags = work.productTags || []
  if (tags.length > 0) {
    var tagLine = []
    for (var k = 0; k < tags.length; k++) {
      if (tags[k]) {
        tagLine.push("#" + String(tags[k]))
      }
    }
    if (tagLine.length > 0) {
      parts.push('<p class="zcool-tags">' + escapeHTML(tagLine.join(" ")) + "</p>")
    }
  }

  var copyright = String(work.copyrightStr || "").replace(/&#10;/g, " ").replace(/^\s+|\s+$/g, "")
  if (copyright) {
    parts.push('<p class="zcool-copyright">' + escapeHTML(copyright) + "</p>")
  }

  parts.push("</article>")

  var body = document.body
  if (!body) {
    return
  }
  body.innerHTML = parts.join("")

  if (title) {
    setMeta(document, "property", "og:title", title)
    $.title = title
  }
  var cover = imageURL(images[0])
  if (cover) {
    setMeta(document, "property", "og:image", cover)
  }
  setMeta(document, "property", "og:type", "article")
  if (author) {
    $.authors = [author]
  }
  if (desc) {
    setMeta(document, "property", "og:description", desc.replace(/\n+/g, " "))
  }
}
