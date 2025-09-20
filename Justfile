# TinyRedis Justfile
# 使用 Just (https://github.com/casey/just) 来运行项目任务

# 设置默认 shell
set shell := ["bash", "-uc"]

# 颜色定义
export RED := "\\033[0;31m"
export GREEN := "\\033[0;32m"
export YELLOW := "\\033[0;33m"
export BLUE := "\\033[0;34m"
export NC := "\\033[0m" # No Color

# 项目变量
project_name := "tiny-redis"
binary_name := "tiny-redis"
docker_image := "tiny-redis:latest"
docker_container := "tiny-redis-container"
default_port := "6379"
log_dir := "./logs"

# Go 相关变量
go_files := `find . -name "*.go" -not -path "./vendor/*"`
test_timeout := "30s"
bench_time := "10s"

# 默认任务 - 显示帮助
default:
    @just --list --unsorted

# 显示彩色帮助信息
help:
    @echo -e "${GREEN}TinyRedis Development Commands${NC}"
    @echo -e "${BLUE}==============================${NC}"
    @just --list --unsorted

# ==================== 构建任务 ====================

# 构建二进制文件
build:
    @echo -e "${BLUE}Building {{project_name}}...${NC}"
    go build -v -o {{binary_name}} ./cmd/tinyredis
    @echo -e "${GREEN}✓ Build complete: {{binary_name}}${NC}"

# 构建发布版本（带优化）
build-release:
    @echo -e "${BLUE}Building release version...${NC}"
    go build -ldflags="-s -w" -o {{binary_name}} ./cmd/tinyredis
    @echo -e "${GREEN}✓ Release build complete${NC}"

# 交叉编译
build-all:
    @echo -e "${BLUE}Building for multiple platforms...${NC}"
    GOOS=linux GOARCH=amd64 go build -o {{binary_name}}-linux-amd64 ./cmd/tinyredis
    GOOS=linux GOARCH=arm64 go build -o {{binary_name}}-linux-arm64 ./cmd/tinyredis
    GOOS=darwin GOARCH=amd64 go build -o {{binary_name}}-darwin-amd64 ./cmd/tinyredis
    GOOS=darwin GOARCH=arm64 go build -o {{binary_name}}-darwin-arm64 ./cmd/tinyredis
    GOOS=windows GOARCH=amd64 go build -o {{binary_name}}-windows-amd64.exe ./cmd/tinyredis
    @echo -e "${GREEN}✓ Cross-compilation complete${NC}"

# ==================== 运行任务 ====================

# 运行服务器
run: build
    @echo -e "${BLUE}Starting TinyRedis server...${NC}"
    ./{{binary_name}}

# 运行服务器（开发模式）
dev:
    @echo -e "${BLUE}Starting development server...${NC}"
    go run ./cmd/tinyredis

# 运行服务器（带自定义端口）
run-port port=default_port: build
    @echo -e "${BLUE}Starting TinyRedis on port {{port}}...${NC}"
    ./{{binary_name}} --port {{port}}

# ==================== 测试任务 ====================

# 运行所有测试
test:
    @echo -e "${BLUE}Running tests...${NC}"
    go test -v -race -timeout {{test_timeout}} ./...
    @echo -e "${GREEN}✓ Tests passed${NC}"

# 运行测试并生成覆盖率报告
test-coverage:
    @echo -e "${BLUE}Running tests with coverage...${NC}"
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo -e "${GREEN}✓ Coverage report generated: coverage.html${NC}"
    @echo -e "${YELLOW}Total coverage: $(go tool cover -func=coverage.out | grep total | awk '{print $3}')${NC}"

# 运行特定包的测试
test-pkg pkg:
    @echo -e "${BLUE}Testing package: {{pkg}}${NC}"
    go test -v -race -timeout {{test_timeout}} ./{{pkg}}/...

# 运行基准测试
bench:
    @echo -e "${BLUE}Running benchmarks...${NC}"
    go test -bench=. -benchmem -benchtime={{bench_time}} ./...

# 运行特定的基准测试
bench-pkg pkg:
    @echo -e "${BLUE}Running benchmarks for {{pkg}}...${NC}"
    go test -bench=. -benchmem -benchtime={{bench_time}} ./{{pkg}}/...

# ==================== 代码质量任务 ====================

# 格式化代码
fmt:
    @echo -e "${BLUE}Formatting code...${NC}"
    gofmt -w .
    go fmt ./...
    @echo -e "${GREEN}✓ Code formatted${NC}"

# 运行 linter
lint:
    @echo -e "${BLUE}Running linter...${NC}"
    @if command -v golangci-lint > /dev/null; then \
        golangci-lint run ./...; \
    else \
        echo -e "${YELLOW}Installing golangci-lint...${NC}"; \
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
        golangci-lint run ./...; \
    fi
    @echo -e "${GREEN}✓ Linting complete${NC}"

# 检查代码风格
vet:
    @echo -e "${BLUE}Running go vet...${NC}"
    go vet ./...
    @echo -e "${GREEN}✓ No issues found${NC}"

# 更新依赖
tidy:
    @echo -e "${BLUE}Tidying dependencies...${NC}"
    go mod tidy
    @echo -e "${GREEN}✓ Dependencies updated${NC}"

# ==================== Docker 任务 ====================

# 构建 Docker 镜像
docker-build:
    @echo -e "${BLUE}Building Docker image...${NC}"
    docker build -t {{docker_image}} .
    @echo -e "${GREEN}✓ Docker image built: {{docker_image}}${NC}"

# 运行 Docker 容器
docker-run: docker-build
    @echo -e "${BLUE}Starting Docker container...${NC}"
    docker run -d --name {{docker_container}} \
        -p {{default_port}}:{{default_port}} \
        -v tinyredis-data:/data \
        {{docker_image}}
    @echo -e "${GREEN}✓ Container started: {{docker_container}}${NC}"

# 停止 Docker 容器
docker-stop:
    @echo -e "${BLUE}Stopping Docker container...${NC}"
    docker stop {{docker_container}} || true
    docker rm {{docker_container}} || true
    @echo -e "${GREEN}✓ Container stopped${NC}"

# 查看 Docker 日志
docker-logs:
    docker logs -f {{docker_container}}

# Docker Compose 启动
compose-up:
    @echo -e "${BLUE}Starting services with Docker Compose...${NC}"
    docker-compose up -d
    @echo -e "${GREEN}✓ Services started${NC}"

# Docker Compose 停止
compose-down:
    @echo -e "${BLUE}Stopping services...${NC}"
    docker-compose down
    @echo -e "${GREEN}✓ Services stopped${NC}"

# ==================== 客户端任务 ====================

# 启动 Redis CLI 客户端
cli:
    @echo -e "${BLUE}Connecting to TinyRedis...${NC}"
    redis-cli -p {{default_port}}

# 运行性能测试
benchmark:
    @echo -e "${BLUE}Running Redis benchmark...${NC}"
    redis-benchmark -p {{default_port}} -t set,get -n 100000 -q

# ==================== 清理任务 ====================

# 清理构建产物
clean:
    @echo -e "${BLUE}Cleaning build artifacts...${NC}"
    rm -f {{binary_name}}
    rm -f {{binary_name}}-*
    rm -f coverage.out coverage.html
    rm -f *.log
    rm -rf {{log_dir}}
    @echo -e "${GREEN}✓ Clean complete${NC}"

# 深度清理（包括依赖缓存）
clean-all: clean
    @echo -e "${BLUE}Deep cleaning...${NC}"
    go clean -cache
    go clean -testcache
    go clean -modcache
    @echo -e "${GREEN}✓ Deep clean complete${NC}"

# ==================== 文档任务 ====================

# 生成 Go 文档
docs:
    @echo -e "${BLUE}Starting documentation server...${NC}"
    @echo -e "${YELLOW}Documentation will be available at http://localhost:8080${NC}"
    godoc -http=:8080

# 生成 README TOC
toc:
    @echo -e "${BLUE}Generating README table of contents...${NC}"
    @if command -v doctoc > /dev/null; then \
        doctoc README.md README_CN.md; \
    else \
        echo -e "${YELLOW}Installing doctoc...${NC}"; \
        npm install -g doctoc; \
        doctoc README.md README_CN.md; \
    fi
    @echo -e "${GREEN}✓ TOC generated${NC}"

# ==================== Git 任务 ====================

# Git 状态
status:
    @git status

# 提交代码
commit message="Auto commit":
    @echo -e "${BLUE}Committing changes...${NC}"
    git add .
    git commit -m "{{message}}"
    @echo -e "${GREEN}✓ Changes committed${NC}"

# 推送到远程
push:
    @echo -e "${BLUE}Pushing to remote...${NC}"
    git push
    @echo -e "${GREEN}✓ Push complete${NC}"

# 拉取最新代码
pull:
    @echo -e "${BLUE}Pulling latest changes...${NC}"
    git pull
    @echo -e "${GREEN}✓ Pull complete${NC}"

# ==================== 安装任务 ====================

# 安装到系统
install: build
    @echo -e "${BLUE}Installing {{binary_name}}...${NC}"
    sudo cp {{binary_name}} /usr/local/bin/
    @echo -e "${GREEN}✓ Installed to /usr/local/bin/{{binary_name}}${NC}"

# 卸载
uninstall:
    @echo -e "${BLUE}Uninstalling {{binary_name}}...${NC}"
    sudo rm -f /usr/local/bin/{{binary_name}}
    @echo -e "${GREEN}✓ Uninstalled${NC}"

# ==================== 性能分析任务 ====================

# CPU 性能分析
profile-cpu:
    @echo -e "${BLUE}Running CPU profiling...${NC}"
    go test -cpuprofile=cpu.prof -bench=. ./...
    go tool pprof -http=:8080 cpu.prof

# 内存性能分析
profile-mem:
    @echo -e "${BLUE}Running memory profiling...${NC}"
    go test -memprofile=mem.prof -bench=. ./...
    go tool pprof -http=:8080 mem.prof

# ==================== 监控任务 ====================

# 查看日志
logs:
    @if [ -d "{{log_dir}}" ]; then \
        tail -f {{log_dir}}/*.log; \
    else \
        echo -e "${YELLOW}No log directory found${NC}"; \
    fi

# 监控系统资源
monitor:
    @echo -e "${BLUE}Monitoring TinyRedis process...${NC}"
    @if pgrep -x "{{binary_name}}" > /dev/null; then \
        PID=$$(pgrep -x "{{binary_name}}"); \
        echo -e "${GREEN}Process found: PID $$PID${NC}"; \
        top -pid $$PID; \
    else \
        echo -e "${RED}TinyRedis is not running${NC}"; \
    fi

# ==================== 实用工具 ====================

# 检查依赖更新
check-updates:
    @echo -e "${BLUE}Checking for dependency updates...${NC}"
    go list -u -m all

# 更新所有依赖
update-deps:
    @echo -e "${BLUE}Updating dependencies...${NC}"
    go get -u ./...
    go mod tidy
    @echo -e "${GREEN}✓ Dependencies updated${NC}"

# 生成命令补全
completion shell="bash":
    @echo -e "${BLUE}Generating {{shell}} completion...${NC}"
    @mkdir -p completion
    @if [ "{{shell}}" = "bash" ]; then \
        echo "complete -W '$$(just --summary)' just" > completion/just-completion.bash; \
    elif [ "{{shell}}" = "zsh" ]; then \
        echo "#compdef just\n_just() {\n  compadd \$$(just --summary)\n}\ncompdef _just just" > completion/just-completion.zsh; \
    fi
    @echo -e "${GREEN}✓ Completion generated: completion/just-completion.{{shell}}${NC}"

# 初始化项目
init:
    @echo -e "${BLUE}Initializing project...${NC}"
    go mod download
    @mkdir -p {{log_dir}}
    @echo -e "${GREEN}✓ Project initialized${NC}"

# 显示项目信息
info:
    @echo -e "${GREEN}TinyRedis Project Information${NC}"
    @echo -e "${BLUE}==============================${NC}"
    @echo -e "Project: {{project_name}}"
    @echo -e "Binary: {{binary_name}}"
    @echo -e "Go version: $$(go version)"
    @echo -e "Dependencies: $$(go list -m all | wc -l) modules"
    @echo -e "Code lines: $$(find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1 | awk '{print $$1}')"

# 快速开始（适合新开发者）
quickstart: init build
    @echo -e "${GREEN}✓ TinyRedis is ready to run!${NC}"
    @echo -e "${YELLOW}Run 'just run' to start the server${NC}"