-- 收敛「本地会话表」：登录态改由 d-auth 的共享 cookie 承载
-- （后端本地验签 access JWT，不查库、不请求 d-auth，也不再自签会话）。
--
-- 为什么单独一个迁移：202610190007 的旧形态会创建 auth_sessions。
-- 已经跑过旧形态的环境库里留有该表；把它的删除放在这里，两类环境
-- （全新库直接跑新版 007 / 已跑过旧版 007 的库）都能收敛到同一最终状态。
-- 幂等：表不存在时 DROP IF EXISTS 直接通过。
DROP TABLE IF EXISTS auth_sessions;