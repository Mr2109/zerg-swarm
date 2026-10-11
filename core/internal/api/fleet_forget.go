package api

// fleet_forget.go —— fleet 名册删除面：`POST /api/fleet/forget`（GAP-20261010-21 · A 单·端点面）。
//
// 背景：`store.Forget` / `store.StaleMachines` 内核早已在 HEAD（GAP-20260925-29），
// 但主控**端点面缺** ⇒ 换件/探针留下的无效条目（probe / tuntest / …）只能等主控重启才消失，
// 既不干净也无法自证。本件补这一半：**复用 `store.Forget`**，不另造第二份删除实现。
//
// 两态语义（照仓内 CLI 写入面的「干跑／真做」惯例）：
//   · `dry_run=true` ⇒ 只报计划（该机在不在册 / 会不会被摘），**一个字节都不动名册**；
//   · 真删必须显式 `confirm=true` ⇒ 缺它一律 400 拒执且不动名册（fail-closed）。
//
// 幂等：不在册的机再删 ⇒ `changed=false`，不是错误（照 `store.Forget` 的返回值语义）。
// 鉴权：走全局 `AuthMiddleware`（`core/cmd/zerg-core/main.go` 顶部 `r.Use`），本件不另设旁路。
// 审计：照既有 fleet 写面形态 —— 与 `HeartbeatHandler` 同用 `h.HeartbeatLogger.Info(事件, 键, 值…)`。

import (
	"encoding/json"
	"net/http"
	"strings"
)

// FleetForgetHandler 从主控运行态名册里摘掉一台机（`POST /api/fleet/forget`）。
//
// 请求体：`{"machine":"<名>","dry_run":true}` 干跑 · `{"machine":"<名>","confirm":true}` 真删。
// 响应：`{"dry_run":bool,"machine":"<名>","present":bool,"changed":bool,"would_forget":bool}`。
// 错误码：METHOD_NOT_ALLOWED(405) · INVALID_BODY(400) · MISSING_MACHINE(400) ·
// CONFIRM_REQUIRED(400) · STORE_NOT_READY(503)。
func (h *Handlers) FleetForgetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}
	if h.Store == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "STORE_NOT_READY", "运行态名册未就绪")
		return
	}
	var req struct {
		Machine string `json:"machine"`
		DryRun  bool   `json:"dry_run"`
		Confirm bool   `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体 JSON 解析失败: "+err.Error())
		return
	}
	machine := strings.TrimSpace(req.Machine)
	if machine == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MACHINE", "machine 字段不能为空")
		return
	}

	present := h.Store.GetSnapshot(machine) != nil

	// 干跑：只报计划，不动名册（fail-closed —— 不显式 confirm 就不删）。
	if req.DryRun {
		h.auditFleetForget(machine, present, false)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"dry_run":      true,
			"machine":      machine,
			"present":      present,
			"would_forget": present,
			"changed":      false,
		})
		return
	}

	// 真删闸：必须显式 confirm=true（照写入面 fail-closed 口径）。
	if !req.Confirm {
		writeErrorCode(w, http.StatusBadRequest, "CONFIRM_REQUIRED",
			"真删需 confirm=true（只想看计划请用 dry_run=true）")
		return
	}

	changed := h.Store.Forget(machine)
	h.auditFleetForget(machine, present, changed)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dry_run":      false,
		"machine":      machine,
		"present":      present,
		"changed":      changed,
		"would_forget": changed,
	})
}

// auditFleetForget 为一次名册摘除落一行审计 —— 照既有 fleet 写面形态
// （`HeartbeatHandler` 的 `h.HeartbeatLogger.Info(事件, 键, 值…)`）。
// 审计失败只告警、不改变动作结果（审计是留痕，不该反过来让动作失败）。
func (h *Handlers) auditFleetForget(machine string, present, changed bool) {
	logger := h.HeartbeatLogger
	if logger == nil {
		return
	}
	logger.Info("fleet-forget",
		"machine", machine,
		"present", present,
		"changed", changed,
	)
}
