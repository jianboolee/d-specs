-- 用户级置顶。
--
-- 每张表本就归属某个用户，所以这个布尔标记天然是按用户隔离的：
-- 置顶只影响该用户自己的列表排序。置顶是展示偏好，不产生 revision。
ALTER TABLE spec_tables ADD COLUMN IF NOT EXISTS pinned BOOLEAN NOT NULL DEFAULT false;
