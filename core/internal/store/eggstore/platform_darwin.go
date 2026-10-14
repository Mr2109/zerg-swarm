//go:build darwin

package eggstore

import (
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// rawVolume 平台原始读数（判据只在 space.go 里算 ⇒ 分平台件只做「取数」一件事）。
type rawVolume struct {
	bsize, blocks, bfree, bavail, files, ffree uint64
	mount, fstype                              string
}

// statfsRaw macOS：`syscall.Statfs_t` 自带 `Mntonname` / `Fstypename`
// ⇒ 挂载点与文件系统类型一步到位（与 `core/internal/store/zdb/volume_darwin.go` 同口径）。
// ★ 可用块取 **`Bavail`**（非特权用户可用）；`Bfree` 只作对拍。
// ★ 可用 inode 取 **`Ffree`**（＝ `statvfs.f_favail`）。
func statfsRaw(path string) (rawVolume, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return rawVolume{}, fmt.Errorf("statfs(%s): %w", path, err)
	}
	return rawVolume{
		bsize: uint64(st.Bsize), blocks: st.Blocks, bfree: st.Bfree,
		bavail: uint64(st.Bavail), files: st.Files, ffree: uint64(st.Ffree),
		mount: cstrInt8(st.Mntonname[:]), fstype: cstrInt8(st.Fstypename[:]),
	}, nil
}

// deviceOf 卷设备号（macOS `Stat_t.Dev` 是 int32）。
func deviceOf(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, fmt.Errorf("stat(%s): %w", path, err)
	}
	return uint64(st.Dev), nil
}

// cstrInt8 把 syscall 的 [N]int8 定长数组当 C 串读。
func cstrInt8(b []int8) string {
	n := len(b)
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			n = i
			break
		}
	}
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, byte(b[i]))
	}
	return string(out)
}

// platformAPFSProbe macOS 补充读数：读 APFS **容器**空闲空间（`diskutil info -plist <挂载点>`）。
//
// 为什么要这一格：同一个 APFS 容器上多卷**共享一个空闲池** ⇒ 逐卷 `df` 会各报不同
// 数字而容器实为一池（设计 §16.2「引擎段同版共享」与「一机 N 枚预算」都踩这条）。
// ★ 边界写在明面：**水线判据只认 df（Bavail）**；本格取不到就留空 + 出声，✗ 不假装。
// ★ 参数是**挂载点**（`Statfs_t.Mntonname`），✗ 不是任意目录 —— `diskutil info` 收的是
//
//	卷/挂载点/设备名，喂普通目录会 rc=1（实测）。
func platformAPFSProbe(mount string) (free *uint64, ref, note string) {
	if strings.TrimSpace(mount) == "" {
		return nil, "", "挂载点未知（Statfs 未报 Mntonname）⇒ 不给结论"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "diskutil", "info", "-plist", mount).Output()
	if err != nil {
		return nil, "", fmt.Sprintf("取数失败（%v）——✗ 不假装有（水线判据只认 df/Bavail）", err)
	}
	kv := plistKV(string(out))
	// 键名实测两态都收：新式 `-plist` 是 `APFSContainerFree`，人读格式那句是
	// `Container Free Space`。两处都试，取到哪个算哪个。
	for _, k := range []string{"APFSContainerFree", "Container Free Space"} {
		if v, ok := kv[k]; ok {
			if n, perr := strconv.ParseUint(strings.TrimSpace(v), 10, 64); perr == nil {
				free = &n
				break
			}
		}
	}
	ref = kv["APFSContainerReference"]
	if free == nil {
		note = "输出里没有 APFSContainerFree / Container Free Space —— 该卷多半不是 APFS（✗ 不假装）"
	}
	return free, ref, note
}

// plistKV 极简 plist 读法：只把「<key>K</key> 紧跟的 <string|integer|real|true|false>」收成 K→V。
// 只为取两个标量键，不做通用 plist 解析（通用解析属另一个面）。
func plistKV(s string) map[string]string {
	out := map[string]string{}
	dec := xml.NewDecoder(strings.NewReader(s))
	pending := ""
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				pending = ""
				if v, err := dec.Token(); err == nil {
					if cd, ok := v.(xml.CharData); ok {
						pending = string(cd)
					}
				}
			case "string", "integer", "real":
				val := ""
				if v, err := dec.Token(); err == nil {
					if cd, ok := v.(xml.CharData); ok {
						val = string(cd)
					}
				}
				if pending != "" {
					out[pending] = val
					pending = ""
				}
			case "true", "false":
				if pending != "" {
					out[pending] = t.Name.Local
					pending = ""
				}
			}
		}
	}
	return out
}
