-- SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE "bookmark" ADD COLUMN note text NOT NULL DEFAULT "";

-- include new field in FTS
DROP VIEW IF EXISTS bookmark_index_view;
CREATE VIEW bookmark_index_view (
    id,
    title,
    description,
    "text",
    site,
    author,
    label,
    note
)
AS SELECT
    id,
    title,
    description,
    "text",
    site_name || ' ' || site || ' ' || domain AS site,
    (
        SELECT group_concat(x.value, ' ')
        FROM json_each(
            CASE json_type(CASE json_valid(authors) WHEN TRUE THEN authors ELSE '0' END)
            WHEN 'array' THEN authors ELSE '[]' END
        ) AS x
    ) AS author,
    (
      SELECT group_concat(x.value, ' ')
      FROM json_each(
          CASE json_type(CASE json_valid(labels) WHEN TRUE THEN labels ELSE '0' END)
          WHEN 'array' THEN labels ELSE '[]' END
      ) AS x
    ) AS label,
    (
      SELECT group_concat(json_extract(x.value, '$.note'), ' ')
      FROM json_each(
          CASE json_type(CASE json_valid(annotations) WHEN TRUE THEN annotations ELSE '0' END)
          WHEN 'array' THEN annotations
          ELSE '[]' END
      ) AS x
      WHERE json_extract(x.value, '$.note') != ''
    ) || ' ' || note AS note
FROM bookmark;