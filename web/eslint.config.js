// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {defineConfig} from "eslint/config"
import js from "@eslint/js"
import ts from "typescript-eslint"
import stylistic from "@stylistic/eslint-plugin"
import globals from "globals"

export default defineConfig([
  js.configs.recommended,
  ...ts.configs.recommended,
  stylistic.configs.recommended,
  {
    languageOptions: {
      globals: {...globals.browser, ...globals.node},
      parserOptions: {
        parser: ts.parser,
      },
    },
    plugins: {
      "@stylistic": stylistic,
    },
    rules: {
      "no-undef": "error",
      "no-console": process.env.NODE_ENV === "production" ? "warn" : "off",
      "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",

      "quotes": ["error", "double", {avoidEscape: true}],
      "semi": ["error", "never"],
      "comma-dangle": [
        "error",
        {
          arrays: "always-multiline",
          objects: "always-multiline",
          imports: "always-multiline",
          exports: "always-multiline",
          functions: "always-multiline",
        },
      ],
      "comma-spacing": ["error", {before: false, after: true}],

      "@typescript-eslint/no-unused-vars": ["off"],
      "@typescript-eslint/no-require-imports": ["off"],

      "@stylistic/arrow-parens": ["error", "always"],
      "@stylistic/brace-style": ["error", "1tbs"],
      "@stylistic/object-curly-spacing": ["error", "never"],
      "@stylistic/quotes": ["error", "double", {allowTemplateLiterals: "avoidEscape", avoidEscape: true}],
    },
  },
])
