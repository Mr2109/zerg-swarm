// contract.go —— 契约登记的**单一真源读取面**（§九 M16 · §十二 `P-084`/`P-085`）。
//
// 形状照仓内先例（`S-a` `core/internal/compat`）：清单用 `go:embed` 进二进制，
// 消费者读的是**同一份** —— 不复制、不转抄（转抄就是第二份真源）。
//
// 谁在用它（引用面**非 0** 是这一条的判据）：`core/cmd/zerg` 的 `zerg help registry`
// 直接读本包渲染成表 ⇒ 登记表不是「放着好看的文件」，它在命令面上真的被读。
package contract

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed registry.json
var raw []byte

// Registry —— 契约登记表（schema/authority/entries/change_flow）。
type Registry struct {
	Schema     string         `json:"schema"`
	Note       string         `json:"note"`
	Authority  map[string]any `json:"authority"`
	Entries    []Entry        `json:"entries"`
	ChangeFlow ChangeFlow     `json:"change_flow"`
}

// Entry —— 一条登记（**指向**真源，不复制真源内容）。
type Entry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Truth        string `json:"truth"`
	TruthShape   string `json:"truth_shape"`
	VersionField string `json:"version_field"`
	Gate         string `json:"gate"`
	ChangeClass  string `json:"change_class"`
	ChangeNote   string `json:"change_note"`
}

// ChangeFlow —— 变更流程（§九 M16 `V0`–`V7`）。
type ChangeFlow struct {
	Carrier                 string   `json:"carrier"`
	Steps                   []string `json:"steps"`
	AuthorNotApprover       bool     `json:"author_not_approver"`
	BreakingChangeDeprecate string   `json:"breaking_change_deprecation"`
}

// Load 解出登记表（解不动 ⇒ 报错，不吞）。
func Load() (*Registry, error) {
	var r Registry
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("contract: 登记表解不动（registry.json 坏了？）: %w", err)
	}
	return &r, nil
}

// Raw 原样返回嵌入的字节（导出物/对拍用）。
func Raw() []byte { return raw }
