package main

import (
	"os"
	"reflect"
	"testing"
)

// TestDocZoneSingleSource —— 冻结区口径必须**只有一个真源**（GAP-20260926-35）。
//
// 病：check-doc-name 与 check-doc-meta 各自硬编码冻结区前缀 ⇒ 一边改一边不改时，
// 同一件会出现「一门说冻结、一门说可改」，且没有任何闸会红。
// 判据：两个门的变量必须**指向真源同一份内容**（不是各写一份字面量恰好相等 ——
// 这里比的是内容，真正的结构约束由「两门只声明 `= docFrozenRepoPrefix`」这条代码形状保证）。
func TestDocZoneSingleSource(t *testing.T) {
	if !reflect.DeepEqual(cdmFrozenPrefix, docFrozenRepoPrefix) {
		t.Fatalf("check-doc-meta 的仓内冻结区 %v ≠ 真源 %v", cdmFrozenPrefix, docFrozenRepoPrefix)
	}
	if !reflect.DeepEqual(cdmFrozenPrefixAlt, docFrozenAltPrefix) {
		t.Fatalf("check-doc-meta 的仓外冻结区 %v ≠ 真源 %v", cdmFrozenPrefixAlt, docFrozenAltPrefix)
	}
	// 负控：真源被改成非空时，两门必须**同时**跟着变（本用例即随之走 else 分支）。
	if len(docFrozenRepoPrefix) == 0 {
		if _, err := os.Stat("docs/项目文档"); err == nil {
			t.Fatal("真源说仓内冻结区为空，但盘上 docs/项目文档 存在 ⇒ 口径与现盘不符")
		}
	}
}
