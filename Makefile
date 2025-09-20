# 定义变量
BINARY_NAME=tiny-redis
DOCKER_IMAGE_NAME=tiny-redis:0.1
COMPLETION_DIR=completion
COMMIT_MESSAGE="Auto commit by Makefile"

# 默认目标
all: build

# 构建二进制文件
build:
	@echo "==> Building binary..."
	@go build -o $(BINARY_NAME) ./cmd/tinyredis

# 运行测试
test:
	@echo "==> Running tests..."
	@go test ./...

# 清理构建文件
clean:
	@echo "==> Cleaning up..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(COMPLETION_DIR)

# 构建 Docker 镜像
docker-build:
	@echo "==> Building Docker image..."
	@docker build -t $(DOCKER_IMAGE_NAME) .

# 运行 Docker 容器
docker-run:
	@echo "==> Running Docker container..."
	@docker run -d --name $(BINARY_NAME) -p 6379:6379 -v tinyredis-data:/data $(DOCKER_IMAGE_NAME)

# 停止 Docker 容器
docker-stop:
	@echo "==> Stopping Docker container..."
	@docker stop $(BINARY_NAME) || true
	@docker rm $(BINARY_NAME) || true

# 生成命令行补全脚本并在当前会话中加载
completion:
	@echo "==> Generating completion scripts..."
	@mkdir -p $(COMPLETION_DIR)
	@if [ "$$0" = "-zsh" ] || [ "$$SHELL" = "/bin/zsh" ] || [ "$$SHELL" = "/usr/bin/zsh" ]; then \
		echo "Detected zsh shell"; \
		./$(BINARY_NAME) completion zsh > $(COMPLETION_DIR)/zsh_completion.sh; \
		echo "Completion script for Zsh has been generated at $(COMPLETION_DIR)/zsh_completion.sh"; \
		echo "To load it, run: source $(COMPLETION_DIR)/zsh_completion.sh"; \
	elif [ "$$0" = "-bash" ] || [ "$$SHELL" = "/bin/bash" ] || [ "$$SHELL" = "/usr/bin/bash" ]; then \
		echo "Detected bash shell"; \
		./$(BINARY_NAME) completion bash > $(COMPLETION_DIR)/bash_completion.sh; \
		echo "Completion script for Bash has been generated at $(COMPLETION_DIR)/bash_completion.sh"; \
		echo "To load it, run: source $(COMPLETION_DIR)/bash_completion.sh"; \
	else \
		echo "Unsupported shell. Please use bash or zsh."; \
	fi
# 运行应用程序
run:
	@echo "==> Running application..."
	@./$(BINARY_NAME)

# Git 提交代码
git-commit:
	@echo "==> Committing changes..."
	@git add .
	@git commit -m "$(COMMIT_MESSAGE)"

# Git 拉取最新代码
git-pull:
	@echo "==> Pulling latest changes..."
	@git pull

# Git 推送代码到远程仓库
git-push:
	@echo "==> Pushing changes to remote..."
	@git push

# 打印帮助信息
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Default target, builds the binary."
	@echo "  build         Builds the binary."
	@echo "  test          Runs tests."
	@echo "  clean         Cleans up build files."
	@echo "  docker-build  Builds the Docker image."
	@echo "  docker-run    Runs the Docker container."
	@echo "  docker-stop   Stops the Docker container."
	@echo "  completion    Generates shell completion scripts."
	@echo "  run           Runs the application."
	@echo "  git-commit    Commits all changes with a default message."
	@echo "  git-pull      Pulls latest changes from the remote repository."
	@echo "  git-push      Pushes changes to the remote repository."
	@echo "  help          Prints this help message."

# 声明伪目标
.PHONY: all build test clean docker-build docker-run docker-stop completion run git-commit git-pull git-push help
