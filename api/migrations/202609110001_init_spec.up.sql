-- 初始 schema：用户 + 参数表 + 参数项 + 版本快照 + 分享
--
-- 核心模型：一张 spec_tables = 一个产品的参数配置表；
-- 表内每行 spec_items = 一条参数（参数名 / 值 / 单位 / 分组）。

-- ==================== 用户 ====================
CREATE TABLE IF NOT EXISTS users (
    id            VARCHAR(32)  PRIMARY KEY,
    email         VARCHAR(255) NOT NULL,
    name          VARCHAR(80)  NOT NULL DEFAULT '',
    password_hash TEXT         NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- 邮箱唯一，但只约束未删除的用户，避免注销后无法重新注册
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_email_active
    ON users (lower(email))
    WHERE deleted_at IS NULL;

-- ==================== 参数表 ====================
CREATE TABLE IF NOT EXISTS spec_tables (
    id               VARCHAR(32)  PRIMARY KEY,
    user_id          VARCHAR(32)  NOT NULL REFERENCES users(id),
    title            VARCHAR(120) NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    industry         VARCHAR(64)  NOT NULL DEFAULT '',
    -- private 仅自己可见 | link 已生成分享链接
    visibility       VARCHAR(16)  NOT NULL DEFAULT 'private',
    -- 乐观锁 + 版本号：写操作必须带 current_revision 比对
    current_revision INTEGER      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_spec_tables_user
    ON spec_tables (user_id, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_spec_tables_industry
    ON spec_tables (user_id, industry)
    WHERE deleted_at IS NULL;

-- ==================== 参数项 ====================
-- 注意：参数项采用硬删除，删除动作完整记录在 spec_revisions.snapshot 中，
-- 撤销时由快照整体还原。这样避免「已删除项」污染查询与 AI 上下文。
CREATE TABLE IF NOT EXISTS spec_items (
    id           VARCHAR(32)  PRIMARY KEY,
    table_id     VARCHAR(32)  NOT NULL REFERENCES spec_tables(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    value        TEXT         NOT NULL DEFAULT '',
    unit         VARCHAR(32)  NOT NULL DEFAULT '',
    -- text | number | bool | enum
    value_type   VARCHAR(16)  NOT NULL DEFAULT 'text',
    -- value_type = number 时冗余存数值，支撑排序、区间筛选与范围校验
    value_num    DOUBLE PRECISION,
    -- value_type = enum 时的候选值
    enum_options JSONB,
    group_name   VARCHAR(64)  NOT NULL DEFAULT '',
    remark       TEXT         NOT NULL DEFAULT '',
    sort_order   INTEGER      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_spec_items_table
    ON spec_items (table_id, sort_order);

-- AI 按参数名定位时走这个索引
CREATE INDEX IF NOT EXISTS idx_spec_items_name
    ON spec_items (table_id, lower(name));

CREATE INDEX IF NOT EXISTS idx_spec_items_group
    ON spec_items (table_id, group_name);

-- ==================== 版本快照 ====================
CREATE TABLE IF NOT EXISTS spec_revisions (
    id         BIGSERIAL    PRIMARY KEY,
    table_id   VARCHAR(32)  NOT NULL REFERENCES spec_tables(id) ON DELETE CASCADE,
    revision   INTEGER      NOT NULL,
    -- { items: [...] } 完整快照，撤销/重做的唯一依据
    snapshot   JSONB        NOT NULL,
    -- user | ai | template | import
    source     VARCHAR(16)  NOT NULL,
    summary    TEXT         NOT NULL DEFAULT '',
    -- 本次变更的 ops 明细，便于审计「AI 到底改了什么」
    ops        JSONB,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (table_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_spec_revisions_table
    ON spec_revisions (table_id, revision DESC);

-- ==================== 分享 ====================
CREATE TABLE IF NOT EXISTS share_links (
    id         VARCHAR(32) PRIMARY KEY,
    table_id   VARCHAR(32) NOT NULL REFERENCES spec_tables(id) ON DELETE CASCADE,
    -- 足够熵的随机串，防枚举爬取
    token      VARCHAR(64) NOT NULL UNIQUE,
    -- 关闭分享时不删记录，便于原链接复用
    enabled    BOOLEAN     NOT NULL DEFAULT true,
    view_count BIGINT      NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_share_links_table
    ON share_links (table_id);

-- ==================== updated_at 触发器 ====================
CREATE OR REPLACE FUNCTION dspecs_set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;
CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION dspecs_set_updated_at();

DROP TRIGGER IF EXISTS trg_spec_tables_set_updated_at ON spec_tables;
CREATE TRIGGER trg_spec_tables_set_updated_at
BEFORE UPDATE ON spec_tables
FOR EACH ROW EXECUTE FUNCTION dspecs_set_updated_at();

DROP TRIGGER IF EXISTS trg_spec_items_set_updated_at ON spec_items;
CREATE TRIGGER trg_spec_items_set_updated_at
BEFORE UPDATE ON spec_items
FOR EACH ROW EXECUTE FUNCTION dspecs_set_updated_at();
