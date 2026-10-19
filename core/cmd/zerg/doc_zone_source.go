package main

// doc_zone_source.go —— 「文档冻结区」的**唯一真源**（GAP-20260926-35）。
//
// 为什么单独成一个件：check-doc-name 与 check-doc-meta 原先**各自**在源码里硬编码冻结区前缀，
// 两边一改一不改就会出现「同一件，一门说冻结、一门说可改」——结论相反，而且没有任何闸会发现。
// 现在两个门都只读本件的两个变量，改口径只改这里一处。
//
// 口径（2026-09-26「批3」之后）：
//   · 仓内冻结区：**空** —— `docs/项目文档/` 已整目录分家到仓外 `Zerg-内部文档`（现盘实测该目录不存在）。
//   · 仓外（Zerg-内部文档）侧：仍是冻结区，带 `..` 段，供按 `--root` 取源时判定。
//
// ★ 改这里 = 同时改两个门的结论面 ⇒ 必须同批跑 check-doc-name 与 check-doc-meta 两门。

// docFrozenRepoPrefix —— 仓内冻结区前缀（现盘为空集，保留切片形态以维持调用点形状）。
var docFrozenRepoPrefix = []string{}

// docFrozenAltPrefix —— 仓外（Zerg-内部文档）侧冻结区前缀（含 `..` 段）。
var docFrozenAltPrefix = []string{"../Zerg-内部文档/项目文档/"}
