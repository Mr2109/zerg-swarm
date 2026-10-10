package api

// fleet_logs_http.go — 机群日志 HTTP 面（POST 落盘 + 四条 GET 查询）。
//
// 定案真源 gateway/protocol.md:73-78：字段名与查询参数逐字照定案，✗ 不造第二套协议：
//   POST /api/fleet/logs                 body {machine, logs:[{ts,level,component,model,request_id,message}]}
//   GET  /api/fleet/logs/query?machine=&component=&since=&level=
//   GET  /api/fleet/logs/trace?request_id=
//   GET  /api/fleet/logs/errors?since=
//   GET  /api/fleet/logs/stats?model=

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// fleetStrField — 取 map 里的字符串字段（不存在/非串 ⇒ 空串）。
func fleetStrField(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

// fleetLogsReceive — POST /api/fleet/logs 的实现（LogsHandler 转发到此）。
//
// 兼容两档：
//
//	· v2 批量（协议档）: {"machine":"node-a","logs":[{ts,level,component,model,request_id,message}]}
//	  —— machine 必填；每条 logs[i] 的 ts/level/component 必填，缺则 400 并点名。
//	· v1 单条（兼容档）: {machine,level,component,message,timestamp,request_id}，或非 JSON 整行。
func (h *Handlers) fleetLogsReceive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	store := h.fleetLogsStoreOr503(w)
	if store == nil {
		return
	}

	var payload map[string]any
	if jerr := json.Unmarshal(body, &payload); jerr != nil {
		// 非 JSON（v1 兼容）：整行当 message 落盘。
		if e := store.Insert("", []FleetLogEntry{{Message: string(body)}}); e != nil {
			writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_INSERT_FAILED", e.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stored": 1})
		return
	}

	machine := strings.TrimSpace(fleetStrField(payload, "machine"))
	if machine == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_FIELD", "缺必填字段：machine")
		return
	}

	if rawLogs, hasLogs := payload["logs"]; hasLogs {
		arr, ok := rawLogs.([]any)
		if !ok || len(arr) == 0 {
			writeErrorCode(w, http.StatusBadRequest, "MISSING_FIELD", "缺必填字段：logs（须为非空数组）")
			return
		}
		entries := make([]FleetLogEntry, 0, len(arr))
		for i, it := range arr {
			m, ok := it.(map[string]any)
			if !ok {
				writeErrorCode(w, http.StatusBadRequest, "INVALID_FIELD", fmt.Sprintf("logs[%d] 不是对象", i))
				return
			}
			e := FleetLogEntry{
				TS:        fleetStrField(m, "ts"),
				Level:     fleetStrField(m, "level"),
				Component: fleetStrField(m, "component"),
				Model:     fleetStrField(m, "model"),
				RequestID: fleetStrField(m, "request_id"),
				Message:   fleetStrField(m, "message"),
			}
			for _, req := range []struct{ name, val string }{
				{"ts", e.TS},
				{"level", e.Level},
				{"component", e.Component},
			} {
				if strings.TrimSpace(req.val) == "" {
					writeErrorCode(w, http.StatusBadRequest, "MISSING_FIELD",
						fmt.Sprintf("缺必填字段：logs[%d].%s", i, req.name))
					return
				}
			}
			entries = append(entries, e)
		}
		if e := store.Insert(machine, entries); e != nil {
			writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_INSERT_FAILED", e.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stored": len(entries)})
		return
	}

	// v1 单条兼容档。
	e := FleetLogEntry{
		TS:        fleetStrField(payload, "ts"),
		Level:     fleetStrField(payload, "level"),
		Component: fleetStrField(payload, "component"),
		Model:     fleetStrField(payload, "model"),
		RequestID: fleetStrField(payload, "request_id"),
		Message:   fleetStrField(payload, "message"),
	}
	if e.TS == "" {
		e.TS = fleetStrField(payload, "timestamp")
	}
	if e.Level == "" && e.Component == "" && e.Message == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_FIELD", "缺必填字段：logs（既无批量 logs 数组，也无单条日志的 level/component/message）")
		return
	}
	if err := store.Insert(machine, []FleetLogEntry{e}); err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_INSERT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stored": 1})
}

// fleetLogsStoreOr503 — 取库；不可用则写 503 并返回 nil（调用方直接 return）。
func (h *Handlers) fleetLogsStoreOr503(w http.ResponseWriter) *FleetLogStore {
	store, err := h.fleetLogStore()
	if err != nil || store == nil {
		msg := "fleet 日志库不可用（状态目录取不到/打不开）——拒收，不落盘"
		if err != nil {
			msg = msg + "：" + err.Error()
		}
		writeErrorCode(w, http.StatusServiceUnavailable, "FLEET_LOG_STORE_UNAVAILABLE", msg)
		return nil
	}
	return store
}

// FleetLogsQueryHandler — GET /api/fleet/logs/query?machine=&component=&since=&level=
func (h *Handlers) FleetLogsQueryHandler(w http.ResponseWriter, r *http.Request) {
	store := h.fleetLogsStoreOr503(w)
	if store == nil {
		return
	}
	q := r.URL.Query()
	rows, err := store.QueryLogs(q.Get("machine"), q.Get("component"), q.Get("since"), q.Get("level"))
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_QUERY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// FleetLogsTraceHandler — GET /api/fleet/logs/trace?request_id=
func (h *Handlers) FleetLogsTraceHandler(w http.ResponseWriter, r *http.Request) {
	store := h.fleetLogsStoreOr503(w)
	if store == nil {
		return
	}
	rows, err := store.TraceByRequest(r.URL.Query().Get("request_id"))
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_QUERY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// FleetLogsErrorsHandler — GET /api/fleet/logs/errors?since=
func (h *Handlers) FleetLogsErrorsHandler(w http.ResponseWriter, r *http.Request) {
	store := h.fleetLogsStoreOr503(w)
	if store == nil {
		return
	}
	rows, err := store.ErrorLogs(r.URL.Query().Get("since"))
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_QUERY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// FleetLogsStatsHandler — GET /api/fleet/logs/stats?model=
func (h *Handlers) FleetLogsStatsHandler(w http.ResponseWriter, r *http.Request) {
	store := h.fleetLogsStoreOr503(w)
	if store == nil {
		return
	}
	stats, err := store.StatsByModel(r.URL.Query().Get("model"))
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "FLEET_LOGS_STATS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
