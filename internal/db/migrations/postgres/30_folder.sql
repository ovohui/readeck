-- SPDX-FileCopyrightText: © 2026 Readeck custom build
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE TABLE IF NOT EXISTS folder (
    id        SERIAL      PRIMARY KEY,
    user_id   integer  NOT NULL,
    parent_id integer  NULL,
    name      text     NOT NULL,
    created   timestamptz NOT NULL,
    updated   timestamptz NOT NULL,

    CONSTRAINT fk_folder_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE,
    CONSTRAINT fk_folder_parent FOREIGN KEY (parent_id) REFERENCES folder(id) ON DELETE CASCADE
);

CREATE INDEX folder_user_idx ON "folder" (user_id);
CREATE INDEX folder_parent_idx ON "folder" (parent_id);

ALTER TABLE "bookmark" ADD COLUMN folder_id integer NULL;
CREATE INDEX bookmark_folder_idx ON "bookmark" (folder_id);
