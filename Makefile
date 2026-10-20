.PHONY: help run build test vet fmt build-linux clean

BIN := bin/server

help:
	@echo "make run          # 本地启动服务（go run）"
	@echo "make build        # 编译到 $(BIN)"
	@echo "make build-linux  # 交叉编译 linux/amd64"
	@echo "make test         # 跑全部测试"
	@echo "make vet          # 静态检查"
	@echo "make fmt          # 格式化代码"
	@echo "make clean        # 清理产物"

run:
	go run ./cmd/server

build:
	go build -trimpath -o $(BIN) ./cmd/server

build-linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -o $(BIN) ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf bin
