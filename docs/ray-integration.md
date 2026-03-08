# Ray 集成指南

本指南介绍如何将 GPU Scheduler 与 Ray 集群集成，实现动态 GPU 资源管理。

## 目录

1. [架构概述](#1-架构概述)
2. [使用场景](#2-使用场景)
3. [快速开始](#3-快速开始)
4. [Python 客户端](#4-python-客户端)
5. [示例脚本](#5-示例脚本)
6. [推理服务集成](#6-推理服务集成)

---

## 1. 架构概述

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Ray Cluster                                  │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  Ray Head Node                                                  ││
│  │  - 推理服务编排 (vLLM / Text Generation Inference)            ││
│  │  - 弹性扩缩容 (Ray Autoscaler)                                 ││
│  │  - 调用 GPU Scheduler API 申请/释放 GPU                         ││
│  └─────────────────────────────────────────────────────────────────┘│
└────────────────────────────────┬────────────────────────────────────┘
                                 │ Ray Client API
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     GPU Scheduler (Go)                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │ GPU Manager │  │Task Schedule│  │  API Server │              │
│  │ - GPU/NPU   │  │ - 分配/释放 │  │ - Ray API   │              │
│  │ - 黑名单    │  │ - 抢占      │  │ - CLI API   │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────────┘
```

### 核心功能

| 功能 | API | 说明 |
|------|-----|------|
| 分配 GPU | `/api/ray/allocate` | Ray 申请 GPU 资源 |
| 释放 GPU | `/api/ray/release` | Ray 释放 GPU 资源 |
| 阻塞 GPU | `/api/ray/block` | 管理员将 GPU 加入黑名单 |
| 解除阻塞 | `/api/ray/unblock` | 管理员将 GPU 移出黑名单 |
| 状态查询 | `/api/ray/status` | 查看集群和任务状态 |

---

## 2. 使用场景

### 场景一：日常推理服务

```
时间：工作日白天

操作：
1. Ray 启动推理服务，调用 GPU Scheduler API 申请 4 GPU
2. GPU Scheduler 返回 [gpu0, gpu1, gpu2, gpu3]
3. Ray 启动 Worker Nodes，运行 vLLM 推理服务

结果：
- 推理服务正常运行，使用全部 4 GPU
- 吞吐量：100 tokens/s
```

### 场景二：其他项目需要 GPU

```
时间：其他项目需要用 GPU 跑训练

操作：
1. 管理员执行 block gpu1 gpu2
2. gpu1, gpu2 进入黑名单，状态变为 blocked

结果：
- Ray 继续使用 gpu1, gpu2（不中断）
- 新任务不会分配到 blocked GPU
- 推理服务继续在 4 GPU 上运行
- gpu1, gpu2 空闲，可供其他项目使用
```

### 场景三：资源释放，推理服务恢复

```
时间：其他项目使用完毕

操作：
1. 管理员执行 unblock gpu1 gpu2
2. gpu1, gpu2 恢复空闲状态

结果：
- Ray Autoscaler 检测到新 GPU 可用
- 自动扩展，使用全部 4 GPU
- 吞吐量恢复为 100 tokens/s
```

---

## 3. 快速开始

### 3.1 启动 GPU Scheduler

```bash
# 使用 Mock 模式
go run cmd/main.go -config config.json

# 或生产模式
go run cmd/main.go -config config.json
```

服务将在 http://localhost:8080 启动。

### 3.2 Ray 申请 GPU

```bash
# 使用 Python 脚本
python ray_integration/ray_gpu_allocator.py \
  --job-id ray-inference \
  --num-gpus 4
```

或使用 CLI 工具：

```bash
python examples/ray_job_example.py allocate ray-job-1 4
```

### 3.3 查看状态

```bash
curl http://localhost:8080/api/ray/status
```

响应示例：

```json
{
  "total_gpus": 4,
  "available_gpus": 0,
  "allocated_gpus": 4,
  "blocked_gpus": 0,
  "ray_tasks": [
    {
      "ray_job_id": "ray-inference",
      "gpu_ids": ["gpu0", "gpu1", "gpu2", "gpu3"],
      "status": "running"
    }
  ],
  "total_ray_tasks": 1
}
```

### 3.4 Ray 释放 GPU

```bash
python examples/ray_job_example.py release ray-inference
```

---

## 4. Python 客户端

### 4.1 安装依赖

```bash
pip install requests
```

### 4.2 客户端代码

```python
import requests

class GPUSchedulerClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url

    def allocate(self, job_id, gpu_count=1, gpu_model="", priority=5):
        """申请 GPU 资源"""
        resp = requests.post(
            f"{self.base_url}/api/ray/allocate",
            json={
                "job_id": job_id,
                "gpu_count": gpu_count,
                "gpu_model": gpu_model,
                "priority": priority
            }
        )
        resp.raise_for_status()
        return resp.json()

    def release(self, job_id, gpu_ids=None):
        """释放 GPU 资源"""
        data = {"job_id": job_id}
        if gpu_ids:
            data["gpu_ids"] = gpu_ids
        resp = requests.post(
            f"{self.base_url}/api/ray/release",
            json=data
        )
        resp.raise_for_status()
        return resp.json()

    def block(self, gpu_ids):
        """阻塞 GPU"""
        resp = requests.post(
            f"{self.base_url}/api/ray/block",
            json={"gpu_ids": gpu_ids}
        )
        resp.raise_for_status()
        return resp.json()

    def unblock(self, gpu_ids):
        """解除阻塞"""
        resp = requests.post(
            f"{self.base_url}/api/ray/unblock",
            json={"gpu_ids": gpu_ids}
        )
        resp.raise_for_status()
        return resp.json()

    def status(self):
        """获取状态"""
        resp = requests.get(f"{self.base_url}/api/ray/status")
        resp.raise_for_status()
        return resp.json()
```

### 4.3 使用示例

```python
client = GPUSchedulerClient("http://localhost:8080")

# 1. 申请 GPU
result = client.allocate("inference-job", gpu_count=4)
print(f"分配的 GPU: {result['gpu_ids']}")

# 2. 查看状态
status = client.status()
print(f"可用 GPU: {status['available_gpus']}")

# 3. 释放 GPU
client.release("inference-job")

# 4. 阻塞 GPU（管理员操作）
client.block(["gpu1", "gpu2"])

# 5. 解除阻塞（管理员操作）
client.unblock(["gpu1", "gpu2"])
```

---

## 5. 示例脚本

### 5.1 ray_gpu_allocator.py

位置：`ray_integration/ray_gpu_allocator.py`

Ray 任务 GPU 分配器：

```bash
python ray_integration/ray_gpu_allocator.py \
  --job-id my-ray-job \
  --num-gpus 2 \
  --priority 8
```

### 5.2 ray_job_example.py

位置：`examples/ray_job_example.py`

综合 CLI 工具：

```bash
# 分配 GPU
python examples/ray_job_example.py allocate ray-job-1 4

# 释放 GPU
python examples/ray_job_example.py release ray-job-1

# 扩容
python examples/ray_job_example.py scale-up ray-job-1 6

# 缩容
python examples/ray_job_example.py scale-down ray-job-1 2

# 阻塞 GPU
python examples/ray_job_example.py block gpu1 gpu2

# 解除阻塞
python examples/ray_job_example.py unblock gpu1 gpu2

# 查看状态
python examples/ray_job_example.py status
```

---

## 6. 推理服务集成

### 6.1 vLLM 集成示例

位置：`examples/inference_service/vllm_example.py`

```python
import subprocess
import time
from gpu_scheduler_client import GPUSchedulerClient

client = GPUSchedulerClient()

# 1. 申请 GPU
result = client.allocate("vllm-inference", gpu_count=4)
gpu_ids = result["gpu_ids"]

# 2. 设置环境变量并启动 vLLM
env = {
    "CUDA_VISIBLE_DEVICES": ",".join(gpu_ids),
    "RAY_ADDRESS": "auto"
}

# 启动推理服务
process = subprocess.Popen(
    ["python", "-m", "vllm.entrypoints.api_server", "--model", "meta-llama/Llama-2-7b-hf"],
    env={**subprocess.os.environ, **env}
)

# 3. 监控 GPU 变化
# 使用 inference_watcher.py 监控
```

### 6.2 GPU 变化监听

位置：`examples/inference_service/inference_watcher.py`

用于检测 GPU 可用性变化，自动触发推理服务扩缩容：

```bash
python examples/inference_service/inference_watcher.py \
  --job-id vllm-inference \
  --check-interval 30
```

---

## API 详细说明

### 分配 GPU

```bash
POST /api/ray/allocate
```

请求体：

```json
{
  "job_id": "ray-job-123",
  "gpu_count": 2,
  "gpu_model": "V100",
  "priority": 8
}
```

响应：

```json
{
  "status": "success",
  "job_id": "ray-job-123",
  "gpu_ids": ["gpu0", "gpu1"],
  "allocated_count": 2
}
```

### 释放 GPU

```bash
POST /api/ray/release
```

请求体：

```json
{
  "job_id": "ray-job-123",
  "gpu_ids": ["gpu0"]  // 可选，不填则释放所有
}
```

### 阻塞 GPU

```bash
POST /api/ray/block
```

请求体：

```json
{
  "gpu_ids": ["gpu1", "gpu2"]
}
```

### 解除阻塞

```bash
POST /api/ray/unblock
```

请求体：

```json
{
  "gpu_ids": ["gpu1", "gpu2"]
}
```

### 状态查询

```bash
GET /api/ray/status
```
