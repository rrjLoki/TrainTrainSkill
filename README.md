# TrainTrainSkill

Go 后端骨架：标准库 HTTP 服务 + OpenAI 兼容的模型调用客户端。
零第三方依赖，`go build` 出来是单个二进制。

## 目录结构

```
.
├── cmd/server/          # 入口：配置加载、路由装配、优雅关闭
├── internal/
│   ├── config/          # 环境变量读取
│   ├── httpapi/         # 路由、中间件、handler
│   ├── llm/             # OpenAI 兼容客户端（普通 / 流式）
│   └── store/           # 内存仓储（可替换为 SQL 实现）
├── .github/workflows/   # CI
├── Makefile
└── .env.example
```

## 快速开始

```bash
# 1. 依赖：Go 1.27+
go version

# 2. 跑起来（环境变量都有默认值，可以不配）
go run ./cmd/server

# 3. 验证
curl -s localhost:8080/healthz
curl -s -X POST localhost:8080/api/items \
  -H 'content-type: application/json' -d '{"title":"第一条"}'
curl -s localhost:8080/api/items
```

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查 |
| GET | `/api/items` | 列出全部待办 |
| POST | `/api/items` | 新建，body `{"title":"..."}` |
| GET | `/api/items/{id}` | 查单条 |
| PATCH | `/api/items/{id}` | 局部更新，body `{"title":"...","done":true}` |
| DELETE | `/api/items/{id}` | 删除，返回 204 |
| POST | `/api/chat` | 非流式对话，body `{"prompt":"..."}` 或 `{"messages":[...]}` |
| POST | `/api/chat/stream` | SSE 流式对话，事件 `data: {"delta":"..."}`，结束标记 `data: [DONE]` |

## 配置

全部通过环境变量，均有默认值：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `ADDR` | `:8080` | 监听地址 |
| `LLM_BASE_URL` | `http://127.0.0.1:8000/v1` | OpenAI 兼容接口根地址（vLLM / Ollama / 云端 API） |
| `LLM_API_KEY` | 空 | 为空时不发送 `Authorization` |
| `LLM_MODEL` | `default` | 模型名，需与推理服务注册名一致 |

## 常用命令

```bash
go run ./cmd/server                  # 开发启动
go build -trimpath -o bin/server ./cmd/server
go test ./...                        # 单元测试
go test -race -count=1 ./...         # 竞态检测（需要 gcc）
go vet ./...                         # 静态检查
gofmt -l -w .                        # 格式化
```

`Makefile` 提供同样的快捷方式（`make run` / `make test` / `make build` / `make build-linux`），
但 Debian/Ubuntu 的精简镜像默认没有 `make`，需要先 `sudo apt install make`。
**没有 make 不影响任何功能**，直接用上面的 `go` 命令即可。

> 交叉编译只需改环境变量：`GOOS=linux GOARCH=amd64 go build -trimpath -o bin/server ./cmd/server`。
> `go test -race` 依赖 cgo + gcc，CI 的 `ubuntu-latest` 自带；本机没装 gcc 时跳过该项即可。

## 设计取舍

- **只用标准库**：路由用 Go 1.22+ 的 `ServeMux` 方法+路径模式，不引第三方 web 框架。依赖越少，升级越省心。
- **store 是内存实现**：接口形态按可替换设计，后续接 PostgreSQL 时只改实现，不动 handler。
- **`/api/chat/stream` 依赖 `Flush()`**：日志中间件包装了 `ResponseWriter`，必须转发 `Flush`，否则流式会退化成一次性响应。
- **不设 `WriteTimeout`**：流式连接可能持续数分钟，超时交给请求 context 控制。
