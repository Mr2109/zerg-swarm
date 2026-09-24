// routepin.go —— 「显式指定机器」的**覆盖表**（单独定制 > 路由默认规则 · 2026-09-24）。
//
// 它回答一个问题：**这一次推理，别走默认择优，就落我点名的那台机器**。
//
// 为什么单独一件（而不是塞进 gateway 或 cmd/zerg）：
//
//	**写者与读者必须是同一份形制** —— 写者 = `zerg route pin|unpin`（命令面 · 唯一写口），
//	读者 = 选机那道门（`core/internal/gateway/route_pin.go`）。两处各写一份结构体 = 两份真源
//	⇒ 钉下去的形状与读回来的形状迟早长歪（本仓「生成物自比较 / 登记面写死派生数字」同族教训）。
//
// 落点（**不在任何仓里** · 与 `gap` 族同一口径 —— 运行态活数据不进顶层、不进索引）：
//
//	`ZERG_ROUTE_PINS` → 缺省 `<状态目录>/route_pins.json`（`ZERG_STATE_DIR` → `~/.zerg/state`）。
//	第二实例（验证/演练）靠这两个环境变量天然隔离。
//
// 三条写死的口径：
//
//	① **读者从不写它** —— 网关侧只 `Load`（`route_pin.go` 里 0 个写盘调用，由用例**源码级**断言）。
//	   「模型写不到覆盖表」这条负控就落在这里：请求面（含请求侧头）**没有任何**写路径。
//	② **TTL 是必需品** —— 「偶尔的要求」不许永久改变默认：每一行都带 `expires_at`。
//	   `0` 不是「不过期」，是用法错（由命令面执行前判拒掉）；本件只认 RFC3339 的 `expires_at`。
//	③ **过期不静默** —— 过期行**不再命中**（于是请求回默认择优），但**不被读者删掉**：删是写，
//	   只有下一次写（钉/撒）顺手清（`Prune`）⇒ 盘面永远能自证「钉过什么、什么时候过期」。
package routepin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// FileName —— 覆盖表的默认件名（落在状态目录下）。
const FileName = "route_pins.json"

// TableID —— 形制主号。认不出的主号一律拒（**不猜、不静默降级** —— 与 §十二 `P-026` 同口径）。
const TableID = "route-pins.v1"

// TableNote —— 写进件里的一行口径（谁写、谁能读、为什么带 TTL）。
const TableNote = "「显式指定机器」覆盖表：唯一写口 = `zerg route pin|unpin`；选机那道门只读；每行带 TTL（过期即不再命中）"

// EnvPath —— 覆盖表落点的环境变量名（测试隔离 / 第二实例隔离）。
const EnvPath = "ZERG_ROUTE_PINS"

// Pin —— 表里一行（**一个模型至多一行**；钉两次 = 覆盖，不叠行）。
type Pin struct {
	Model      string `json:"model"`
	Machine    string `json:"machine"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at"`
	TTLSeconds int64  `json:"ttl_seconds"`
	By         string `json:"by,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Table —— 整件（`id` / `note` 是**人可读的形制声明**，`pins` 是真身）。
type Table struct {
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
	Pins []Pin  `json:"pins"`
}

// Path —— 覆盖表落点：`ZERG_ROUTE_PINS` → 缺省 `<状态目录>/route_pins.json`。
func Path() string {
	if p := strings.TrimSpace(os.Getenv(EnvPath)); p != "" {
		return p
	}
	return statepath.File(FileName)
}

// Load —— **只读**取表。三条边界：
//
//	缺件 / 空件  ⇒ 空表 + nil（「没有覆盖」是**正常态**，不是错误 —— 今天的行为就是它的行为）；
//	读不到      ⇒ error（不是「没有覆盖」—— 是真读不到，调用侧据此**不命中**，不许当空表壮胆）；
//	坏件 / 主号不认 ⇒ error（**不猜、不静默降级**）。
func Load(path string) (*Table, error) {
	if strings.TrimSpace(path) == "" {
		path = Path()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Table{ID: TableID}, nil
		}
		return nil, fmt.Errorf("覆盖表读不到（%s）：%w", path, err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return &Table{ID: TableID}, nil
	}
	var t Table
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("覆盖表解读不了（%s）：%w", path, err)
	}
	if t.ID != TableID {
		return nil, fmt.Errorf("覆盖表的形制主号 %q 不认（本版只认 %q）", t.ID, TableID)
	}
	return &t, nil
}

// Save —— **唯一写口**用它（命令面 `route pin|unpin`）。原子：临时件 + `fsync` + `rename`
// （半写的表不许留在地上 —— 「写失败即拒」的同一条纪律）。
//
// ★ 过期行在写的时候顺手清掉（`Prune` 由调用方先跑）—— 但**读者永不删行**（见文件头 ③）。
func (t *Table) Save(path string) error {
	if strings.TrimSpace(path) == "" {
		path = Path()
	}
	if t.ID == "" {
		t.ID = TableID
	}
	if t.Note == "" {
		t.Note = TableNote
	}
	if t.Pins == nil {
		t.Pins = []Pin{}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("覆盖表目录建不出来（%s）：%w", dir, err)
	}
	buf, err := json.MarshalIndent(t, "", " ")
	if err != nil {
		return fmt.Errorf("覆盖表序列化不了：%w", err)
	}
	buf = append(buf, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return fmt.Errorf("覆盖表写不进去（%s）：%w", tmp, err)
	}
	if f, err := os.Open(tmp); err == nil { // fsync：落盘后再换名
		_ = f.Sync()
		_ = f.Close()
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("覆盖表换名失败（%s → %s）：%w", tmp, path, err)
	}
	return nil
}

// Expiry —— 该行的到期时刻（RFC3339）。读不出来 ⇒ error（调用侧按「不命中」处理：宁可不钉，
// 也**不许**把「时刻读不出来」当「永不过期」—— 那正好是「永久改变默认」这条路）。
func (p Pin) Expiry() (time.Time, error) {
	e, err := time.Parse(time.RFC3339, strings.TrimSpace(p.ExpiresAt))
	if err != nil {
		return time.Time{}, fmt.Errorf("覆盖表里的 `expires_at` %q 不是 RFC3339：%w", p.ExpiresAt, err)
	}
	return e, nil
}

// Active —— 这一行在 `now` 还生效吗（读不出到期时刻 ⇒ 不生效，fail-closed 见 Expiry）。
func (p Pin) Active(now time.Time) bool {
	e, err := p.Expiry()
	if err != nil {
		return false
	}
	return now.Before(e)
}

// Remaining —— 到 `now` 为止剩多久（已过期/读不出 ⇒ 负值或 0，调用方自己按 `Active` 判）。
func (p Pin) Remaining(now time.Time) time.Duration {
	e, err := p.Expiry()
	if err != nil {
		return 0
	}
	return e.Sub(now)
}

// HostFor —— 该模型**当前生效**的显式指定机器。命中 ⇒ (pin, true)；否则 (零值, false)。
//
// 命中口径（写死，避免「表里有两行」时靠遍历顺序说话）：**第一条生效的行**胜；
// 一行都不生效（过期 / 时刻读不出 / 压根没有该模型）⇒ 没命中 ⇒ 调用侧走默认择优。
func (t *Table) HostFor(model string, now time.Time) (Pin, bool) {
	if t == nil {
		return Pin{}, false
	}
	for _, p := range t.Pins {
		if p.Model != model {
			continue
		}
		if p.Active(now) {
			return p, true
		}
	}
	return Pin{}, false
}

// RowsOf —— 该模型的全部行（`route ls --model` 用；含过期行 —— 「过期的也要看得见」）。
func (t *Table) RowsOf(model string) []Pin {
	if t == nil {
		return nil
	}
	var out []Pin
	for _, p := range t.Pins {
		if model == "" || p.Model == model {
			out = append(out, p)
		}
	}
	return out
}

// Upsert —— 钉一行：**同一个模型整条替换**（钉两次是覆盖式，不是叠两行）。
func (t *Table) Upsert(p Pin) {
	if t == nil {
		return
	}
	out := make([]Pin, 0, len(t.Pins)+1)
	for _, old := range t.Pins {
		if old.Model != p.Model {
			out = append(out, old)
		}
	}
	t.Pins = append(out, p)
}

// Remove —— 撒一行（`model == ""` ⇒ 撒全部）。返回真正撤掉的行（供命令面回吐 · 幂等：没撤到 ⇒ 空切片）。
func (t *Table) Remove(model string) []Pin {
	if t == nil {
		return nil
	}
	var removed, kept []Pin
	for _, p := range t.Pins {
		if model == "" || p.Model == model {
			removed = append(removed, p)
			continue
		}
		kept = append(kept, p)
	}
	t.Pins = kept
	return removed
}

// Prune —— 清掉**已过期**的行，返回被清掉的那几行（**只有写者调用它**；读者一次都不调）。
func (t *Table) Prune(now time.Time) []Pin {
	if t == nil {
		return nil
	}
	var dropped, kept []Pin
	for _, p := range t.Pins {
		if p.Active(now) {
			kept = append(kept, p)
			continue
		}
		dropped = append(dropped, p)
	}
	t.Pins = kept
	return dropped
}
