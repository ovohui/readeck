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
