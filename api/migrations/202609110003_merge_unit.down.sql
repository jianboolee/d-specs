-- 回滚：加回 unit 列。
--
-- 无法还原原来的拆分 —— 合并后「5000mAh」是一个整体，无从判断哪部分是单位。
-- 这里只把列加回来（默认空），避免用错误的猜测污染数据。
ALTER TABLE spec_items ADD COLUMN IF NOT EXISTS unit VARCHAR(32) NOT NULL DEFAULT '';
