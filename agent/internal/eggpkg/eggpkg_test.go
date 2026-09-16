package eggpkg

import (
	"os"
	"path/filepath"
	"testing"
)

// 跨实现对读：testdata/demo.egg 由 **Python 工具**（scripts/zerg-egg.py）产出，
// 本包（Go）必须能读 ⇒ 两个实现共用同一份格式（§11.4「读写两端同源」的跨语言版）。
func TestReadIndex_PythonMadeEgg(t *testing.T) {
	egg := filepath.Join("testdata", "demo.egg")
	if _, err := os.Stat(egg); err != nil {
		t.Fatalf("缺 testdata/demo.egg（应由 scripts/zerg-egg.py 生成）：%v", err)
	}
	ix, err := ReadIndex(egg)
	if err != nil {
		t.Fatalf("读 Python 产出的卵失败：%v", err)
	}
	if ix.Magic != Magic || ix.Version != FormatVersion {
		t.Fatalf("magic/version 不符：%+v", ix)
	}
	if len(ix.Entries) != 2 {
		t.Fatalf("段数应为 2，得 %d", len(ix.Entries))
	}
	for _, e := range ix.Entries {
		if e.Off%ix.Align != 0 {
			t.Fatalf("段 %s 偏移未对齐", e.Path)
		}
	}
	pl, err := BuildPlan(egg, ix)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Targets) != len(ix.Entries) || pl.TotalOut <= 0 {
		t.Fatalf("计划不对：%+v", pl)
	}
}

// 负例：畸形卵必须被拒（每条都对应一条不变量）。
func TestReadIndex_RejectsMalformed(t *testing.T) {
	good, err := os.ReadFile(filepath.Join("testdata", "demo.egg"))
	if err != nil {
		t.Fatal(err)
	}
	write := func(b []byte) string {
		p := filepath.Join(t.TempDir(), "x.egg")
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// ① 坏 magic
	b := append([]byte{}, good...)
	copy(b[0:4], "XXXX")
	if _, err := ReadIndex(write(b)); err == nil {
		t.Fatal("坏 magic 必须被拒")
	}
	// ② 坏版本
	b = append([]byte{}, good...)
	b[4] = 99
	if _, err := ReadIndex(write(b)); err == nil {
		t.Fatal("不支持的版本必须被拒")
	}
	// ③ align 非 2 的幂
	b = append([]byte{}, good...)
	b[8] = 0x01
	b[9] = 0x10 // 4097
	if _, err := ReadIndex(write(b)); err == nil {
		t.Fatal("align 非 2 的幂必须被拒")
	}
	// ④ 截断（索引长度超出文件）
	b = append([]byte{}, good[:HeaderPrefix]...)
	if _, err := ReadIndex(write(b)); err == nil {
		t.Fatal("截断的卵必须被拒")
	}
}

func TestIndex_ValidateRejectsBadEntries(t *testing.T) {
	base := func() *Index {
		return &Index{Magic: Magic, Version: FormatVersion, Align: 4096,
			Entries: []Entry{{Path: "a/b.bin", Off: 4096, Size: 10, SHA256: string(make([]byte, 64))}}}
	}
	ix := base()
	ix.Entries[0].Path = "../esc"
	if err := ix.validate(8192); err == nil {
		t.Fatal("path 含 .. 必须被拒")
	}
	ix = base()
	ix.Entries[0].Off = 4097
	if err := ix.validate(8192); err == nil {
		t.Fatal("偏移未对齐必须被拒")
	}
	ix = base()
	ix.Entries[0].Off, ix.Entries[0].Size = 4096, 1<<20
	if err := ix.validate(8192); err == nil {
		t.Fatal("段越出文件末尾必须被拒")
	}
	ix = base()
	ix.Entries = append(ix.Entries, ix.Entries[0])
	if err := ix.validate(8192); err == nil {
		t.Fatal("path 重复必须被拒")
	}
	ix = base()
	ix.Entries[0].SHA256 = "short"
	if err := ix.validate(8192); err == nil {
		t.Fatal("sha256 长度不对必须被拒")
	}
}
