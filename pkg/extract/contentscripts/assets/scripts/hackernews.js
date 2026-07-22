// SPDX-FileCopyrightText: © 2026 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.host == "news.ycombinator.com"
}

/**
 * @param {Node} document
 */
exports.documentLoaded = function (document) {
  const body = document.body
  if (!body) throw "document body missing"

  const bigbox = document.querySelector("#bigbox")
  if (!bigbox) return

  const fatitem = bigbox.querySelector(".fatitem")
  body.append(fatitem.querySelector(".submission .titleline > a[href]"))
  const byline = document.createElement("p")
  const hnuser = fatitem.querySelector(".hnuser")
  const age = fatitem.querySelector(".age[title]")
  byline.append("posted by ", hnuser, " on ", ageToTimeElement(document, age))
  body.append(byline)

  const toptext = fatitem.querySelector(".toptext")
  if (toptext) body.append(toptext, document.createElement("hr"))

  const comments = bigbox.querySelectorAll(".commtext")
  const commentsHeading = document.createElement("h2")
  commentsHeading.append(
    `${comments.length} ${comments.length > 1 ? "comments" : "comment"}`,
  )
  body.append(commentsHeading)

  const dl = document.createElement("dl")

  comments.forEach((commtext) => {
    const comtr = commtext.closest(".comtr")
    const hnuser = comtr.querySelector(".comhead .hnuser")
    const age = comtr.querySelector(".comhead .age[title]")
    const itemlink = age.querySelector("a[href]")
    itemlink.firstChild?.remove()
    itemlink.append(ageToTimeElement(document, age))
    const byline = document.createElement("p")
    byline.append(hnuser, " • ", itemlink)

    // turn top level div into p
    if (commtext.nodeName.toLowerCase() == "div") {
      const e = document.createElement("p")
      e.append(...commtext.childNodes)
      commtext = e
    }

    // extract quoted message
    commtext.childNodes.forEach((e) => {
      const text = e.textContent.trim()
      if (
        e.nodeName.toLowerCase() == "i" &&
        e.previousSibling?.nodeType == e.TEXT_NODE &&
        e.previousSibling?.textContent.startsWith(">")
      ) {
        // can be "> <i>text</i>"
        commtext.removeChild(e.previousSibling)
        const quote = document.createElement("blockquote")
        quote.append(...e.childNodes)
        commtext.replaceChild(quote, e)
      } else if (text.startsWith(">") && text != ">") {
        // or a node starting with ">"
        const quote = document.createElement("blockquote")
        quote.appendChild(document.createTextNode(text.substring(1).trim()))
        commtext.replaceChild(quote, e)
      }
    })

    const dt = document.createElement("dt")
    const dd = document.createElement("dd")
    dt.append(byline)
    dd.append(commtext)
    dl.append(dt, dd)
  })

  body.append(dl)

  bigbox.closest("table").remove()
  $.readability = false
}

/**
 * @param {Node} document
 * @param {Node} age
 * @returns Element
 */
function ageToTimeElement(document, age) {
  const ts = age.getAttribute("title").replace(/ \d+$/, "")
  const time = document.createElement("time")
  time.setAttribute("datetime", ts)
  time.append(ts.replace("T", " ").replace(/:\d{2}$/, ""))
  return time
}
