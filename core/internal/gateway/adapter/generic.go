package adapter

import (
	"io"
	"net/http"
)

// Generic 兜底适配器：未知客户端原样透传（不转换）。
type Generic struct{}

func (a *Generic) Name() string { return "generic" }

func (a *Generic) Detect(r *http.Request, body []byte) bool { return true }

// TransformRequest 原样透传（generic 不转换）。
func (a *Generic) TransformRequest(r *http.Request, body []byte) ([]byte, string, error) {
	return body, r.URL.Path, nil
}

// TransformResponse 原样透传：流式 SSE 透传，非流式 JSON 透传。
func (a *Generic) TransformResponse(w http.ResponseWriter, resp *http.Response, req *http.Request, body []byte) {
	// 复制响应头
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// TransformError 原样透传错误。
func (a *Generic) TransformError(w http.ResponseWriter, status int, code, msg string) {
	http.Error(w, msg, status)
}
