# GPU 推理服务动态资源调度系统 - 使用场景

## 系统概述

本系统用于在公司空闲 GPU/NPU 资源上运行开源大模型推理服务（如 CodeGeeX、DeepSeek 等），同时支持动态释放资源给其他项目使用，抢占过程不中断推理服务。

## 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Ray Cluster                                  │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  Ray Head Node                                                  ││
│  │  - 推理服务编排 (vLLM / Text Generation Inference)            ││
│  │  - 弹性扩缩容 (Ray Autoscaler)                                 ││
│  │  - 多 Worker Node 管理                                         ││
│  │  - 调用 GPU Scheduler API 申请/释放 GPU                         ││
│  └─────────────────────────────────────────────────────────────────┘│
└────────────────────────────────┬────────────────────────────────────┘
                                 │ Ray Client API
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     GPU Scheduler (Go)                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │
│  │ GPU Manager │  │Task Schedule│  │  API Server │                │
│  │ - GPU/NPU   │  │ - 分配/释放 │  │ - Ray API   │                │
│  │ - 黑名单    │  │ - 抢占      │  │ - CLI API   │                │
│  └─────────────┘  └─────────────┘  └─────────────┘                │
└─────────────────────────────────────────────────────────────────────┘
```

### 角色分工

| 组件 | 职责 |
|------|------|
| **Ray** | 推理服务编排、多节点管理、弹性扩缩容、任务调度 |
| **GPU Scheduler** | 物理 GPU/NPU 资源管理、分配、释放、黑名单机制 |

### 工作流程

```
1. Ray 启动推理服务 → 调用 GPU Scheduler API 申请 GPU
2. GPU Scheduler 分配 GPU → 返回 GPU 列表
3. Ray 启动 Worker Nodes → 使用分配的 GPU 运行推理服务

4. 管理员执行 block → GPU 进入黑名单
   → Ray 继续使用该 GPU（不受影响）
   → 新分配请求不会获得 blocked GPU

5. 管理员执行 unblock → GPU 恢复空闲
   → Ray 检测到新 GPU 可用
   → Ray 自动扩展，使用更多 GPU
```

## 核心概念

### GPU 状态

| 状态 | 说明 |
|------|------|
| `idle` | 空闲，可分配 |
| `allocated` | 已分配给任务 |
| `offline` | 离线（故障）|
| `blocked` | 被阻塞（CLI释放，不能用于推理）|

### 黑名单机制

当管理员通过 CLI 释放 GPU 时：
1. 该 GPU 状态变为 `blocked`
2. **正在运行的推理服务不受影响**，继续使用该 GPU
3. 新任务申请 GPU 时，**不会分配到 blocked 的 GPU**
4. 管理员解除阻塞后，GPU 恢复空闲，推理服务可检测并恢复使用

## 使用场景

### 场景一：日常推理服务

**时间**：工作日白天

**初始状态**：4 GPU 全部空闲

**操作**：
```bash
# 1. 启动推理服务，分配 4 GPU
python examples/ray_job_example.py allocate codegeeX-job 4

# 2. 查看状态
python examples/ray_job_example.py status
```

**结果**：
- 推理服务正常运行，使用全部 4 GPU
- 吞吐量：~100 tokens/s

### 场景二：其他项目需要 GPU

**时间**：其他项目需要用 GPU 跑训练

**操作**：
```bash
# 阻塞 2 个 GPU（释放给其他项目使用）
python examples/ray_job_example.py block gpu1 gpu2

# 查看状态
python examples/ray_job_example.py status
```

**状态变化**：
```
blocked_gpus: 0 → 2
```

**结果**：
- gpu1, gpu2 状态变为 `blocked`
- 推理服务继续在 gpu0, gpu3 上运行（不中断）
- 吞吐量下降为 ~50 tokens/s
- gpu1, gpu2 可供其他项目使用

### 场景三：资源释放，推理服务恢复

**时间**：其他项目使用完毕

**操作**：
```bash
# 解除阻塞，恢复用于推理
python examples/ray_job_example.py unblock gpu1 gpu2
```

**状态变化**：
```
blocked_gpus: 2 → 0
```

**结果**：
- gpu1, gpu2 恢复空闲状态
- 推理服务检测到新 GPU 可用
- 推理服务自动扩展，使用全部 4 GPU
- 吞吐量恢复为 ~100 tokens/s

## CLI 命令参考

### 分配 GPU

```bash
# 分配 4 个 GPU
python examples/ray_job_example.py allocate my-job 4

# 指定 GPU 型号
python examples/ray_job_example.py allocate my-job 2 V100
```

### 释放 GPU

```bash
# 缩容：从 4 GPU 减少到 2 GPU
python examples/ray_job_example.py scale-down my-job 2

# 释放所有 GPU
python examples/ray_job_example.py release my-job
```

### 阻塞/解除 GPU（核心功能）

```bash
# 阻塞 GPU（释放给其他用途）
python examples/ray_job_example.py block gpu1
python examples/ray_job_example.py block gpu1 gpu2 gpu3

# 解除阻塞（恢复用于推理）
python examples/ray_job_example.py unblock gpu1
python examples/ray_job_example.py unblock gpu1 gpu2
```

### 查看状态

```bash
# 查看集群状态
python examples/ray_job_example.py status
```

输出示例：
```
==================================================
Cluster Status
==================================================
Total GPUs:      4
Available GPUs:  2
Allocated GPUs:  0
Ray Tasks:       1
==================================================
```

## Ray 集成

Ray 是本系统的核心编排层，负责推理服务的生命周期管理。

### Ray 与 GPU Scheduler 交互

```python
# Ray 使用 GPU Scheduler 客户端
from ray_integration.gpu_scheduler_client import GPUSchedulerClient

# 创建客户端
client = GPUSchedulerClient("http://gpu-scheduler:8080")

# Ray Worker 启动时申请 GPU
gpu_ids = client.allocate(
    job_id="ray-job-xxx",
    num_gpus=4,
    gpu_model="V100"
)

# 设置环境变量，Ray Worker 使用这些 GPU
os.environ["CUDA_VISIBLE_DEVICES"] = ",".join(gpu_ids)
```

### Ray Autoscaler 集成

Ray 的 autoscaler 可以与 GPU Scheduler 配合，实现动态扩缩容：

```python
# ray_autoscale.py - Ray Autoscaler 配置
from ray import tune
from ray.tune import JupyterNotebookReporter

# 自定义资源报告器
def get_gpu_resources():
    client = GPUSchedulerClient("http://gpu-scheduler:8080")
    status = client.get_status()
    return {
        "GPU": status["available_gpus"]
    }

# 配置 Ray Autoscaler
ray.init(
    resources=get_gpu_resources(),
    autoscaler_config={
        "provider": {
            "type": "external",
            "module": "ray.autoscaler.external_comm"
        }
    }
)
```

### 完整 Ray + GPU Scheduler 工作流

```
┌─────────────────────────────────────────────────────────────┐
│ 1. 启动 Ray Cluster                                       │
│    ray start --head --num-gpus=0                          │
│                                                             │
│ 2. Ray 申请 GPU (通过 GPU Scheduler Client)                │
│    client.allocate("ray-inference", num_gpus=4)          │
│    返回: ["gpu0", "gpu1", "gpu2", "gpu3"]                │
│                                                             │
│ 3. Ray 启动 Worker                                        │
│    CUDA_VISIBLE_DEVICES=gpu0,gpu1,gpu2,gpu3             │
│    ray start --address=$HEAD_ADDRESS                       │
│                                                             │
│ 4. 在 Ray 上部署推理服务                                   │
│    vLLM/TGI 使用这些 GPU                                   │
│                                                             │
│ 5. 管理员 block gpu1                                       │
│    → gpu1 进入黑名单                                      │
│    → Ray Worker 继续使用 gpu1（不中断）                   │
│    → Ray 不会分配新任务到 gpu1                             │
│                                                             │
│ 6. 管理员 unblock gpu1                                     │
│    → gpu1 恢复空闲                                        │
│    → Ray Autoscaler 检测到新资源                           │
│    → 自动扩展使用更多 GPU                                  │
└─────────────────────────────────────────────────────────────┘
```

### 使用 Ray 的优势

1. **多节点支持**：Ray 可以管理多台机器的 GPU
2. **弹性扩缩容**：Ray Autoscaler 根据负载自动调整
3. **容错**：节点故障自动恢复
4. **统一接口**：支持多种推理框架（vLLM、TGI、Transformers Server）

## 推理服务集成

### 1. 启动推理服务

```python
from gpu_scheduler_client import GPUSchedulerClient

client = GPUSchedulerClient("http://localhost:8080")

# 分配 GPU
gpu_ids = client.allocate("vllm-inference", num_gpus=4)
print(f"Allocated GPUs: {gpu_ids}")

# 启动 vLLM 服务
# ...
```

### 2. 检测 GPU 变化（后台运行）

```python
import time
import subprocess

def sync_with_scheduler():
    """定期检测 GPU 变化，同步到推理服务"""
    last_available = set()

    while True:
        # 获取当前可用 GPU
        gpus = client.get_available_gpus()
        current_available = set(gpus)

        # 检测变化
        if current_available != last_available:
            added = current_available - last_available
            removed = last_available - current_available

            if added:
                print(f"New GPUs available: {added}")
                # 通知推理服务添加 GPU
                # scale_up()

            if removed:
                print(f"GPUs removed: {removed}")
                # 通知推理服务移除 GPU
                # scale_down()

            last_available = current_available

        time.sleep(5)  # 每 5 秒检查一次
```

### 3. 完整示例

参考 `examples/inference_service/inference_watcher.py`

## API 参考

### 分配 GPU

```bash
curl -X POST http://localhost:8080/api/ray/allocate \
  -H "Content-Type: application/json" \
  -d '{"job_id": "my-job", "gpu_count": 4, "priority": 8}'
```

响应：
```json
{
  "task_id": "20260308103045001abc123",
  "job_id": "my-job",
  "status": "running",
  "gpu_ids": ["gpu0", "gpu1", "gpu2", "gpu3"],
  "message": "allocated successfully"
}
```

### 阻塞 GPU

```bash
curl -X POST http://localhost:8080/api/ray/block \
  -H "Content-Type: application/json" \
  -d '{"gpu_ids": ["gpu1", "gpu2"]}'
```

响应：
```json
{
  "status": "blocked",
  "blocked": ["gpu1", "gpu2"],
  "message": "2 GPU(s) blocked successfully"
}
```

### 解除阻塞

```bash
curl -X POST http://localhost:8080/api/ray/unblock \
  -H "Content-Type: application/json" \
  -d '{"gpu_ids": ["gpu1"]}'
```

响应：
```json
{
  "status": "unblocked",
  "unblocked": ["gpu1"],
  "message": "GPU(s) unblocked"
}
```

### 查看状态

```bash
curl http://localhost:8080/api/ray/status
```

响应：
```json
{
  "total_gpus": 4,
  "available_gpus": 2,
  "allocated_gpus": 2,
  "blocked_gpus": 1,
  "ray_tasks": [...],
  "total_ray_tasks": 1
}
```

## 注意事项

1. **block 不停止推理服务**：block 只是将 GPU 标记为不可用，不会停止正在运行的推理服务
2. **推理服务需主动检测**：推理服务需要定期查询可用 GPU 并动态调整
3. **优先级**：blocked 的 GPU 优先级最低，不会被分配给新任务
4. **NPU 支持**：后续版本将支持 NPU

## 故障排查

### GPU 状态异常

```bash
# 查看所有 GPU 状态
curl http://localhost:8080/api/gpus
```

### 推理服务无法获取 GPU

1. 检查 GPU 状态是否为 `blocked`
2. 如果是，执行 `unblock` 解除

### 推理服务异常

```bash
# 查看任务列表
curl http://localhost:8080/api/tasks

# 查看特定任务
curl http://localhost:8080/api/tasks/{task_id}

# 杀死任务
curl -X POST http://localhost:8080/api/tasks/{task_id}/kill
```

## 未来计划

- [ ] NPU 支持
- [ ] 多节点调度
- [ ] 资源预留
- [ ] 优先级调度
- [ ] Web UI
