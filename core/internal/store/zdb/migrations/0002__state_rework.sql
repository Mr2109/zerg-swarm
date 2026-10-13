-- 0002__state_rework.sql — `tasks.state` 闭集重模（五值：queued/running/done/failed/cancelled）
-- 真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §九.2（状态机 · 用户已拍）
--       ＋ §十.3（库侧落地：旧值归一 / 三字段 / 判据）
-- 用户口径（2026-10-14 已拍）：
--   · 闭集改为 queued/running/done/failed/cancelled（活态 2 ＋ 终态 3）
--   · steered      降为**事件 ＋ 字段**（✗ 不再是态名；旧行 ⇒ running，留痕进 last_steer_at）
--   · paused       并入 queued（＋ paused 旗标 / paused_at / paused_reason）
--   · waiting_retry 并入 queued（＋ retry_count / next_attempt_at）
--   · cancelled    提为**正式终态**
-- 纪律：已发布迁移永不修改、只追加（0001 一字不改）；本件与 `PRAGMA user_version = 2`
--       在**同一事务**里跑（见 migrate.go 的 applyMigration）。
--
-- 机理：SQLite 改 CHECK 无 ALTER 路 ⇒ 照官方 12 步法重建表（建新表 → 搬数据 → 删旧表 → 改名 → 重建索引）。
--   ★ 外键：`task_events(task_id) REFERENCES tasks(id)`。`PRAGMA foreign_keys` 在事务内是 **no-op**
--     （用不了），改用 `PRAGMA defer_foreign_keys = ON` —— 它**可以**在事务内设，
--     把「DROP TABLE tasks 的隐式 DELETE 会瞬时违反外键」推迟到 COMMIT 校验；
--     而 COMMIT 时新表已改名就位、行数一致 ⇒ 不违反。✗ 不删、不重建 task_events。

PRAGMA defer_foreign_keys = ON;

-- ① 新表：五值 CHECK ＋ 三字段（paused_at / last_steer_at / next_attempt_at）
CREATE TABLE tasks_new (
    id                TEXT PRIMARY KEY,
    slice_id          TEXT NOT NULL,
    depends_on        TEXT,
    parent_task_id    TEXT,
    session_id        TEXT,
    model             TEXT,
    caller            TEXT,
    -- state 状态机**闭集**（用户已拍 · 五值：活态 queued/running ＋ 终态 done/failed/cancelled）
    state             TEXT NOT NULL CHECK (state IN ('queued','running','done','failed','cancelled')),
    descr             TEXT,          -- 对应 --desc（`desc` 是 SQL 关键字 ⇒ 列名取 descr）
    ctx_hash          TEXT,
    exit_reason       TEXT,          -- 结局词闭集（与回执对齐）：timeout/stalled/max_attempts_exceeded ⇒ failed · cancelled 单独
    failure_reason    TEXT,
    schema_valid      INTEGER,
    result_hash       TEXT,
    truncated         INTEGER,
    api_calls         INTEGER,
    duration          REAL,
    retry_count       INTEGER NOT NULL DEFAULT 0,
    next_attempt_at   REAL,          -- ★新加：waiting_retry 并入 queued 后的重投时钟（NULL ⇒ 立即可投）
    heartbeat_at      REAL,          -- liveness 时钟
    last_progress_at  REAL,          -- progress 时钟（看门狗判卡**只看**它）
    claim_token       TEXT,          -- 原子认领 · 防双执行
    claimed_by        TEXT,
    paused            INTEGER NOT NULL DEFAULT 0,  -- 暂停是**旗标**，✗ 不是第 6 个状态名（§九.2）
    paused_at         REAL,
    paused_reason     TEXT,
    last_steer_at     REAL,          -- ★改名（旧名 last_steered_at）：steered 降为「事件 ＋ 字段」⇒ 最近一次注入的留痕 ≠ 当前态
    idempotency_key   TEXT,
    request_hash      TEXT,          -- 幂等锚：专抓「同键不同载荷」
    worktree          TEXT,          -- 现场指针（现场**不入库**：库只留指针与指纹 · §十.2③）
    diff_stat         TEXT,
    log_path          TEXT,
    created_at        REAL,
    updated_at        REAL
);

-- ② 搬数据 ＋ **旧值归一**（旧两代血脉都收：0001 闭集里的 steered ＋ 旧文件机的 paused/waiting_retry）
--    旧闭集 steered ⇒ running；旧文件机 paused/waiting_retry ⇒ queued；其余原值保留（cancelled 保留）。
INSERT INTO tasks_new (
    id, slice_id, depends_on, parent_task_id, session_id, model, caller, state, descr, ctx_hash,
    exit_reason, failure_reason, schema_valid, result_hash, truncated, api_calls, duration,
    retry_count, next_attempt_at, heartbeat_at, last_progress_at, claim_token, claimed_by,
    paused, paused_at, paused_reason, last_steer_at, idempotency_key, request_hash,
    worktree, diff_stat, log_path, created_at, updated_at
)
SELECT
    id, slice_id, depends_on, parent_task_id, session_id, model, caller,
    CASE state
        WHEN 'paused'        THEN 'queued'
        WHEN 'waiting_retry' THEN 'queued'
        WHEN 'steered'       THEN 'running'
        ELSE state
    END,
    descr, ctx_hash,
    exit_reason, failure_reason, schema_valid, result_hash, truncated, api_calls, duration,
    retry_count, NULL, heartbeat_at, last_progress_at, claim_token, claimed_by,
    CASE WHEN state = 'paused' THEN 1 ELSE COALESCE(paused, 0) END,
    paused_at, paused_reason, last_steered_at, idempotency_key, request_hash,
    worktree, diff_stat, log_path, created_at, updated_at
FROM tasks;

DROP TABLE tasks;
ALTER TABLE tasks_new RENAME TO tasks;

-- ③ 索引重建（照 0001 同一形状；必须在旧表 DROP 之后建，否则索引名撞车）
CREATE INDEX IF NOT EXISTS idx_tasks_state             ON tasks(state);
CREATE INDEX IF NOT EXISTS idx_tasks_state_hb_created  ON tasks(state, heartbeat_at, created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_caller            ON tasks(caller);
