package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// resources_pin.go —— 资源管理器 pin/unpin（《设计-资源管理器》批 4，§八 Q5）。
//
// 路由（注册在 cmd/zerg-core/main.go，与既有 /api/* 同一套 AuthMiddleware 鉴权）：
//
//	POST /api/resources/pin   → 体 {"host":"x3","model":"example-35b-v2","ttl_s":600}
//	POST /api/resources/unpin → 体 {"host":"x3","model":"example-35b-v2"}
//
// pin = "在 TTL 内不被自动驱逐"（§八 Q5：必须带 TTL——无 TTL 的 pin 等同内存泄漏）。
//
// 【红线（本接口不得绕过；测试逐条有断言）】
//   - 在飞绝不驱逐：pin 不改变任何驻留的在飞状态（它只是加一把有期限的锁）；
//   - 未托管绝不接管：pin 只对"已在驻留清单内、且由我们托管（managed=true）"的项生效；
//     非驻留 / 未托管项一律**明确拒绝**，绝不因此启动/接管任何进程（§八 Q6）。
//   - 主控只做校验与转发：真正的锁定/解除在驻留端（子端 backend.Manager，批 3 已实现）。
//     拒绝路径**不调用**动作侧（可用假 manager 断言零调用）。

// maxResourceActionBody 是 pin/unpin 请求体读取上限（防超大 body）。
const maxResourceActionBody = 4096

// ResourcePinController 是 pin/unpin 的动作侧（把主控的锁定意图落到实际驻留端）。
// 主控只转发；真正的锁定/解除与红线拦截在驻留端（子端 backend.Manager.Pin/Unpin）。
type ResourcePinController interface {
	PinResource(host, model string, ttlS int) error
	UnpinResource(host, model string) error
}

// resourceActionReq 是 pin/unpin 的请求体。
type resourceActionReq struct {
	Host  string `json:"host"`
	Model string `json:"model"`
	TTLS  int    `json:"ttl_s"`
}

// decodeResourceAction 解析 pin/unpin 请求体；失败时直接写 400 并返回 ok=false。
func decodeResourceAction(w http.ResponseWriter, r *http.Request) (resourceActionReq, bool) {
	var req resourceActionReq
	if r.Body == nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "缺少请求体")
		return req, false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxResourceActionBody))
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体读取失败: "+err.Error())
		return req, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体为空")
		return req, false
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体解析失败: "+err.Error())
		return req, false
	}
	return req, true
}

// resolveResident 在账本里定位目标驻留项，返回可寻址标识（别名优先）。
//
// 返回 (target, ok, status, code, msg)：ok=false 时调用方必须直接写错，
// 且**不得**调用动作侧——这是"未托管绝不接管"的落地点（非驻留/未托管项到此为止）。
func (h *Handlers) resolveResident(host, model string) (string, bool, int, string, string) {
	if h.Store == nil {
		return "", false, http.StatusServiceUnavailable, "STORE_NOT_READY", "资源账本未就绪（未注入 Store）"
	}
	snap := h.Store.GetSnapshot(host)
	if snap == nil {
		return "", false, http.StatusNotFound, "MACHINE_UNKNOWN", "未知机器（无心跳账本）: " + host
	}
	for _, r := range snap.Resident {
		if r.Alias != model && r.Digest != model {
			continue
		}
		if !r.Managed {
			// §八 Q6：未托管项只标注，不接管——pin 它不是我们的动作
			return "", false, http.StatusConflict, "UNMANAGED_NOT_PINNABLE",
				"目标未托管（managed=false）：资源管理器不接管、不启动外部进程（Q6）——拒绝 pin"
		}
		target := resources.TargetID(r)
		if target == "" {
			return "", false, http.StatusConflict, "MODEL_UNRESOLVABLE", "驻留项无别名/摘要，无法寻址"
		}
		return target, true, 0, "", ""
	}
	return "", false, http.StatusConflict, "MODEL_NOT_RESIDENT",
		"模型不在该机驻留清单内: " + model + "@" + host + "（pin 绝不启动/接管进程）"
}

// ResourcePinHandler — POST /api/resources/pin（需鉴权）
func (h *Handlers) ResourcePinHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeResourceAction(w, r)
	if !ok {
		return
	}
	host := strings.TrimSpace(req.Host)
	model := strings.TrimSpace(req.Model)
	if host == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_HOST", "host 不能为空")
		return
	}
	if model == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MODEL", "model 不能为空")
		return
	}
	// §八 Q5：pin 必须带正 TTL——无 TTL 的 pin 等同内存泄漏
	if req.TTLS <= 0 {
		writeErrorCode(w, http.StatusBadRequest, "PIN_TTL_REQUIRED",
			"pin 必须带正 TTL（ttl_s>0）：无 TTL 的 pin 等同内存泄漏（拍板 Q5）")
		return
	}
	target, ok, status, code, msg := h.resolveResident(host, model)
	if !ok {
		writeErrorCode(w, status, code, msg)
		return
	}
	if h.ResourcePins == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "RESOURCE_PIN_NOT_READY", "pin 动作侧未接线")
		return
	}
	if err := h.ResourcePins.PinResource(host, target, req.TTLS); err != nil {
		writeErrorCode(w, http.StatusBadGateway, "PIN_FORWARD_FAILED", "pin 转发失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"host":   host,
		"model":  model,
		"ttl_s":  req.TTLS,
		"pinned": true,
	})
}

// ResourceUnpinHandler — POST /api/resources/unpin（需鉴权）
//
// 与 pin 同理：同样只对"已在驻留清单内、且托管"的项生效；非驻留 / 未托管项一律拒绝。
func (h *Handlers) ResourceUnpinHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeResourceAction(w, r)
	if !ok {
		return
	}
	host := strings.TrimSpace(req.Host)
	model := strings.TrimSpace(req.Model)
	if host == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_HOST", "host 不能为空")
		return
	}
	if model == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MODEL", "model 不能为空")
		return
	}
	target, ok, status, code, msg := h.resolveResident(host, model)
	if !ok {
		writeErrorCode(w, status, code, msg)
		return
	}
	if h.ResourcePins == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "RESOURCE_PIN_NOT_READY", "pin 动作侧未接线")
		return
	}
	if err := h.ResourcePins.UnpinResource(host, target); err != nil {
		writeErrorCode(w, http.StatusBadGateway, "UNPIN_FORWARD_FAILED", "unpin 转发失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"host":     host,
		"model":    model,
		"pinned":   false,
		"unpinned": true,
	})
}
