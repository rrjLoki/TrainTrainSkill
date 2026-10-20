package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rrjLoki/TrainTrainSkill/internal/llm"
)

type chatRequest struct {
	// Messages 是完整对话历史，最后一条通常是 user。
	Messages []llm.Message `json:"messages"`
	// Prompt 是便捷写法：只给一句用户输入时自动拼成单条 user 消息。
	Prompt string `json:"prompt"`
}

func (req chatRequest) toMessages() ([]llm.Message, string) {
	if len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if strings.TrimSpace(m.Content) == "" {
				return nil, "messages 中存在空内容"
			}
		}
		return req.Messages, ""
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, "prompt 与 messages 至少提供一个"
	}
	return []llm.Message{{Role: "user", Content: prompt}}, ""
}

func (a *api) chat(w http.ResponseWriter, r *http.Request) {
	if a.llm == nil {
		writeError(w, http.StatusServiceUnavailable, "未配置模型服务")
		return
	}

	var req chatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	msgs, msg := req.toMessages()
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	answer, err := a.llm.Chat(r.Context(), msgs)
	if err != nil {
		a.logger.Error("llm chat failed", "err", err)
		writeError(w, http.StatusBadGateway, "模型服务调用失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"model":  a.llm.Model(),
		"answer": answer,
	})
}

// chatStream 以 Server-Sent Events 逐段返回模型输出。
// 事件格式：data: {"delta":"..."}  …  data: [DONE]
func (a *api) chatStream(w http.ResponseWriter, r *http.Request) {
	if a.llm == nil {
		writeError(w, http.StatusServiceUnavailable, "未配置模型服务")
		return
	}

	var req chatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	msgs, msg := req.toMessages()
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "当前连接不支持流式输出")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	err := a.llm.ChatStream(r.Context(), msgs, func(delta string) error {
		payload, err := json.Marshal(map[string]string{"delta": delta})
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		a.logger.Error("llm stream failed", "err", err)
		payload, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", payload)
		return
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}
