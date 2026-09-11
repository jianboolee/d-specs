-- 参数表简化为「参数名 + 参数值」两列，不再把单位拆成独立字段。
--
-- 参数值本身包含单位更贴近真实规格表的写法（5000mAh、8小时、198g），
-- 也免得出现「值有、单位空」或反之的半截数据。
--
-- 先把单位并入参数值（无损），再删列。
-- 注意 value 为空的占位项：不能把单位提升成值，那样会出现一条值为「g」的怪数据，
-- 所以只处理 value 非空的行。

UPDATE spec_items
SET value = value || unit
WHERE unit <> ''
  AND value <> '';

ALTER TABLE spec_items DROP COLUMN IF EXISTS unit;
