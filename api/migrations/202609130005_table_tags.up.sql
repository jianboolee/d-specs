-- 产品类型标签。
--
-- 由「AI 分析」根据表内容自动识别产品类型（房车、配件、电子消费品…）后写入，
-- 用于分享页展示与后续归类。用 JSONB 存字符串数组而不是独立的关联表：
-- 标签是给展示与检索用的轻量元信息，数量少、无需单独索引与引用完整性。
--
-- 刻意不产生 revision：与 title / description 一样属于表元信息，
-- 版本快照只用于参数内容的撤销。

ALTER TABLE spec_tables ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;
