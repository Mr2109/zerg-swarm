package zdb

import (
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// migrationsFS —— 迁移件内嵌（设计稿 §三.7：//go:embed migrations/*.sql，≈30 行自管、✗ 不引迁移框架）。
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// loadMigrations —— 解析 migrations/NNNN_*.sql，返「版本号 → SQL」与最大版本号。
// 版本号真源 = 文件名前缀（与 PRAGMA user_version 对齐）。
func loadMigrations() (map[int]string, int, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, 0, fmt.Errorf("zdb: 读内嵌迁移目录失败: %w", err)
	}
	out := map[int]string{}
	max := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".sql")
		i := strings.IndexByte(base, '_')
		if i <= 0 {
			return nil, 0, fmt.Errorf("zdb: 迁移件命名须为 NNNN_<名>.sql（实得 %s）", e.Name())
		}
		v, err := strconv.Atoi(base[:i])
		if err != nil || v <= 0 {
			return nil, 0, fmt.Errorf("zdb: 迁移件版本前缀非法（%s）", e.Name())
		}
		if _, dup := out[v]; dup {
			return nil, 0, fmt.Errorf("zdb: 迁移版本号重复（%d）", v)
		}
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, 0, fmt.Errorf("zdb: 读迁移件 %s 失败: %w", e.Name(), err)
		}
		out[v] = string(b)
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return nil, 0, fmt.Errorf("zdb: 内嵌迁移集为空（migrations/*.sql 一件都没有）")
	}
	// 版本号必须从 1 连续到 max（✗ 不许有洞——否则 user_version 语义会含糊）。
	for v := 1; v <= max; v++ {
		if _, ok := out[v]; !ok {
			return nil, 0, fmt.Errorf("zdb: 迁移版本号有洞（缺 %d）", v)
		}
	}
	return out, max, nil
}

// UserVersion —— 读 PRAGMA user_version（库自带的应用私有迁移号）。
func (d *DB) UserVersion() (int, error) {
	var v int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("zdb: 读 user_version 失败: %w", err)
	}
	return v, nil
}

// mustMigrations —— 已解析的内嵌迁移集（版本号 → SQL）。失败即返回（migrate 的唯一入口）。
func mustMigrations() (map[int]string, error) {
	migs, _, err := loadMigrations()
	return migs, err
}

// migrate —— 自管迁移（设计稿 §三.7）：
//   - 每步 SQL 与 `PRAGMA user_version = N` 在**同一事务**里（崩则整体回滚、下次干净重跑）。
//   - 已发布迁移永不修改、只追加。
//   - 库版本高于本二进制支持上限 ⇒ 拒启（✗ 静默用新 schema）。
//   - 版本号已是最新 ⇒ **幂等空跑**（不重跑任何 DDL）。
func (d *DB) migrate() error {
	migs, err := mustMigrations()
	if err != nil {
		return err
	}
	max := 0
	for v := range migs {
		if v > max {
			max = v
		}
	}
	cur, err := d.UserVersion()
	if err != nil {
		return err
	}
	if cur > max {
		return fmt.Errorf("%w：库 user_version=%d，本二进制支持上限=%d ⇒ 拒启（✗ 静默用新 schema）", ErrSchemaTooNew, cur, max)
	}
	if cur == max {
		return nil // 幂等重跑：无迁移可做
	}
	versions := make([]int, 0, len(migs))
	for v := range migs {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for _, v := range versions {
		if v <= cur {
			continue
		}
		if err := d.applyMigration(v, migs[v]); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration —— 单步迁移：DDL 与 user_version 同事务。
func (d *DB) applyMigration(version int, sqlText string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 事务开始失败: %w", version, err)
	}
	if _, err := tx.Exec(sqlText); err != nil {
		tx.Rollback()
		return fmt.Errorf("zdb: 迁移 v%d 建表失败: %w", version, err)
	}
	// PRAGMA user_version 不可参数化（版本号来自内嵌件名，非外部输入）。
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		tx.Rollback()
		return fmt.Errorf("zdb: 迁移 v%d 写 user_version 失败: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("zdb: 迁移 v%d 提交失败: %w", version, err)
	}
	return nil
}
