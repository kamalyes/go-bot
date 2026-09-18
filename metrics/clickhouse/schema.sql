-- bot_send_events 表结构（go-bot ClickHouse 统计后端）
-- 对应 metrics/clickhouse.SendEvent 写入模型；ts 为 naive DateTime64(3)，
-- 会话固定 session_timezone=UTC，写入前统一折算 UTC

CREATE TABLE IF NOT EXISTS bot_send_events
(
    ts            DateTime64(3),
    platform      LowCardinality(String),
    bot_id        LowCardinality(String),
    target_id     String,
    msg_type      LowCardinality(String),
    success       Bool,
    latency_ms    Int64,
    content_bytes Int64,
    error_kind    LowCardinality(String),
    error_code    LowCardinality(String)
)
ENGINE = MergeTree
ORDER BY (platform, ts);
