// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 12

/**
 * Zhihu columns show videos as <a class="video-box"> links with a poster.
 * The archived page therefore kept only the cover and a link to
 * www.zhihu.com/video/<id>.
 *
 * Replace the link with the same kind of poster placeholder used by the
 * Bilibili script. The archiver's embedded-video resolver already knows the
 * Zhihu host and asks yt-dlp to fetch /video/<id>, so the clip is stored in
 * the archive and the reader plays it offline.
 */
exports.isActive = function () {
  return /(^|\.)zhihu\.com$/.test($.domain)
}

function videoID(a) {
  const lens = a.getAttribute("data-lens-id") || ""
  if (/^\d{6,}$/.test(lens)) {
    return lens
  }
  let raw = a.getAttribute("href") || ""
  try {
    raw = decodeURIComponent(raw)
  } catch (e) {
    // keep the raw href
  }
  let m = /video\/(\d{6,})/.exec(raw)
  if (!m) {
    m = /video(?:%2F|\/)(\d{6,})/i.exec(a.getAttribute("href") || "")
  }
  return m ? m[1] : ""
}

exports.documentLoaded = function (document) {
  document.querySelectorAll('sup[data-draft-type="reference"], sup.reference').forEach(function (node) {
    node.remove()
  })
  const links = Array.from(document.querySelectorAll("a.video-box"))
  for (const a of links) {
    const id = videoID(a)
    if (!id) continue

    const pageURL = "https://www.zhihu.com/video/" + id
    const poster = a.getAttribute("data-poster") || ""
    const figure = document.createElement("figure")
    const img = document.createElement("img")
    if (poster) img.setAttribute("src", poster)
    img.setAttribute("alt", "video")
    img.setAttribute("data-iframe-params", "src=" + encodeURIComponent(pageURL))
    figure.appendChild(img)

    const title = a.querySelector(".title")
    if (title && title.textContent.trim()) {
      const caption = document.createElement("figcaption")
      caption.textContent = title.textContent.trim()
      figure.appendChild(caption)
    }
    a.replaceWith(figure)
  }
}
