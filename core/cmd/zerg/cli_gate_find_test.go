// cli_gate_find_test.go —— `zerg gate find <片段>` 的**真二进制**判据（缺口 `GAP-20260927-07`）。
//
// 为什么这条缺口值得一枚判据：本命令补的是**门件名 ⇒ 步名 的桥** —— 桥的两端各自早就有了
// （`gate ls` 出步名、`zerg find` 出件名），缺的恰好是**从手上的名字走到键**那一步。判据就钉这一步，
// 并把它与 `zerg code find` 的**零命中口径**钉在一起（同一个方言，别处再写一套必然漂）。
//
// 钉六件（全走**合成门禁脚本** · 不碰真门禁、不跑任何步骤 · 合成仓与夹具复用 `cli_gate_show_test.go`）：
//
//	① 正控①：片段 = **门件名**（`synthetic.py`，只出现在命令串里）⇒ rc=0 且命中步名 `合成步`；
//	② 正控②：片段 = **步名里的词**（`合成`）⇒ 同一格也命中（匹配面两段都在，缺一段就不算桥）；
//	③ 机器面：`--json` ⇒ stdout 是 JSON 且带 `name`（**零命中时不写 stdout** 是另一格，见 ④）；
//	④ 负控①：零命中 ⇒ rc=**1**（**不是 2** —— 与 `zerg code find` 同口径：「没有」不是「错」）；
//	⑤ 负控②：缺片段 ⇒ rc=2（用法错）· stdout **一个字节都不写**（K2 的 0 字节档）；
//	⑥ 只读：跑前 / 跑后整个合成仓逐件 sha256 清单**逐字相同**（本命令不写任何件）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestCLIGateFindBridgesFileToStep —— 桥本身 + 零命中口径 + 只读（缺口 `GAP-20260927-07`）。
func TestCLIGateFindBridgesFileToStep(t *testing.T) {
	bin := zergBinary(t)
	root := showRepo(t) // 合成仓：两行 add_step（`gofmt -l core` / `合成步`，命令串含 `synthetic.py`）

	// ① 片段 = 门件名 —— 只出现在**命令串**里（桥的存在理由）
	rc, out, errb := execCase(t, bin, root, "gate", "find", "synthetic.py")
	if rc != 0 {
		t.Fatalf("门件名应命中步名（rc=0），实得 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "合成步") {
		t.Fatalf("命中面必须给出步名「合成步」（这是桥的另一端），实得 stdout=%q", out)
	}

	// ② 片段 = 步名里的词 —— 匹配面两段（步名 + 命令串）缺一段就不算桥
	if rc2, out2, errb2 := execCase(t, bin, root, "gate", "find", "合成"); rc2 != 0 || !strings.Contains(out2, "合成步") {
		t.Fatalf("步名里的词应同样命中（rc=0 且含「合成步」），实得 rc=%d stdout=%q stderr=%q", rc2, out2, errb2)
	}

	// ③ 机器面：`--json` ⇒ stdout 是可解析的 JSON 且带 `name`
	rc3, out3, errb3 := execCase(t, bin, root, "gate", "find", "synthetic.py", "--json")
	if rc3 != 0 {
		t.Fatalf("--json 正控 rc=0，实得 rc=%d（stderr=%s）", rc3, errb3)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out3), &payload); err != nil {
		t.Fatalf("--json 的 stdout 必须是可解析 JSON（%v），实得 %q", err, out3)
	}
	if !strings.Contains(out3, `"name"`) {
		t.Fatalf("--json 的 items 必须带 name 格，实得 %q", out3)
	}

	// ④ 零命中 ⇒ rc=1（与 `zerg code find` 同口径）· stdout 空
	rc4, out4, errb4 := execCase(t, bin, root, "gate", "find", "zzz-没有这个片段-zzz")
	if rc4 != 1 {
		t.Fatalf("零命中必须 rc=1（不是 2 —— 与 `zerg code find` 同口径），实得 rc=%d\nstderr=%s", rc4, errb4)
	}
	if strings.TrimSpace(out4) != "" {
		t.Fatalf("零命中不许往 stdout 写东西（K2 的 0 字节档），实得 stdout=%q", out4)
	}

	// ⑤ 缺片段 ⇒ rc=2 · stdout 空
	rc5, out5, _ := execCase(t, bin, root, "gate", "find")
	if rc5 != 2 {
		t.Fatalf("缺片段必须 rc=2，实得 rc=%d", rc5)
	}
	if strings.TrimSpace(out5) != "" {
		t.Fatalf("用法错不许往 stdout 写东西，实得 stdout=%q", out5)
	}

	// ⑥ 只读：跑前 / 跑后整个合成仓逐件 sha256 逐字相同
	before := repoFingerprint(t, root)
	if rc6, _, _ := execCase(t, bin, root, "gate", "find", "synthetic.py", "--json"); rc6 != 0 {
		t.Fatalf("只读负控的前置跑失败：rc=%d", rc6)
	}
	after := repoFingerprint(t, root)
	if before != after {
		t.Fatalf("本命令必须**只读**：跑前/跑后合成仓指纹不同\n跑前：\n%s\n跑后：\n%s", before, after)
	}
}

// TestCLIGateFindWiringFaceForNonStepFile —— 第二半的判据：**门件不在步骤表里**时，
// 本命令必须给出「在哪跑得到、在哪跑不到」三处接线面的现读结论（缺口 `GAP-20260927-07` 的第二半）。
//
// 为什么这条判据必须存在：合成仓里的 `scripts/gates/precommit-gates.sh` 是**真件**、却**不在**
// `add_step` 命令串里（它是自举件本身）⇒ 旧行为只回「零命中」（= 把人引到「不存在」✗）。
func TestCLIGateFindWiringFaceForNonStepFile(t *testing.T) {
	bin := zergBinary(t)
	root := showRepo(t)

	rc, out, errb := execCase(t, bin, root, "gate", "find", "precommit-gates.sh", "--json", "name,wiring")
	if rc != 0 {
		t.Fatalf("命中真件名应 rc=0（「零命中」与「不存在」不许同形），实得 rc=%d\nstderr=%s", rc, errb)
	}
	if !strings.Contains(out, "precommit-gates.sh") {
		t.Fatalf("门件行必须点名该件，实得 stdout=%q", out)
	}
	if !strings.Contains(out, "wiring") {
		t.Fatalf("门件行必须带 wiring 格（三处接线面的现读结论），实得 stdout=%q", out)
	}
	if !strings.Contains(out, "all.sh") {
		t.Fatalf("wiring 必须含 all.sh 面（合成仓里该件是 .sh ⇒ 应记「不收」），实得 stdout=%q", out)
	}
}

// repoFingerprint —— 合成仓的逐件 sha256 清单（按相对路径排序 · 只看**内容**，不看 mtime）。
func repoFingerprint(t *testing.T, root string) string {
	t.Helper()
	lines := []string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		lines = append(lines, rel+"  "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("读合成仓失败：%v", err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
