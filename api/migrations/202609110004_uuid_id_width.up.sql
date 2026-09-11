-- 实体 ID 从 20 位 nanoid 改为 UUID v7，标准写法是 36 字符（含 4 个连字符），
-- 原来的 VARCHAR(32) 装不下，需要加宽。
--
-- 只加宽、不重写既有数据：ID 对外是不透明字符串，老的 20 位 nanoid 继续有效，
-- 混存不影响任何逻辑。若要统一格式，得另做一次重写 ID 的迁移（会连带失效
-- 现存的登录 token，因为 JWT 的 sub 就是 user id）。

ALTER TABLE users           ALTER COLUMN id         TYPE VARCHAR(36);
ALTER TABLE users           ALTER COLUMN id         SET NOT NULL;

ALTER TABLE spec_tables     ALTER COLUMN id         TYPE VARCHAR(36);
ALTER TABLE spec_tables     ALTER COLUMN user_id    TYPE VARCHAR(36);

ALTER TABLE spec_items      ALTER COLUMN id         TYPE VARCHAR(36);
ALTER TABLE spec_items      ALTER COLUMN table_id   TYPE VARCHAR(36);

ALTER TABLE spec_revisions  ALTER COLUMN table_id   TYPE VARCHAR(36);

ALTER TABLE share_links     ALTER COLUMN id         TYPE VARCHAR(36);
ALTER TABLE share_links     ALTER COLUMN table_id   TYPE VARCHAR(36);

ALTER TABLE chat_sessions   ALTER COLUMN id         TYPE VARCHAR(36);
ALTER TABLE chat_sessions   ALTER COLUMN table_id   TYPE VARCHAR(36);
ALTER TABLE chat_sessions   ALTER COLUMN user_id    TYPE VARCHAR(36);

ALTER TABLE chat_messages   ALTER COLUMN id         TYPE VARCHAR(36);
ALTER TABLE chat_messages   ALTER COLUMN session_id TYPE VARCHAR(36);

ALTER TABLE ai_operations   ALTER COLUMN table_id   TYPE VARCHAR(36);
ALTER TABLE ai_operations   ALTER COLUMN session_id TYPE VARCHAR(36);
