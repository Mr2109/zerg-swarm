-- 0003__ledger_domains.sql — 三域「库腿」建表（A1 缺口账 / A6 升级回执 / B1 提案）
-- 真源：设计稿 §三.4 附 G①（单库内分域 = 表名 `<域>_` 前缀 · 单链全局连续序号 · ✗ 不搞伪 schema）
--       · §三.4 附 G②（迁移单链四条硬约束：① 序号只全局连续追加 ② ✗ 不许插空/回填旧号
--         ③ 域归属写进文件名与件头注 ④ ✗ 不许并链跑自动迁移）
--       · §十.1 逐类判定（A1 缺口账 / A6 升级回执 / B1 提案 = **双轨**：库腿 ＋ 文件腿，文件腿是仲裁真源）
--       · §十.3①（A1 非只追加 ⇒ 触发 B 多列聚合 ＋ C 原地改态 ⇒ 库腿）· ③（A6 跨机去重 ⇒ 库腿）
--       · §四.1 M0–M4（双写 → 切读 → 删旧）
-- 迁移号真源 = PRAGMA user_version（本件 = 3；由 migrate.go 的 applyMigration 在同事务里写）。
-- 纪律一：已发布迁移**永不修改、只追加**（0001 / 0002 一字不改）。
-- 纪律二：每步 DDL 与 `PRAGMA user_version = N` 在**同一事务**里跑（见 migrate.go 的 applyMigration）。
-- 纪律三：本件只做**纯新增**（CREATE TABLE ＋ CREATE INDEX）—— ✗ 不含任何重建表 / 改列名 / 删列语句。
--         判据 isDestructiveMigration 按**原文扫描（含注释）**：命中即走破坏性重路（多一份前置快照 ＋
--         多一道收尾外键检查）；本件是「加三张新表」⇒ 必须落在纯新增那条原路上（✗ 别把简单路变复杂）。
-- 纪律四：✗ 本件**不写** `PRAGMA auto_vacuum`（那一刀另批）。
-- 命名（附 G①）：三域前缀 gap_ / rcpt_ / prop_ —— 新域一律挂域前缀，✗ 不挂「库名 / 单元名」。
--   ★ 列集「逐列照 A1 / A6 / B1 现读字段」（文件腿的真身）：
--     A1 = `~/.zerg/state/zerg-cli-gaps.jsonl`（一条一行）
--     A6 = `~/.zerg/update_receipts/<UTC>.json`（一件一文件 · 每机各写各件 · 跨机收集）
--     B1 = `~/.zerg/proposals/DEV-*.json`（一件一文件 · 号不复用 · 人审要 diff ⇒ 文件腿不可省）

-- ───────────────────────── 域 gap_ · A1 缺口账库腿 ─────────────────────────
-- 现读字段（19 键 · 逐列落）：id / fp / symptom / handmade / impact / prio / state /
--   want_family / want_action / repro_cmd / verify_cmd / found_at / last_verified_at /
--   solved_at / solved_evidence / unit / module / unit_source
-- 时间面逐字留**文本**（A1 文件腿存 RFC3339 串 ⇒ 库腿照存原值，✗ 不做时间类型转换 —— 两腿对拍才逐字可比）。
CREATE TABLE IF NOT EXISTS gap_entries (
    id               TEXT PRIMARY KEY,   -- 账内主键（GAP-YYYYMMDD-NN）
    fp               TEXT,               -- 指纹（sha256 hex）
    symptom          TEXT,
    handmade         TEXT,               -- 手工绕过面（对照「现读拄证」）
    impact           TEXT,               -- 影响面闭集（命令面 / …）
    prio             TEXT,               -- P0/P1/P2…
    state            TEXT,               -- 缺口态（仍缺 / 已解 / …）
    want_family      TEXT,               -- 想要族（gap / task / dev…）
    want_action      TEXT,
    repro_cmd        TEXT,
    verify_cmd       TEXT,
    found_at         TEXT,               -- RFC3339（文件腿原值）
    last_verified_at TEXT,
    solved_at        TEXT,
    solved_evidence  TEXT,
    unit             TEXT,               -- 归属件 / 单元
    module           TEXT,
    unit_source      TEXT
);

-- 索引「按看板真会问的查询」建（§三.5：`gap ls --by unit/module/tier/day` · state 筛选）：
CREATE INDEX IF NOT EXISTS idx_gap_entries_state  ON gap_entries(state);
CREATE INDEX IF NOT EXISTS idx_gap_entries_prio   ON gap_entries(prio);
CREATE INDEX IF NOT EXISTS idx_gap_entries_unit   ON gap_entries(unit);
CREATE INDEX IF NOT EXISTS idx_gap_entries_module ON gap_entries(module);

-- ───────────────────────── 域 rcpt_ · A6 升级回执库腿 ─────────────────────────
-- 现读字段（A6 件 JSON 逐键）：schema / at / result / failed_at / prefix / platform / source /
--   from{core,agent} / to{tag,commit} / now{core,agent} / inflight_tasks
--   ⇒ 嵌套两格摊平（from_core / from_agent · to_tag / to_commit · now_core / now_agent）。
-- ★ 另加两列（域本身要求的键，非臆造）：
--   `machine` —— A6 的身份是「**每机每升级一件**」、库腿的用途正是**跨机聚合去重**（§十.1 / §十.3③）
--                ⇒ 机名必须进键（单机版本矩阵核对 / 跨机去重都靠它）。
--   `received_at` —— **主控收件钟**（§三.5 附 H④：排序权威不信节点钟；节点钟 `at` 只当提示）⇒ REAL 存主控 time。
-- 键 = (machine, at)：同机同秒的重投视作同一条（照 zdb 幂等体例的 UNIQUE 仲裁点：唯一性落在 DB 约束、
--   ✗ 不落应用代码 / 内存）。
CREATE TABLE IF NOT EXISTS rcpt_receipts (
    machine        TEXT    NOT NULL,   -- 上报机名（跨机聚合去重的键）
    at             TEXT    NOT NULL,   -- 回执自带的节点钟戳（件名同源）
    schema         INTEGER,            -- 回执 schema 号
    result         TEXT,               -- ok / fail…
    failed_at      TEXT,               -- 失败阶段（成功为 NULL）
    prefix         TEXT,               -- 升级根前缀
    platform       TEXT,               -- darwin-arm64 / linux-amd64…
    source         TEXT,               -- 来源（file://… / release URL）
    from_core      TEXT,               -- from.core 摊平
    from_agent     TEXT,               -- from.agent 摊平
    to_tag         TEXT,               -- to.tag 摊平
    to_commit      TEXT,               -- to.commit 摊平
    now_core       TEXT,               -- now.core 摊平
    now_agent      TEXT,               -- now.agent 摊平
    inflight_tasks INTEGER,            -- 升级时在飞任务数
    received_at    REAL,               -- 主控收件钟（§三.5 附 H④）
    PRIMARY KEY (machine, at)
);
CREATE INDEX IF NOT EXISTS idx_rcpt_receipts_machine ON rcpt_receipts(machine);
CREATE INDEX IF NOT EXISTS idx_rcpt_receipts_result  ON rcpt_receipts(result);

-- ───────────────────────── 域 prop_ · B1 提案件库腿 ─────────────────────────
-- 现读字段（B1 件 JSON 逐键）：id / title / target / goal / evidence / rollback_ref / criterion /
--   files / by / subject / subject_kind / egg_id / approver / approver_kind / state / created_at /
--   contract / path
-- ★ evidence / files 是**数组** ⇒ 库腿存**原文 JSON 文本**（文件腿是仲裁真源、人审要 diff；库腿只作索引
--   与聚合查，✗ 不把数组拆成子表 —— 拆了反而不逐字可对拍）。
-- ★ `"by"` 加引号：`BY` 是 SQL 关键字（GROUP BY / ORDER BY）⇒ 作列名须引号化，✗ 改名（改名就不是「逐列照现读字段」了）。
-- ★ 件本体随提案**分发 / 人审 diff**（§十.1 B1 行）⇒ 文件腿不可省；库腿只落索引面。
CREATE TABLE IF NOT EXISTS prop_proposals (
    id             TEXT PRIMARY KEY,   -- DEV-NNNN（号不复用）
    title          TEXT,
    target         TEXT,               -- 承接待办编号（D/E/F/G 族）
    goal           TEXT,
    evidence       TEXT,               -- JSON 数组原文
    rollback_ref   TEXT,
    criterion      TEXT,
    files          TEXT,               -- JSON 数组原文
    "by"           TEXT,               -- 对应 JSON 的 by（关键字 ⇒ 引号化）
    subject        TEXT,
    subject_kind   TEXT,               -- human / ai / egg / ci
    egg_id         TEXT,
    approver       TEXT,
    approver_kind  TEXT,               -- human
    state          TEXT,               -- 未决 / 已批 / 已落…
    created_at     TEXT,               -- RFC3339（文件腿原值）
    contract       TEXT,
    path           TEXT                -- 件本体路径（文件腿指针）
);
CREATE INDEX IF NOT EXISTS idx_prop_proposals_state  ON prop_proposals(state);
CREATE INDEX IF NOT EXISTS idx_prop_proposals_target ON prop_proposals(target);
