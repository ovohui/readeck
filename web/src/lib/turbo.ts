// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {session as _session} from "@hotwired/turbo"
import type {TurboSession} from "@hotwired/turbo"

interface turboObserver {
  start(): void
}

interface turboSession extends Omit<TurboSession, "history"> {
  cacheObserver: turboObserver
  linkPrefetchObserver: turboObserver
  formLinkClickObserver: turboObserver
  linkClickObserver: turboObserver
  formSubmitObserver: turboObserver
  scrollObserver: turboObserver
  streamObserver: turboObserver
  frameRedirector: turboObserver
  preloader: turboObserver
  history: turboObserver

  started: boolean
  enabled: boolean
}

const session = <unknown>_session as turboSession

// The disableTurboStart esbuild plugin in this project prevents Turbo from auto-starting on import.
// Instead, start it manually by only enabling components that we use.
// https://github.com/hotwired/turbo/blob/v8.0.23/src/core/session.js#L52
//
// session.pageObserver.start() // avoid setting history.scrollRestoration = "manual"
// session.linkPrefetchObserver.start()
// session.history.start()
// session.preloader.start()
session.cacheObserver.start()
session.formLinkClickObserver.start()
session.linkClickObserver.start()
session.formSubmitObserver.start()
session.scrollObserver.start()
session.streamObserver.start()
session.frameRedirector.start()
session.started = true
session.enabled = true

const cspNonce = document.querySelector<HTMLMetaElement>(
  'html>head>meta[name="csp-nonce"]',
)!.content

const staleRefreshKey = "stale-refresh"

function markStaleRefresh() {
  window.localStorage.setItem(staleRefreshKey, requestUri(window.location))
}

function requestUri(location: Location): string {
  return location.pathname + location.search
}

// After navigating Back in the browser, check if another page content was marked stale and force
// reload of the current page to bust the browser's cache.
window.addEventListener("pageshow", (event) => {
  const staleUri = window.localStorage.getItem(staleRefreshKey)
  if (!staleUri || staleUri == requestUri(window.location)) {
    return
  }
  window.localStorage.removeItem(staleRefreshKey)

  if (
    event.persisted || (
      window.performance.getEntriesByType("navigation") as PerformanceNavigationTiming[]
    )[0]?.type == "back_forward"
  ) {
    window.location.reload()
  }
})

document.addEventListener("turbo:before-fetch-request", (evt) => {
  // Method MUST be uppercase
  evt.detail.fetchOptions.method = evt.detail.fetchOptions.method?.toUpperCase()

  // Mark the request for turbo rendering
  evt.detail.fetchOptions.headers["X-Turbo"] = "1"
  evt.detail.fetchOptions.headers["X-Turbo-Nonce"] = cspNonce
})

document.addEventListener("turbo:submit-end", (evt) => {
  // Empty children with data-turbo-empty-submit-end
  // attribute after form submission.
  if (evt.target instanceof Element) {
    evt.target.querySelectorAll("[data-turbo-empty-submit-end]").forEach((x) => {
      if ("value" in x) {
        x.value = ""
      }
    })
  }

  // Mark any unsafe request for refresh on back/forward navigation.
  if (
    evt.detail.success
    && !["GET", "HEAD", "OPTIONS"].includes(
      evt.detail.formSubmission.method.toUpperCase(),
    )
  ) {
    markStaleRefresh()
  }
})

let scrollPosition = 0

// Turbo uses `document.body.replaceWith(newBody)` to replace the page with the contents received
// from the server, but that resets scroll position on WebKit, even with turbo-refresh-scroll being
// set to "preserve". This manually preserves scroll.
document.addEventListener("turbo:before-render", () => {
  scrollPosition = window.scrollY
})
document.addEventListener("turbo:render", () => {
  if (window.scrollY == 0 && scrollPosition > 0) {
    window.scrollTo({top: scrollPosition})
    scrollPosition = 0
  }
})
document.addEventListener("turbo:before-frame-render", () => {
  scrollPosition = window.scrollY
})
document.addEventListener("turbo:frame-render", () => {
  if (window.scrollY == 0 && scrollPosition > 0) {
    window.scrollTo({top: scrollPosition})
    scrollPosition = 0
  }
})

export {cspNonce, markStaleRefresh}
