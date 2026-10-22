package main

// 命令树真源的**读取面**（缺口 `GAP-20261011-03` 按族拆件的配套）。
//
// 病：拆件前「命令树源码」＝ `core/cmd/zerg/main.go` 一件；拆件后表搬进了 61 件
//       `cmdtable_<族>.go` ⇒ 所有按**文件路径**去读 main.go 的门与判据件都会读到空树。
//      ★ 现读实证：四门全部 fail-closed（`rc=2 不给结论`）—— 没静默变绿，但也因此堵死提交。
//
// 治：这里给出**合成真源** `readCmdTreeSynth` —— 把各族的表条目按原样拼回拆件前的那一整段字面量：
//
//	commands = []*command{
//		{ path: []string{"..."}, ... },
//		...
//	}
//
// 形状与拆件前**逐字同构** ⇒ 下游按 marker（`commands = []*command{` … `\n\t}\n`）切片的解析器
// **零逻辑改动**，只把「读 main.go」换成「读本函数」。★ 不许让门各自去 glob 一遍再各写一套切法。
//
// 纪律：
//  1. 本文件只做**读**：不写盘、不缓存、不重排族序（族序 = cmdtable 文件名的字典序，与解析无关）；
//  2. 一件都读不到 ⇒ 返回错误（✗ 不许回空串冒充「树是空的」—— 那会让门静默给绿）；
//  3. 表条目之外的代码（旗标册 / 解析面 / 常量）仍在 main.go，读那些面的门**照旧读 main.go**。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cmdTableFileRels —— 命令表分件的相对仓根路径（排序后，稳定）。
func cmdTableFileRels(root string) ([]string, error) {
	dir := filepath.Dir(pathCmdMainGo)
	abs, err := filepath.Glob(filepath.Join(root, dir, "cmdtable_*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(abs)
	out := make([]string, 0, len(abs))
	for _, fp := range abs {
		rel, err := filepath.Rel(root, fp)
		if err != nil {
			rel = fp
		}
		out = append(out, rel)
	}
	return out, nil
}

// cmdTableEntryTexts —— 每个族件里「`return []*command{` 与收尾 `\t}` 之间」的原文条目。
func cmdTableEntryTexts(root string) ([]string, error) {
	rels, err := cmdTableFileRels(root)
	if err != nil {
		return nil, err
	}
	const sig = "return []*command{"
	var out []string
	for _, rel := range rels {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return nil, fmt.Errorf("读命令表分件失败（%s）：%v", rel, err)
		}
		s := string(b)
		i := strings.Index(s, sig)
		if i < 0 {
			continue // 不是表件（或形状变了）—— 交给下面的「一件都没读到」兜底
		}
		s = s[i+len(sig):]
		j := strings.LastIndex(s, "\n\t}")
		if j < 0 {
			continue
		}
		out = append(out, s[:j])
	}
	return out, nil
}

// readCmdTreeSynth —— 合成真源（形状与拆件前的 `commands = []*command{…}` 逐字同构）。
//
// ★ 回落：一件 `cmdtable_*.go` 都没有（= 拆件前的旧树 / 只用 main.go 造的老夹具）⇒ 直接读
// main.go 里那一段真表；**两处都没有 ⇒ 报错**（✗ 不许回空串 —— 门会因此静默给绿）。
func readCmdTreeSynth(root string) (string, error) {
	parts, err := cmdTableEntryTexts(root)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		b, rerr := os.ReadFile(filepath.Join(root, pathCmdMainGo))
		if rerr != nil {
			return "", fmt.Errorf("命令树合成真源为空，且读不到 %s（%v）—— 判据不可判", pathCmdMainGo, rerr)
		}
		main := string(b)
		const sig = "commands = []*command{"
		i := strings.Index(main, sig)
		if i < 0 {
			return "", fmt.Errorf("命令树合成真源为空，且 %s 里也没有 `%s` —— 判据不可判", pathCmdMainGo, sig)
		}
		j := strings.Index(main[i:], "\n	}\n")
		if j < 0 {
			return "", fmt.Errorf("命令树合成真源为空，且 %s 的命令表没有收尾行 —— 形状变了，判据不可判", pathCmdMainGo)
		}
		return main[i : i+j+len("\n	}\n")], nil
	}
	return "commands = []*command{" + strings.Join(parts, "") + "\n	}\n", nil
}
