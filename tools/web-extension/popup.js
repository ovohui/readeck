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
 * Runs in the page's main world (world: "MAIN"). A content script lives in an
 * isolated world: `window.__INITIAL_STATE__` is invisible there, which is why a
 * Xiaohongshu note used to come back as a handful of 640px thumbnails. This
 * probe copies out only the fields the archiver needs. It must stay self
 * contained.
 *
 * @returns {Promise<{note: object|null}>}
 */
async function xhsProbe() {
  const out = {note: null}

  const slimImage = (item) => {
    if (!item) {
      return null
    }
    return {
      urlDefault: item.urlDefault,
      urlPre: item.urlPre,
      infoList: item.infoList,
      width: item.width,
      height: item.height,
    }
  }

  try {
    const state = window.__INITIAL_STATE__
    const map = state && state.note && state.note.noteDetailMap
    if (map) {
      const hit = (location.pathname.match(/\/(?:explore|discovery\/item)\/([0-9a-zA-Z]+)/) || [])[1]
      let note = hit && map[hit] && map[hit].note
      if (!note) {
        const keys = Object.keys(map)
        if (keys.length === 1 && map[keys[0]] && map[keys[0]].note) {
          note = map[keys[0]].note
        }
      }
      if (note) {
        out.note = {
          type: note.type,
          title: note.title,
          desc: note.desc,
          time: note.time,
          ipLocation: note.ipLocation,
          user: note.user ? {nickname: note.user.nickname} : null,
          imageList: (note.imageList || []).map(slimImage),
        }
      }
    }
  } catch (e) {
    // state moved or changed shape: the caller falls back to the DOM
  }

  return out
}

/**
 * Runs inside the page. Must be self contained: it is serialized and injected
 * with chrome.scripting.executeScript.
 *
 * @param {{inline: boolean, scroll: boolean, xhs: object|null}} opts
 * @returns {Promise<{html: string, url: string, title: string}>}
 */
async function capturePage(opts) {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
  const isXhs = /(^|\.)xiaohongshu\.com$/i.test(location.hostname)
  // Set by a site specific rebuild when the page title is not the work title.
  let titleOverride = ""

  /**
   * Browsers fold the inbox counter into document.title on sites that have one,
   * and every template appends its own name. Both are noise in a bookmark card:
   *
   *   (99+ 封私信 / 80 条消息) 神舟二十四号载人飞船瞄准11月25日8时18分发射 - 知乎
   *   -> 神舟二十四号载人飞船瞄准11月25日8时18分发射
   *
   * A leading bracketed year "(2026)" is part of the headline, so a badge must
   * either name an inbox or be nothing but a small counter.
   */
  function cleanTitle(raw) {
    let t = String(raw || "").trim()
    if (!t) {
      return t
    }

    const banner = t.match(/^[(\[（【][^)\]）】]{1,60}[)\]）】]\s*/)
    if (banner) {
      const inner = banner[0].replace(/[()[\]（）【】\s]/g, "")
      const rest = t.slice(banner[0].length).trim()
      const digits = (inner.match(/\d/g) || []).length
      const named = /私信|消息|未读|通知|条新|inbox|unread|messages|notifications/i.test(inner)
      const bare = digits > 0 && digits <= 3 && /^[\d+/,.kK百千万]*$/.test(inner)
      if (rest && (named || bare)) {
        t = rest
      }
    }

    for (let i = 0; i < 2; i++) {
      const m = t.match(/^(.+?)(?:\s+[-–—]\s*|\s*[_|·]\s*)([^-–—|_·\s]{1,20})$/)
      if (!m) {
        break
      }
      if (/[.。,，!！?？:：;；]/.test(m[2]) || m[2].length * 2 > t.length) {
        break
      }
      t = m[1].trim()
    }

    // News templates append a tag list after the headline
    // ("…的决定|总书记|中央纪委"). Only short, punctuation-free trailing
    // segments are removed, and a single tag only when a real headline
    // remains, so "设计|界面" keeps its title.
    {
      const parts = t.split("|")
      let end = parts.length
      while (end > 1 && parts.length - end < 4) {
        const tag = parts[end - 1].trim()
        if (!tag || tag.length > 8 || /[.。,，!！?？:：;；、()（）]/.test(tag)) {
          break
        }
        end--
      }
      const removed = parts.length - end
      if (removed > 0 && (removed > 1 || parts.slice(0, end).join("|").length >= 10)) {
        t = parts.slice(0, end).join("|").trim()
      }
    }

    // Templates that append a whole brand line ("- OSCHINA - 开源 × AI · 开发者
    // 生态社区", "_财新网_财新网", "_哔哩哔哩_bilibili") are not covered by the
    // last-word rule above. Cut at the last known site name when it sits behind
    // a separator and what follows is short.
    const labels = [
      "哔哩哔哩", "bilibili", "财新网", "界面新闻", "网易政务", "澎湃新闻",
      "中国政库", "the paper", "oschina", "ithome", "少数派", "什么值得买",
      "36氪", "虎扑社区", "开源中国", "腾讯新闻", "微信公众平台", "小红书",
      "smzdm", "sspai", "infoq", "cnblogs", "juejin", "douban", "hupu",
    ]
    for (let pass = 0; pass < 3; pass++) {
      const lower = t.toLowerCase()
      let best = -1
      for (const label of labels) {
        const at = lower.lastIndexOf(label)
        if (at <= best) {
          continue
        }
        const prev = t[at - 1] || ""
        const after = t.slice(at + label.length)
        if (!/[\s_|·\-–—:]/.test(prev)) {
          continue
        }
        if (after && !/^[\s_|·\-–—:]/.test(after)) {
          continue
        }
        if (Array.from(after).length > 48) {
          continue
        }
        best = at
      }
      if (best <= 0) {
        break
      }
      t = t.slice(0, best).replace(/[\s_|·\-–—:]+$/, "").trim()
    }

    return t.trim()
  }

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
    // A 25-picture forum post is taller than 30k. Stopping there left the
    // GIFs at the bottom unloaded, so the saved page had 25 photos and no
    // animation. Walk the whole document (with a sane cap) instead.
    const step = Math.max(Math.floor(window.innerHeight * 0.8), 600)
    const max = Math.min(document.body.scrollHeight, 160000)
    for (let y = 0; y < max; y += step) {
      window.scrollTo(0, y)
      await sleep(80)
    }
    window.scrollTo(0, 0)
    await sleep(1200)
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
      const rsp = await fetch(url, {
        credentials: "include",
        cache: "force-cache",
        headers: {"Accept": "image/gif,image/png,image/jpeg,image/*;q=0.8"},
      })
      if (rsp.ok) {
        const blob = await rsp.blob()
        if (blob.size <= 32 * 1024 * 1024) {
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

  const lazyImageAttrs = [
    "data-src", "data-original", "data-lazy-src", "data-lazy",
    "data-actualsrc", "data-real-src", "data-echo", "data-file",
    "file", "zoomfile",
  ]

  // Discuz keeps the real picture in file/zoomfile while src may still be a
  // placeholder. Pick the best source before fetching, not just img.src.
  const resolveImageURL = (el) => {
    const src = el.getAttribute("src") || ""
    if (/^data:/i.test(src)) {
      return src
    }
    const absolute = (v) => {
      try {
        return new URL(v, location.href).href
      } catch (e) {
        return ""
      }
    }
    for (const attr of lazyImageAttrs) {
      const v = el.getAttribute(attr)
      if (v && !/^data:/i.test(v)) {
        return absolute(v)
      }
    }
    return src ? absolute(src) : ""
  }

  const inlineImages = async (root) => {
    const nodes = Array.from(root.querySelectorAll("img"))

    let index = 0
    const worker = async () => {
      while (index < nodes.length) {
        const el = nodes[index++]
        const src = resolveImageURL(el)
        if (!src) {
          continue
        }
        if (/^data:/i.test(src)) {
          el.setAttribute("src", src)
        } else {
          try {
            el.setAttribute("src", await toDataURL(src))
          } catch (e) {
            // Leave the remote URL in place; the server will try to fetch it.
            el.setAttribute("src", src)
          }
        }
        for (const attr of lazyImageAttrs) {
          el.removeAttribute(attr)
        }
        el.removeAttribute("srcset")
      }
    }
    await Promise.all([worker(), worker(), worker(), worker()])
    root.querySelectorAll("picture source").forEach((s) => s.remove())
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

  // A video note has no usable stream in the browser: the payload stops at
  // the cover, the player asks a signed API for the clip, and that URL
  // expires. Players do not survive archiving either - the extractor removes
  // every <video> and <iframe> from the content and rebuilds one from the page
  // URL. Leaving the note URL on the cover gets yt-dlp to do the download on
  // the server, where the time budget is minutes rather than a popup's second.
  const xhsIsVideoNote = (note) => {
    if (note && /^video/i.test(String(note.type || ""))) {
      return true
    }
    const meta = document.querySelector('meta[property="og:type"]')
    return !!meta && /video/i.test(meta.getAttribute("content") || "")
  }

  const markVideoCover = (img) => {
    const src = img.getAttribute("src") || ""
    if (!/^https?:/i.test(src)) {
      return
    }
    const params = new URLSearchParams()
    params.set("src", location.href)
    params.set("w", img.getAttribute("width") || "0")
    params.set("h", img.getAttribute("height") || "0")
    img.setAttribute("data-iframe-params", params.toString())
    // The cover doubles as the poster of the archived video.
    img.setAttribute("poster", src)
  }

  const pad = (n) => (n < 10 ? "0" + n : String(n))
  // Published time in UTC+8, the same clock the server side pass renders, so a
  // note archived from either side shows the same line.
  const formatTime = (ms) => {
    const t = Number(ms)
    if (!t || !isFinite(t)) {
      return ""
    }
    const d = new Date(t + 8 * 3600 * 1000)
    return (
      d.getUTCFullYear() + "-" + pad(d.getUTCMonth() + 1) + "-" + pad(d.getUTCDate()) +
      " " + pad(d.getUTCHours()) + ":" + pad(d.getUTCMinutes())
    )
  }

  // The state payload is the single source of truth: 8 images for an 8 image
  // note, in the author's order, with their real dimensions. The isolated
  // world cannot read it, so prefer the copy handed over by xhsProbe.
  const xhsNote = () => {
    if (opts.xhs && opts.xhs.note) {
      return opts.xhs.note
    }
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

    const desc = String(note.desc || "").replace(/\[话题\]#/g, "").trim()
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
    const videoNote = xhsIsVideoNote(note)
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

      // The cover of a video note stands in for the player: the server
      // downloads the clip and swaps this image for a local <video>.
      if (videoNote && i === 0) {
        markVideoCover(img)
      }
    }

    if (root.querySelector("img") === null) {
      return null
    }

    return {title, root}
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
      cleanTitle(document.title)

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
      const raw = (descEl.innerText || descEl.textContent || "").replace(/\[话题\]#/g, "")
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

    // No state payload means no image list either, so the first picture of the
    // container doubles as the player placeholder.
    if (xhsIsVideoNote(null)) {
      markVideoCover(root.querySelector("img"))
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
        title: article.title || cleanTitle(document.title),
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

  // 4c. A player never survives extraction: the archiver strips every <video>
  //     and <iframe> and rebuilds only the ones it can download itself. On a
  //     Zhihu answer the clip is rendered by a player, so the archive would
  //     lose it. Replacing the player with its poster plus the URL of the
  //     page that hosts the clip hands the download to yt-dlp on the server.
  //
  //     Two cases:
  //       - the player already points at a real stream (not blob:): keep it as
  //         a <video>, the server archives it like any other media resource;
  //       - it plays from MediaSource (blob:) or the clip sits in an iframe:
  //         poster + page URL, and the server asks yt-dlp for the file.
  if (/(^|\.)zhihu\.com$/i.test(location.hostname)) {
    const answerPage = () => {
      const hit = location.pathname.match(/\/question\/(\d+)\/answer\/(\d+)/)
      return hit
        ? "https://www.zhihu.com/question/" + hit[1] + "/answer/" + hit[2]
        : location.href
    }

    const placeholder = (el, poster, page) => {
      // Without a frame of its own, the page's share thumbnail stands in: the
      // archiver needs an <img> to hang the download on, and a bare link would
      // archive nothing at all.
      let cover = poster
      if (!cover) {
        const og = document.querySelector("meta[property='og:image'], meta[name='og:image']")
        cover = og ? og.getAttribute("content") || "" : ""
      }
      const figure = document.createElement("figure")
      if (cover) {
        const img = document.createElement("img")
        img.setAttribute("src", cover)
        img.setAttribute("poster", cover)
        img.setAttribute("data-iframe-params", "src=" + encodeURIComponent(page))
        figure.appendChild(img)
      } else {
        const link = document.createElement("a")
        link.setAttribute("href", page)
        link.textContent = page
        figure.appendChild(link)
      }
      el.replaceWith(figure)
    }

    clone.querySelectorAll("video").forEach((el) => {
      const src = el.getAttribute("src") || ""
      const stream = /^https?:/i.test(src) && /\.(mp4|m3u8|webm)(\?|$)/i.test(src)
      if (stream) {
        // Everything the player added on top of the file itself is noise.
        el.removeAttribute("autoplay")
        el.setAttribute("controls", "")
        el.setAttribute("preload", "metadata")
        const source = el.querySelector("source[src]")
        if (source) {
          el.setAttribute("src", source.getAttribute("src"))
          source.remove()
        }
        return
      }
      const poster = el.getAttribute("poster") || ""
      const holder = el.closest("a[href*='/video/'], a[href*='/zvideo/']")
      placeholder(el, poster, holder ? holder.href : answerPage())
    })

    // A column video is only an <a class="video-box"> with a poster.
    clone.querySelectorAll("a.video-box, a[data-lens-id]").forEach((el) => {
      const raw = el.getAttribute("data-lens-id") || el.getAttribute("href") || ""
      let decoded = raw
      try {
        decoded = decodeURIComponent(raw)
      } catch (e) {
        // keep raw
      }
      const hit = decoded.match(/video\/(\d{6,})/)
      if (!hit) {
        return
      }
      const poster =
        el.getAttribute("data-poster") ||
        (el.querySelector("img") ? el.querySelector("img").getAttribute("src") : "") ||
        ""
      placeholder(el, poster, "https://www.zhihu.com/video/" + hit[1])
    })

    // A clip embedded as a player frame: keep the frame's own URL, which is a
    // canonical /video/<id> page yt-dlp knows how to resolve.
    clone.querySelectorAll("iframe[src*='zhihu.com/video/']").forEach((el) => {
      const page = el.getAttribute("src") || ""
      if (page) {
        placeholder(el, "", page)
      }
    })
  }

  // 4d. ZCOOL (站酷) work pages. The desktop host answers anything that is not
  //     a real browser with an Aliyun WAF challenge, so the server cannot read
  //     the page itself; the work only exists in the Next.js payload anyway,
  //     and the gallery is painted by React. Rebuild the work from that payload
  //     so the archive keeps the pictures instead of an empty shell.
  if (/(^|\.)zcool\.com\.cn$/i.test(location.hostname) && /\/work\//.test(location.pathname)) {
    let work = null
    try {
      const node = document.querySelector("script#__NEXT_DATA__")
      const data = node ? JSON.parse(node.textContent) : null
      work = data && data.props && data.props.pageProps && data.props.pageProps.workData
    } catch (e) {
      work = null
    }

    if (work && (work.productImages || []).length > 0) {
      const title = String(work.title || document.title || "").trim()
      const author = work.creatorObj && work.creatorObj.username ? work.creatorObj.username : ""
      const when = (() => {
        const ms = Number(work.publishTime || work.createTime)
        if (!ms || !isFinite(ms)) {
          return ""
        }
        // Rendered in UTC+8: the archive shows the site's clock, not the
        // browser's.
        const d = new Date(ms + 8 * 3600 * 1000)
        const pad = (n) => (n < 10 ? "0" + n : String(n))
        return (
          d.getUTCFullYear() + "-" + pad(d.getUTCMonth() + 1) + "-" + pad(d.getUTCDate())
        )
      })()

      const meta = []
      if (author) {
        meta.push("作者：" + author)
      }
      const kind = [work.typeStr, [work.cateName, work.subcateName].filter(Boolean).join(" / ")]
        .filter(Boolean)
        .join(" · ")
      if (kind) {
        meta.push(kind)
      }
      if (when) {
        meta.push(when)
      }

      const article = document.createElement("article")
      article.className = "zcool-work"

      if (meta.length > 0) {
        const p = document.createElement("p")
        p.className = "zcool-meta"
        p.textContent = meta.join(" · ")
        article.appendChild(p)
      }

      for (const item of work.productImages) {
        const url = item && (item.urlBig || item.url)
        if (!url) {
          continue
        }
        const figure = document.createElement("figure")
        const img = document.createElement("img")
        img.setAttribute("src", url)
        img.setAttribute("alt", title)
        if (item.width && item.height) {
          img.setAttribute("width", item.width)
          img.setAttribute("height", item.height)
        }
        figure.appendChild(img)
        article.appendChild(figure)
      }

      const tags = (work.productTags || []).filter(Boolean).map((t) => "#" + t)
      if (tags.length > 0) {
        const p = document.createElement("p")
        p.className = "zcool-tags"
        p.textContent = tags.join(" ")
        article.appendChild(p)
      }

      const body = clone.querySelector("body")
      if (body) {
        body.innerHTML = ""
        body.appendChild(article)
      }

      // The page title carries the site name; the work title does not.
      const head = clone.querySelector("head")
      if (head && title) {
        let metaTag = head.querySelector("meta[property='og:title']")
        if (!metaTag) {
          metaTag = document.createElement("meta")
          metaTag.setAttribute("property", "og:title")
          head.appendChild(metaTag)
        }
        metaTag.setAttribute("content", title)
        const t = head.querySelector("title")
        if (t) {
          t.textContent = title
        }
      }
      titleOverride = title
    }
  }

  // 5. Optionally inline the images so hotlink protection or a login cookie
  //    cannot break the archive later.
  if (opts.inline) {
    await inlineImages(clone)
  }

  // The DOM is Unicode and the payload is UTF-8, but the cloned page may still
  // carry <meta charset="gbk">. Replace it, otherwise the server decodes UTF-8
  // bytes as GBK and Chinese text becomes mojibake (52pojie).
  const head = clone.querySelector("head")
  if (head) {
    head.querySelectorAll("meta[charset], meta[http-equiv='Content-Type']").forEach((el) => el.remove())
    const charset = document.createElement("meta")
    charset.setAttribute("charset", "utf-8")
    head.insertBefore(charset, head.firstChild)
  }

  const comment = document.createComment(" Saved by Readeck 剪藏助手 — source: " + location.href + " ")
  clone.insertBefore(comment, clone.firstChild)

  return {
    html: "<!DOCTYPE html>\n" + clone.outerHTML,
    url: location.href,
    title: cleanTitle(titleOverride || document.title),
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

    // Xiaohongshu keeps the note (images, signed video stream) in the page's
    // main world. Read it there first, then hand the copy to the isolated
    // capture: an isolated content script cannot see __INITIAL_STATE__.
    let xhs = null
    if (/:\/\/[^/]*xiaohongshu\.com/i.test(tab.url || "")) {
      try {
        const [probe] = await chrome.scripting.executeScript({
          target: {tabId: tab.id},
          world: "MAIN",
          func: xhsProbe,
        })
        xhs = (probe && probe.result) || null
      } catch (e) {
        xhs = null
      }
    }

    const [res] = await chrome.scripting.executeScript({
      target: {tabId: tab.id},
      func: capturePage,
      args: [{inline: $("inline").checked, scroll: $("scroll").checked, xhs}],
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
