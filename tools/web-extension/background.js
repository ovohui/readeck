// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// Service worker: 图片转 data URL。
//
// Content scripts are bound by CORS, so a picture served from a CDN without
// permissive headers cannot be read there. The worker holds <all_urls> host
// permissions, which lifts that restriction and also sends the site cookies
// (credentials: "include").

const MAX_SIZE = 8 * 1024 * 1024

function toBase64(buffer) {
  const bytes = new Uint8Array(buffer)
  let binary = ""
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk))
  }
  return btoa(binary)
}

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.type !== "fetchDataURL") {
    return false
  }

  ;(async () => {
    try {
      const rsp = await fetch(msg.url, {
        credentials: "include",
        referrer: msg.referrer,
      })
      if (!rsp.ok) {
        sendResponse({error: "HTTP " + rsp.status})
        return
      }
      const buffer = await rsp.arrayBuffer()
      if (buffer.byteLength > MAX_SIZE) {
        sendResponse({error: "too large"})
        return
      }
      const type = rsp.headers.get("content-type") || "image/jpeg"
      sendResponse({dataURL: "data:" + type + ";base64," + toBase64(buffer)})
    } catch (err) {
      sendResponse({error: String(err && err.message ? err.message : err)})
    }
  })()

  return true
})
