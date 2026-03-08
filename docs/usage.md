# GPU Scheduler 使用指南

## 目录

1. [基础概念](#1-基础概念)
2. [配置说明](#2-配置说明)
3. [任务管理](#3-任务管理)
4. [GPU 管理](#4-gpu-管理)
5. [CLI 使用](#5-cli-使用)
6. [调度策略](#6-调度策略)

---

## 1. 基础概念

### 1.1 任务状态

| 状态 | 说明 |
|------|------|
| `pending` | 等待分配 GPU 资源 |
| `running` | 正在运行，已分配 GPU |
| `completed` | 任务完成 |
| `failed` | 任务失败 |

### 1.2 GPU 状态

| 状态 | 说明 |
|------|------|
| `idle` | 空闲，可分配 |
| `allocated` | 已分配给任务 |
| `offline` | 离线（故障） |
| `blocked` | 阻塞（黑名单） |

### 1.3 优先级

- 优先级范围：1-10
- 数值越高优先级越高
- 高优先级任务可以抢占低优先级任务的 GPU

---

## 2. 配置说明

编辑 `config.json`:

```json
{
  "server_host": "0.0.0.0",
  "server_port": 8080,
  "mock_mode": true,
  "mock_gpus": [
    {"id": "gpu0", "model": "V100", "memory": 32768, "node": "node1"},
    {"id": "gpu1", "model": "V100", "memory": 32768, "node": "node1"},
    {"id": "gpu2", "model": "3090", "memory": 24576, "node": "node2"},
    {"id": "gpu3", "model": "4090", "memory": 24576, "node": "node3"}
  ],
  "preempt_enabled": true,
  "default_priority": 5
}
```

### 配置项说明

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| server_host | string | 服务监听地址 | "0.0.0.0" |
| server_port | int | 服务监听端口 | 8080 |
| mock_mode | bool | Mock 模式 | true |
| mock_gpus | array | Mock GPU 配置 | [] |
| docker_endpoint | string | Docker 地址 | "unix:///var/run/docker.sock" |
| default_priority | int | 默认优先级 | 5 |
| preempt_enabled | bool | 启用抢占 | true |

---

## 3. 任务管理

### 3.1 提交任务

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "llm-training",
    "image": "pytorch/pytorch:2.0",
    "command": "python train.py",
    "gpu_required": 2,
    "gpu_model": "V100",
    "priority": 5
  }'
```

### 3.2 获取任务列表

```bash
# 所有任务
curl http://localhost:8080/api/tasks

# 按状态筛选
curl http://localhost:8080/api/tasks?status=running
curl http://localhost:8080/api/tasks?status=pending
curl http://localhost:8080/api/tasks?status=completed
```

### 3.3 获取单个任务

```bash
curl http://localhost:8080/api/tasks/{task_id}
```

### 3.4 杀死任务

```bash
curl -X POST http://localhost:8080/api/tasks/{task_id}/kill
```

---

## 4. GPU 管理

### 4.1 获取 GPU 列表

```bash
curl http://localhost:8080/api/gpus
```

响应示例：

```json
{
  "gpus": [
    {
      "id": "gpu0",
      "model": "V100",
      "memory": 32768,
      "node": "node1",
      "status": "allocated",
      "task_id": "task-123"
    }
  ],
  "total": 4
}
```

### 4.2 获取统计信息

```bash
curl http://localhost:8080/api/stats
```

---

## 5. CLI 使用

### 5.1 编译 CLI

```bash
go build -o gpu-cli cli/main.go
```

### 5.2 命令参考

```bash
# 查看帮助
./gpu-cli help

# 提交任务
./gpu-cli submit --name "llm-test" \
  --image "pytorch/pytorch:2.0" \
  --command "python train.py" \
  --gpu 2 \
  --priority 8

# 查看 GPU 列表
./gpu-cli gpus

# 查看任务列表
./gpu-cli tasks

# 杀死任务
./gpu-cli kill <task-id>

# 查看统计
./gpu-cli stats
```

---

## 6. 调度策略

### 6.1 优先级调度

任务按优先级排序：
- 优先级数值越高越先分配
- 同优先级按提交时间排序

### 6.2 GPU 型号匹配

任务可以指定需要的 GPU 型号：

```json
{
  "gpu_model": "V100"  // 只分配 V100
}
```

如果不指定，则分配任意可用 GPU。

### 6.3 任务抢占

启用抢占后，高优先级任务可以抢占低优先级任务的 GPU：

```json
{
  "preempt_enabled": true
}
```

### 6.4 等待队列

当 GPU 不足时，任务自动进入等待队列，待资源释放后自动执行。

---

## 完整示例

### 部署 LLaMA 训练任务

```bash
# 1. 查看可用 GPU
curl http://localhost:8080/api/gpus

# 2. 提交训练任务
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "llama-7b-training",
    "image": "bigai_llama2_training:v1.0",
    "command": "deepspeed --num_gpus=4 train_llama.py",
    "gpu_required": 4,
    "gpu_model": "V100",
    "priority": 10
  }'

# 3. 监控任务
curl http://localhost:8080/api/tasks?status=running

# 4. 杀死任务（释放资源）
curl -X POST http://localhost:8080/api/tasks/<task-id>/kill
```
