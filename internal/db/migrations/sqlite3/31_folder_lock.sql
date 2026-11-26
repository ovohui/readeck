-- SPDX-FileCopyrightText: © 2026 Readeck custom build
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE "folder" ADD COLUMN password_hash text NOT NULL DEFAULT '';
ALTER TABLE "folder" ADD COLUMN is_locked integer NOT NULL DEFAULT 0;
