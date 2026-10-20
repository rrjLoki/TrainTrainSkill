// Package httpapi 组装 HTTP 路由与中间件。
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/rrjLoki/TrainTrainSkill/internal/llm"
	"github.com/rrjLoki/TrainTrainSkill/internal/store"
)

// Deps 是构造 Handler 所需依赖。LLM 可为 nil（未配置模型服务时降级为 503）。
type Deps struct {
	Store  *store.Store
	LLM    *llm.Client
	Logger *slog.Logger
}

type api struct {
	store  *store.Store
	llm    *llm.Client
	logger *slog.Logger
}

// New 返回带日志与 panic 恢复中间件的 Handler。
func New(d Deps) http.Handler {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}

	a := &api{store: d.Store, llm: d.LLM, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/items", a.listItems)
	mux.HandleFunc("POST /api/items", a.createItem)
	mux.HandleFunc("GET /api/items/{id}", a.getItem)
	mux.HandleFunc("PATCH /api/items/{id}", a.updateItem)
	mux.HandleFunc("DELETE /api/items/{id}", a.deleteItem)
	mux.HandleFunc("POST /api/chat", a.chat)
	mux.HandleFunc("POST /api/chat/stream", a.chatStream)

	return withRecover(logger, withLogging(logger, mux))
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// maxBodyBytes 限制请求体大小，避免被超大 JSON 拖垮内存。
const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.New("请求体不是合法 JSON 或缺少必要字段")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// 响应头已发出，此处只能记录，无法再改状态码。
		slog.Default().Error("write json response failed", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush 让日志包装后的 ResponseWriter 仍然支持流式输出。
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func withRecover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				logger.Error("panic recovered", "value", v, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "服务内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
