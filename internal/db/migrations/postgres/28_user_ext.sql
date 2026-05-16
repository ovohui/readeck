-- SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE "user" ADD COLUMN ext_id varchar(128) UNIQUE NULL;
ALTER TABLE "user" ADD COLUMN ext_info jsonb NULL;
