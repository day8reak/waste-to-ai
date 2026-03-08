# API 参考文档

## 目录

1. [基础 API](#1-基础-api)
2. [Ray API](#2-ray-api)
3. [响应格式](#3-响应格式)
4. [错误码](#4-错误码)

---

## 1. 基础 API

### 1.1 健康检查

```bash
GET /health
```

响应：

```json
{
  "status": "healthy"
}
```

### 1.2 提交任务

```bash
POST /api/tasks
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 任务名称 |
| command | string | 是 | 执行命令 |
| image | string | 是 | Docker 镜像 |
| gpu_required | int | 是 | 需要 GPU 数量 |
| gpu_model | string | 否 | GPU 型号 (V100/3090/4090) |
| priority | int | 否 | 优先级 (1-10)，默认 5 |

示例：

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "llm-training",
    "image": "pytorch/pytorch:2.0",
    "command": "python train.py",
    "gpu_required": 2,
    "gpu_model": "V100",
    "priority": 8
  }'
```

响应：

```json
{
  "id": "task-abc123",
  "name": "llm-training",
  "status": "running",
  "gpu_assigned": ["gpu0", "gpu1"],
  "created_at": "2024-01-01T00:00:00Z"
}
```

### 1.3 获取任务列表

```bash
GET /api/tasks
```

查询参数：

| 参数 | 说明 |
|------|------|
| status | 筛选状态 (pending/running/completed/failed) |

示例：

```bash
curl http://localhost:8080/api/tasks?status=running
```

### 1.4 获取单个任务

```bash
GET /api/tasks/{task_id}
```

### 1.5 杀死任务

```bash
POST /api/tasks/{task_id}/kill
```

### 1.6 获取 GPU 列表

```bash
GET /api/gpus
```

响应：

```json
{
  "gpus": [
    {
      "id": "gpu0",
      "model": "V100",
      "memory": 32768,
      "node": "node1",
      "status": "allocated",
      "task_id": "task-abc123"
    }
  ],
  "total": 4
}
```

### 1.7 获取统计信息

```bash
GET /api/stats
```

响应：

```json
{
  "total_tasks": 10,
  "running": 3,
  "pending": 2,
  "completed": 5,
  "total_gpus": 4,
  "available_gpus": 1,
  "allocated_gpus": 3
}
```

---

## 2. Ray API

### 2.1 分配 GPU

```bash
POST /api/ray/allocate
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| job_id | string | 是 | Ray Job ID |
| gpu_count | int | 否 | GPU 数量，默认 1 |
| gpu_model | string | 否 | GPU 型号 |
| priority | int | 否 | 优先级 (1-10)，默认 5 |

示例：

```bash
curl -X POST http://localhost:8080/api/ray/allocate \
  -H "Content-Type: application/json" \
  -d '{
    "job_id": "ray-inference",
    "gpu_count": 4,
    "gpu_model": "V100",
    "priority": 8
  }'
```

响应：

```json
{
  "status": "success",
  "job_id": "ray-inference",
  "gpu_ids": ["gpu0", "gpu1", "gpu2", "gpu3"],
  "allocated_count": 4
}
```

### 2.2 释放 GPU

```bash
POST /api/ray/release
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| job_id | string | 是 | Ray Job ID |
| gpu_ids | array | 否 | 要释放的 GPU ID 列表，不填则释放所有 |

示例：

```bash
# 释放所有 GPU
curl -X POST http://localhost:8080/api/ray/release \
  -H "Content-Type: application/json" \
  -d '{"job_id": "ray-inference"}'

# 释放指定 GPU（缩容）
curl -X POST http://localhost:8080/api/ray/release \
  -H "Content-Type: application/json" \
  -d '{"job_id": "ray-inference", "gpu_ids": ["gpu3"]}'
```

响应：

```json
{
  "status": "success",
  "job_id": "ray-inference",
  "released": ["gpu3"],
  "remaining": ["gpu0", "gpu1", "gpu2"]
}
```

### 2.3 状态查询

```bash
GET /api/ray/status
```

响应：

```json
{
  "total_gpus": 4,
  "available_gpus": 1,
  "allocated_gpus": 3,
  "blocked_gpus": 0,
  "ray_tasks": [
    {
      "ray_job_id": "ray-inference",
      "gpu_ids": ["gpu0", "gpu1", "gpu2"],
      "status": "running",
      "priority": 8
    }
  ],
  "total_ray_tasks": 1
}
```

### 2.4 阻塞 GPU

```bash
POST /api/ray/block
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gpu_ids | array | 是 | 要阻塞的 GPU ID 列表 |

示例：

```bash
curl -X POST http://localhost:8080/api/ray/block \
  -H "Content-Type: application/json" \
  -d '{"gpu_ids": ["gpu1", "gpu2"]}'
```

响应：

```json
{
  "status": "success",
  "blocked": ["gpu1", "gpu2"]
}
```

注意：阻塞 GPU 不会影响正在运行的任务，但新任务不会分配到这些 GPU。

### 2.5 解除阻塞

```bash
POST /api/ray/unblock
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| gpu_ids | array | 是 | 要解除阻塞的 GPU ID 列表 |

示例：

```bash
curl -X POST http://localhost:8080/api/ray/unblock \
  -H "Content-Type: application/json" \
  -d '{"gpu_ids": ["gpu1", "gpu2"]}'
```

响应：

```json
{
  "status": "success",
  "unblocked": ["gpu1", "gpu2"]
}
```

---

## 3. 响应格式

### 成功响应

```json
{
  "status": "success",
  // ... 其他字段
}
```

### 错误响应

```json
{
  "status": "error",
  "error": "错误信息"
}
```

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求错误（参数错误、JSON 解析失败） |
| 404 | 资源不存在 |
| 409 | 资源冲突（GPU 不足） |
| 500 | 服务器内部错误 |

---

## 4. 错误码

| 错误码 | 说明 |
|--------|------|
| GPU_NOT_FOUND | GPU 不存在 |
| GPU_INSUFFICIENT | GPU 数量不足 |
| GPU_BLOCKED | GPU 已被阻塞 |
| TASK_NOT_FOUND | 任务不存在 |
| TASK_ALREADY_RUNNING | 任务已在运行 |
| INVALID_PARAMETER | 参数无效 |

---

## 完整示例

### Ray 推理服务完整流程

```bash
# 1. 启动 GPU Scheduler
go run cmd/main.go &

# 2. 申请 GPU（4 张 V100）
curl -X POST http://localhost:8080/api/ray/allocate \
  -H "Content-Type: application/json" \
  -d '{
    "job_id": "vllm-inference",
    "gpu_count": 4,
    "gpu_model": "V100",
    "priority": 8
  }'

# 3. 查看状态
curl http://localhost:8080/api/ray/status

# 4. 其他项目需要 GPU，阻塞部分 GPU
curl -X POST http://localhost:8080/api/ray/block \
  -H "Content-Type: application/json" \
  -d '{"gpu_ids": ["gpu2", "gpu3"]}'

# 5. 查看阻塞状态
curl http://localhost:8080/api/ray/status

# 6. 其他项目使用完毕，解除阻塞
curl -X POST http://localhost:8080/api/ray/unblock \
  -H "Content-Type: application/json" \
  -d '{"gpu_ids": ["gpu2", "gpu3"]}'

# 7. 释放所有 GPU
curl -X POST http://localhost:8080/api/ray/release \
  -H "Content-Type: application/json" \
  -d '{"job_id": "vllm-inference"}'
```
