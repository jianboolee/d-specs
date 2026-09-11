-- 对话与 AI 操作审计
--
-- chat_messages 保留完整的对话历史（含 tool 回合），这样重新打开一张表时
-- 能还原当时的上下文；ai_operations 单独记录 AI 到底改了什么，便于追溯。

-- ==================== 会话 ====================
CREATE TABLE IF NOT EXISTS chat_sessions (
    id         VARCHAR(32)  PRIMARY KEY,
    table_id   VARCHAR(32)  NOT NULL REFERENCES spec_tables(id) ON DELETE CASCADE,
    user_id    VARCHAR(32)  NOT NULL REFERENCES users(id),
    title      VARCHAR(120) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_table
    ON chat_sessions (table_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user
    ON chat_sessions (user_id, updated_at DESC);

-- ==================== 消息 ====================
CREATE TABLE IF NOT EXISTS chat_messages (
    id           VARCHAR(32) PRIMARY KEY,
    session_id   VARCHAR(32) NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    -- user | assistant | tool
    role         VARCHAR(16) NOT NULL,
    content      TEXT        NOT NULL DEFAULT '',
    -- assistant 回合请求的工具调用
    tool_calls   JSONB,
    tool_call_id VARCHAR(64) NOT NULL DEFAULT '',
    -- token 用量，用于成本核算
    usage        JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session
    ON chat_messages (session_id, created_at);

-- ==================== AI 操作审计 ====================
-- 一次 propose_ops 落一条，无论成功与否都记录：
-- 「AI 到底改了什么、为什么没改成」是排查线上问题的第一手材料。
CREATE TABLE IF NOT EXISTS ai_operations (
    id         BIGSERIAL   PRIMARY KEY,
    table_id   VARCHAR(32) NOT NULL REFERENCES spec_tables(id) ON DELETE CASCADE,
    session_id VARCHAR(32) REFERENCES chat_sessions(id) ON DELETE SET NULL,
    revision   INTEGER,
    ops        JSONB       NOT NULL,
    summary    TEXT        NOT NULL DEFAULT '',
    -- applied | rejected | failed
    status     VARCHAR(16) NOT NULL,
    error      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_operations_table
    ON ai_operations (table_id, created_at DESC);

-- ==================== updated_at 触发器 ====================
DROP TRIGGER IF EXISTS trg_chat_sessions_set_updated_at ON chat_sessions;
CREATE TRIGGER trg_chat_sessions_set_updated_at
BEFORE UPDATE ON chat_sessions
FOR EACH ROW EXECUTE FUNCTION dspecs_set_updated_at();
