-- SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE "user" ADD COLUMN ext_id text NULL;
CREATE UNIQUE INDEX "user_ext_id_idx" on "user" (ext_id);
ALTER TABLE "user" ADD COLUMN ext_info json NULL;
