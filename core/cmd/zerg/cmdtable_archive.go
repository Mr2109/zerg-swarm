package main

// 命令表 · archive 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_archive() []*command {
	return []*command{
		// ---- 波① · T1 `G-07`：归档 / 仓外仓命令面（**全账唯一 P0** · 2026-09-23）----
		// 为什么是它：归档六步全靠手写 shell（源件实测 1072 次 `shasum` + 1072 条临时索引）；
		// 形状照业界现成接口（`apt-ftparchive` 的逐件摘要清单 · coreutils 的一进程吃 N 件 ·
		// RFC 8493 BagIt 的「载荷 + 逐件清单 + 标签清单」）。三条都是只读或**只写指定落点**。
		{
			path:     []string{"archive", "hash"},
			kind:     "ArchiveHash",
			summary:  "一批件算 sha256（**一进程吃 N 件**）—— 逐行 `sha256␣␣路径`，与 `shasum -a 256` 逐字相同（手搓 1072 次 shasum 的替身）",
			usage:    "zerg archive hash <件|目录>… [--json <字段>]",
			arity:    "any",
			args:     []string{"件或目录（目录 ⇒ 顶层逐件 · 可给多个）"},
			fields:   []string{"path", "sha256", "bytes"},
			endpoint: "",
			run:      cmdArchiveHash,
		},
		// ── `danger`（**D2**）：本表项**真写盘**（修前无 `danger` ⇒ `zerg help` 把它判成安全档 ✗）
		//   定档理由（逐条读 `family_archive.go:cmdArchiveManifest` 得）：真跑按 `--out` **新建**
		//   一只 BagIt 袋（`data/` + 两份清单）；落点非空**即拒**（不覆盖别人的件）；写失败 ⇒
		//   `os.RemoveAll(out)` **回滚**（盘上不留半个袋）；归档区/载荷**一个字不碰**（不 chmod /
		//   不搬件 / 不删件）⇒「可逆的写」不是「不可逆的破坏」⇒ **D2**（`--yes` 即可），不是 D3。
		//   `usage` 串**2026-09-27 反向对齐**：本条真判「两枚同给 ⇒ 2」（`family_archive.go:263` 调 `dryRunYesConflict`）
		//   ⇒ 逐字回改成互斥形态 `[--dry-run | --yes]`（与准形 `family_gate_matrix.go:980` 同一形态）。
		{
			path:    []string{"archive", "manifest"},
			kind:    "ArchiveManifest",
			summary: "出归档**三件套**（RFC 8493 BagIt：载荷 `data/` + `manifest-sha256.txt` + `tagmanifest-sha256.txt`）· `--dry-run` 先出逐件清单 · 真写要 `--yes` · 失败回滚",
			usage:   "zerg archive manifest <载荷目录> --out <袋目录> [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"载荷目录", "袋落点（--out）"},
			fields:  []string{"bag", "entry", "sha256"},
			danger: &dangerSpec{dangerD2, "袋落点（`--out`）",
				"按 `--out` 新建一只 BagIt 袋（`data/` + `manifest-sha256.txt` + `tagmanifest-sha256.txt`）；落点非空即拒 · 写失败即回滚（删掉本次建的袋）；归档区不碰（可逆：删掉新建的袋目录即回原状）",
				"缺口-命令面-20260921 §一 G-07 · RFC 8493 BagIt · 同族写面先例 `zerg gap export`（同样真写盘）· §九 M3 C1", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdArchiveManifest,
		},
		{
			path:     []string{"archive", "verify"},
			kind:     "ArchiveVerify",
			summary:  "校验一只袋（**重算载荷** ↔ 清单逐件对拍：清单被抹一条 / 载荷改一字节 / 多出未登记件 ⇒ 判红）",
			usage:    "zerg archive verify <袋目录> [--json <字段>]",
			arity:    "any",
			args:     []string{"袋目录"},
			fields:   []string{"entry", "want", "got", "verdict"},
			endpoint: "",
			run:      cmdArchiveVerify,
		},
		// ---- 接线批（2026-10-08）：四件「已真搬/真壳 · 命令面未接线」接进命令树 ----
		// 依据：`~/.zerg/state/退役-其余簇-接线复核-20261008.tsv`（`archive-mount` 工具面「未进命令树」）·
		// `A档18件-收CLI设计稿-20261007.md` A11/A4/A7（archive/build/eval **现役族扩 action** · 在闭集 ✓）·
		// 缺口 `GAP-20260928-12`（`script inventory` 只读口）。真实现分别在 `archive_mount.go`
		// （`cmdArchiveMount` · **真搬**）· `eval_run_all.go`（`cmdEvalRunAll` · **真搬**）·
		// `build_wall.go`（`cmdBuildWall` · 壳转账）· `family_script_inventory.go`（`cmdScriptInventoryVerify` · 只读）。
		// 入口宿主 = `ui_start.go`（门78 面④ 与 `cmdSandboxProbe` 同规 · 本单禁碰 `公开标记.tsv`）。
		{
			path:     []string{"archive", "mount"},
			kind:     "ArchiveMount",
			summary:  "归档区**按需挂/卸**（on|off|status · **真搬**：除 `hdiutil`/`mount` 两道系统工具薄壳外零 exec · 脚本移出仓读数逐字同 ⇒ 不退 8）；`status` 未挂 ⇒ 退 1（悬空符号链接不许当正常）· 不给结论 2 · 用法错 64",
			usage:    "zerg archive mount <on|off|status>",
			arity:    "any",
			args:     []string{"动作（on 挂 / off 卸 / status 报挂载态）"},
			endpoint: "",
			run:      cmdArchiveMountFamily,
		},
	}
}
