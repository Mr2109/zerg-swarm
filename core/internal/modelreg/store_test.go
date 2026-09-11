package modelreg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 批 3 验收用例（反例优先：门禁必须能红、写盘必须原子、重复跑必须一致）──
//
// 所有写盘一律落在 t.TempDir()，不碰真实家目录（任务硬规则）。

// goodRecord 造一条**合法的合成记录**：用于验目录布局与写盘机制。
// 它是测试夹具，不是任何真实模型的许可声明（许可唯一来源以 probe 产物 + 人工补为准）。
func goodRecord(id, seed string) *Record {
	return &Record{
		Schema: SchemaV1,
		ID:     id,
		Digest: "sha256:" + strings.Repeat(seed, 64)[:64],
		Files:  []File{{Role: "weights", Name: "model.gguf", SHA256: "aa", Size: 1}},
		Capabilities: []Capability{
			{Name: "text", Value: true, Source: "probed", Evidence: EvidenceText},
		},
		License:   License{SPDX: "Apache-2.0", Commercial: "yes", SourceURL: "https://example.org"},
		SourceURL: "https://example.org",
	}
}

func wantVersion(seed string) string {
	return "sha256-" + strings.Repeat(seed, 64)[:12]
}

// 门禁：不合标准的记录必须被拒，且**什么都不写**（标准 §十二.1：verify 不过直接拒）。
func TestStoreAdmissionGateRefusesAndWritesNothing(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*Record)
		field string // 期望 findings 里出现的字段
	}{
		{
			name: "comm_commercial_no_缺许可留痕",
			mut: func(r *Record) {
				r.License.Commercial = "no" // 受限许可：必须留痕
				r.License.AcceptedBy, r.License.AcceptedAt = "", ""
			},
			field: "license",
		},
		{
			name:  "缺 digest",
			mut:   func(r *Record) { r.Digest = "" },
			field: "digest",
		},
		{
			name:  "能力标签取值表外",
			mut:   func(r *Record) { r.Capabilities[0].Name = "super_vision" },
			field: "capabilities[0].name",
		},
		{
			name:  "probed 无 evidence",
			mut:   func(r *Record) { r.Capabilities[0].Evidence = "" },
			field: "capabilities[0].evidence",
		},
		{
			name:  "files 为空",
			mut:   func(r *Record) { r.Files = nil },
			field: "files",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			st := NewStore(root)
			rec := goodRecord("qwen3-vl-8b-instruct", "a")
			tc.mut(rec)

			res, err := st.Put(rec, false)
			var adm *AdmissionError
			if !errors.As(err, &adm) {
				t.Fatalf("必须被门禁拒绝（AdmissionError），实际 err=%v", err)
			}
			if CountErrors(adm.Findings) == 0 {
				t.Fatalf("拒绝时必须给出至少一条 error 级 findings：%+v", adm.Findings)
			}
			found := false
			for _, f := range adm.Findings {
				if f.Field == tc.field && f.Level == "error" {
					found = true
				}
			}
			if !found {
				t.Fatalf("findings 里应有 error 且 field=%s，实际 %+v", tc.field, adm.Findings)
			}
			if res.Changed {
				t.Fatal("被拒的记录不许标记为已写")
			}
			assertNothingWritten(t, st)
		})
	}
}

// 待修补 #21 的核心：commercial=unknown 且无留痕 → 门禁放行、记录真正落盘（写入通路跑通）；
// 同时守住不许开洞的反例：no 无留痕、占位留痕都必须仍被拒。
func TestStoreAdmitsUnknownLicenseWithoutTrace(t *testing.T) {
	root := t.TempDir()
	rec := goodRecord("unknown-lic-model", "9")
	rec.License.Commercial = "unknown"
	rec.License.AcceptedBy, rec.License.AcceptedAt = "", ""
	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("unknown 无留痕不该有 error（待修补 #21），实际 %d 条", n)
	}
	res, err := NewStore(root).Put(rec, false)
	if err != nil {
		t.Fatalf("unknown 无留痕应能入目录（写入通路）：%v", err)
	}
	if !res.Changed {
		t.Fatal("首次写入应 changed=true")
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Fatalf("文件没落盘：%v", err)
	}

	// 反例一：no 无留痕必须仍被拒（规则不许被放宽）
	restricted := goodRecord("restricted-model", "9")
	restricted.License.Commercial = "no"
	if _, err := NewStore(root).Put(restricted, false); err == nil {
		t.Fatal("no 且无留痕必须被门禁拒绝")
	}
	// 反例二：占位留痕必须仍被拒
	ph := goodRecord("placeholder-model", "9")
	ph.License.Commercial = "unknown"
	ph.License.AcceptedBy, ph.License.AcceptedAt = "unset", "2026-09-12T00:00:00Z"
	var adm *AdmissionError
	if _, err := NewStore(root).Put(ph, false); !errors.As(err, &adm) {
		t.Fatalf("占位留痕必须被拒（AdmissionError），实际 %v", err)
	}
}

// assertNothingWritten：目录与文件都不该被创建（门禁拒绝 = 零副作用）。
func assertNothingWritten(t *testing.T, st *Store) {
	t.Helper()
	if _, err := os.Stat(st.ManifestsDir()); err == nil || !os.IsNotExist(err) {
		t.Fatalf("门禁拒绝时不许创建目录：%s（err=%v）", st.ManifestsDir(), err)
	}
}

// strict 模式：只有 warn 的记录在非严格下可入目录，严格下必须被拒（标准 §十三.1）。
func TestStoreStrictModeRefusesWarns(t *testing.T) {
	rec := goodRecord("warn-only-model", "b")
	rec.SourceURL = ""
	rec.License.SourceURL = "" // → 两条 warn（source_url / notes），零 error

	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("本用例前提是零 error，实际 %d 条", n)
	}
	if len(Verify(rec, false)) == 0 {
		t.Fatal("本用例前提是至少一条 warn")
	}

	root := t.TempDir()
	if _, err := NewStore(root).Put(rec, false); err != nil {
		t.Fatalf("非严格模式：只有 warn 的记录应可入目录，实际 %v", err)
	}

	root2 := t.TempDir()
	_, err := NewStore(root2).Put(rec, true)
	var adm *AdmissionError
	if !errors.As(err, &adm) {
		t.Fatalf("严格模式：warn 也算不通过，应报 AdmissionError，实际 %v", err)
	}
	assertNothingWritten(t, NewStore(root2))
}

// 目录布局符合标准 §三：<root>/manifests/<id>/<version>.json，且落盘内容本身合法可回读。
func TestStoreLayoutAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("qwen3-vl-8b-instruct", "c")

	res, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("合法记录应能入目录：%v", err)
	}
	wantPath := filepath.Join(root, "manifests", "qwen3-vl-8b-instruct", wantVersion("c")+".json")
	if res.Path != wantPath {
		t.Fatalf("落盘位置不合标准 §三：\n  want=%s\n  got =%s", wantPath, res.Path)
	}
	if !res.Changed {
		t.Fatal("首次写入应标记 changed=true")
	}
	if res.Version != wantVersion("c") {
		t.Fatalf("version 应由 digest 推出：want=%s got=%s", wantVersion("c"), res.Version)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("文件没落盘：%v", err)
	}
	// 目录里不得残留临时文件（原子写必须清理干净）
	assertNoTempLeftovers(t, filepath.Dir(wantPath))

	back, err := Load(wantPath)
	if err != nil {
		t.Fatalf("回读失败：%v", err)
	}
	if back.ID != rec.ID || back.Digest != rec.Digest || back.License.Commercial != rec.License.Commercial {
		t.Fatalf("回读内容不一致：%+v", back)
	}
	if n := CountErrors(Verify(back, false)); n != 0 {
		t.Fatalf("能进目录的记录回读后必须仍然合法，实际 %d 条 error", n)
	}
}

func assertNoTempLeftovers(t *testing.T, dir string) {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录失败：%v", err)
	}
	for _, e := range ents {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("原子写残留临时文件：%s", e.Name())
		}
	}
}

// 幂等：同一条记录重复入目录，第二次必须不重写文件（内容与 mtime 都不变）。
func TestStorePutIsIdempotent(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("repeat-model", "d")

	first, err := st.Put(rec, false)
	if err != nil {
		t.Fatalf("首次入目录失败：%v", err)
	}
	if !first.Changed {
		t.Fatal("首次应 changed=true")
	}
	b1, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	fi1, err := os.Stat(first.Path)
	if err != nil {
		t.Fatal(err)
	}

	for i := 2; i <= 3; i++ {
		again, err := st.Put(rec, false)
		if err != nil {
			t.Fatalf("第 %d 次入目录失败（幂等要求不许报错）：%v", i, err)
		}
		if again.Changed {
			t.Fatalf("第 %d 次应 changed=false（内容一致不重写）", i)
		}
		b2, _ := os.ReadFile(first.Path)
		if string(b1) != string(b2) {
			t.Fatalf("第 %d 次内容变了：\n  first=%q\n  again=%q", i, b1, b2)
		}
		fi2, _ := os.Stat(first.Path)
		if !fi1.ModTime().Equal(fi2.ModTime()) {
			t.Fatalf("第 %d 次重写了文件（mtime 变了：%s → %s）", i, fi1.ModTime(), fi2.ModTime())
		}
	}
}

// 防覆盖：同摘要同路径已有**不同内容**时拒绝覆盖（保护人工补的许可留痕）。
func TestStorePutRefusesToClobber(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	rec := goodRecord("keep-model", "e")

	first, err := st.Put(rec, false)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}

	edited := goodRecord("keep-model", "e")
	edited.Notes = "人工补的许可证留痕（accepted_by/accepted_at）写在这里"
	_, err = st.Put(edited, false)
	var conf *ConflictError
	if !errors.As(err, &conf) {
		t.Fatalf("同摘要不同内容必须拒绝覆盖（ConflictError），实际 %v", err)
	}
	after, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("拒绝覆盖时磁盘上的文件不许被改动")
	}
}

// 原子写盘：新建 / 同内容不重写 / 换内容覆盖 / 自动建目录 / 无残留。
func TestWriteFileAtomic(t *testing.T) {
	t.Run("新建与幂等重写", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "sub", "rec.json") // 目标目录还不存在
		changed, err := WriteFileAtomic(p, []byte("A\n"))
		if err != nil || !changed {
			t.Fatalf("首次写入应成功且 changed=true，err=%v changed=%v", err, changed)
		}
		if b, _ := os.ReadFile(p); string(b) != "A\n" {
			t.Fatalf("内容不对：%q", b)
		}
		changed, err = WriteFileAtomic(p, []byte("A\n"))
		if err != nil || changed {
			t.Fatalf("同内容应不重写：err=%v changed=%v", err, changed)
		}
		changed, err = WriteFileAtomic(p, []byte("B\n"))
		if err != nil || !changed {
			t.Fatalf("换内容应重写：err=%v changed=%v", err, changed)
		}
		if b, _ := os.ReadFile(p); string(b) != "B\n" {
			t.Fatalf("覆盖后内容不对：%q", b)
		}
		assertNoTempLeftovers(t, filepath.Dir(p))
	})

	t.Run("反例：目标不可写则报错且不留半个文件", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Skip("无法改权限，跳过")
		}
		defer os.Chmod(dir, 0o700)
		p := filepath.Join(dir, "x.json")
		if _, err := WriteFileAtomic(p, []byte("X")); err == nil {
			t.Fatal("不可写目录必须报错")
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("失败时不许留下目标文件：%v", err)
		}
	})
}

// version 由 digest 决定（内容寻址）：digest 变 → version 变；id 变 → 路径变。
func TestVersionIsContentAddressed(t *testing.T) {
	a := goodRecord("m-one", "f")
	b := goodRecord("m-one", "0") // 同 id，digest 不同
	if VersionOf(a) == VersionOf(b) {
		t.Fatalf("digest 不同 version 必须不同：%s", VersionOf(a))
	}
	if VersionOf(a) != VersionOf(goodRecord("m-two", "f")) {
		t.Fatal("version 只由 digest 决定，不该受 id 影响")
	}
	st := NewStore("/tmp/does-not-matter")
	if st.PathFor(a) == st.PathFor(goodRecord("m-two", "f")) {
		t.Fatal("id 不同落盘路径必须不同")
	}
	if strings.Contains(VersionOf(a), "/") || strings.Contains(VersionOf(a), " ") {
		t.Fatalf("version 不能含路径分隔符/空格：%s", VersionOf(a))
	}
}

// list：空（且不存在）目录不报错也不创建；写入后按 (id, version) 排序；
// 目录里混进坏文件时如实记 Err 而不崩（反例优先）。
func TestStoreList(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)

	rows, err := st.List()
	if err != nil || len(rows) != 0 {
		t.Fatalf("不存在的目录应视为空目录：rows=%v err=%v", rows, err)
	}
	if _, err := os.Stat(st.ManifestsDir()); !os.IsNotExist(err) {
		t.Fatalf("list 是只读的，不该创建目录：%v", err)
	}

	// 两个 id；其中一个 id 两个版本
	for _, r := range []*Record{
		goodRecord("z-model", "a"),
		goodRecord("a-model", "b"),
		goodRecord("a-model", "c"),
	} {
		if _, err := st.Put(r, false); err != nil {
			t.Fatalf("入目录失败：%v", err)
		}
	}
	// 反例材料：坏 JSON / 非 json / 隐藏文件 / 一条"许可未知且无留痕"的记录
	dir := filepath.Join(st.ManifestsDir(), "a-model")
	if err := os.WriteFile(filepath.Join(dir, "sha256-broken.json"), []byte("{ 不是 json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("忽略我"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := goodRecord("a-model", "d")
	bad.License.Commercial = "no" // 受限许可且无留痕 → 目录健康检查应看见它的 error
	if _, err := st.Put(bad, false); err == nil {
		t.Fatal("受限许可(no)无留痕的记录不该被门禁放行（本用例前提）")
	}
	bad.License.AcceptedBy, bad.License.AcceptedAt = "人工审许可（测试夹具）", "2026-09-12T00:00:00Z"
	if _, err := st.Put(bad, false); err != nil {
		t.Fatalf("补齐留痕后应能入目录（正文记录案例）：%v", err)
	}

	rows, err = st.List()
	if err != nil {
		t.Fatalf("list 不该报错：%v", err)
	}
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r.ID+"/"+r.Version)
	}
	t.Logf("list 结果：%v", ids)

	// 期望 5 行：a-model 的 b、c、d 三个版本 + broken 一行 + z-model 一行；README/.hidden 不计
	if len(rows) != 5 {
		t.Fatalf("应有 5 行（忽略非 json 与隐藏文件），实际 %d 行：%v", len(rows), ids)
	}
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1].ID + "/" + rows[i-1].Version
		cur := rows[i].ID + "/" + rows[i].Version
		if prev > cur {
			t.Fatalf("排序不是 (id, version) 升序：%s 在 %s 之前", prev, cur)
		}
	}

	var broken, healthy, noLic *StoredRecord
	for i := range rows {
		switch rows[i].Version {
		case "sha256-broken":
			broken = &rows[i]
		case wantVersion("b"):
			healthy = &rows[i]
		case wantVersion("d"):
			noLic = &rows[i]
		}
	}
	if broken == nil || broken.Err == "" {
		t.Fatalf("坏 JSON 必须如实记 Err 而不是被吞掉：%+v", rows)
	}
	if healthy == nil || healthy.Errors != 0 || !healthy.DefaultEligible {
		t.Fatalf("合法记录该行应为 0 error 且可作默认项：%+v", healthy)
	}
	if healthy.Capabilities == nil || len(healthy.Capabilities) != 1 || healthy.Capabilities[0] != "text" {
		t.Fatalf("能力列不对：%+v", healthy)
	}
	if noLic == nil {
		t.Fatalf("没找到受限许可那条：%+v", rows)
	}
	if noLic.DefaultEligible {
		t.Fatal("commercial != yes 必须标为不可作默认项（标准 §五 红线）")
	}
	if noLic.Errors != 0 {
		t.Fatalf("补齐留痕后该行不该有 error：%+v", noLic)
	}
}

// 环境变量换根 + 显式 manifests 目录（给 --out <目录> 用）。
func TestStoreRootSelection(t *testing.T) {
	t.Setenv(ModelsDirEnv, filepath.Join(t.TempDir(), "envmodels"))
	want := os.Getenv(ModelsDirEnv)
	if got := NewStore("").ManifestsDir(); got != filepath.Join(want, "manifests") {
		t.Fatalf("$%s 应改变默认根：want=%s got=%s", ModelsDirEnv, filepath.Join(want, "manifests"), got)
	}
	if got := NewStoreAtManifests("/tmp/m").ManifestsDir(); got != "/tmp/m" {
		t.Fatalf("显式 manifests 目录应原样使用：%s", got)
	}
}
