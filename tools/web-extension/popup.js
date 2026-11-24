// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// Readeck 剪藏助手 — popup 逻辑。
//
// The page is captured from the tab itself (chrome.scripting.executeScript),
// which gives us the DOM *after* JavaScript ran: images loaded by React, lazy
// images, infinite scroll content, everything the server never sees. The
// serialized document is posted to Readeck's /api/bookmarks endpoint, which
// accepts a full HTML payload as the main resource.
//
// Two capture modes:
//   - generic: clone documentElement, resolve lazy attributes, inline images.
//   - Xiaohongshu: the explore page is an infinite feed, so a clone carries the
//     author avatar, the like/collect counters, the "说点什么…" composer and
//     dozens of unrelated note covers. The note itself only lives in
//     window.__INITIAL_STATE__, so the article is rebuilt from that payload.

const $ = (id) => document.getElementById(id)
const statusEl = () => $("status")

function setStatus(text, kind) {
  const el = statusEl()
  el.textContent = text
  el.className = kind || ""
}

async function loadSettings() {
  const {server = "", token = "", inline = true, scroll = true} = await chrome.storage.local.get([
    "server",
    "token",
    "inline",
    "scroll",
  ])
  $("server").value = server
  $("token").value = token
  $("inline").checked = inline
  $("scroll").checked = scroll
}

async function saveSettings() {
  const server = $("server").value.trim().replace(/\/+$/, "")
  const token = $("token").value.trim()
  const inline = $("inline").checked
  const scroll = $("scroll").checked
  await chrome.storage.local.set({server, token, inline, scroll})
  setStatus("设置已保存", "ok")
}

/**
 * Runs inside the page. Must be self contained: it is serialized and injected
 * with chrome.scripting.executeScript.
 *
 * @param {{inline: boolean, scroll: boolean}} opts
 * @returns {Promise<{html: string, url: string, title: string}>}
 */
async function capturePage(opts) {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
  const isXhs = /(^|\.)xiaohongshu\.com$/i.test(location.hostname)

  // 0. The page must be fully parsed before we start scrolling.
  if (document.readyState !== "complete") {
    await Promise.race([
      new Promise((resolve) => window.addEventListener("load", resolve, {once: true})),
      sleep(4000),
    ])
  }

  // 1. Scroll through the page so lazy loaded images get their real src.
  //    Not needed on Xiaohongshu: the note comes from the state payload and
  //    scrolling there only loads more unrelated notes into the feed.
  if (opts.scroll && !isXhs) {
    const step = Math.max(window.innerHeight, 400)
    const max = Math.min(document.body.scrollHeight, 30000)
    for (let y = 0; y < max; y += step) {
      window.scrollTo(0, y)
      await sleep(120)
    }
    window.scrollTo(0, 0)
    await sleep(200)
  }

  // 2. Wait (at most 6s) for the images that are already in the DOM.
  if (!isXhs) {
    const pending = Array.from(document.images)
      .filter((img) => !img.complete)
      .map(
        (img) =>
          new Promise((resolve) => {
            img.addEventListener("load", resolve, {once: true})
            img.addEventListener("error", resolve, {once: true})
          }),
      )
    await Promise.race([Promise.all(pending), sleep(6000)])
  }

  const escapeHTML = (s) =>
    String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")

  // Convert every <img> below root to a data: URL. Direct fetch works for
  // same-origin or CORS friendly pictures; everything else goes through the
  // service worker, which holds <all_urls> host permissions and is therefore
  // not bound by CORS.
  const toDataURL = async (url) => {
    try {
      const rsp = await fetch(url, {credentials: "include", cache: "force-cache"})
      if (rsp.ok) {
        const blob = await rsp.blob()
        if (blob.size <= 8 * 1024 * 1024) {
          return await new Promise((resolve, reject) => {
            const fr = new FileReader()
            fr.onload = () => resolve(fr.result)
            fr.onerror = reject
            fr.readAsDataURL(blob)
          })
        }
      }
    } catch (e) {
      // ignored, fall back to the background worker
    }

    const resp = await chrome.runtime.sendMessage({
      type: "fetchDataURL",
      url,
      referrer: location.href,
    })
    if (!resp || !resp.dataURL) {
      throw new Error(resp && resp.error ? resp.error : "image fetch failed")
    }
    return resp.dataURL
  }

  const inlineImages = async (root) => {
    const nodes = Array.from(root.querySelectorAll("img[src]")).filter((el) => {
      const src = el.getAttribute("src")
      return /^https?:/i.test(src) || /^\/\//.test(src)
    })

    let index = 0
    const worker = async () => {
      while (index < nodes.length) {
        const el = nodes[index++]
        try {
          let src = el.getAttribute("src")
          if (src.startsWith("//")) {
            src = location.protocol + src
          }
          el.setAttribute("src", await toDataURL(src))
        } catch (e) {
          // Leave the remote URL in place; the server will try to fetch it.
        }
      }
    }
    await Promise.all([worker(), worker(), worker(), worker()])
  }

  // ---------------------------------------------------------------------
  // Xiaohongshu
  // ---------------------------------------------------------------------
  const secureURL = (u) => String(u || "").replace(/^http:\/\//i, "https://")

  const bestImageURL = (item) => {
    if (!item) {
      return ""
    }
    if (item.urlDefault) {
      return secureURL(item.urlDefault)
    }
    if (item.urlPre) {
      return secureURL(item.urlPre)
    }
    const list = item.infoList || []
    for (let i = 0; i < list.length; i++) {
      if (list[i] && list[i].url) {
        return secureURL(list[i].url)
      }
    }
    return ""
  }

  const streamURL = (node) => {
    if (!node) {
      return ""
    }
    const stream = (node.media && node.media.stream) || node.stream
    if (!stream) {
      return ""
    }
    const codecs = ["h264", "h265", "h264_hd", "av1"]
    for (let i = 0; i < codecs.length; i++) {
      const list = stream[codecs[i]] || []
      for (let j = 0; j < list.length; j++) {
        if (list[j] && list[j].masterUrl) {
          return secureURL(list[j].masterUrl)
        }
      }
    }
    return ""
  }

  const pad = (n) => (n < 10 ? "0" + n : String(n))
  const formatTime = (ms) => {
    const t = Number(ms)
    if (!t || !isFinite(t)) {
      return ""
    }
    const d = new Date(t)
    return (
      d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) +
      " " + pad(d.getHours()) + ":" + pad(d.getMinutes())
    )
  }

  // The state payload is the single source of truth: 8 images for an 8 image
  // note, in the author's order, with their real dimensions.
  const xhsNote = () => {
    const state = window.__INITIAL_STATE__
    const map = state && state.note && state.note.noteDetailMap
    if (!map) {
      return null
    }
    const hit = (location.pathname.match(/\/(?:explore|discovery\/item)\/([0-9a-zA-Z]+)/) || [])[1]
    if (hit && map[hit] && map[hit].note) {
      return map[hit].note
    }
    const keys = Object.keys(map)
    if (keys.length === 1 && map[keys[0]] && map[keys[0]].note) {
      return map[keys[0]].note
    }
    return null
  }

  // A logged-in page carries no stream URL: the player asks a signed API for
  // it, and the SSR payload stops at the thumbnail. The very same note served
  // to an anonymous visitor still ships the stream, so fetch a copy without
  // cookies and read the URL there. Only video notes pay for this request.
  const xhsAnonymousVideo = async (pageUrl) => {
    try {
      const rsp = await fetch(pageUrl, {
        credentials: "omit",
        cache: "no-store",
        headers: {Accept: "text/html,application/xhtml+xml"},
      })
      if (!rsp.ok) {
        return ""
      }
      const html = await rsp.text()
      const idx = html.indexOf('"noteDetailMap"')
      if (idx < 0) {
        return ""
      }
      const start = html.indexOf("{", idx)
      if (start < 0) {
        return ""
      }
      let depth = 0
      let inStr = false
      let esc = false
      for (let i = start; i < html.length; i++) {
        const c = html.charAt(i)
        if (inStr) {
          if (esc) {
            esc = false
          } else if (c === "\\") {
            esc = true
          } else if (c === '"') {
            inStr = false
          }
          continue
        }
        if (c === '"') {
          inStr = true
        } else if (c === "{") {
          depth++
        } else if (c === "}") {
          depth--
          if (depth === 0) {
            try {
              const state = JSON.parse(html.slice(start, i + 1).replace(/:\s*undefined/g, ":null"))
              const keys = Object.keys(state)
              for (let k = 0; k < keys.length; k++) {
                const entry = state[keys[k]]
                const url = streamURL(entry && entry.note && entry.note.video)
                if (url) {
                  return url
                }
              }
            } catch (e) {
              // fall through to the empty result
            }
            return ""
          }
        }
      }
    } catch (e) {
      // network error, ad blocker, offline: keep the note without a video
    }
    return ""
  }

  const xhsArticle = async () => {
    const note = xhsNote()
    if (!note) {
      return null
    }

    const title = (note.title || "").trim() || document.title
    const user = note.user || {}
    const meta = []
    if (user.nickname) {
      meta.push("作者：" + user.nickname)
    }
    const when = formatTime(note.time)
    if (when) {
      meta.push(when)
    }
    if (note.ipLocation) {
      meta.push(note.ipLocation)
    }

    const root = document.createElement("article")
    root.className = "xhs-note"

    if (title) {
      const h1 = document.createElement("h1")
      h1.textContent = title
      root.appendChild(h1)
    }

    if (meta.length > 0) {
      const p = document.createElement("p")
      p.className = "xhs-meta"
      p.textContent = meta.join(" · ")
      root.appendChild(p)
    }

    const desc = String(note.desc || "").replace(/\[话题\]#/g, " ").trim()
    desc.split(/\n+/).forEach((line) => {
      const text = line.trim()
      if (!text) {
        return
      }
      const p = document.createElement("p")
      p.textContent = text
      root.appendChild(p)
    })

    const images = note.imageList || []
    for (let i = 0; i < images.length; i++) {
      const url = bestImageURL(images[i])
      if (!url) {
        continue
      }
      const figure = document.createElement("figure")
      const img = document.createElement("img")
      img.setAttribute("src", url)
      img.setAttribute("alt", title)
      if (images[i].width && images[i].height) {
        img.setAttribute("width", images[i].width)
        img.setAttribute("height", images[i].height)
      }
      figure.appendChild(img)
      root.appendChild(figure)

      // Live photo: the still picture has a short video attached to it.
      const live = streamURL(images[i])
      if (live) {
        const fig = document.createElement("figure")
        const video = document.createElement("video")
        video.setAttribute("src", live)
        video.setAttribute("controls", "")
        video.setAttribute("playsinline", "")
        video.setAttribute("preload", "metadata")
        fig.appendChild(video)
        root.appendChild(fig)
      }
    }

    let video = streamURL(note.video)
    if (!video && String(note.type || "").indexOf("video") === 0) {
      video = await xhsAnonymousVideo(location.href)
    }
    if (video) {
      const fig = document.createElement("figure")
      const el = document.createElement("video")
      el.setAttribute("src", video)
      el.setAttribute("controls", "")
      el.setAttribute("playsinline", "")
      el.setAttribute("preload", "metadata")
      fig.appendChild(el)
      root.appendChild(fig)
    }

    if (root.querySelector("img, video") === null) {
      return null
    }

    return {title, root}
  }

  // Last chance to find the stream before giving up: a video that is already
  // playing has its URL on a media element or in an inline script, even when
  // the state payload no longer carries it.
  const xhsVideoFromDOM = () => {
    const media = document.querySelectorAll("video[src], source[src]")
    for (let i = 0; i < media.length; i++) {
      const src = media[i].getAttribute("src") || ""
      if (/^https?:/i.test(src) && /\.mp4/i.test(src)) {
        return src
      }
    }
    const rx = /https?:[^"'\s]{6,400}\.mp4[^"'\s]{0,200}/g
    const scripts = document.querySelectorAll("script")
    for (let i = 0; i < scripts.length; i++) {
      // Inline state is JSON-escaped: "https:\/\/x" and "\u0026". Normalise
      // the text before matching so the URL itself is clean.
      const text = (scripts[i].textContent || "").replace(/\\/g, "")
      let m
      while ((m = rx.exec(text)) !== null) {
        const url = m[0].replace(/u0026/gi, "&")
        if (/xhscdn\.com|xhsimg\.com/.test(url)) {
          return url
        }
      }
    }
    return ""
  }

  // Safety net: if the state payload ever moves or changes shape, fall back to
  // the note container. Scoping to #noteContainer is what keeps the feed covers
  // out: the explore page is an infinite list, the note is one box inside it.
  const xhsArticleFromDOM = async () => {
    const container =
      document.querySelector("#noteContainer") || document.querySelector(".note-container")
    if (!container) {
      return null
    }

    const text = (sel) => {
      const el = container.querySelector(sel)
      return el ? (el.textContent || "").trim() : ""
    }

    const title =
      text("#detail-title") ||
      text(".note-content .title") ||
      (document.title || "").replace(/\s*[-–|]\s*小红书.*$/, "").trim()

    const meta = []
    const author = text(".author-container .name") || text(".author-wrapper .name")
    if (author) {
      meta.push("作者：" + author)
    }
    const date = text(".bottom-container .date") || text(".date")
    if (date) {
      meta.push(date)
    }

    const root = document.createElement("article")
    root.className = "xhs-note"

    if (title) {
      const h1 = document.createElement("h1")
      h1.textContent = title
      root.appendChild(h1)
    }
    if (meta.length > 0) {
      const p = document.createElement("p")
      p.className = "xhs-meta"
      p.textContent = meta.join(" · ")
      root.appendChild(p)
    }

    const descEl = container.querySelector("#detail-desc") || container.querySelector(".note-content .desc")
    if (descEl) {
      const raw = (descEl.innerText || descEl.textContent || "").replace(/\[话题\]#/g, " ")
      raw.split(/\n+/).forEach((line) => {
        const value = line.trim()
        if (!value) {
          return
        }
        const p = document.createElement("p")
        p.textContent = value
        root.appendChild(p)
      })
    }

    const seen = {}
    container.querySelectorAll("img").forEach((el) => {
      const src = el.currentSrc || el.getAttribute("src") || ""
      if (!/xhscdn\.com|xhsimg\.com/.test(src)) {
        return
      }
      if (/avatar|emoji|logo|fe-platform/i.test(src)) {
        return
      }
      if (el.closest(".author-container, .author-wrapper, [class*='avatar']")) {
        return
      }
      const width = Number(el.getAttribute("width") || el.naturalWidth || 0)
      if (width > 0 && width <= 160) {
        return
      }
      const key = src.split("?")[0]
      if (seen[key]) {
        return
      }
      seen[key] = true

      const figure = document.createElement("figure")
      const img = document.createElement("img")
      img.setAttribute("src", src)
      img.setAttribute("alt", title)
      const height = Number(el.getAttribute("height") || el.naturalHeight || 0)
      if (width && height) {
        img.setAttribute("width", width)
        img.setAttribute("height", height)
      }
      figure.appendChild(img)
      root.appendChild(figure)
    })

    if (root.querySelector("img") === null) {
      return null
    }

    if (!root.querySelector("video")) {
      const video = xhsVideoFromDOM() || (await xhsAnonymousVideo(location.href))
      if (video) {
        const fig = document.createElement("figure")
        const el = document.createElement("video")
        el.setAttribute("src", video)
        el.setAttribute("controls", "")
        el.setAttribute("playsinline", "")
        el.setAttribute("preload", "metadata")
        fig.appendChild(el)
        root.appendChild(fig)
      }
    }

    return {title, root}
  }

  if (isXhs) {
    const article = (await xhsArticle()) || (await xhsArticleFromDOM())
    if (article) {
      if (opts.inline) {
        await inlineImages(article.root)
      }
      return {
        html:
          '<!DOCTYPE html>\n<html><head><meta charset="utf-8"><title>' +
          escapeHTML(article.title) +
          "</title></head><body>" +
          article.root.outerHTML +
          "</body></html>",
        url: location.href,
        title: article.title || document.title,
      }
    }
  }

  // ---------------------------------------------------------------------
  // Generic page
  // ---------------------------------------------------------------------
  // 3. Build a clean copy of the document.
  const clone = document.documentElement.cloneNode(true)
  clone.querySelectorAll("script, link[rel='preload'], link[rel='modulepreload'], link[rel='manifest']").forEach((el) => el.remove())
  clone.querySelectorAll("[loading='lazy']").forEach((el) => el.removeAttribute("loading"))
  clone.querySelectorAll("[srcset]").forEach((el) => el.removeAttribute("srcset"))

  // 4. Resolve lazy attributes to their real source.
  const lazyAttrs = ["data-src", "data-original", "data-lazy-src", "data-actualsrc", "data-url", "file", "zoomfile"]
  clone.querySelectorAll("img, source, video").forEach((el) => {
    if (!el.getAttribute("src") || /^data:image\/(gif|svg)/i.test(el.getAttribute("src") || "")) {
      for (const attr of lazyAttrs) {
        const v = el.getAttribute(attr)
        if (v) {
          el.setAttribute("src", v)
          break
        }
      }
    }
    el.removeAttribute("loading")
  })

  // 4b. Sites that detect WebP support rewrite every picture to
  //     "foo.gif@880w_880h.webp" (Bilibili does it on the fly). The CDN then
  //     serves a single static frame, so animated GIFs are archived as still
  //     images. Dropping the trailing ".webp" asks for the original format
  //     back. See tools/web-extension/README.md.
  const restoreOriginal = (url) =>
    url.replace(/(\.(?:gif|png|jpe?g|bmp|webp)(?:@[^/]*?)?)\.webp$/i, "$1")
  clone.querySelectorAll("img[src], source[src], video[src]").forEach((el) => {
    if (/\.webp$/i.test(el.getAttribute("src"))) {
      el.setAttribute("src", restoreOriginal(el.getAttribute("src")))
    }
  })

  // 5. Optionally inline the images so hotlink protection or a login cookie
  //    cannot break the archive later.
  if (opts.inline) {
    await inlineImages(clone)
  }

  const comment = document.createComment(" Saved by Readeck 剪藏助手 — source: " + location.href + " ")
  clone.insertBefore(comment, clone.firstChild)

  return {
    html: "<!DOCTYPE html>\n" + clone.outerHTML,
    url: location.href,
    title: document.title,
  }
}

async function savePage() {
  const server = $("server").value.trim().replace(/\/+$/, "")
  const token = $("token").value.trim()
  if (!server || !token) {
    setStatus("请先填写服务器地址和 API 令牌", "err")
    return
  }

  const button = $("save")
  button.disabled = true
  setStatus("正在抓取页面…")

  try {
    await chrome.storage.local.set({
      server,
      token,
      inline: $("inline").checked,
      scroll: $("scroll").checked,
    })

    const [tab] = await chrome.tabs.query({active: true, currentWindow: true})
    if (!tab || !tab.id || !/^https?:/.test(tab.url || "")) {
      throw new Error("当前标签页无法剪藏（只支持 http/https 页面）")
    }

    const [res] = await chrome.scripting.executeScript({
      target: {tabId: tab.id},
      func: capturePage,
      args: [{inline: $("inline").checked, scroll: $("scroll").checked}],
    })
    const page = res && res.result
    if (!page || !page.html) {
      throw new Error("页面抓取失败")
    }

    setStatus("正在提交（" + Math.round(page.html.length / 1024) + " KB）…")

    const rsp = await fetch(server + "/api/bookmarks", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        Authorization: "Bearer " + token,
      },
      body: JSON.stringify({url: page.url, title: page.title, html: page.html}),
    })

    if (rsp.status === 401 || rsp.status === 403) {
      throw new Error("令牌无效或没有写权限（HTTP " + rsp.status + "）")
    }
    if (!rsp.ok) {
      const text = await rsp.text()
      throw new Error("HTTP " + rsp.status + " " + text.slice(0, 160))
    }

    const data = await rsp.json().catch(() => ({}))
    setStatus("已提交" + (data.id ? "（" + data.id + "）" : "") + "，几秒后可刷新查看", "ok")
  } catch (err) {
    setStatus(String(err && err.message ? err.message : err), "err")
  } finally {
    button.disabled = false
  }
}

$("save").addEventListener("click", savePage)
$("settings").addEventListener("click", saveSettings)
loadSettings()
