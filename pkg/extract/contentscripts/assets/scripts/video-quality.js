// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

// Selects a preferred quality for HLS (m3u8) streams and direct video sources.
// The preference comes from the server config (READECK_VIDEO_QUALITY or
// [bookmarks] video_quality) exposed through $.config when available.
//
// Accepted values: best (default), 1080p, 720p, 480p, worst.

exports.priority = 20

exports.isActive = function () {
  return true
}

const QUALITY_ORDER = ["worst", "480p", "720p", "1080p", "best"]

function wantedQuality() {
  let q = "best"
  if (typeof readeckConfig !== "undefined" && readeckConfig.videoQuality) {
    q = String(readeckConfig.videoQuality).toLowerCase()
  }
  if (QUALITY_ORDER.indexOf(q) < 0) {
    q = "best"
  }
  return q
}

// Resolve an HLS master playlist into the URL of the chosen variant.
function resolveHls(url, quality) {
  if (!/\.m3u8(\?|$)/i.test(url)) {
    return url
  }
  try {
    const rsp = $.requests.get(url)
    if (!rsp || rsp.status != 200) {
      return url
    }
    const lines = rsp.text().split(/\r?\n/)
    const variants = []
    let current = null
    for (const raw of lines) {
      const line = raw.trim()
      if (!line) continue
      if (line.indexOf("#EXT-X-STREAM-INF") === 0) {
        const match = /RESOLUTION=(\d+)x(\d+)/.exec(line)
        current = match ? parseInt(match[2], 10) : 0
      } else if (line[0] !== "#") {
        if (current !== null) {
          variants.push({ height: current, url: new URL(line, url).href })
          current = null
        }
      }
    }
    if (variants.length === 0) {
      return url
    }
    variants.sort((a, b) => a.height - b.height)
    let pick
    if (quality === "best") {
      pick = variants[variants.length - 1]
    } else if (quality === "worst") {
      pick = variants[0]
    } else {
      const target = parseInt(quality, 10)
      pick = variants[0]
      for (const v of variants) {
        if (v.height <= target) pick = v
      }
    }
    return pick.url
  } catch (e) {
    return url
  }
}

exports.documentReady = function (document) {
  const quality = wantedQuality()

  document.querySelectorAll("source[src]").forEach((node) => {
    const src = node.getAttribute("src")
    if (src && /\.m3u8(\?|$)/i.test(src)) {
      node.setAttribute("src", resolveHls(src, quality))
    }
  })

  document.querySelectorAll("video[src]").forEach((node) => {
    const src = node.getAttribute("src")
    if (src && /\.m3u8(\?|$)/i.test(src)) {
      node.setAttribute("src", resolveHls(src, quality))
    }
  })
}
