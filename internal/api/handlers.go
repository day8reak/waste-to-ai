// ============================================================================
// REST API 处理器
// 提供HTTP接口用于任务提交、查询和管理
// 使用 gorilla/mux 作为路由框架
// ============================================================================

package api

import (
	"encoding/json"
	"fmt"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/models"
	"gpu-scheduler/internal/scheduler"
	"net/http"

	"github.com/gorilla/mux"
)

// Handler API请求处理器
//
// 负责处理HTTP请求并调用底层调度器和GPU管理器
// 所有方法遵循RESTful设计原则
type Handler struct {
	scheduler *scheduler.Scheduler // 任务调度器
	gpuMgr    gpu.GPUManager     // GPU管理器
}

// NewHandler 创建API处理器
//
// 参数：
//   - sched: 任务调度器实例
//   - gpuMgr: GPU管理器实例
//
// 返回：
//   - *Handler: API处理器实例
func NewHandler(sched *scheduler.Scheduler, gpuMgr gpu.GPUManager) *Handler {
	return &Handler{
		scheduler: sched,
		gpuMgr:    gpuMgr,
	}
}

// ============================================================================
// 请求/响应结构体定义
// ============================================================================

// SubmitTaskRequest 任务提交请求结构
//
// JSON示例：
// {
//   "name": "training-job-1",
//   "command": "python train.py",
//   "image": "pytorch:2.0",
//   "gpu_required": 2,
//   "gpu_model": "V100",
//   "priority": 5
// }
type SubmitTaskRequest struct {
	Name           string `json:"name"`            // 任务名称（可选）
	Command        string `json:"command"`          // 执行命令（必需）
	Image          string `json:"image"`            // Docker镜像（必需）
	GPURequired    int    `json:"gpu_required"`    // 需要GPU数量，默认1
	MinGPURequired int    `json:"min_gpu_required"` // 最低GPU数量（动态任务）
	MaxGPURequired int    `json:"max_gpu_required"` // 最高GPU数量（动态任务）
	Dynamic        bool   `json:"dynamic"`         // 是否支持动态调整GPU
	GPUModel       string `json:"gpu_model"`       // 指定GPU型号（可选）
	Priority       int    `json:"priority"`        // 优先级1-10，默认5
}

// SubmitTaskResponse 任务提交响应结构
type SubmitTaskResponse struct {
	TaskID string `json:"task_id"` // 任务ID
	Status string `json:"status"`  // 任务状态
}

// RayAllocateRequest Ray分配GPU请求结构
//
// JSON示例：
// {
//   "job_id": "ray-job-123",
//   "gpu_count": 4,
//   "gpu_model": "V100",
//   "priority": 8
// }
type RayAllocateRequest struct {
	JobID          string `json:"job_id"`            // Ray Job ID（必需）
	GPUCount       int    `json:"gpu_count"`        // 需要GPU数量，默认1
	MinGPURequired int    `json:"min_gpu_required"` // 最低GPU数量（动态任务）
	MaxGPURequired int    `json:"max_gpu_required"` // 最高GPU数量（动态任务）
	Dynamic        bool   `json:"dynamic"`          // 是否支持动态调整GPU
	GPUModel       string `json:"gpu_model"`       // 指定GPU型号（可选）
	Priority       int    `json:"priority"`         // 优先级1-10，默认5
}

// RayAllocateResponse Ray分配GPU响应结构
type RayAllocateResponse struct {
	TaskID   string   `json:"task_id"`    // 任务ID
	JobID    string   `json:"job_id"`    // Ray Job ID
	Status   string   `json:"status"`    // 任务状态
	GPUIDs   []string `json:"gpu_ids"`   // 分配的GPU ID列表
	Message  string   `json:"message"`   // 附加信息
}

// RayReleaseRequest Ray释放GPU请求结构
//
// JSON示例：
// {
//   "job_id": "ray-job-123",
//   "gpu_ids": ["gpu0", "gpu1"]
// }
type RayReleaseRequest struct {
	JobID  string   `json:"job_id"`   // Ray Job ID（必需）
	GPUIDs []string `json:"gpu_ids"`  // 要释放的GPU ID列表（可选）
}

// RayReleaseResponse Ray释放GPU响应结构
type RayReleaseResponse struct {
	JobID   string `json:"job_id"`   // Ray Job ID
	Status  string `json:"status"`   // 操作状态
	Message string `json:"message"`  // 附加信息
}

// RayStatusResponse Ray集群状态响应结构
type RayStatusResponse struct {
	TotalGPUs     int             `json:"total_gpus"`      // 总GPU数量
	AvailableGPUs int            `json:"available_gpus"`  // 可用GPU数量
	AllocatedGPUs int            `json:"allocated_gpus"` // 已分配GPU数量
	BlockedGPUs  int            `json:"blocked_gpus"`   // 被阻塞GPU数量
	RayTasks      []*models.Task  `json:"ray_tasks"`      // Ray任务列表
	TotalRayTasks int            `json:"total_ray_tasks"` // Ray任务总数
}

// RayBlockRequest 阻塞GPU请求结构（CLI手动释放GPU）
//
// JSON示例：
// {
//   "gpu_ids": ["gpu0", "gpu1"]
// }
type RayBlockRequest struct {
	GPUIDs []string `json:"gpu_ids"` // 要阻塞的GPU ID列表
}

// RayBlockResponse 阻塞GPU响应结构
type RayBlockResponse struct {
	Status    string   `json:"status"`     // 操作状态
	Blocked   []string `json:"blocked"`    // 已阻塞的GPU列表
	Message   string   `json:"message"`    // 附加信息
}

// RayUnblockRequest 解除GPU阻塞请求结构
//
// JSON示例：
// {
//   "gpu_ids": ["gpu0"]
// }
type RayUnblockRequest struct {
	GPUIDs []string `json:"gpu_ids"` // 要解除阻塞的GPU ID列表
}

// RayUnblockResponse 解除GPU阻塞响应结构
type RayUnblockResponse struct {
	Status    string   `json:"status"`     // 操作状态
	Unblocked []string `json:"unblocked"`   // 已解除阻塞的GPU列表
	Message   string   `json:"message"`    // 附加信息
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Error string `json:"error"` // 错误信息
}

// ============================================================================
// 任务管理接口
// ============================================================================

// SubmitTask 提交任务
//
// HTTP方法：POST
// 路径：/api/tasks
//
// 功能说明：
// 1. 解析JSON请求体
// 2. 验证必需字段（command, image）
// 3. 设置默认值（gpu_required, priority）
// 4. 创建任务并提交给调度器
// 5. 返回任务ID和状态
//
// 请求体：
// {
//   "name": "任务名",
//   "command": "执行的命令",
//   "image": "Docker镜像",
//   "gpu_required": 1,    // 可选，默认1
//   "gpu_model": "V100",  // 可选，不指定则任意
//   "priority": 5         // 可选，默认5
// }
//
// 响应（200）：
// {
//   "task_id": "20260308103045001abc123",
//   "status": "running"
// }
//
// 响应（202）：任务加入等待队列
// {
//   "error": "task 20260308103045001abc123: task queued: not enough available GPUs"
// }
func (h *Handler) SubmitTask(w http.ResponseWriter, r *http.Request) {
	// 步骤1: 解析请求体
	var req SubmitTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 步骤2: 验证必需字段
	if req.Command == "" || req.Image == "" {
		h.writeError(w, http.StatusBadRequest, "command and image are required")
		return
	}

	// 步骤3: 设置默认值
	if req.GPURequired <= 0 {
		req.GPURequired = 1
	}
	if req.Priority <= 0 {
		req.Priority = 5
	}
	// 默认值处理
	if req.MinGPURequired <= 0 {
		req.MinGPURequired = req.GPURequired
	}
	if req.MaxGPURequired <= 0 {
		req.MaxGPURequired = req.GPURequired
	}

	// 步骤4: 创建任务对象
	task := models.NewTask(req.Name, req.Command, req.Image, req.GPURequired, req.GPUModel, req.Priority)
	// 设置动态调整字段
	task.MinGPURequired = req.MinGPURequired
	task.MaxGPURequired = req.MaxGPURequired
	task.Dynamic = req.Dynamic

	// 步骤5: 提交任务到调度器
	if err := h.scheduler.SubmitTask(task); err != nil {
		// 任务可能进入等待队列（返回 Accepted）
		h.writeError(w, http.StatusAccepted, fmt.Sprintf("task %s: %s", task.ID, err.Error()))
		return
	}

	// 步骤6: 返回成功响应
	h.writeJSON(w, http.StatusOK, SubmitTaskResponse{
		TaskID: task.ID,
		Status: string(task.Status),
	})
}

// GetTasks 获取任务列表
//
// HTTP方法：GET
// 路径：/api/tasks
// 参数：status (可选) - pending/running/completed
//
// 功能说明：
// - 不带参数：返回所有任务
// - 带status参数：返回指定状态的任务
//
// 响应（200）：
// {
//   "tasks": [...],
//   "total": 10
// }
func (h *Handler) GetTasks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	tasks := h.scheduler.GetTasks(status)
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// GetTask 获取单个任务
//
// HTTP方法：GET
// 路径：/api/tasks/{id}
//
// 功能说明：根据任务ID查询任务详情
//
// 响应（200）：返回任务详情JSON
// 响应（404）：任务不存在
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, err := h.scheduler.GetTaskByID(taskID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, task)
}

// KillTask 杀死任务
//
// HTTP方法：POST
// 路径：/api/tasks/{id}/kill
//
// 功能说明：
// - 停止任务关联的Docker容器
// - 释放占用的GPU资源
// - 更新任务状态为killed
//
// 响应（200）：
// {
//   "message": "task killed"
// }
//
// 响应（400）：任务不存在或不在运行状态
func (h *Handler) KillTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	if err := h.scheduler.KillTask(taskID); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{
		"message": "task killed",
	})
}

// ============================================================================
// GPU管理接口
// ============================================================================

// GetGPUs 获取GPU列表
//
// HTTP方法：GET
// 路径：/api/gpus
//
// 功能说明：获取所有GPU设备列表及状态
//
// 响应（200）：
// {
//   "gpus": [
//     {
//       "id": "gpu0",
//       "model": "V100",
//       "status": "allocated",
//       "task_id": "task-123"
//     }
//   ],
//   "total": 4
// }
func (h *Handler) GetGPUs(w http.ResponseWriter, r *http.Request) {
	gpus, err := h.gpuMgr.GetGPUs()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"gpus":  gpus,
		"total": len(gpus),
	})
}

// ============================================================================
// 统计与健康检查接口
// ============================================================================

// GetStats 获取统计信息
//
// HTTP方法：GET
// 路径：/api/stats
//
// 功能说明：获取调度器统计信息
//
// 响应（200）：
// {
//   "pending": 2,
//   "running": 3,
//   "completed": 100
// }
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.scheduler.GetStats()
	h.writeJSON(w, http.StatusOK, stats)
}

// Health 健康检查
//
// HTTP方法：GET
// 路径：/health
//
// 功能说明：服务健康检查接口
// 常用于负载均衡器和容器编排系统的健康探测
//
// 响应（200）：
// {
//   "status": "ok"
// }
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// ============================================================================
// Ray集成接口
// ============================================================================

// RayAllocate 分配GPU给Ray任务
//
// HTTP方法：POST
// 路径：/api/ray/allocate
//
// 功能说明：
// 1. 解析JSON请求体
// 2. 验证必需字段（job_id）
// 3. 创建Ray任务并分配GPU
// 4. 返回分配的GPU ID列表
//
// 请求体：
// {
//   "job_id": "ray-job-123",
//   "gpu_count": 4,
//   "gpu_model": "V100",
//   "priority": 8
// }
//
// 响应（200）：
// {
//   "task_id": "20260308103045001abc123",
//   "job_id": "ray-job-123",
//   "status": "running",
//   "gpu_ids": ["gpu0", "gpu1", "gpu2", "gpu3"],
//   "message": "allocated successfully"
// }
func (h *Handler) RayAllocate(w http.ResponseWriter, r *http.Request) {
	// 步骤1: 解析请求体
	var req RayAllocateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 步骤2: 验证必需字段
	if req.JobID == "" {
		h.writeError(w, http.StatusBadRequest, "job_id is required")
		return
	}

	// 步骤3: 设置默认值
	if req.GPUCount <= 0 {
		req.GPUCount = 1
	}
	if req.Priority <= 0 {
		req.Priority = 5
	}
	// 默认值处理
	if req.MinGPURequired <= 0 {
		req.MinGPURequired = req.GPUCount
	}
	if req.MaxGPURequired <= 0 {
		req.MaxGPURequired = req.GPUCount
	}

	// 步骤4: 创建Ray任务
	task := models.NewRayTask(req.JobID, req.GPUCount, req.GPUModel, req.Priority)
	// 设置动态调整字段
	task.MinGPURequired = req.MinGPURequired
	task.MaxGPURequired = req.MaxGPURequired
	task.Dynamic = req.Dynamic

	// 步骤5: 提交任务到调度器
	if err := h.scheduler.SubmitTask(task); err != nil {
		h.writeError(w, http.StatusAccepted, fmt.Sprintf("task %s: %s", task.ID, err.Error()))
		return
	}

	// 步骤6: 返回成功响应
	h.writeJSON(w, http.StatusOK, RayAllocateResponse{
		TaskID:  task.ID,
		JobID:   req.JobID,
		Status:  string(task.Status),
		GPUIDs:  task.GPUAssigned,
		Message: "allocated successfully",
	})
}

// RayRelease 释放Ray任务占用的GPU
//
// HTTP方法：POST
// 路径：/api/ray/release
//
// 功能说明：
// 1. 解析JSON请求体
// 2. 根据Ray Job ID查找任务
// 3. 释放指定的GPU资源（支持动态缩容）
// 4. 返回操作结果
//
// 请求体（释放全部GPU）：
// {
//   "job_id": "ray-job-123"
// }
//
// 请求体（部分释放，动态缩容）：
// {
//   "job_id": "ray-job-123",
//   "gpu_ids": ["gpu0"]
// }
//
// 响应（200）：
// {
//   "job_id": "ray-job-123",
//   "status": "released",
//   "message": "released 1 GPU(s)"
// }
func (h *Handler) RayRelease(w http.ResponseWriter, r *http.Request) {
	// 步骤1: 解析请求体
	var req RayReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 步骤2: 验证必需字段
	if req.JobID == "" {
		h.writeError(w, http.StatusBadRequest, "job_id is required")
		return
	}

	// 步骤3: 根据Ray Job ID查找任务
	task, err := h.scheduler.GetTaskByRayJobID(req.JobID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, fmt.Sprintf("ray job %s not found", req.JobID))
		return
	}

	// 步骤4: 释放GPU资源
	releasedCount, err := h.scheduler.ReleaseGPUFromTask(task.ID, req.GPUIDs)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 步骤5: 返回成功响应
	message := fmt.Sprintf("released %d GPU(s)", releasedCount)
	if releasedCount == 0 {
		message = "no GPUs to release"
	}

	h.writeJSON(w, http.StatusOK, RayReleaseResponse{
		JobID:   req.JobID,
		Status:  "released",
		Message: message,
	})
}

// RayStatus 获取Ray集群状态
//
// HTTP方法：GET
// 路径：/api/ray/status
//
// 功能说明：获取GPU集群整体状态和Ray任务列表
//
// 响应（200）：
// {
//   "total_gpus": 4,
//   "available_gpus": 2,
//   "allocated_gpus": 2,
//   "ray_tasks": [...],
//   "total_ray_tasks": 1
// }
func (h *Handler) RayStatus(w http.ResponseWriter, r *http.Request) {
	// 获取GPU信息
	gpus, err := h.gpuMgr.GetGPUs()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 统计GPU数量
	totalGPUs := len(gpus)
	availableGPUs := 0
	allocatedGPUs := 0
	blockedGPUs := 0
	for _, gpu := range gpus {
		if gpu.Status == models.GPUStatusIdle {
			availableGPUs++
		} else if gpu.Status == models.GPUStatusAllocated {
			allocatedGPUs++
		} else if gpu.Status == models.GPUStatusBlocked {
			blockedGPUs++
		}
	}

	// 获取Ray任务列表
	rayTasks := h.scheduler.GetRayTasks()

	h.writeJSON(w, http.StatusOK, RayStatusResponse{
		TotalGPUs:     totalGPUs,
		AvailableGPUs: availableGPUs,
		AllocatedGPUs: allocatedGPUs,
		BlockedGPUs:   blockedGPUs,
		RayTasks:      rayTasks,
		TotalRayTasks: len(rayTasks),
	})
}

// RayBlock 阻塞GPU（CLI手动释放GPU，使其不能用于推理）
//
// HTTP方法：POST
// 路径：/api/ray/block
//
// 功能说明：
// 1. 将指定的GPU列入阻塞列表（黑名单）
// 2. 这些GPU不能再用于新的推理任务
// 3. 正在使用这些GPU的任务不受影响
// 4. 推理服务可通过/api/ray/status查询可用GPU
//
// 请求体：
// {
//   "gpu_ids": ["gpu0", "gpu1"]
// }
//
// 响应（200）：
// {
//   "status": "blocked",
//   "blocked": ["gpu0", "gpu1"],
//   "message": "2 GPU(s) blocked successfully"
// }
func (h *Handler) RayBlock(w http.ResponseWriter, r *http.Request) {
	// 步骤1: 解析请求体
	var req RayBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 步骤2: 验证参数
	if len(req.GPUIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "gpu_ids is required")
		return
	}

	// 步骤3: 释放GPU（从推理服务中真正释放）
	// 释放后GPU变为idle，可给其他训练任务使用
	// 推理服务检测到GPU减少后会自动调整（降低吞吐）
	// 注意：ReleaseGPUFromTask 内部已经调用了 gpuManager.ReleaseGPU 使GPU变为idle，
	// 所以这里不需要再调用 BlockGPU
	var released []string
	var affectedTasks []string
	for _, gpuID := range req.GPUIDs {
		// 查找使用该GPU的任务
		tasks := h.scheduler.GetTasksByGPUID(gpuID)
		for _, task := range tasks {
			// 从任务中释放该GPU（内部会调用gpuManager.ReleaseGPU使GPU变为idle）
			_, err := h.scheduler.ReleaseGPUFromTask(task.ID, []string{gpuID})
			if err != nil {
				// 记录错误但继续处理其他GPU
				fmt.Printf("Warning: failed to release GPU %s from task %s: %v\n", gpuID, task.ID, err)
			}
			affectedTasks = append(affectedTasks, fmt.Sprintf("%s(from %s)", gpuID, task.Name))
		}

		// 检查GPU是否已经是idle状态（如果没有任务使用它）
		gpu, err := h.gpuMgr.GetGPUByID(gpuID)
		if err == nil && gpu.Status != models.GPUStatusIdle {
			// GPU仍被占用但没有任务关联，可能是孤立状态，强制释放
			if err := h.gpuMgr.BlockGPU(gpuID); err != nil {
				h.writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		released = append(released, gpuID)
	}

	// 步骤4: 返回成功响应
	message := fmt.Sprintf("%d GPU(s) released successfully. Released GPUs can now be used by other tasks (e.g., training). Inference tasks will detect GPU loss and reduce throughput accordingly.", len(released))
	h.writeJSON(w, http.StatusOK, RayBlockResponse{
		Status:  "released",
		Blocked: released,
		Message: message,
	})
}

// RayUnblock 解除GPU阻塞（恢复可用于推理）
//
// HTTP方法：POST
// 路径：/api/ray/unblock
//
// 功能说明：
// 1. 将指定的GPU从阻塞列表中移除
// 2. 这些GPU恢复为空闲状态，可用于新的推理任务
// 3. 推理服务应检测到新GPU可用，增加吞吐量
//
// 请求体：
// {
//   "gpu_ids": ["gpu0"]
// }
//
// 响应（200）：
// {
//   "status": "unblocked",
//   "unblocked": ["gpu0"],
//   "message": "GPU unblocked, inference service can now use it"
//
// }
func (h *Handler) RayUnblock(w http.ResponseWriter, r *http.Request) {
	// 步骤1: 解析请求体
	var req RayUnblockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 步骤2: 验证参数
	if len(req.GPUIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "gpu_ids is required")
		return
	}

	// 步骤3: 解除阻塞
	var unblocked []string
	for _, gpuID := range req.GPUIDs {
		if err := h.gpuMgr.UnblockGPU(gpuID); err != nil {
			h.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		unblocked = append(unblocked, gpuID)
	}

	// 步骤4: 返回成功响应
	message := fmt.Sprintf("GPU(s) unblocked, inference service can now use them to increase throughput")
	h.writeJSON(w, http.StatusOK, RayUnblockResponse{
		Status:    "unblocked",
		Unblocked: unblocked,
		Message:   message,
	})
}

// ============================================================================
// 辅助方法
// ============================================================================

// writeJSON 写入JSON响应
//
// 功能说明：
// - 设置Content-Type为application/json
// - 设置HTTP状态码
// - 序列化数据为JSON写入响应体
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
//
// 功能说明：
// - 将错误信息封装为ErrorResponse
// - 调用writeJSON写入响应
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, ErrorResponse{Error: message})
}

// ============================================================================
// 路由注册
// ============================================================================

// RegisterRoutes 注册API路由
//
// 路由清单：
// | 方法   | 路径                      | 处理函数    | 说明               |
// |--------|--------------------------|------------|-------------------|
// | POST   | /api/tasks               | SubmitTask | 提交新任务         |
// | GET    | /api/tasks               | GetTasks   | 获取任务列表       |
// | GET    | /api/tasks/{id}          | GetTask    | 获取单个任务详情   |
// | POST   | /api/tasks/{id}/kill     | KillTask   | 杀死指定任务       |
// | GET    | /api/gpus                | GetGPUs    | 获取GPU列表        |
// | GET    | /api/stats               | GetStats   | 获取统计信息       |
// | GET    | /health                  | Health     | 健康检查           |
// | POST   | /api/ray/allocate        | RayAllocate| Ray请求分配GPU    |
// | POST   | /api/ray/release         | RayRelease | Ray释放GPU        |
// | GET    | /api/ray/status          | RayStatus  | 查询Ray集群状态   |
// | POST   | /api/ray/block           | RayBlock   | 阻塞GPU(CLI释放)  |
// | POST   | /api/ray/unblock        | RayUnblock | 解除GPU阻塞       |
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// 任务管理API
	router.HandleFunc("/api/tasks", h.SubmitTask).Methods("POST")      // 提交任务
	router.HandleFunc("/api/tasks", h.GetTasks).Methods("GET")        // 获取任务列表
	router.HandleFunc("/api/tasks/{id}", h.GetTask).Methods("GET")    // 获取任务详情
	router.HandleFunc("/api/tasks/{id}/kill", h.KillTask).Methods("POST") // 杀死任务

	// GPU管理API
	router.HandleFunc("/api/gpus", h.GetGPUs).Methods("GET")          // 获取GPU列表

	// 统计API
	router.HandleFunc("/api/stats", h.GetStats).Methods("GET")        // 获取统计信息

	// Ray集成API
	router.HandleFunc("/api/ray/allocate", h.RayAllocate).Methods("POST") // Ray分配GPU
	router.HandleFunc("/api/ray/release", h.RayRelease).Methods("POST")  // Ray释放GPU
	router.HandleFunc("/api/ray/status", h.RayStatus).Methods("GET")    // Ray集群状态
	router.HandleFunc("/api/ray/block", h.RayBlock).Methods("POST")    // 阻塞GPU
	router.HandleFunc("/api/ray/unblock", h.RayUnblock).Methods("POST") // 解除阻塞

	// 健康检查
	router.HandleFunc("/health", h.Health).Methods("GET")            // 健康检查
}
