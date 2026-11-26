// ==UserScript==
// @name         Readeck 网页保存器（发送渲染后的 DOM）
// @namespace    readeck-saver
// @version      1.0
// @description  把当前已完整加载的网页（含 JS 渲染出的图片）直接发送到 Readeck 归档，绕过服务端抓取不到 JS 内容的问题
// @author       Readeck custom build
// @match        *://*/*
// @grant        GM_xmlhttpRequest
// @grant        GM_setValue
// @grant        GM_getValue
// @grant        GM_registerMenuCommand
// @connect      *
// @run-at       document-idle
// ==/UserScript==

/*
 * 为什么需要这个脚本
 * ------------------
 * Readeck 是服务端抓取的：它拿到的是原始 HTML，正文由 JavaScript 渲染的页面
 * （小红书、新版贴吧、站酷、以及所有滚动才加载图片的站点）在服务端就是空壳。
 * Readeck 的 API 支持直接提交 HTML：
 *
 *   POST /api/bookmarks
 *   {"url": "页面地址", "html": "完整的 HTML 源码"}
 *
 * 本脚本在页面里跑，拿的是渲染完成后的 DOM，POST 过去之后 Readeck 就直接用
 * 这份 HTML 提取正文和图片，不会再自己去抓一遍。
 */

;(function () {
  "use strict"

  const K_SERVER = "readeck_server"
  const K_TOKEN = "readeck_token"

  /* ---------------- 配置 ---------------- */

  function getConfig() {
    let server = GM_getValue(K_SERVER, "")
    let token = GM_getValue(K_TOKEN, "")

    if (!server) {
      server = window.prompt(
        "Readeck 服务器地址（例如 https://myreadeck.example.com:8000，不要带结尾斜杠）",
        "",
      )
      if (!server) {
        return null
      }
      server = server.replace(/\/+$/, "")
      GM_setValue(K_SERVER, server)
    }

    if (!token) {
      token = window.prompt(
        "Readeck API 令牌\n（Readeck 里：右上角头像 → 设置 → API 令牌 → 新建）",
        "",
      )
      if (!token) {
        return null
      }
      token = token.trim()
      GM_setValue(K_TOKEN, token)
    }

    return {server, token}
  }

  /* ---------------- 提示 ---------------- */

  function toast(text, timeout) {
    let el = document.getElementById("__readeck_saver_tip")
    if (!el) {
      el = document.createElement("div")
      el.id = "__readeck_saver_tip"
      el.style.cssText =
        "position:fixed;top:16px;right:16px;z-index:2147483647;" +
        "background:rgba(20,20,20,.92);color:#fff;padding:10px 16px;border-radius:8px;" +
        "font:14px/1.5 system-ui,sans-serif;box-shadow:0 4px 16px rgba(0,0,0,.3);" +
        "max-width:320px;pointer-events:none;"
      document.body.appendChild(el)
    }
    el.textContent = text
    if (timeout) {
      setTimeout(() => el.remove(), timeout)
    }
  }

  /* ---------------- 图片处理 ---------------- */

  const LAZY_ATTRS = [
    "data-src",
    "data-original",
    "data-lazy-src",
    "data-lazy",
    "data-actualsrc",
    "data-real-src",
    "data-echo",
    "data-url",
    "data-image",
    "file",
    "zoomfile",
  ]

  /** 找出 img 的真实地址（懒加载属性 / srcset / currentSrc）。 */
  function resolveSrc(img) {
    for (const attr of LAZY_ATTRS) {
      const v = img.getAttribute(attr)
      if (v && /^(https?:|\/)/.test(v)) {
        try {
          return new URL(v, location.href).href
        } catch (e) {
          /* ignore */
        }
      }
    }
    const set = img.getAttribute("data-srcset") || img.getAttribute("srcset")
    if (set) {
      const parts = set
        .split(",")
        .map((s) => s.trim().split(/\s+/)[0])
        .filter(Boolean)
      if (parts.length) {
        try {
          return new URL(parts[parts.length - 1], location.href).href
        } catch (e) {
          /* ignore */
        }
      }
    }
    return img.currentSrc || img.src || ""
  }

  function fetchAsDataURL(url, timeout) {
    return new Promise((resolve) => {
      GM_xmlhttpRequest({
        method: "GET",
        url,
        responseType: "blob",
        headers: {Referer: location.href},
        timeout: timeout || 20000,
        onload: (r) => {
          if (r.status < 200 || r.status >= 300) {
            return resolve(null)
          }
          const blob = r.response
          if (!blob || blob.size > 20 * 1024 * 1024) {
            return resolve(null)
          }
          const fr = new FileReader()
          fr.onload = () => resolve(fr.result)
          fr.onerror = () => resolve(null)
          fr.readAsDataURL(blob)
        },
        onerror: () => resolve(null),
        ontimeout: () => resolve(null),
      })
    })
  }

  /** 有限并发地处理数组。 */
  async function pool(items, limit, fn) {
    const queue = items.slice()
    const workers = new Array(Math.min(limit, queue.length)).fill(0).map(async () => {
      while (queue.length) {
        const item = queue.shift()
        await fn(item)
      }
    })
    await Promise.all(workers)
  }

  /**
   * 把图片转成 data: URL，保证服务端一定能存下来。
   * blob: 地址必须先转，离开页面就失效。
   */
  async function inlineImages(root, progress) {
    const imgs = Array.from(root.querySelectorAll("img"))
    let done = 0
    await pool(imgs, 6, async (img) => {
      const src = resolveSrc(img)
      if (!src || src.startsWith("data:")) {
        return
      }
      const data = await fetchAsDataURL(src)
      if (data) {
        img.setAttribute("src", data)
      } else {
        img.setAttribute("src", src)
      }
      img.removeAttribute("srcset")
      LAZY_ATTRS.forEach((a) => img.removeAttribute(a))
      done++
      if (progress && done % 5 === 0) {
        progress(done, imgs.length)
      }
    })
    root.querySelectorAll("picture source").forEach((s) => s.remove())
  }

  /** 保留懒加载属性的真实地址，不做内联（体积小，但依赖服务端能访问图床）。 */
  function resolveWithoutInlining(root) {
    root.querySelectorAll("img").forEach((img) => {
      const src = resolveSrc(img)
      if (src) {
        img.setAttribute("src", src)
      }
      img.removeAttribute("srcset")
      LAZY_ATTRS.forEach((a) => img.removeAttribute(a))
    })
    root.querySelectorAll("picture source").forEach((s) => s.remove())
  }

  /* ---------------- 序列化 ---------------- */

  /** 把处理过的克隆体序列化成完整 HTML 文档。 */
  function serializeClone(clone) {
    // 脚本与预加载提示对归档没有意义，删掉能显著减小体积
    clone
      .querySelectorAll("script, link[rel='preload'], link[rel='modulepreload'], link[rel='manifest']")
      .forEach((el) => el.remove())
    clone.querySelectorAll("[loading='lazy']").forEach((el) => el.removeAttribute("loading"))
    const comment = document.createComment(
      " Saved by Readeck userscript — source: " + location.href + " ",
    )
    clone.insertBefore(comment, clone.firstChild)
    return "<!DOCTYPE html>\n" + clone.outerHTML
  }

  /* ---------------- 提交 ---------------- */

  /**
   * 去掉标题尾巴上的站点名。document.title 几乎总会带一份：
   * "标题_哔哩哔哩_bilibili"、"标题 - 少数派"、"标题 | 知乎"。
   * @param {string} raw
   * @returns {string}
   */
  function cleanTitle(raw) {
    let t = String(raw || "").trim()
    if (!t) {
      return t
    }
    t = t
      .replace(/\s*[-_|]\s*哔哩哔哩\s*[-_|]?\s*bilibili\s*$/i, "")
      .replace(/\s*[-_|]\s*哔哩哔哩\s*$/i, "")
      .replace(/\s*[-_|]\s*bilibili\s*$/i, "")

    for (let i = 0; i < 2; i++) {
      const m = t.match(/^(.+?)(?:\s+[-–—]\s*|\s*[_|·]\s*)([^-–—|_·\s]{1,20})$/)
      if (!m || /[.。,，!！?？:：;；]/.test(m[2]) || m[2].length * 2 > t.length) {
        break
      }
      t = m[1].trim()
    }

    // 新闻站会把标签接在标题后面：…的决定|总书记|中央纪委
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

    return t.trim()
  }

  function postToReadeck(cfg, html) {
    return new Promise((resolve, reject) => {
      GM_xmlhttpRequest({
        method: "POST",
        url: cfg.server + "/api/bookmarks",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
          Authorization: "Bearer " + cfg.token,
        },
        data: JSON.stringify({
          url: location.href,
          title: cleanTitle(document.title),
          html: html,
        }),
        timeout: 180000,
        onload: (r) => {
          if (r.status >= 200 && r.status < 300) {
            let id = ""
            try {
              id = JSON.parse(r.responseText).id || ""
            } catch (e) {
              /* ignore */
            }
            resolve({id})
          } else if (r.status === 401 || r.status === 403) {
            reject(new Error("令牌无效或没有权限（HTTP " + r.status + "）"))
          } else {
            reject(new Error("HTTP " + r.status + " " + (r.responseText || "").slice(0, 200)))
          }
        },
        onerror: () => reject(new Error("网络错误，检查服务器地址和证书")),
        ontimeout: () => reject(new Error("超时")),
      })
    })
  }

  /* ---------------- 主流程 ---------------- */

  let busy = false

  async function save(inline) {
    if (busy) {
      return
    }
    const cfg = getConfig()
    if (!cfg) {
      return
    }

    busy = true
    try {
      toast(inline ? "⏳ 正在内嵌图片…" : "⏳ 正在读取页面…")

      // 内联必须在克隆体上做，避免弄脏当前页面
      const clone = document.documentElement.cloneNode(true)
      if (inline) {
        await inlineImages(clone, (done, total) => {
          toast("⏳ 图片 " + done + "/" + total)
        })
      } else {
        resolveWithoutInlining(clone)
      }

      const html = serializeClone(clone)

      toast("⏳ 正在发送到 Readeck…（" + Math.round(html.length / 1024) + " KB）")
      const {id} = await postToReadeck(cfg, html)
      toast("✅ 已提交到 Readeck" + (id ? "（" + id + "）" : "") + "，稍等几秒即可查看", 4000)
    } catch (e) {
      toast("❌ " + e.message, 6000)
    } finally {
      busy = false
    }
  }

  /* ---------------- 入口 ---------------- */

  GM_registerMenuCommand("📥 保存到 Readeck（图片内嵌，最稳）", () => save(true))
  GM_registerMenuCommand("📥 保存到 Readeck（保留图片地址，体积小）", () => save(false))
  GM_registerMenuCommand("⚙ 重新设置服务器 / 令牌", () => {
    GM_setValue(K_SERVER, "")
    GM_setValue(K_TOKEN, "")
    toast("已清除配置，下次保存时会重新询问")
  })

  window.addEventListener("keydown", (e) => {
    if (e.ctrlKey && e.altKey && e.shiftKey && e.code === "KeyR") {
      e.preventDefault()
      save(true)
    }
  })
})()
