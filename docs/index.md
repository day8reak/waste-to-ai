# GPU Scheduler 文档

本项目是一个轻量级的 GPU 任务调度系统，支持动态资源回收、多型号 GPU 管理，以及与 Ray 集群的集成。

## 文档目录

| 文档 | 说明 |
|------|------|
| [快速开始](setup.md) | 开发环境搭建和运行服务 |
| [使用指南](usage.md) | 基础功能使用说明 |
| [Ray 集成](ray-integration.md) | Ray 集群集成、动态资源抢占 |
| [API 参考](api-reference.md) | REST API 详细说明 |
| [测试文档](testing.md) | 测试覆盖率和运行测试 |
| [使用场景详解](USAGE_SCENARIOS.md) | Ray 集成完整场景分析 |

## 主要特性

- ✅ 任务队列管理
- ✅ GPU 自动分配
- ✅ 动态资源回收（抢占低优先级任务）
- ✅ 多型号 GPU 支持（V100, 3090, 4090 等）
- ✅ Mock 模式（无需真实 GPU 即可测试）
- ✅ REST API
- ✅ CLI 命令行工具
- ✅ **Ray 集群集成**（GPU 动态扩缩容）
- ✅ **GPU 黑名单机制**（block/unblock）

## 快速开始

```bash
# 1. 克隆项目
git clone https://github.com/your-repo/gpu-scheduler.git
cd gpu-scheduler

# 2. 安装依赖
go mod tidy

# 3. 运行服务（Mock 模式）
go run cmd/main.go -config config.json

# 4. 启动成功后访问
# API: http://localhost:8080
# 健康检查: http://localhost:8080/health
```

## 使用场景

### 场景一：日常推理服务（Ray 集成）

```bash
# 1. 启动 GPU Scheduler
go run cmd/main.go

# 2. Ray 申请 GPU
python ray_integration/ray_gpu_allocator.py --job-id ray-inference --num-gpus 4

# 3. Ray 启动推理服务
```

### 场景二：资源抢占

```bash
# 阻塞 GPU（不影响正在运行的任务）
python examples/ray_job_example.py block gpu1 gpu2

# 其他项目使用完毕后解除阻塞
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

## 相关链接

- [Ray 集成详解](ray-integration.md)
- [API 接口说明](api-reference.md)
- [测试覆盖率报告](testing.md)
