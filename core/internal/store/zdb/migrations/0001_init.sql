-- 0001_init.sql — 虫族库 M0 地基（第一张真表族）
-- 真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §十.1 · 设计-虫族数据库-20261013.md §三.3/§三.7
-- 纪律：tasks 允许原地改态；task_events 只增不改（append-only 审计链）；
--       改态与插事件**必须在同一事务**内（见 events.go 的 ApplyStateChange）。
-- 迁移号真源 = PRAGMA user_version（本件 = 1）。

-- tasks —— 当前态表（一行 = 一枚子代理的一生）。原地改态。
CREATE TABLE IF NOT EXISTS tasks (
    id                TEXT PRIMARY KEY,
    slice_id          TEXT NOT NULL,
    depends_on        TEXT,
    parent_task_id    TEXT,
    session_id        TEXT,
    model             TEXT,
    caller            TEXT,
    -- state 状态机**闭集**（用户口径 · §九.2 / §十.1 五值；✗ 不拿它存大正文）
    state             TEXT NOT NULL CHECK (state IN ('queued','running','steered','done','failed')),
    descr             TEXT,          -- 对应 --desc（`desc` 是 SQL 关键字 ⇒ 列名取 descr）
    ctx_hash          TEXT,
    exit_reason       TEXT,
    failure_reason    TEXT,
    schema_valid      INTEGER,
    result_hash       TEXT,
    truncated         INTEGER,
    api_calls         INTEGER,
    duration          REAL,
    retry_count       INTEGER NOT NULL DEFAULT 0,
    heartbeat_at      REAL,          -- liveness 时钟
    last_progress_at  REAL,          -- progress 时钟（看门狗判卡**只看**它）
    claim_token       TEXT,          -- 原子认领 · 防双执行
    claimed_by        TEXT,
    paused            INTEGER NOT NULL DEFAULT 0,  -- 暂停是**旗标**，✗ 不是第 6 个状态名（§九.2）
    paused_at         REAL,
    paused_reason     TEXT,
    last_steered_at   REAL,          -- 「最近一次注入的留痕」≠ 当前态
    idempotency_key   TEXT,
    request_hash      TEXT,          -- 幂等锚：专抓「同键不同载荷」
    worktree          TEXT,          -- 现场指针（现场**不入库**：库只留指针与指纹 · §十.2③）
    diff_stat         TEXT,
    log_path          TEXT,
    created_at        REAL,
    updated_at        REAL
);
-- 索引按「看板真会问的查询」建（§十.1 ②）；✗ 不给大 payload 建索引。
CREATE INDEX IF NOT EXISTS idx_tasks_state             ON tasks(state);
CREATE INDEX IF NOT EXISTS idx_tasks_state_hb_created  ON tasks(state, heartbeat_at, created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_caller            ON tasks(caller);

-- task_events —— append-only 审计流（改态必须同时落一条；✗ 不许把状态回写进本表）。
-- 每条带 prev_hash / audit_hash（审计链锚），audit_seq 每任务内单调无缺口（单写者）。
CREATE TABLE IF NOT EXISTS task_events (
    row_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id     TEXT NOT NULL REFERENCES tasks(id),
    audit_seq   INTEGER NOT NULL CHECK (audit_seq > 0),
    from_state  TEXT,
    to_state    TEXT NOT NULL,
    reason      TEXT,
    actor       TEXT,
    detail      TEXT,
    ts          REAL NOT NULL,
    prev_hash   TEXT,
    audit_hash  TEXT,
    UNIQUE (task_id, audit_seq)
);
CREATE INDEX IF NOT EXISTS idx_task_events_task_ts ON task_events(task_id, ts);

-- idempotency_keys —— 幂等表（§十.1 ③）。唯一仲裁点 = **DB 唯一约束**（✗ 应用代码 / 缓存 / 内存）。
-- 命中回原 task_id；fingerprint 抓「同键异参」；expires_at 建议 72h。
CREATE TABLE IF NOT EXISTS idempotency_keys (
    slice_id         TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    fingerprint      TEXT,
    state            TEXT,
    status_code      INTEGER,
    response         TEXT,
    task_id          TEXT,
    created_at       REAL NOT NULL,
    expires_at       REAL NOT NULL,
    UNIQUE (slice_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_idem_expires ON idempotency_keys(expires_at);
