// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * Subset of data received by reddit JSON endpoint.
 *
 * @typedef {{
 *  author: string,
 *  created_utc: number,
 *  domain: string,
 *  is_self: boolean,
 *  selftext_html: string,
 *  title: string,
 *  url: string,
 *  gallery_data?: {
 *    items: Array<{
 *      id: number,
 *      media_id: string,
 *    }>,
 *  },
 *  media?: {
 *    reddit_video?: {
 *      duration: number,
 *      hls_url: string,
 *    }
 *  },
 *  media_metadata?: Record<string, {
 *    e: string,
 *    s: {
 *      u: string,
 *      x: number,
 *      y: number,
 *    },
 *  }>,
 *  preview?: {
 *   images: Array<{
 *    id: string,
 *    source: {
 *     url: string,
 *    },
 *   }>
 *  },
 * }} redditData
 */

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "reddit.com" && !!new URL($.url).pathname.match(/^\/r\//)
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  // Discard the reddit site config
  $.overrideConfig(config, "")
}

exports.documentDone = () => {
  $.site = "Reddit"
}

/**
 * @param {Node} document
 */
exports.documentLoaded = (document) => {

  // when the user submits a selection from the extension, note via $.readability false.
  // Leave submitted content as is.
  //
  // Addresses: https://codeberg.org/readeck/readeck/issues/1224
  if ($.readability === false) {
    return
  }

  const url = new URL($.url)
  url.pathname += ".json"

  let rsp
  try {
    rsp = requests.get(url)
    rsp.raiseForStatus()
  } catch (e) {
    console.error("reddit: could not fetch JSON endpoint", {
      url: url.toString(),
      status: rsp ? rsp.status : undefined,
      error: String(e),
    })
    // the json endpoint is 403-ing.  In that case, fallback to extracting from dom.
    //
    // split extractors between old. and everything else
    const isOld = url.hostname.split(".")[0] === "old"
    if (isOld) {
      extractFromDOMOld(document)
    } else {
      extractFromDOMNew(document)
    }
    return
  }

  /** @type {redditData} */
  const data = rsp.json()[0]?.data?.children[0]?.data

  $.properties["json"] = data

  // Get the title
  if (data.title) {
    $.title = data.title
    $.meta["graph.title"] = [$.title]
  }

  // Get the author
  if (data.author) {
    $.authors = [data.author]
  }

  // Set date
  if (data.created_utc) {
    $.meta["html.date"] = [new Date(data.created_utc * 1000).toISOString()]
  }

  // Load HTML content first
  let html = "<!-- -->"
  if (data.selftext_html) {
    html = unescapeHTML(data.selftext_html)
  }

  // Get preview image
  const preview = findPreview(data)
  if (preview) {
    $.meta["x.picture_url"] = [unescapeHTML(preview)]
  }

  if (data.domain == "i.redd.it") {
    // Single image
    // https://www.reddit.com/r/catstairs/comments/1nhmpt9/a_pet_as_you_pass_pls/
    // https://www.reddit.com/r/EarthPorn/comments/1nj04yr/golden_layers_southern_californiaoc_3000x1839/
    $.type = "photo"
    $.meta["x.picture_url"] = [data.url]
  } else if (data.media?.reddit_video?.hls_url) {
    // Video
    // https://www.reddit.com/r/cats/comments/1nid3kk/michelle/
    // https://www.reddit.com/r/nextfuckinglevel/comments/1nidkkb/incredible_pirates_of_the_caribbean_parade_float/
    $.type = "video"
    $.meta["oembed.html"] = [
      `<hls src="${data.media.reddit_video.hls_url}"></hls>`,
    ]
    $.meta["x.duration"] = [String(data.media.reddit_video.duration)]
  } else if (!!data.gallery_data?.items) {
    // Image gallery
    // https://www.reddit.com/r/pics/comments/1nciozc/oc_leaflets_ordering_residents_to_immediately/
    // https://www.reddit.com/r/cats/comments/1neucd9/i_made_bio_sheets_for_our_catsitter/
    // https://www.reddit.com/r/cats/comments/1m8fhyq/is_my_cat_okay/
    const images = data.gallery_data.items
      .map((item) => {
        const img = data.media_metadata?.[item.media_id]?.s?.u
        return img
      })
      .filter((item) => !!item)

    for (let img of images) {
      html += `\n<figure><img src="${img}" alt=""></figure>`
    }

    if (images.length > 0) {
      $.meta["x.picture_url"] = [unescapeHTML(images[0])]
    }
  } else if (!data.is_self) {
    // Not a self post, get the link
    // https://www.reddit.com/r/UplifitingNews/comments/8mh3pt/dog_adopts_nine_orphaned_ducklings/
    html += `<p>Link to <a href="${data.url}">${data.url}</a></p>`
  }

  document.body.innerHTML = html
  $.readability = false
}

/**
 * for old.reddit.com: extracts the post's content directly from rendered
 * DOM.
 *
 * @param {Node} document
 *
 * tested on
 * content post
 *   https://old.reddit.com/r/houseplants/comments/1ts8gbs/showing_off_built_on_greenhouse/
 * external link post
 *   https://old.reddit.com/r/worldnews/comments/1tyz96q/albania_freezes_assets_in_kushner_resort_probe/
 * single image post
 *   https://old.reddit.com/r/MurderedByWords/comments/1tywa7o/hunter_calls_out_the_shameful_maga_lies_about/
 *
 */
function extractFromDOMOld(document) {
  let html = ""

  // post author, scoped to the post itself (".thing.link")
  const author = document.querySelector("#siteTable .thing.link .tagline .author")
  if (author) {
    $.authors = [author.textContent]
  }

  // submission date: from sidebar "submitted on" timestamp
  const dateEl = document.querySelector(".linkinfo .date time")
  if (dateEl) {
    $.meta["html.date"] = [dateEl.getAttribute("datetime")]
  }

  // post text (selftext) scoped to siteTable to avoid comments
  const selftext = document.querySelector("#siteTable .usertext-body .md")
  if (selftext) {
    html += selftext.innerHTML
  }

  const images = document
    .querySelectorAll(".gallery-item-thumbnail-link")
    .map((a) => a.getAttribute("href"))
    .filter((href) => !!href)

  if (images.length > 0) {
    $.meta["x.picture_url"] = [unescapeHTML(images[0])]
    html += images
      .map((src) => `<figure><img src="${unescapeHTML(src)}" alt=""></figure>`)
      .join("\n")
  } else {
    // not a gallery: this could be a single image post
    const mediaPreview = document.querySelector(".entry .media-preview")
    const src =
      mediaPreview?.querySelector("a")?.getAttribute("href") ||
      mediaPreview?.querySelector("img")?.getAttribute("src")

    if (src) {
      $.meta["x.picture_url"] = [unescapeHTML(src)]
      html += `<figure><img src="${unescapeHTML(src)}" alt=""></figure>`
    }
  }

  // when post's link points off-redit, surface it
  // this covers link-type posts
  // filter links to reddit because a self-post links to reddit
  let link = document.querySelector("div.sitetable .title a")?.getAttribute("href")
  if (link && !link.includes("reddit.com")) {
    html += `<p>Link: <a href="${link}">${link}</a></p>`

  } else {
    link = undefined
  }

  if (html == "") {
    return
  }

  $.html = `<section id="main">${html}</section>`
  $.readability = false

  console.info("reddit: content extracted from DOM fallback", {
    author: author ? author.textContent : undefined,
    date: dateEl ? dateEl.getAttribute("datetime") : undefined,
    has_selftext: !!selftext,
    image_count: images.length,
    link,
  })
}

/**
 * non old.reddit.com dom extractor
 *
 * @param {Node} document
 *
 * tested on
 * content post
 *   https://www.reddit.com/r/houseplants/comments/1ts8gbs/showing_off_built_on_greenhouse/
 * external link post
 *   https://www.reddit.com/r/worldnews/comments/1tyz96q/albania_freezes_assets_in_kushner_resort_probe/
 * single image post
 *   https://old.reddit.com/r/MurderedByWords/comments/1tywa7o/hunter_calls_out_the_shameful_maga_lies_about/
 *
 */
function extractFromDOMNew(document) {
  let html = ""

  // post author: look for slot="authorName" then rip from aria 
  const authorSlot = document.querySelector('[slot="authorName"]')
  const authorLink = authorSlot?.querySelector('a[aria-label^="Author:"]')
  const authorName = authorLink
    ?.getAttribute("aria-label")
    ?.replace("Author: ", "")
    .trim()
  if (authorName) {
    $.authors = [authorName]
  }

  // submission date: slot="credit-bar" is ISO datetime
  const creditBar = document.querySelector('[slot="credit-bar"]')
  const dateEl = creditBar?.querySelector("time")
  if (dateEl) {
    $.meta["html.date"] = [dateEl.getAttribute("datetime")]
  }

  // post content: custom (currently shreddit-post-text-body) then fall
  // back to property="schema:articleBody" if not found
  const selftext =
    document.querySelector("shreddit-post-text-body") ||
    document.querySelector('[slot="text-body"] [property="schema:articleBody"]')
  if (selftext) {
    html += selftext.innerHTML
  }

  // extract media: grab highest-resolution candidate from srcsets, and
  // the outbound link for "link posts" rendered as a lightbox preview.
  const images = []
  let link
  const mediaContainer = document.querySelector('[slot="post-media-container"]')
  if (mediaContainer) {
    // for single image posts: "#post-image" is the actual picture with
    // a (duplicative) preview.  special case to keep just the one image
    const postImage = mediaContainer.querySelector("#post-image")
    if (postImage) {
      const src = bestImageSrc(postImage)
      if (src) {
        images.push(src)
      }
    } else {
      for (const img of mediaContainer.querySelectorAll("img")) {
        const src = bestImageSrc(img)
        if (!src) {
          continue
        }
        const key = src.split("?")[0]
        if (images.some((x) => x.split("?")[0] === key)) {
          continue
        }
        images.push(src)
      }
    }

    link = mediaContainer
      .querySelector('[source="post_lightbox"] a')
      ?.getAttribute("href")
  }

  if (images.length > 0) {
    $.meta["x.picture_url"] = [images[0]]
    html += images
      .map((src) => `<figure><img src="${src}" alt=""></figure>`)
      .join("\n")
  }

  // always add a post's link if it points off reddit
  if (link) {
    const linkURL = new URL(link, $.url)
    if (linkURL.hostname.endsWith("reddit.com")) {
      link = undefined
    } else {
      link = linkURL.toString()
      html += `<p>Link: <a href="${link}">${link}</a></p>`
    }
  }

  if (html == "") {
    return
  }

  $.html = `<section id="main">${html}</section>`
  $.readability = false

  console.info("reddit: content extracted from DOM fallback", {
    author: authorName,
    date: dateEl ? dateEl.getAttribute("datetime") : undefined,
    has_selftext: !!selftext,
    image_count: images.length,
    link,
  })
}

/**
 * extract highest resolution image url from srcset 
 *
 * @param {Node} img
 * @returns {string | undefined}
 */
function bestImageSrc(img) {
  const srcset = img.getAttribute("srcset")
  if (srcset) {
    let bestURL
    let bestWidth = 0
    for (const entry of srcset.split(",")) {
      const [url, width] = entry.trim().split(/\s+/)
      const w = parseInt(width, 10)
      if (url && !isNaN(w) && w > bestWidth) {
        bestWidth = w
        bestURL = url
      }
    }
    if (bestURL) {
      return unescapeHTML(bestURL)
    }
  }

  const src = img.getAttribute("src")
  return src ? unescapeHTML(src) : undefined
}

/**
 *
 * @param {redditData} data
 * @returns {string | undefined}
 */
function findPreview(data) {
  if (data.preview?.images[0]?.source?.url) {
    return unescapeHTML(data.preview?.images[0]?.source?.url)
  }

  if (data.media_metadata) {
    const img = Object.entries(data.media_metadata).find(([_, o]) => {
      return !!o.e && o.e.toLowerCase() == "image" && !!o.s?.u
    })[1].s.u
    if (img) {
      return unescapeHTML(img)
    }
  }
}
