# 测试文档

本项目包含全面的单元测试，覆盖所有核心功能。

## 测试文件列表

### 基础测试文件

| 文件 | 描述 |
|------|------|
| `internal/models/models_test.go` | 基础模型测试 |
| `internal/config/config_test.go` | 配置加载测试 |
| `internal/gpu/gpu_test.go` | GPU 管理器测试 |
| `internal/docker/docker_test.go` | Docker 管理器测试 |
| `internal/scheduler/scheduler_test.go` | 调度器基础测试 |
| `internal/scheduler/scheduler_extended_test.go` | 调度器扩展测试 |
| `internal/api/handlers_test.go` | API 处理器测试 |

### Ray 集成测试文件

| 文件 | 描述 |
|------|------|
| `internal/models/ray_models_test.go` | Ray 任务模型测试 |
| `internal/gpu/block_test.go` | GPU 阻塞/解除测试 |
| `internal/scheduler/ray_scheduler_test.go` | Ray 调度器测试 |
| `internal/api/ray_handlers_test.go` | Ray API 测试 |

---

## 测试覆盖范围

### Models 层

- [x] `NewTask` - 创建任务
- [x] `NewRayTask` - 创建 Ray 任务
- [x] `NewGPUDevice` - 创建 GPU 设备
- [x] 任务状态常量
- [x] GPU 状态常量（含 Blocked 状态）
- [x] ID 生成

### GPU Manager 层

- [x] `GetGPUs` - 获取所有 GPU
- [x] `GetAvailableGPUs` - 获取可用 GPU（排除 blocked）
- [x] `GetAllocatedGPUs` - 获取已分配 GPU
- [x] `AllocateGPU` - 分配 GPU
- [x] `ReleaseGPU` - 释放 GPU
- [x] `BlockGPU` - 阻塞 GPU（新增）
- [x] `UnblockGPU` - 解除阻塞（新增）
- [x] `GetBlockedGPUs` - 获取阻塞的 GPU（新增）
- [x] `CheckHealth` - 健康检查
- [x] `MarkGPUsOffline` - 标记离线
- [x] `SimulateGPUFailure` - 模拟故障

### Scheduler 层

- [x] `SubmitTask` - 提交任务
- [x] `KillTask` - 杀死任务
- [x] `GetTasks` - 获取任务列表
- [x] `GetTaskByID` - 根据 ID 获取任务
- [x] `GetTaskByRayJobID` - 根据 Ray Job ID 获取任务（新增）
- [x] `ReleaseGPUFromTask` - 释放任务 GPU（新增）
- [x] `GetRayTasks` - 获取所有 Ray 任务（新增）
- [x] `GetStats` - 获取统计信息
- [x] `HandleGPUFailure` - 处理 GPU 故障
- [x] 任务抢占
- [x] 等待队列处理

### API 层

- [x] `POST /api/tasks` - 提交任务
- [x] `GET /api/tasks` - 获取任务列表
- [x] `GET /api/tasks/{id}` - 获取任务详情
- [x] `POST /api/tasks/{id}/kill` - 杀死任务
- [x] `GET /api/gpus` - 获取 GPU 列表
- [x] `GET /api/stats` - 获取统计信息
- [x] `GET /health` - 健康检查
- [x] `POST /api/ray/allocate` - Ray 分配 GPU（新增）
- [x] `POST /api/ray/release` - Ray 释放 GPU（新增）
- [x] `GET /api/ray/status` - Ray 集群状态（新增）
- [x] `POST /api/ray/block` - 阻塞 GPU（新增）
- [x] `POST /api/ray/unblock` - 解除阻塞（新增）

---

## 运行测试

### 基本命令

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test -cover ./...

# 运行特定包的测试
go test -v ./internal/models/...
go test -v ./internal/scheduler/...
go test -v ./internal/api/...
```

### 生成覆盖率报告

```bash
# 运行测试并生成覆盖率文件
go test -coverprofile=coverage.out ./...

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html

# 在终端查看覆盖率摘要
go tool cover -func=coverage.out
```

### 高级选项

```bash
# 运行特定测试
go test -run TestNewRayTask ./internal/models/...

# 显示详细输出
go test -v ./...

# 跳过慢测试
go test -short ./...

# 并行运行测试
go test -parallel 4 ./...
```

---

## 测试分类

### 单元测试

- 模型测试
- GPU 管理器测试
- 调度器测试

### 集成测试

- API 处理器测试
- 端到端工作流测试

### 压力测试

- 并发任务提交
- 大量任务处理

---

## 预期覆盖率

通过新增的测试文件，预期覆盖率将达到：

| 模块 | 覆盖率 |
|------|--------|
| Models | > 95% |
| GPU Manager | > 95% |
| Scheduler | > 90% |
| API | > 85% |
| Docker | > 85% |
| Config | > 85% |
| **Overall** | **> 90%** |

---

## 持续集成

测试可以在 CI/CD 管道中自动运行：

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      - name: Run tests
        run: go test -cover ./...
      - name: Upload coverage
        uses: codecov/codecov-action@v1
```

---

## 测试最佳实践

### 命名规范

- 测试文件：`xxx_test.go`
- 测试函数：`TestXxx(t *testing.T)`
- 子测试：`t.Run("description", func(t *testing.T) {...})`

### Mock 模式

测试使用 Mock 模式，无需真实 GPU：

```go
cfg := config.DefaultConfig()
cfg.MockMode = true
cfg.MockGPUs = []config.MockGPUConfig{
    {ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
}
```

### 常见测试场景

```go
func TestExample(t *testing.T) {
    // 1. 准备测试数据
    cfg := config.DefaultConfig()
    cfg.MockMode = true

    // 2. 创建被测对象
    gpuMgr := gpu.NewGPUManager(cfg)

    // 3. 执行操作
    err := gpuMgr.BlockGPU("gpu0")

    // 4. 验证结果
    if err != nil {
        t.Fatalf("BlockGPU failed: %v", err)
    }

    gpu, _ := gpuMgr.GetGPUByID("gpu0")
    if gpu.Status != models.GPUStatusBlocked {
        t.Errorf("Expected blocked status, got %s", gpu.Status)
    }
}
```
