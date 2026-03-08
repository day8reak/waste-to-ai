# GPU Scheduler 开发环境设置

## 环境要求

- **Go 1.21+**：编译和运行项目
- **Git**：版本控制
- **Docker**（可选）：容器运行环境
- **Python 3.8+**（可选）：运行 Ray 集成脚本

## 1. 安装 Go

### Windows

```powershell
# 使用 winget（推荐）
winget install Golang.Go

# 或下载安装包
# 访问 https://go.dev/dl/
```

### macOS

```bash
# 使用 Homebrew
brew install go

# 或下载安装包
# 访问 https://go.dev/dl/
```

### Linux

```bash
# 下载并安装
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

## 2. 验证安装

```bash
go version
# 应该显示 go version go1.21.x
```

## 3. 克隆项目

```bash
git clone https://github.com/your-repo/gpu-scheduler.git
cd gpu-scheduler
```

## 4. 安装依赖

```bash
go mod tidy
```

## 5. 运行测试

```bash
# 运行所有测试
go test ./... -v

# 运行特定包的测试
go test -v ./internal/scheduler/...

# 运行测试并显示覆盖率
go test -cover ./...
```

## 6. 运行服务

### Mock 模式（推荐用于开发测试）

```bash
go run cmd/main.go -config config.json
```

服务将在 http://localhost:8080 启动。

### 生产模式

```bash
# 编辑 config.json，将 mock_mode 设为 false
# 配置真实的 GPU 信息
go run cmd/main.go -config config.json
```

## 7. 使用 CLI

```bash
# 编译 CLI
go build -o gpu-cli cli/main.go

# 查看帮助
./gpu-cli help

# 提交任务
./gpu-cli submit --name "test" --image "pytorch/pytorch:2.0" --command "python train.py" --gpu 1

# 查看 GPU
./gpu-cli gpus
```

## 8. Ray 集成（可选）

如果需要使用 Ray 集成功能，还需要安装 Python 依赖：

```bash
# 安装 Python 依赖
pip install requests

# 或使用项目提供的 requirements.txt
pip install -r ray_integration/requirements.txt
```

### Ray 集成快速测试

```bash
# 1. 启动 GPU Scheduler
go run cmd/main.go

# 2. 测试 Ray API
python -c "
import requests
# 分配 GPU
r = requests.post('http://localhost:8080/api/ray/allocate', json={
    'job_id': 'test-ray-job',
    'gpu_count': 2
})
print(r.json())
"
```

## 常见问题

### Q: 测试显示 "no test files"

确保在项目目录下运行 `go test ./...`

### Q: 如何跳过慢测试？

使用 `-short` 标志：`go test -short ./...`

### Q: 如何只运行特定测试？

使用 `-run` 参数：`go test -run TestSubmitTask ./internal/scheduler/...`

### Q: 端口被占用？

修改 `config.json` 中的 `server_port` 或使用命令行参数：

```bash
go run cmd/main.go -port 8081
```

## 快速命令汇总

```bash
# 进入目录
cd gpu-scheduler

# 安装依赖
go mod tidy

# 运行测试
go test ./... -v

# 运行服务
go run cmd/main.go -config config.json

# 构建 CLI
go build -o gpu-cli cli/main.go
```
