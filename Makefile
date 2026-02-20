.PHONY: build test clean install fmt vet lint coverage bridge bridge-install bridge-build bridge-run webui-install webui-build webui-dev webfetch-install up up-daemon down-daemon restart-daemon docker-build docker-run

# 变量
BINARY_NAME=nanobot-go
BUILD_DIR=build
MAIN_FILE=cmd/nanobot/main.go
BRIDGE_DIR=bridge
BRIDGE_PORT?=3001
WEBFETCH_DIR=webfetcher

# 默认目标
all: build

# 构建
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# 安装到 GOPATH/bin
install:
	go install $(MAIN_FILE)

# 运行测试
test:
	go test -v ./...

# WhatsApp Bridge（Node.js）
bridge-install:
	cd $(BRIDGE_DIR) && npm install

bridge-build:
	cd $(BRIDGE_DIR) && npm run build

bridge-run:
	cd $(BRIDGE_DIR) && BRIDGE_PORT=$(BRIDGE_PORT) npm start

bridge: bridge-install bridge-build bridge-run

webui-install:
	cd webui && npm install

webui-build:
	cd webui && npm run build

webui-dev:
	cd webui && npm run dev

webfetch-install:
	cd $(WEBFETCH_DIR) && npm install

up:
	FORCE_BRIDGE_KILL=1 FORCE_GATEWAY_KILL=1 ./scripts/start_all.sh

up-daemon:
	FORCE_BRIDGE_KILL=1 FORCE_GATEWAY_KILL=1 ./scripts/start_daemon.sh

down-daemon:
	./scripts/stop_daemon.sh

restart-daemon:
	./scripts/restart_daemon.sh

docker-build:
	docker build -t nanobot-go .

docker-run:
	docker run --rm -v ~/.nanobot:/home/nanobot/.nanobot -p 18890:18890 nanobot-go gateway

# 运行测试并生成覆盖率报告
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# 格式化代码
fmt:
	go fmt ./...

# 代码检查
vet:
	go vet ./...

# 清理构建产物
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Cleaned"

# 运行	run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# 开发模式 (热重载需安装 air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# 检查依赖
mod:
	go mod tidy
	go mod verify

# 检查所有
lint: fmt vet
	@echo "Linting complete"

# Electron Desktop App
electron-install:
	cd electron && npm install

electron-dev:
	cd electron && npm run dev

electron-build:
	cd electron && npm run build

electron-dist: build
	cd electron && npm run dist

electron-dist-mac: build
	cd electron && npm run dist:mac

electron-dist-win: build
	cd electron && npm run dist:win

electron-dist-linux: build
	cd electron && npm run dist:linux

# 帮助
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  test       - Run tests"
	@echo "  coverage   - Run tests with coverage"
	@echo "  install    - Install to GOPATH/bin"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  lint       - Run fmt and vet"
	@echo "  clean      - Clean build artifacts"
	@echo "  run        - Build and run"
	@echo "  dev        - Run with hot reload (requires air)"
	@echo "  mod        - Tidy and verify modules"
	@echo "  bridge     - Install/build/run WhatsApp bridge"
	@echo "  bridge-install - Install bridge dependencies"
	@echo "  bridge-build   - Build bridge"
	@echo "  bridge-run     - Run bridge (BRIDGE_PORT=$(BRIDGE_PORT))"
	@echo "  webui-install  - Install web UI dependencies"
	@echo "  webui-build    - Build web UI"
	@echo "  webui-dev      - Run web UI dev server"
	@echo "  webfetch-install - Install Playwright web fetcher"
	@echo "  electron-install - Install Electron app dependencies"
	@echo "  electron-dev     - Run Electron app in dev mode"
	@echo "  electron-build   - Build Electron app"
	@echo "  electron-dist    - Create Electron distributable"
	@echo "  electron-dist-mac - Create macOS distributable"
	@echo "  electron-dist-win - Create Windows distributable"
	@echo "  electron-dist-linux - Create Linux distributable"
	@echo "  up         - Start bridge + gateway"
	@echo "  up-daemon  - Start bridge + gateway in background"
	@echo "  down-daemon - Stop background bridge + gateway"
	@echo "  restart-daemon - Restart background bridge + gateway"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run - Run gateway in Docker"
	@echo "  help       - Show this help"
