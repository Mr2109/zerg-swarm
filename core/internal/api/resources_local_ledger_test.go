package api

// resources_local_ledger_test.go —— 本机行（local）"如实口径"的反例优先测试**已随角色退役**。
//
// ⚠ 这是**明确的退役说明**，不是悄悄删除（2026-09-16，Mr2109 拍"所有可推理的计算机都是子端"）。
//
// 原测试覆盖（本文件此前）：#29（消费方不误判）、#30（本机账本有/无驻留都如实、显存未知 vram_known=false）、
// 以及"显存未知 fail-closed / 统一内存不 fail-closed"在 /api/resources/ledger 的可见性 —— 主题是
// **"本机那一行的诚实口径"**。
//
// 退役原因：本机角色（localback）退役 ⇒ 机群里不再有 `local` 行、`Handlers.LocalBack` 已注入 nil
// ⇒ 本文件失去可测对象（继续留着只会是"给不存在的角色写测试"）。
//
// **保证没有被放弃，而是转移了** —— 且已有实测证据（换件 d75c5203 后实测）：
//   · 子端**确实**上报驻留明细：/api/fleet/status 的 Mr2109 行 `resident` 有明细
//     （alias / state / req_count / managed / source / weights_bytes ✓，与 x3 同形）；
//   · "拿不到显存就如实说没有"在观测面**可见**：Mr2109 行 `vram_known=false` 出现（x3 为 `true` 作对照）
//     —— 该格此前被响应侧 `omitempty` 吞掉，已修（store.go：false 是有意义的值，不许省略）；
//   · 子端侧出处：`agent/internal/heartbeat/heartbeat.go` 明确写 `body["vram_known"] = false`
//     （注释原话"拿不到显存：如实标不可用，不出假值"）；驻留明细由 `backend.Manager.ResidentDetail()`
//     从 `m.procs` 取（`manager.go:432` 登记，裸 exec 与孵化**两条路都登记**）。
//
// 若将来需要恢复"本机行"这一概念（例如主控所在机器重新拥有独立角色），应**连同本文件一道复活**，
// 而不是在别处重写一份新口径 —— 口径唯一，避免两个观测面漂移。
