// version_agreement_test.go — §二十一 已红第 2 条（T-24）的成对负控：
//
//	① `core/internal/api/control.go` 里**零**版本字面量（原病灶是 :206 那枚 `"zerg-core v2"`）；
//	② `/api/core/status` 的 `version` 与 `/api/capabilities` 的 `version` **同源同值**
//	   （两处都取 `version.Tag` 这一处真源）；
//	③ 旧字段**不删**（两个响应里的 `version` 键仍在 —— 旧消费者零改动）。
//
// 为什么走真处理器（httptest）而不是只 grep 源码：本条的病灶形态是「**同一进程自报两个版本号**」——
// 只有把两个端点各跑一次、把两个值拿来比对，才判得到「同一件东西两处口径」这件事本身。
// grep 那一半（①）也在本文件里落成断言（含一条「检查器有牙」的成对负控）。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// versionLiteralRe 命中「像版本号的字面量」——含原病灶那枚 `zerg-core v2`。
// 口径：`v?<数字>[.<数字>...]`，且**前面不是** `version.Tag/Version/Commit` 这类真源标识符。
// 为什么这样写：注释里可以自由讨论（本仓的注释密度很高），但**字符串字面量**里出现版本号
// 就是又立了一处真源 —— 判据要抓的是后者。
var versionLiteralRe = regexp.MustCompile(`"(?:zerg|zerg-core|虫族)[^"]*v?[0-9]+(?:\.[0-9]+)*"|"v?[0-9]+\.[0-9]+(?:\.[0-9]+)?"`)

// TestVersionLiteralCheckerHasTeeth —— 成对负控之一：检查器对原病灶那枚字面量必须命中。
// （没有这一格，下面那条「0 命中」的断言在任何实现下都可能是假绿。）
func TestVersionLiteralCheckerHasTeeth(t *testing.T) {
	for _, bad := range []string{`"zerg-core v2"`, `"version": "zerg-core v2",`, `"2.5.9"`} {
		if !versionLiteralRe.MatchString(bad) {
			t.Errorf("检查器漏判（无牙）：%s 应被 versionLiteralRe 命中", bad)
		}
	}
	for _, good := range []string{`"version": version.Tag,`, `"code_sha": version.Commit,`, `"最大循环轮数（默认 100）"`} {
		if versionLiteralRe.MatchString(good) {
			t.Errorf("检查器误判（假红）：%s 不该被命中", good)
		}
	}
}

// TestControlSourceHasNoVersionLiteral —— 判据 ①：control.go 里零版本字面量 + 必须取真源。
func TestControlSourceHasNoVersionLiteral(t *testing.T) {
	raw, err := os.ReadFile("control.go")
	if err != nil {
		t.Fatalf("读 control.go：%v", err)
	}
	text := string(raw)
	if hits := versionLiteralRe.FindAllString(text, -1); len(hits) != 0 {
		t.Errorf("control.go 又立了一处版本真源（该文件必须只取 version.Tag）：%v", hits)
	}
	if !strings.Contains(text, "version.Tag") {
		t.Errorf("control.go 里找不到 version.Tag —— 主控自报版本必须取自唯一真源")
	}
	if strings.Contains(text, "zerg-core v2") {
		t.Errorf("control.go 里仍有已红第 2 条点名的那枚硬编码版本号")
	}
}

// TestCoreVersionAgreement_HTTP —— 判据 ②③：真跑两个处理器，逐字比对 version。
func TestCoreVersionAgreement_HTTP(t *testing.T) {
	ctrl := NewControlHandlers("", &config.FleetConfig{})
	h := newTestHandlers()

	// /api/core/status
	req := httptest.NewRequest(http.MethodGet, "/api/core/status", nil)
	w := httptest.NewRecorder()
	ctrl.CoreStatusHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("core/status 应 200，得到 %d", w.Code)
	}
	var coreStatus map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&coreStatus); err != nil {
		t.Fatalf("decode core/status：%v", err)
	}

	// /api/capabilities
	req = httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	w = httptest.NewRecorder()
	h.CapabilitiesHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("capabilities 应 200，得到 %d", w.Code)
	}
	var caps map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&caps); err != nil {
		t.Fatalf("decode capabilities：%v", err)
	}

	// ③ 旧字段不删
	coreVer, ok1 := coreStatus["version"].(string)
	if !ok1 {
		t.Fatalf("core/status 的旧字段 version 不见了（旧消费者会零改动地崩）")
	}
	capVer, ok2 := caps["version"].(string)
	if !ok2 {
		t.Fatalf("capabilities 的旧字段 version 不见了")
	}

	// ② 同源同值
	if coreVer != capVer {
		t.Errorf("同进程自报两个版本号（已红第 2 条的形态）：core/status=%q capabilities=%q", coreVer, capVer)
	}
	if coreVer != version.Tag {
		t.Errorf("两处都必须取自真源 version.Tag=%q，得到 core/status=%q", version.Tag, coreVer)
	}
	if versionLiteralRe.MatchString(`"`+coreVer+`"`) == false {
		t.Errorf("自检：真源值 %q 应当被字面量检查器认出（否则 ① 那条断言是假绿）", coreVer)
	}
}
