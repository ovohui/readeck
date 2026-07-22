// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10

/**
 * @returns {boolean}
 */
exports.isActive = function () {
  return $.domain == "spotify.com"
}

/**
 * @param {Config} config
 */
exports.setConfig = function (config) {
  config.httpHeaders["User-Agent"] = "curl/7.0"
}
