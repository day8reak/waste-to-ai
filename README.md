# GPU Scheduler

一个轻量级的 GPU 任务调度系统，支持动态资源回收、多型号 GPU 管理，以及 Ray 集群集成。

## 特性

- ✅ 任务队列管理
- ✅ GPU 自动分配
- ✅ 动态资源回收（抢占低优先级任务）
- ✅ 多型号 GPU 支持（V100, 3090, 4090 等）
- ✅ Mock 模式（无需真实 GPU 即可测试）
- ✅ REST API
- ✅ CLI 命令行工具
- ✅ **Ray 集群集成**（GPU 动态扩缩容）
- ✅ **GPU 黑名单机制**（block/unblock，不中断推理服务）

## 快速开始

```bash
# 克隆项目
git clone https://github.com/your-repo/gpu-scheduler.git
cd gpu-scheduler

# 安装依赖
go mod tidy

# 运行服务（Mock 模式）
go run cmd/main.go -config config.json

# 编译 CLI
go build -o gpu-cli cli/main.go
```

服务将在 http://localhost:8080 启动。

## 文档

详细文档请参阅 [docs/](docs/) 目录：

| 文档 | 说明 |
|------|------|
| [docs/index.md](docs/index.md) | 文档首页 |
| [docs/setup.md](docs/setup.md) | 开发环境搭建 |
| [docs/usage.md](docs/usage.md) | 基础功能使用 |
| [docs/ray-integration.md](docs/ray-integration.md) | Ray 集成指南 |
| [docs/api-reference.md](docs/api-reference.md) | API 参考 |
| [docs/testing.md](docs/testing.md) | 测试文档 |

## 使用场景

### 场景一：Ray 推理服务

```bash
# 1. 启动 GPU Scheduler
go run cmd/main.go

# 2. Ray 申请 GPU
python ray_integration/ray_gpu_allocator.py --job-id ray-inference --num-gpus 4

# 3. Ray 启动推理服务
```

### 场景二：动态资源抢占

```bash
# 阻塞 GPU（不影响正在运行的任务）
python examples/ray_job_example.py block gpu1 gpu2

# 解除阻塞（推理服务自动恢复）
python examples/ray_job_example.py unblock gpu1 gpu2
```

## 项目结构

```
gpu-scheduler/
├── cmd/                    # 服务入口
├── internal/
│   ├── api/               # HTTP API
│   ├── config/            # 配置
│   ├── docker/            # Docker 管理
│   ├── gpu/               # GPU 管理
│   ├── models/            # 数据模型
│   └── scheduler/        # 调度器
├── cli/                   # CLI 工具
├── ray_integration/       # Ray 集成脚本
├── examples/              # 示例脚本
├── docs/                  # 文档
└── config.json           # 配置文件
```

## 测试

```bash
# 运行所有测试
go test ./...

# 显示覆盖率
go test -cover ./...
```

## License

MIT
