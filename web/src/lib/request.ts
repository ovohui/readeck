// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/* global RequestInit BodyInit */

interface requestInit extends RequestInit {
  headers: Headers
}

interface requestOptions {
  headers?: Record<string, string>
  query?: string | Record<string, string>
  method?: string
  body?: BodyInit
}

async function request(path: string, options: requestOptions) {
  const {headers, query, method = "GET", body} = options

  // Prep options
  const reqOptions: requestInit = {
    method: method.toUpperCase(),
    headers: new Headers({
      ...headers,
    }),
    body: body || null,
  }

  // Convert FormData to a shorter url-encoded value
  if (body instanceof FormData) {
    let params: URLSearchParams | null = new URLSearchParams()
    for (const e of body) {
      if (e[1] instanceof Blob) { // only if no blob is present
        params = null
        break
      }
      if (typeof e[1] == "string") {
        params.append(e[0], e[1])
      }
    }

    if (params !== null) {
      reqOptions.body = params.toString()
      reqOptions.headers.set("Content-Type", "application/x-www-form-urlencoded")
    }
  }

  // Prep URL
  let qs = ""
  if (query) {
    qs = new URLSearchParams(query).toString()
    qs = qs && `?${qs}`
  }

  const req = new Request(`${path}${qs}`, reqOptions)
  return await fetch(req)
}

export {request}

export type {requestOptions}
