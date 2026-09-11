-- 回滚：把 ID 列收回到 VARCHAR(32)。
--
-- 如果此时已经存在 UUID v7 生成的 36 字符 ID，这一步会失败 —— 这是有意的：
-- 静默截断 ID 会造成外键指向不存在的行。确实需要回滚时，
-- 应当先清理掉 36 字符的数据，或者接受这一步报错并只回滚代码。
ALTER TABLE ai_operations   ALTER COLUMN table_id   TYPE VARCHAR(32);
ALTER TABLE ai_operations   ALTER COLUMN session_id TYPE VARCHAR(32);

ALTER TABLE chat_messages   ALTER COLUMN session_id TYPE VARCHAR(32);
ALTER TABLE chat_messages   ALTER COLUMN id         TYPE VARCHAR(32);

ALTER TABLE chat_sessions   ALTER COLUMN user_id    TYPE VARCHAR(32);
ALTER TABLE chat_sessions   ALTER COLUMN table_id   TYPE VARCHAR(32);
ALTER TABLE chat_sessions   ALTER COLUMN id         TYPE VARCHAR(32);

ALTER TABLE share_links     ALTER COLUMN table_id   TYPE VARCHAR(32);
ALTER TABLE share_links     ALTER COLUMN id         TYPE VARCHAR(32);

ALTER TABLE spec_revisions  ALTER COLUMN table_id   TYPE VARCHAR(32);

ALTER TABLE spec_items      ALTER COLUMN table_id   TYPE VARCHAR(32);
ALTER TABLE spec_items      ALTER COLUMN id         TYPE VARCHAR(32);

ALTER TABLE spec_tables     ALTER COLUMN user_id    TYPE VARCHAR(32);
ALTER TABLE spec_tables     ALTER COLUMN id         TYPE VARCHAR(32);

ALTER TABLE users           ALTER COLUMN id         TYPE VARCHAR(32);
