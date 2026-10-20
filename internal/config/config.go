// Package config 负责从环境变量读取运行时配置。
//
// 所有配置项都有合理默认值，因此二进制在不设任何环境变量时也能启动。
package config

import (
	"os"
	"strings"
)

// Config 是服务运行所需的全部配置。
type Config struct {
	// Addr 是 HTTP 监听地址，例如 ":8080"。
	Addr string
	// LLMBaseURL 是 OpenAI 兼容接口的根地址，末尾不要带斜杠。
	// 本地 vLLM 通常是 http://127.0.0.1:8000/v1。
	LLMBaseURL string
	// LLMAPIKey 为空时不会发送 Authorization 头（本地 vLLM 一般不需要）。
	LLMAPIKey string
	// LLMModel 是模型名，必须与推理服务注册的名字一致。
	LLMModel string
}

// Load 从环境变量构造 Config。
func Load() Config {
	return Config{
		Addr:       env("ADDR", ":8080"),
		LLMBaseURL: strings.TrimRight(env("LLM_BASE_URL", "http://127.0.0.1:8000/v1"), "/"),
		LLMAPIKey:  env("LLM_API_KEY", ""),
		LLMModel:   env("LLM_MODEL", "default"),
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
