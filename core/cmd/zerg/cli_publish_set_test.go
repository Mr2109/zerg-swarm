// cli_publish_set_test.go —— `zerg publish set` 的**成对判据**（病 · 缺口 `GAP-20260927-407`：
// 「这件会不会发」在命令面上没有**免树**正门）。
//
// 本件把这条判据的两个半边都钉住（跑的是**当前源码**的行为 · `zerg.RunForTest`；
// **不碰**产出树、不写任何件、不出网）：
//
//	① 正控 · 三枚「表说 `public=no`、两器已收」的件 ⇒ `会发=否` 且 `拦的层=EXCLUDES`
//	② 正控 · `core/cmd/zerg/main.go` ⇒ `会发=是` 且 `公开路径` = 它自己（仓内相对路径恒等映射）
//	③ 逐条带口径（`caliber` 非空）——「凡「会不会发」的答案都带口径」那一句的「凡」
//	④ `--json` 机器面：五格出参；缺身份格（`caliber`）⇒ **rc=2 且 stdout 0 字节**
//	⑤ 用法面：缺路径 / 路径形状不合口径 / 裸 `--json` ⇒ 一律 **rc=2 · stdout 0 字节**
//	⑥ 字段表**两处同值**（命令树登记 ⟷ 裸 `--json` 时 stderr 印的那一行）
package main_test

import (
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// publishSetRun —— 跑一条命令 ⇒ (rc, stdout, stderr)。
func publishSetRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// publishSetGateBlockedFiles —— 上一批门 B9 判据⑥ 报的那三件（现读：两器排除面已收 ⇒ 链上「不发」）。
var publishSetGateBlockedFiles = []string{
	"scripts/gates/check-git-path-outlet.py",
	"scripts/gates/check-help-shape.py",
	"scripts/gates/check-lineref-limit.py",
}

// TestPublishSet_ThreeBlockedOneControl —— 必验①：三枚 blocked + 一件正控（现读真仓真源件）。
func TestPublishSet_ThreeBlockedOneControl(t *testing.T) {
	const control = "core/cmd/zerg/main.go"
	argv := append([]string{"publish", "set"}, publishSetGateBlockedFiles...)
	argv = append(argv, control, "--json", "path,caliber,will_publish,layer,public_path")
	rc, out, errb := publishSetRun(t, argv...)
	t.Logf("rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	if rc != 0 {
		t.Fatalf("四件都给出了答案（「不发」也是答案）⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	items := publishTreeItems(t, out) // 同一套包封解析（`--json` 面同形）
	if len(items) != 4 {
		t.Fatalf("点名 4 件 ⇒ 必须 4 条读数，得到 %d 条：%v", len(items), items)
	}
	for i, rel := range publishSetGateBlockedFiles {
		if items[i]["path"] != rel {
			t.Errorf("第 %d 条的件名不是点名的那个：%q ≠ %q", i, items[i]["path"], rel)
		}
		if items[i]["will_publish"] != "否" {
			t.Errorf("%s 该是「不发」（两器排除面已收）得到 %q", rel, items[i]["will_publish"])
		}
		if items[i]["layer"] != "EXCLUDES" {
			t.Errorf("%s 该报「哪一层拦的 = EXCLUDES」，得到 %q（层名空 = 那一格没答）",
				rel, items[i]["layer"])
		}
		if items[i]["public_path"] != "—" {
			t.Errorf("%s 不发却给了公开路径 %q（那两格自相矛盾）", rel, items[i]["public_path"])
		}
	}
	// ② 正控：白名单命中 + 两器都不排 ⇒ 会发，且公开路径 = 仓内相对路径恒等映射。
	ctl := items[3]
	if ctl["path"] != control {
		t.Fatalf("第 4 条的件名不是正控：%q", ctl["path"])
	}
	if ctl["will_publish"] != "是" {
		t.Fatalf("正控 %s 该是「会发」（白名单 `core/` 命中 · 两器都不排），得到 %q（层=%s）",
			control, ctl["will_publish"], ctl["layer"])
	}
	if ctl["public_path"] != control {
		t.Errorf("正控的公开路径该是恒等映射 %q，得到 %q", control, ctl["public_path"])
	}
	// ③ 逐条带口径（判据那句的「凡」）。
	for _, it := range items {
		if strings.TrimSpace(it["caliber"]) == "" {
			t.Errorf("有一条读数**没带口径**：%v", it)
		}
	}
}

// TestPublishSet_PrivateFaceHardGate —— 必验①：真在 `publish/private-paths.txt` 里的件 ⇒ 不发 + 层名
// `私有面硬门禁`；不在清单里的件 ⇒ 该层名**不出现**（两半都在一条口令里现跑对拍）。
//
// 真源现读：`scripts/gates/check-history-secrets.py` 是 `publish/private-paths.txt:73` 逐字一行
// （发布机制自身 · 只增不减）；`core/cmd/zerg/main.go` 不在那份清单里（控制件）。
func TestPublishSet_PrivateFaceHardGate(t *testing.T) {
	const privateFile = "scripts/gates/check-history-secrets.py"
	const control = "core/cmd/zerg/main.go"
	rc, out, errb := publishSetRun(t, "publish", "set", privateFile, control,
		"--json", "path,caliber,will_publish,layer,public_path,private_face,non_blob_basis")
	t.Logf("rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	if rc != 0 {
		t.Fatalf("两件都给出了答案（「不发」也是答案）⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	items := publishTreeItems(t, out)
	if len(items) != 2 {
		t.Fatalf("点名 2 件 ⇒ 必须 2 条读数，得到 %d 条", len(items))
	}
	// ① 私有面件：不发 + 层名 = 私有面硬门禁 + `private_face` 报出命中的规则。
	if items[0]["will_publish"] != "否" {
		t.Errorf("%s 该是「不发」（私有面硬门禁），得到 %q", privateFile, items[0]["will_publish"])
	}
	if items[0]["layer"] != "私有面硬门禁" {
		t.Errorf("%s 该报「拦的层 = 私有面硬门禁」，得到 %q", privateFile, items[0]["layer"])
	}
	if f := items[0]["private_face"]; f == "" || f == "—" {
		t.Errorf("%s 的 `private_face` 该报出命中的规则，得到 %q", privateFile, f)
	}
	if items[0]["public_path"] != "—" {
		t.Errorf("%s 不发却给了公开路径 %q", privateFile, items[0]["public_path"])
	}
	// ② 控制件：该层名**不出现**（读数仍是既有五层那一套）。
	if items[1]["layer"] == "私有面硬门禁" {
		t.Errorf("%s 不在私有面清单里，却报了 `私有面硬门禁`（假命中）", control)
	}
	if items[1]["private_face"] != "—" {
		t.Errorf("%s 的 `private_face` 该是「未命中」，得到 %q", control, items[1]["private_face"])
	}
}

// TestPublishSet_NonBlobLayerUnreachable —— 必验②：⑥ 非 blob 层**拿不到结论**（免树口径不读树/索引）
// ⇒ 逐条照实登记在 `non_blob_basis` 那一格里（**声明**，不是猜），且**不动**既有五层的读数。
//
// 真跑形态：本仓 `git ls-files -s` 现读有 2 条 gitlink（`vendor/rtk` / `vendor/searxng`，模式 `160000`），
// 但本命令**不读**索引/树 ⇒ 拿不到模式 ⇒ 那一格恒为「拿不到结论」（拿一个**真** gitlink 跑也照此）。
func TestPublishSet_NonBlobLayerUnreachable(t *testing.T) {
	const gitlink = "vendor/rtk" // 现读：git ls-files -s ⇒ `160000 … vendor/rtk`
	rc, out, errb := publishSetRun(t, "publish", "set", gitlink,
		"--json", "path,caliber,will_publish,layer,public_path,private_face,non_blob_basis")
	t.Logf("rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	if rc != 0 {
		t.Fatalf("「拿不到结论」落在**单层**这一格上 ⇒ 命令仍是 rc=0，得到 %d", rc)
	}
	items := publishTreeItems(t, out)
	if len(items) != 1 {
		t.Fatalf("点名 1 件 ⇒ 必须 1 条读数，得到 %d 条", len(items))
	}
	basis := items[0]["non_blob_basis"]
	if !strings.Contains(basis, "非 blob（子模块 gitlink）") {
		t.Errorf("`non_blob_basis` 里该点名层名 `非 blob（子模块 gitlink）`，得到 %q", basis)
	}
	if !strings.Contains(basis, "拿不到结论") {
		t.Errorf("`non_blob_basis` 该逐字说「拿不到结论」，得到 %q", basis)
	}
	if strings.Contains(basis, ".gitmodules") && !strings.Contains(basis, "顶替") {
		t.Errorf("`non_blob_basis` 提到了 `.gitmodules` 却说不出「不拿它顶替」：%q", basis)
	}
	// ⑥ 层**永不**出现在 `layer` 那一格（判不出来的答案不许冒充答案）。
	if items[0]["layer"] == "非 blob（子模块 gitlink）" {
		t.Errorf("⑥ 层恒「拿不到结论」，却出现在 `layer` 那一格：%q", items[0]["layer"])
	}
	// 既有五层读数一字未动：gitlink 件未被白名单收 ⇒ 仍是 `白名单未命中`。
	if items[0]["layer"] != "白名单未命中" {
		t.Errorf("既有五层读数被动过：%s 该是「白名单未命中」，得到 %q", gitlink, items[0]["layer"])
	}
	if items[0]["will_publish"] != "否" {
		t.Errorf("既有五层读数被动过：%s 该是「不发」，得到 %q", gitlink, items[0]["will_publish"])
	}
}

// TestPublishSet_JSONMachineFace —— 必验④：`--json` 机器面 + 缺身份格 ⇒ rc=2 且 stdout 0 字节。
func TestPublishSet_JSONMachineFace(t *testing.T) {
	rc, out, _ := publishSetRun(t, "publish", "set", "core/cmd/zerg/main.go",
		"--json", "path,caliber,will_publish,layer,public_path,private_face,non_blob_basis")
	if rc != 0 {
		t.Fatalf("给了合法 `--json` ⇒ rc=0，得到 %d", rc)
	}
	for _, want := range []string{`"items"`, `"caliber"`, `"will_publish"`, `"layer"`, `"public_path"`,
		`"private_face"`, `"non_blob_basis"`} {
		if !strings.Contains(out, want) {
			t.Errorf("`--json` 出参里没有 %s：%s", want, out)
		}
	}
	// 缺身份格（口径）：投影掉它 ⇒ 拒（那格是读数的身份，不是可选投影格）。
	rc, out, _ = publishSetRun(t, "publish", "set", "core/cmd/zerg/main.go",
		"--json", "path,will_publish")
	if rc != 2 {
		t.Fatalf("`--json` 里缺 `caliber` ⇒ rc=2，得到 %d", rc)
	}
	// ★ `--json` 这一档的「0 字节」口径照本仓现读的**包封语义**：拒执时**不出读数**（`items` 空）
	//   + 包封里带 `error.kind=usage`（与「人面 stdout 0 字节」是同一件事的机器面写法 ——
	//   带 `--json` 时那条错误进包封，不带 `--json` 时 stdout 才是真的 0 字节，见下一格）。
	if items := publishTreeItems(t, out); len(items) != 0 {
		t.Errorf("拒执却出了读数：%v", items)
	}
	if !strings.Contains(out, `"identity_cells_required"`) {
		t.Errorf("拒执的包封里没点出 kind/detail：%s", out)
	}
}

// TestPublishSet_UsageFace —— 必验⑤：用法面三格；⑥ 字段表两处同值。
func TestPublishSet_UsageFace(t *testing.T) {
	for _, c := range []struct {
		name string
		argv []string
	}{
		{"缺路径", []string{"publish", "set"}},
		{"路径是绝对路径", []string{"publish", "set", "/etc/hosts"}},
		{"路径带 `..`", []string{"publish", "set", "../README.md"}},
		{"路径带前导 `./`", []string{"publish", "set", "./README.md"}},
	} {
		rc, out, _ := publishSetRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s ⇒ rc=2，得到 %d", c.name, rc)
		}
		if out != "" {
			t.Errorf("%s ⇒ stdout 必须 0 字节，得到 %q", c.name, out)
		}
	}
	// 裸 `--json`：stderr 印的「可选字段」那一行必须与命令树登记同值（两处同值）。
	rc, _, errb := publishSetRun(t, "publish", "set", "core/cmd/zerg/main.go", "--json")
	if rc != 2 {
		t.Fatalf("裸 `--json` ⇒ rc=2，得到 %d", rc)
	}
	var line string
	for _, ln := range strings.Split(errb, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "可选字段:") {
			line = ln
		}
	}
	if line == "" {
		t.Fatalf("裸 `--json` 的 stderr 里没有「可选字段」那一行：%s", errb)
	}
	for _, f := range []string{"path", "caliber", "will_publish", "layer", "public_path"} {
		if !strings.Contains(line, f) {
			t.Errorf("「可选字段」那一行里缺 %s（登记与实现漂了）：%s", f, line)
		}
	}
}
