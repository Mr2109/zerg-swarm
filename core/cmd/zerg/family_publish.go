// family_publish.go —— 产出树「在 / 不在」**只读**一格（组4 §二.4 `W-50` · `研-禁:129` `R-16` ·
// 任务单序132 · 与组3 `Q-026` 同条）。
//
// 为什么它排这一波（源件原话，不重述）：`/tmp/zerg-pub-*` 这类产出树**不在 glob 内**
// ⇒ 同一句「在树否」**两代树答案相反**（`缺口-命令面-20260922.md:62` 的 `G-12`：
// 「缓存数字**读屏抄**；「现产出树」靠手敲 `/tmp` 下的目录名对拍」）—— 每个会话各自手搓一次，
// 而且**读出来的那个数没有口径、没有身份** ⇒ 两代树混着看必然得出相反结论。
//
// 口径（照源件 `R-16` 逐字，不自造）：
//
//	形态 `zerg publish tree has <件> --tree <树>… [--json <字段>]`（**只读** · 不写盘 · 不改状态 ·
//	     不新开端点 —— 只读本机既有的产出树目录 + 那棵树自己的 git 头）
//	判据 凡「在 / 不在」的答案都要带**口径 + 树 `head_sha`**；
//	     两代产出树答案不同 ⇒ 判「**口径不同**」· **不许并成一个数**（一树一行 ⇒ 两行）
//	退码 `0` 每棵树都给出了答案（「不在」也是一条答案）· `8` **不给结论**（树不在盘 / 树身份读不到）
//	     · `2` 用法错（缺件名 / 缺 `--tree` / `--json` 缺身份两格 / 未知旗标 —— 照既有 `K2` 一套）
//
// ★ 退码 8 的**两档分得很清**（与 `net probe` 同一条纪律：把两者并成同一个形状会让消费侧丢掉答案）：
//
//	① **树不在盘**（点名的目录不是目录 / 不存在）⇒ rc=8 且**一行都不出**
//	② **树身份读不到**（该树自己不带 `.git` ⇒ 拿不到它自己的压平提交）⇒ rc=8 且**一行都不出**
//	   为什么 ② 也归 8 而不是「head_sha 写个 `-` 照出」：源件那句是「**凡**「在 / 不在」的答案
//	   **都要**带口径 + 树 `head_sha`」—— 身份写不出来的那条**不是一条合规读数**；
//	   出半张表会被消费侧当成若干条完整读数（与 `net probe` 档① 同一条理由）。
//
// ★ 为什么树身份取**该树自己的** `git rev-parse HEAD`：
//
//	`scripts/build/publish-public.sh` 第 5 步「生成单个压平提交（SQUASH）」⇒ 一棵产出树**自带**
//	一个提交，那个提交 sha 就是这棵树的身份（换一棵树 ⇒ 换一个 sha）。
//	**不许**拿上层仓的头顶替（`git -C <子目录> rev-parse HEAD` 会往上走 ⇒ 父仓的头上来了，
//	两棵不同的树会读出**同一个** head_sha —— 那正好是源件要治的病）⇒ 本命令先判「树自己有没有
//	`.git`」，判不过就 **8**、一行不出。
//
// ★ 两处「不许」（源件 `R-16` 引 GitHub 的那句「The filepath **has to be consistent across the
// runs** to enable a computation of a stable fingerprint」的落点）：
//
//	① **件名口径写死**：相对树根的相对路径 · `/` 分隔 · 无前导 `./` · 大小写敏感 · 不许 `..` 出树
//	② **树名口径写死**：一律印**绝对路径**（相对写法随 cwd 变 ⇒ 同一棵树会读出两个名字）
//
// ★ 本命令**绝不去重、绝不合并**：同一棵树点名两次 ⇒ **两行**（「并成一行 ⇒ 判红」的那半边）。
//
// ★ 一处照实记的偏离：源件（`缺口-命令面-20260922.md:62`）的建议形态是
// `zerg publish tree ls [--json <字段>]`（给产出树一个**清单**面）；本枚只落**判据栏钉的那一格**
// ——「在 / 不在」的读数带口径 + `head_sha`、两代树两行。清单面（`tree ls`）**未落** ✗
// （照实登记在回执 §七，不扩面、不半落）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// publishTreeCaliber —— 「在 / 不在」那条读数的**口径**（写死一处 · 人面与机器面都读它）。
//
// 为什么抽成一个常量：源件 `R-16` 的整个要点就是「读数**必须**带口径」。口径散在文案里
// ⇒ 下一位只会看见「在：否」两个字，仍然讲不出「按什么算的否」。
const publishTreeCaliber = "rel-path-exact+lstat（相对树根 · `/` 分隔 · 大小写敏感 · 不跟随符号链接）"

// publishTreeHeadCaliber —— 树身份那一格的口径（同样写死一处）。
const publishTreeHeadCaliber = "该树自己的压平提交（`git rev-parse HEAD` · 树自己不带 `.git` ⇒ 读不到 ⇒ 8）"

// publishTreeHasFields —— `--json` 可取的四格（**顺序即人面四列的顺序**）。
//
// ★ 与 `main.go` 里那条登记**同一个值**（登记那一处必须写成 `[]string{…}` 字面量 ——
// 原因写在 `main.go` `net probe` 那条的注释里：契约脚本的 `FIELDS_RE` 只认字面量）。
// 两处同值由 `cli_publish_tree_test.go` 的 `TestPublishTreeHas_UsageFace` 用现跑对拍钉住
// （裸给 `--json` 时 stderr 印的那行「可选字段」就是登记那一份）。
var publishTreeHasFields = []string{"tree", "caliber", "head_sha", "present"}

// publishTreeHasRequired —— 每条「在 / 不在」读数**必须**带的两格（源件 `R-16` 的落点）。
//
// 为什么它俩不接受被投影掉：`--json <字段>` 是**字段投影**（§4.1 `K1`/`K2`），而这两格是这条
// 读数的**身份**（「按什么口径算的」「算的是哪一棵树」）—— 投影掉任一件，剩下的那两个字符
// 就又变成了源件要治的那个病（「在 / 不在」不带口径与身份）。⇒ 缺任一件**拒**（退码 2）。
var publishTreeHasRequired = []string{"caliber", "head_sha"}

// publishPathBad 判 `件名` 的形状 ⇒ 不合格时给一句人话（空串 = 合格）。
//
// 口径写死在这里（一处），不散在文案里：相对路径 · `/` 分隔 · 无前导 `./` · 不许空段 ·
// 不许 `..` 出树 · 不许绝对路径。
func publishPathBad(rel string) string {
	switch {
	case rel == "":
		return "件名是空的"
	case strings.HasPrefix(rel, "/"):
		return "件名是绝对路径（口径 = **相对树根**的相对路径）"
	case strings.HasPrefix(rel, "./"):
		return "件名带前导 `./`（口径：不带前导 `./`）"
	}
	for _, seg := range strings.Split(rel, "/") {
		switch seg {
		case "":
			return "件名里有空路径段（`//` 或结尾 `/`）"
		case "..":
			return "件名里有 `..`（会走出树 —— 那已经不是「在不在**这棵**树里」）"
		}
	}
	return ""
}

// publishTreeAbs —— 树名口径（写死一处）：一律印**绝对路径**。
//
// 源件 `R-16` 引的那句「The filepath has to be consistent across the runs」正是为此：
// 相对写法随 cwd 变 ⇒ 同一棵树会读出两个名字 ⇒ 两代树对拍时又多一处假差异。
func publishTreeAbs(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// hasFieldName —— 用户给的字段序里点名了哪几格（供 `--json` 身份两格的必查）。
func hasFieldName(fields []string, want string) bool {
	for _, f := range fields {
		if f == want {
			return true
		}
	}
	return false
}

// cmdPublishTreeHas —— `zerg publish tree has <件> --tree <树>…`：一树一行、行行带口径 + `head_sha`。
func cmdPublishTreeHas(inv *invocation, stdout, stderr io.Writer) int {
	// ── ① `K2` 甲档：给了 `--json` 而不给字段 ⇒ 退码取自退码表（`usage` = 2）+ stdout 0 字节 ──
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		// ② 身份两格必查（源件 `R-16`）：`--json` 里缺 `caliber` / `head_sha` ⇒ 拒。
		missing := []string{}
		for _, need := range publishTreeHasRequired {
			if !hasFieldName(inv.fields, need) {
				missing = append(missing, need)
			}
		}
		if len(missing) > 0 {
			inv.setErr("usage", "identity_cells_required", "在 / 不在 的读数必须带口径 + 树 head_sha")
			fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：`--json` 里缺 %s\n",
				progName, strings.Join(missing, " · "))
			fmt.Fprintf(stderr, "%s: 理由：凡「在 / 不在」的答案**都要**带「口径 + 树 `head_sha`」（源件 `R-16`）"+
				"—— 这两格是**读数的身份**，不是可选投影格 ⇒ 缺任一件就不出这一条读数\n", progName)
			return exitUsage
		}
	}

	// ── ③ 位置参数：**恰好一件**（件名） ──
	if len(inv.args) != 1 {
		inv.setErr("usage", "need_one_file", "要恰好一个件名（相对树根的相对路径）")
		fmt.Fprintf(stderr, "%s: 用法：%s publish tree has <件> --tree <树>… [--json <字段>]（现读给了 %d 个位置参数）\n",
			progName, progName, len(inv.args))
		return exitUsage
	}
	rel := strings.TrimSpace(inv.args[0])
	if bad := publishPathBad(rel); bad != "" {
		inv.setErr("usage", "bad_rel_path", bad)
		fmt.Fprintf(stderr, "%s: 件名不合格：%s（口径 = %s）\n", progName, bad, publishTreeCaliber)
		return exitUsage
	}

	// ── ④ `--tree` 必须**点名**（≥1 次）：不许拿本机 glob 猜 ──
	//
	// 为什么不做「不给 `--tree` 就扫 `/tmp/zerg-pub-*`」：那正是源件 `Q-026` 的病根形态
	// （「手敲 `/tmp` 下目录名对拍」—— 名字是**猜**出来的）。点名之后，每一条读数的树身份
	// 都是**我点的那一棵**，两代树对拍不会把「扫漏了一棵」读成「那棵树里没有」。
	trees := []string{}
	for _, t := range inv.flagVals("--tree") {
		if s := strings.TrimSpace(t); s != "" {
			trees = append(trees, s)
		}
	}
	if len(trees) == 0 {
		inv.setErr("usage", "need_tree_flag", "要点名产出树（--tree 至少一次）")
		fmt.Fprintf(stderr, "%s: 用法：%s publish tree has <件> --tree <树>… [--json <字段>]\n", progName, progName)
		fmt.Fprintf(stderr, "%s: 口径：树**要点名**（一树一行）—— 不许拿本机 glob 猜（那是 `Q-026` 的病根形态："+
			"手敲目录名对拍 ⇒ 两代树混着看必然得出相反结论）\n", progName)
		return exitUsage
	}

	// ── ⑤ 逐树取「身份 + 答案」；任一棵取不到 ⇒ **不给结论**（一行都不出） ──
	type pubTreeRow struct{ tree, head, present string }
	rows := []pubTreeRow{}
	for _, t := range trees {
		abs := publishTreeAbs(t)
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			inv.setErr("blocked", "tree_absent", "点名的产出树不在盘（或不是目录）")
			fmt.Fprintf(stderr, "%s: 树 %s 不在盘（或不是目录）⇒ **不给结论**（退码 8 · 一行都不出）\n", progName, abs)
			return exitBlocked
		}
		// 树身份的真源 = **该树自己**的压平提交 ⇒ 先判它自己有没有 `.git`
		// （不判的话 `git -C <子目录>` 会往上走到父仓、两棵树读出同一个 head_sha）。
		if _, err := os.Lstat(filepath.Join(abs, ".git")); err != nil {
			inv.setErr("blocked", "tree_identity_unreadable", "树自己不带 .git ⇒ 树身份读不到")
			fmt.Fprintf(stderr, "%s: 树 %s **自己不带 `.git`** ⇒ 树身份（`head_sha`）读不到 ⇒ "+
				"**不给结论**（退码 8 · 一行都不出）\n", progName, abs)
			fmt.Fprintf(stderr, "%s: 口径：%s —— 不许拿上层仓的头顶替、也不许用目录名当身份\n",
				progName, publishTreeHeadCaliber)
			return exitBlocked
		}
		raw, err := gitRun(abs, "rev-parse", "HEAD")
		if err != nil {
			inv.setErr("blocked", "tree_identity_unreadable", "该树自己的 HEAD 读不到")
			fmt.Fprintf(stderr, "%s: 树 %s 的 `git rev-parse HEAD` 跑不动 ⇒ **不给结论**（退码 8 · 一行都不出）：%v\n",
				progName, abs, err)
			return exitBlocked
		}
		head := strings.TrimSpace(raw)
		if head == "" {
			inv.setErr("blocked", "tree_identity_unreadable", "该树自己的 HEAD 是空的")
			fmt.Fprintf(stderr, "%s: 树 %s 的 HEAD 是空的 ⇒ **不给结论**（退码 8 · 一行都不出）\n", progName, abs)
			return exitBlocked
		}
		present := "否"
		if _, err := os.Lstat(filepath.Join(abs, filepath.FromSlash(rel))); err == nil {
			present = "是"
		}
		rows = append(rows, pubTreeRow{tree: abs, head: head, present: present})
	}

	// ── ⑥ 人面：口径 + 树身份 + **逐行**照出；两代树答案不同 ⇒ 明说「口径不同」 ──
	fmt.Fprintf(stderr, "%s: 口径 = %s · 树身份 = %s\n", progName, publishTreeCaliber, publishTreeHeadCaliber)
	fmt.Fprintf(stderr, "%s: 一树一行 · **绝不去重、绝不合并**（同一棵树点名两次 ⇒ 两行）· 本跑 %d 棵树\n",
		progName, len(rows))
	yes, no := 0, 0
	for _, r := range rows {
		if r.present == "是" {
			yes++
		} else {
			no++
		}
		fmt.Fprintf(stderr, "%s: 件=%s · 树=%s · 口径=%s · head_sha=%s · 在=%s\n",
			progName, rel, r.tree, publishTreeCaliber, r.head, r.present)
	}
	if len(rows) >= 2 && yes > 0 && no > 0 {
		fmt.Fprintf(stderr, "%s: ★ 同一句「在 / 不在」问 %d 棵树 ⇒ **答案不同**（在 %d / 不在 %d）"+
			"⇒ 判「**口径不同**」· **不许并成一个数** —— 逐行的 `head_sha` 就是它俩的身份\n",
			progName, len(rows), yes, no)
	}

	items := []map[string]string{}
	for _, r := range rows {
		items = append(items, map[string]string{
			"tree":     r.tree,
			"caliber":  publishTreeCaliber,
			"head_sha": r.head,
			"present":  r.present,
		})
	}
	// 「不在」也是一条答案（与 `net probe` 档② 同一条口径：答案是「否」⇒ 照出 · 退 0）：
	// 本命令的 `8` 只留给「讲不出答案」（树不在盘 / 身份读不到）。
	return listCmd(inv, stdout, stderr, publishTreeHasFields, items)
}
