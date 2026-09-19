// migrations.go — 一次性迁移函数表（清单的 migrate_func 指向这里的函数）。
//
// 约定：清单里 migrate_func="X" ⇒ 本文件必须存在 func migrateX。
// 门禁 scripts/gates/check-compat-manifest.py 按同一约定 grep 本文件，缺一个就 rc=1。
//
// 迁移函数是**纯 JSON 变换**（不碰磁盘、不读环境）：读、备份、写回、回读校验由 migrate.go 的
// 层级负责。这样迁移本身可以单测到字节，且与「谁拥有这个文件」（Go 侧 / UI 侧）无关
// —— 即便 UI 没在跑，zerg-compat 也能把它的状态文件迁到当前 schema。
package compat

import (
	"encoding/json"
	"strconv"
)

// MigrationFunc —— 把一个状态文件的旧字节变换成新字节（不落盘）。
type MigrationFunc func(e Entry, raw []byte) ([]byte, error)

// migrations —— migrate_func 名 → 实现。清单引用了不存在的名字 ⇒ 清单自检（MustManifest）直接 panic。
var migrations = map[string]MigrationFunc{
	"AddSchemaField": migrateAddSchemaField,
	"KeepPayload":    migrateKeepPayload,
}

// migrateAddSchemaField —— inband 信封的通用迁移：给顶层对象补上/纠正 schema 字段。
//
// 语义（三个方向都对）：
//   - 无 schema（旧版文件）  ⇒ 补上 current；载荷逐字段保留（一个都不丢）。
//   - schema < current       ⇒ 抬到 current；载荷保留。
//   - schema == current      ⇒ 幂等：输出与输入等价（调用方在更早一步就短路了，不会走到这里）。
//
// 只会改**这一个**键；载荷非对象（数组/标量）会报错而不是猜——数组类载荷请改用 sidecar 信封。
func migrateAddSchemaField(e Entry, raw []byte) ([]byte, error) {
	obj, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	if v, has := schemaOf(obj, e.SchemaKey()); !has || v != e.CurrentSchema {
		obj[e.SchemaKey()] = json.RawMessage(strconv.Itoa(e.CurrentSchema))
	}
	return marshalOrdered(e.SchemaKey(), obj)
}

// migrateKeepPayload —— sidecar 信封的迁移：载荷逐字节不动（版本号由旁路文件承载）。
//
// 为什么需要它而不是「什么都不做」：载荷虽然不该变，但**必须仍然是合法 JSON**——
// 半个坏文件被当成「迁移成功」是本类事故里最难查的一种，所以这里显式校验一次。
func migrateKeepPayload(_ Entry, raw []byte) ([]byte, error) {
	if _, err := validateJSON(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
