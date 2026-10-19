-- d-auth 账户中心接入：删除本地用户体系
--
-- 用户身份的唯一事实源是 d-auth（auth.x-rv.com），本库不再保留 users 表：
--   - JWT sub 与各业务表 user_id 直接存 d-auth 用户 id（22 位字母数字串）；
--   - 资料（昵称/头像/脱敏手机号）经 API Key 现查 d-auth，本地无镜像；
--   - 登录态由 d-auth 下发的共享 cookie 承载（本地验签），本库不再存会话。
--
-- ⚠ 本迁移会清空全库业务数据（TRUNCATE CASCADE 沿外键链清掉所有表）。
-- 只在「切换到账户中心 + 放弃存量数据」的节点执行一次。
--
-- 兼容两种前置状态：未跑过本迁移旧版的库（users 无 dauth_user_id 列），
-- 以及跑过旧版镜像/BFF 方案的库（除 users 外还留有一张 auth_sessions）——
-- 都在本脚本里收敛为最终形态（不再有任何本地会话表）。

-- 清空存量：CASCADE 沿外键链连带 spec_tables / spec_items / spec_revisions /
-- share_links / chat_sessions / chat_messages / ai_operations
TRUNCATE TABLE users CASCADE;

-- 去掉指向 users 的外键（约束名为 PG 默认命名）
ALTER TABLE spec_tables   DROP CONSTRAINT IF EXISTS spec_tables_user_id_fkey;
ALTER TABLE chat_sessions DROP CONSTRAINT IF EXISTS chat_sessions_user_id_fkey;

-- users 及其索引（uniq_users_email_active 等）与触发器随表消失
DROP TABLE IF EXISTS users;

-- 旧的本地会话表随「本地验签即认人」一并废弃（旧版方案可能建过引用 users 的表）
DROP TABLE IF EXISTS auth_sessions;