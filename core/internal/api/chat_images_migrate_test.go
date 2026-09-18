package api

// chat_images_migrate_test.go — 对话图片目录路径统一（2026-09-18 修 /tmp 硬编码缺陷）用例。
//
// 缺陷: chat_handlers.go 原写死 `const chatImageDir = "/tmp/zerg-chat/images"` ——
// macOS 重启 /tmp 即清 + tmp_cleaner 3 天未访问即删（实测本机两者都在）⇒ 历史消息里的 image_path
// 变死链（图上不去、模型「看不见」曾经发的图）；且对话库本身已迁出 /tmp（甲批 T1）——图片与库不同存亡。
//
// 修后契约（本文件逐条钉死）:
//  ① 写只写统一状态目录派生目录（statepath.File → ZERG_STATE_DIR → ~/.zerg/state/zerg-chat-images），
//     目录首次使用自动建；旧 /tmp/zerg-chat/images 永不写、不删、不改（sha256 + mtime + 目录清单三证）；
//  ② 读（readChatImage）先按存库绝对路径原样读 ⇒ 存量 /tmp 绝对路径零改动可读（旧目录还在就命中）；
//  ③ 按文件名回退：新目录优先、旧目录兜底 ⇒ 旧目录里的老图仍读得到（目录类口径: 只读保留不搬）；
//  ④ 旧目录不存在 ⇒ 读目录序列只有写落点（首次运行，不多 stat 结果）；
//  ⑤ 显式覆盖（测试隔离）⇒ 读目录序列只有覆盖目录，不复旧路径（防测试读真机 /tmp）。
//
// 隔离纪律: 一律用 setChatImagePaths 切包级变量（t.Cleanup 严格恢复）+ ZERG_STATE_DIR 指 temp，
// 绝不让用例碰真机 /tmp/zerg-chat/images。

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setChatImagePaths 切包级图片路径（覆盖/旧目录），用例结束严格恢复。
func setChatImagePaths(t *testing.T, override, legacy string) {
	t.Helper()
	oldOverride, oldLegacy := chatImagesDirOverride, legacyChatImagesDir
	chatImagesDirOverride, legacyChatImagesDir = override, legacy
	t.Cleanup(func() {
		chatImagesDirOverride, legacyChatImagesDir = oldOverride, oldLegacy
	})
}

// pngDataURL 造一个 PNG data URL（saveChatImage 的入参形态）。
func pngDataURL(raw []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

// writeLegacyImage 造一张「旧版本存下的图」（固定过去 mtime——被改写必然变）。
func writeLegacyImage(t *testing.T, dir, name string, raw []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("造旧图片目录失败 %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatalf("造旧图片失败 %s: %v", p, err)
	}
	if err := os.Chtimes(p, pastTime, pastTime); err != nil {
		t.Fatalf("设旧图片 mtime 失败 %s: %v", p, err)
	}
	return p
}

// dirEntryNames 目录清单（稳定排序——验证旧目录没被搬空/没被塞新文件）。
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录失败 %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sortStrings(names)
	return names
}

// ① 写只写新目录（目录自动创建、内容与输入逐字节一致）；旧目录不删、不改、不被写入。
func TestChatImages_WriteOnlyToStateDir_LegacyUntouched(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 目录尚不存在——验证首次自动建
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacyDir := filepath.Join(t.TempDir(), "legacy-images")
	legacyImg := writeLegacyImage(t, legacyDir, "old-legacy.png", []byte("旧版本的图"))
	setChatImagePaths(t, "", legacyDir)

	sumBefore, mtimeBefore := fileSHA256(t, legacyImg), fileMtime(t, legacyImg)
	namesBefore := dirEntryNames(t, legacyDir)

	raw := []byte("新图正文-\x89PNG\r\n")
	got := saveChatImage(pngDataURL(raw))

	wantDir := filepath.Join(stateDir, "zerg-chat-images")
	if !strings.HasPrefix(got, wantDir+string(filepath.Separator)) {
		t.Fatalf("图片应落在派生目录 %s 下，实得 %s", wantDir, got)
	}
	saved, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("落盘图片读不回 %s: %v", got, err)
	}
	if !bytes.Equal(saved, raw) {
		t.Fatalf("落盘内容与输入不一致：%q vs %q", saved, raw)
	}
	if sumNow := fileSHA256(t, legacyImg); sumNow != sumBefore {
		t.Fatalf("旧图内容被改了！sha256 %s → %s", sumBefore, sumNow)
	}
	if mtimeNow := fileMtime(t, legacyImg); !mtimeNow.Equal(mtimeBefore) {
		t.Fatalf("旧图被改写了！mtime %v → %v", mtimeBefore, mtimeNow)
	}
	if namesNow := dirEntryNames(t, legacyDir); strings.Join(namesNow, ",") != strings.Join(namesBefore, ",") {
		t.Fatalf("旧目录清单变了（不该搬走/不该塞新文件）：%v → %v", namesBefore, namesNow)
	}
}

// ② 新图存完即读得回：绝对路径直读 + 裸文件名回退两条路都通（历史重建 chatMessageToReq 用后者）。
func TestChatImages_SavedImageReadableByPathAndName(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setChatImagePaths(t, "", filepath.Join(t.TempDir(), "no-legacy"))

	raw := []byte("round-trip-\x00\x01\x02")
	saved := saveChatImage(pngDataURL(raw))
	if saved == "" {
		t.Fatal("saveChatImage 应返回落盘路径")
	}
	if b, err := readChatImage(saved); err != nil || !bytes.Equal(b, raw) {
		t.Fatalf("按存库路径应读回原图：err=%v bytes=%q", err, b)
	}
	if b, err := readChatImage(filepath.Base(saved)); err != nil || !bytes.Equal(b, raw) {
		t.Fatalf("按裸文件名应在派生目录里读回原图：err=%v bytes=%q", err, b)
	}
}

// ③ 新目录无、旧目录有 ⇒ 旧图仍可读（只读保留不搬——不搬文件，只多一个只读查找位置）。
func TestChatImages_ReadFallsBackToLegacyDir(t *testing.T) {
	stateDir := t.TempDir() // 新目录下没有任何图
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacyDir := filepath.Join(t.TempDir(), "legacy-images")
	legacyRaw := []byte("旧版本存下的图正文")
	writeLegacyImage(t, legacyDir, "zerg-legacy-probe.png", legacyRaw)
	setChatImagePaths(t, "", legacyDir)

	// 裸文件名（换机/换盘后老绝对路径失效的形态）⇒ 回退到旧目录命中原图
	b, err := readChatImage("zerg-legacy-probe.png")
	if err != nil {
		t.Fatalf("旧目录里的图必须仍可读（只读保留不搬）：%v", err)
	}
	if !bytes.Equal(b, legacyRaw) {
		t.Fatalf("读回的旧图内容不对：%q vs %q", b, legacyRaw)
	}
	// 旧绝对路径（存量 image_path 的形态）⇒ 原样命中
	if b2, err2 := readChatImage(filepath.Join(legacyDir, "zerg-legacy-probe.png")); err2 != nil || !bytes.Equal(b2, legacyRaw) {
		t.Fatalf("存量绝对路径应原样可读：err=%v bytes=%q", err2, b2)
	}
	// 两处都没有的图 ⇒ 报错（不哄上层）
	if _, err := readChatImage("never-existed.png"); err == nil {
		t.Fatal("都不存在的图必须报错（绝不返回空内容哄上层）")
	}
}

// ④ 读目录序列：写落点永远第一；旧目录存在才追加（不存在 ⇒ 只有一个；显式覆盖 ⇒ 不复旧）。
func TestChatImages_ReadDirsWriteFirstLegacyAppendedOnlyWhenExists(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	writeDir := filepath.Join(stateDir, "zerg-chat-images")

	missingLegacy := filepath.Join(t.TempDir(), "no-such-dir")
	setChatImagePaths(t, "", missingLegacy)
	if dirs := chatImageReadDirs(); len(dirs) != 1 || dirs[0] != writeDir {
		t.Fatalf("旧目录不存在时读目录序列应只有写落点 %s，实得 %v", writeDir, dirs)
	}

	legacyDir := filepath.Join(t.TempDir(), "legacy-images")
	writeLegacyImage(t, legacyDir, "old.png", []byte("x"))
	setChatImagePaths(t, "", legacyDir)
	dirs := chatImageReadDirs()
	if len(dirs) != 2 || dirs[0] != writeDir || dirs[1] != legacyDir {
		t.Fatalf("旧目录存在时应为 [写落点, 旧目录] 且写落点第一，实得 %v", dirs)
	}

	overrideDir := filepath.Join(t.TempDir(), "override-images")
	setChatImagePaths(t, overrideDir, legacyDir)
	if dirs := chatImageReadDirs(); len(dirs) != 1 || dirs[0] != overrideDir {
		t.Fatalf("显式覆盖时读目录序列应只有覆盖目录（不复旧），实得 %v", dirs)
	}
}

// ⑤ 显式覆盖（测试隔离口径）时写/读都只用覆盖目录——防测试写真机 /tmp、读真机老图。
func TestChatImages_ExplicitOverrideSkipsLegacy(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", filepath.Join(t.TempDir(), "state")) // 即便设了状态目录也不该被用到
	overrideDir := filepath.Join(t.TempDir(), "override-images")
	legacyDir := filepath.Join(t.TempDir(), "legacy-images")
	writeLegacyImage(t, legacyDir, "zerg-legacy-probe.png", []byte("旧图"))
	sumBefore := fileSHA256(t, filepath.Join(legacyDir, "zerg-legacy-probe.png"))
	setChatImagePaths(t, overrideDir, legacyDir)

	raw := []byte("覆盖目录里的图")
	saved := saveChatImage(pngDataURL(raw))
	if !strings.HasPrefix(saved, overrideDir+string(filepath.Separator)) {
		t.Fatalf("显式覆盖时写落点应是覆盖目录 %s，实得 %s", overrideDir, saved)
	}
	if _, err := readChatImage("zerg-legacy-probe.png"); err == nil {
		t.Fatal("显式覆盖时不该退旧目录读图（覆盖即唯一来源）")
	}
	if sumNow := fileSHA256(t, filepath.Join(legacyDir, "zerg-legacy-probe.png")); sumNow != sumBefore {
		t.Fatalf("旧图被改了！sha256 %s → %s", sumBefore, sumNow)
	}
}

// ⑥ 默认目录由 statepath 统一状态目录派生（ZERG_STATE_DIR 可覆盖）；旧字面量只在 legacy 变量上。
func TestChatImages_DefaultDirDerivedFromStateDir(t *testing.T) {
	if chatImagesLegacyDefaultDir != "/tmp/zerg-chat/images" {
		t.Fatalf("旧路径字面量应保持 /tmp/zerg-chat/images（只读来源），实得 %q", chatImagesLegacyDefaultDir)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setChatImagePaths(t, "", filepath.Join(t.TempDir(), "legacy"))

	want := filepath.Join(stateDir, "zerg-chat-images")
	if got := chatImagesWriteDir(); got != want {
		t.Fatalf("写目录应由状态目录派生：want %s, got %s", want, got)
	}
	if got := chatImagesWriteDir(); strings.HasPrefix(got, "/tmp/") {
		t.Fatalf("默认落点不该再是 /tmp：%s", got)
	}
	if name := filepath.Base(chatImagesWriteDir()); name != chatImagesDirName {
		t.Fatalf("状态目录下目录名应为 %s，实得 %s", chatImagesDirName, name)
	}
}
