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

  // 1. Scroll through the page so lazy loaded images get their real src.
  if (opts.scroll) {
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

  // 5. Optionally inline the images so hotlink protection or a login cookie
  //    cannot break the archive later.
  if (opts.inline) {
    const toDataURL = async (url) => {
      // Direct fetch works for same-origin or CORS-friendly images. Everything
      // else goes through the service worker, which holds <all_urls> host
      // permissions and is therefore not bound by CORS.
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

      const resp = await chrome.runtime.sendMessage({type: "fetchDataURL", url})
      if (!resp || !resp.dataURL) {
        throw new Error(resp && resp.error ? resp.error : "image fetch failed")
      }
      return resp.dataURL
    }

    const nodes = Array.from(clone.querySelectorAll("img[src]")).filter((el) => {
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
