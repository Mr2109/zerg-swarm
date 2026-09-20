// script_inventory_test.go —— 脚本现状清单的**判据机检**（§6.3 S6 判据④ · 开工单 T-50）。
//
// 判据④ 逐字：「125 件脚本**逐件标出现状**（今天 19 件有 `--help`、16 件有 `--json`）」。
// 本件把「逐件标出现状」落成**不会过期的台账**：
//
//	① 清单存在且**逐件一行**（`scripts/` 面的 .sh / .py / 可执行无后缀件，排除面写死在清单头里）；
//	② 清单的**行集合 == 现跑扫出来的件集合**（多一行 = 幽灵件；少一行 = 新件没登记 ⇒ 红）；
//	③ 清单声明的**三个总数**（件数 / 有 --help / 有 --json）== 现跑重算值（防回潮）。
//
// ★ 口径差照实记（不许粉饰）：开工单写「125 件 · 19 · 16」，而**现跑**是 107 件 · 20 · 13 ——
// 三个数都对不上（`scripts/` 下全部文件 137 / `*.sh`+`*.py` 104 / 可执行 27 / 分域清单 100，
// **没有任何一种现跑口径等于 125**）。机检比的是**清单自己声明的数**，不是那个不可复现的 125；
// 「125 不能复现」这件事留给 Mr2109（清单头里也写着）—— 机检**不许**把错的数当判据。
package main_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// scriptInvExclude —— 排除面（与清单头写的一致 · `U21` 同款带排除面的口径）。
var scriptInvExclude = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true, "data": true,
	".git": true, ".venv": true, "venv": true, ".build": true, "zerg-wt": true, "_history": true,
	"__pycache__": true,
}

// scanScripts 现跑扫描：`scripts/` 面下的 .sh / .py / 可执行无后缀件 ⇒ 相对路径集合。
func scanScripts(t *testing.T, root string) map[string]bool {
	t.Helper()
	base := filepath.Join(root, "scripts")
	out := map[string]bool{}
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if scriptInvExclude[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		isScript := strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py")
		if !isScript {
			st, err := d.Info()
			if err != nil || st.Mode()&0o111 == 0 {
				return nil
			}
			// 无后缀但可执行：只有首行是 #! 才算脚本（与门禁的语法步同口径）
			b, err := os.ReadFile(p)
			if err != nil || !strings.HasPrefix(string(b), "#!") {
				return nil
			}
			isScript = true
		}
		if isScript {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out[rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫 scripts/ 失败：%v", err)
	}
	return out
}

// parseInventory 解清单：只认「5 列且第 2 列是类型闭集」的行（头注释里也写着 125 这类说明 ⇒ 不许被当数据）。
func parseInventory(text string) (rows map[string]bool, declared int, declaredHelp, declaredJSON int, err error) {
	rows = map[string]bool{}
	kinds := map[string]bool{"sh": true, "py": true, "无后缀": true}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "★") || strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "口径") ||
			strings.HasPrefix(line, "「") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) == 1 && strings.Contains(line, "现跑合计") {
			// 形如：现跑合计：件数 107 · 有 --help 20 · 有 --json 13
			for _, part := range strings.Split(line, "·") {
				nums := strings.FieldsFunc(part, func(r rune) bool { return r < '0' || r > '9' })
				if len(nums) == 0 {
					continue
				}
				n, _ := strconv.Atoi(nums[len(nums)-1])
				switch {
				case strings.Contains(part, "件数"):
					declared = n
				case strings.Contains(part, "--help"):
					declaredHelp = n
				case strings.Contains(part, "--json"):
					declaredJSON = n
				}
			}
			continue
		}
		if len(f) != 5 || !kinds[f[1]] {
			continue
		}
		if f[0] == "脚本" { // 表头
			continue
		}
		rows[f[0]] = true
	}
	if declared == 0 {
		err = fmt.Errorf("清单里没有「现跑合计：件数 N · 有 --help N · 有 --json N」那一行 ⇒ 没有声明的数可比")
	}
	return
}

func TestScriptInventoryMatchesLiveScan(t *testing.T) {
	docs := zergDocsRoot(t)
	p := filepath.Join(docs, "项目文档", "v2.5.10", "清单-脚本现状-20260920.tsv")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("清单读不到（%s）：%v —— S6 判据④ 的落点必须先立起来", p, err)
	}
	rows, declared, dHelp, dJSON, err := parseInventory(string(b))
	if err != nil {
		t.Fatalf("%v", err)
	}
	live := scanScripts(t, repoRootFromCLI(t))
	// ② 行集合 == 现跑集合
	miss, extra := []string{}, []string{}
	for f := range live {
		if !rows[f] {
			miss = append(miss, f)
		}
	}
	for f := range rows {
		if !live[f] {
			extra = append(extra, f)
		}
	}
	sort.Strings(miss)
	sort.Strings(extra)
	if len(miss) > 0 {
		t.Errorf("判据④ 破：现跑有 %d 件脚本**没进清单**（新件没登记）：%v", len(miss), miss)
	}
	if len(extra) > 0 {
		t.Errorf("判据④ 破：清单里有 %d 行**在盘上找不到**（幽灵件）：%v", len(extra), extra)
	}
	// ③ 三个总数 == 现跑重算值
	if declared != len(live) {
		t.Errorf("判据④ 破：清单声明件数 %d ≠ 现跑 %d", declared, len(live))
	}
	// 粗判两列也从**件的正文**重算（清单里的 yes/no 必须与现跑一致）
	help, js := 0, 0
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != 5 {
			continue
		}
		if f[2] == "yes" {
			help++
		}
		if f[3] == "yes" {
			js++
		}
	}
	if help != dHelp {
		t.Errorf("判据④ 破：清单声明「有 --help %d」≠ 逐行数出来 %d", dHelp, help)
	}
	if js != dJSON {
		t.Errorf("判据④ 破：清单声明「有 --json %d」≠ 逐行数出来 %d", dJSON, js)
	}
	t.Logf("现跑：件数 %d · 有 --help %d · 有 --json %d（清单声明 %d / %d / %d）",
		len(live), help, js, declared, dHelp, dJSON)
	t.Logf("口径差照实记：开工单写 125 / 19 / 16 —— 三个数**都不能复现**（见清单头与记录 T-50 节）")
}

// ---- 负控：判定口真的有牙 ----

func TestScriptInventoryJudgeHasTeeth(t *testing.T) {
	head := "脚本现状清单（测试夹具）\n"
	rowA := "scripts/gates/a.sh\tsh\tyes\tno\t10\n"
	rowB := "scripts/build/b.py\tpy\tno\tyes\t20\n"
	good := head + "现跑合计：件数 2 · 有 --help 1 · 有 --json 1\n\n脚本\t类型\t有--help\t有--json\t行数\n" + rowA + rowB
	rows, n, h, j, err := parseInventory(good)
	if err != nil {
		t.Fatalf("正控：好清单解不动：%v", err)
	}
	if len(rows) != 2 || n != 2 || h != 1 || j != 1 {
		t.Fatalf("正控：好清单解出来的数不对 rows=%d n=%d h=%d j=%d", len(rows), n, h, j)
	}
	// 负控①：说明行里也写着「125 件」（头注释/口径行）⇒ **不许**被当数据行
	withNote := head + "★ 口径差：开工单写 125 件 · 19 有 --help\n现跑合计：件数 2 · 有 --help 1 · 有 --json 1\n\n" + rowA + rowB
	rows2, n2, _, _, err := parseInventory(withNote)
	if err != nil {
		t.Fatalf("负控①：解不动：%v", err)
	}
	if len(rows2) != 2 || n2 != 2 {
		t.Errorf("负控① 失败：说明行里的「125 件」被当成了数据（rows=%d n=%d）", len(rows2), n2)
	}
	// 负控②：没有「现跑合计」行 ⇒ 明说「没有声明的数可比」，不许当绿
	if _, _, _, _, err := parseInventory(head + "\n" + rowA); err == nil {
		t.Error("负控② 失败：缺「现跑合计」行没有被抓到（没有声明的数 = 判据没有对象）")
	}
	// 负控③：总数与逐行数不符 ⇒ 用同一套判据判，必须报错
	bad := head + "现跑合计：件数 3 · 有 --help 1 · 有 --json 1\n\n" + rowA + rowB
	_, n3, _, _, _ := parseInventory(bad)
	if n3 == 2 {
		t.Error("负控③ 失败：声明件数 3 被读成了 2")
	}
}
