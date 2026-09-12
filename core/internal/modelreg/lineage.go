package modelreg

import "strings"

// ── 血缘声明：parent / base_model（待修补 #16）─────────────────────────────────
//
// 为什么需要（评审 example-35b-v2 观点 5）：同 id 不同量化/形态时，系统分不清"新版"与"另一模型"；
// 重刷量化会丢掉血缘。故记录新增两个**可选**字段：
//
//	parent     —— 同一 model_id 的上一版（version 形如 sha256-<hex>，或 digest 形如 sha256:<64hex>）
//	base_model —— 量化/微调前的基座 id
//
// ⛔ 为什么**不**由存储层自动推断（本条目最关键的取舍）：
//
//	自动推断 parent 只能靠"写入顺序/时间/目录里已有什么"，而这些都不是建材的一部分。
//	铁律是「**同一批建材 → 记录正文逐字节相同**」（正文才是内容寻址的锚：digest 只吃
//	role+sha256）。同一批建材在两台机器上、按不同顺序入库，就会推出不同的 parent，于是
//	"同一条记录"在不同环境下字节不同 —— 不变量当场失效，重探还会撞上 Store.Put 的
//	防覆盖保护（ConflictError，看起来像"重复跑就报错"）。
//
//	所以取值**只能由调用方显式传入**（`zerg-model probe … --parent <version> --base <id>`），
//	写进正文：同一条命令 + 同一批建材 = 同样的 parent/base = 逐字节相同，确定性保持。
//
// 为什么不另开兄弟文件（<version>.lineage.json）：
//
//	兄弟文件承载的是"现状/可刷新"的信息（能力快照、探测留痕的耗时与生成时间）。血缘不是
//	现状——它是身份的一部分，且已经被调用方显式给出、不随探测波动；再开一个可刷新的兄弟
//	文件只会造出两个真相来源（正文里的 parent 与兄弟文件里的 parent 不一致时听谁的？）。
//	故本条目只做"显式传入 → 写正文"这一条路径。
//
// 校验边界（verify）：只校**形状**（可选字段、给了就校）与占位词；离线的 verify 看不到
// 目录里的其他记录，故"parent 指向的那条记录确实存在"属跨记录关联检查，不在本条目范围。

// placeholderLineage 是血缘字段专用的占位词（在通用占位词表之外）。
// 这些值看起来像"填过了"，实际什么都没说——留着它就等于给 #16 开洞。
var placeholderLineage = map[string]bool{
	"self": true, "base": true, "parent": true, "prev": true,
	"previous": true, "latest": true, "inherit": true, "inherited": true,
}

// IsPlaceholderLineage 判定一个血缘值是否为占位词（大小写不敏感，两侧空白先 trim）。
// 复用许可证留痕的同一张表（unset/pending/tbd/todo/n/a/na/placeholder/unknown/待定/未定/
// none/null/-）——"一个值填了但等于没填"在两条红线上是同一种毛病。
func IsPlaceholderLineage(s string) bool {
	v := strings.ToLower(strings.TrimSpace(s))
	return placeholderLicenseTrace[v] || placeholderLineage[v]
}

// ValidID 判定模型 id 形状（标准 §三：小写且不含空格）。
// id 与 base_model 共用这一条形状规则——两者都是"模型的名字"。
func ValidID(s string) bool {
	return s != "" && strings.ToLower(s) == s && !strings.ContainsAny(s, " \t")
}

// ValidLineageRef 判定血缘引用形状：version（VersionOf 的产物，sha256-<hex>）或
// digest（sha256:<64hex>）。与 VersionOf/Verify 的既有口径一致：
// version 是 digest 前 12 位（更短的摘要原样保留），digest 必为 sha256: + 64 位十六进制。
func ValidLineageRef(s string) bool {
	switch {
	case strings.HasPrefix(s, "sha256-"):
		return isHexN(s[len("sha256-"):], 1, 64)
	case strings.HasPrefix(s, "sha256:"):
		return isHexN(s[len("sha256:"):], 64, 64)
	}
	return false
}

// isHexN 判定 s 是否恰为 lo..hi 个小写或大写十六进制字符。
func isHexN(s string, lo, hi int) bool {
	if len(s) < lo || len(s) > hi {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// lineageSelfRef 判定 parent 是否指向自己（version 或 digest）。
// 自指不是形状错误而是**无意义的取值**——填了等于没填，与占位词同类，故一并拒。
func lineageSelfRef(ref string, r *Record) bool {
	if r == nil {
		return false
	}
	d := strings.TrimSpace(r.Digest)
	if d == "" {
		return false
	}
	return ref == d || ref == VersionOf(r)
}

// lineageNote 生成血缘声明的 notes 行（只在确实填了血缘时出现；文案随调用方入参确定，无时间）。
func lineageNote(parent, baseModel string) string {
	var b strings.Builder
	b.WriteString("\n血缘声明（由调用方显式传入，非存储层按写入顺序/时间推断）：")
	if parent != "" {
		b.WriteString("parent=" + parent)
	}
	if baseModel != "" {
		if parent != "" {
			b.WriteString("；")
		}
		b.WriteString("base_model=" + baseModel)
	}
	b.WriteString("（待修补 #16）")
	return b.String()
}
