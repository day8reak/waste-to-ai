package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// ============================================================================
// 任务状态定义
// ============================================================================

// TaskStatus 任务状态枚举
//   - Pending: 任务已提交，等待分配GPU
//   - Running: 任务正在运行
//   - Completed: 任务正常完成
//   - Failed: 任务执行失败
//   - Killed: 任务被手动终止或被抢占
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 等待分配资源
	TaskStatusRunning   TaskStatus = "running"   // 正在运行
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 执行失败
	TaskStatusKilled   TaskStatus = "killed"   // 被终止
)

// ============================================================================
// 任务定义
// ============================================================================

// Task 任务结构体
// 表示一个需要GPU资源执行的计算任务
//
// 字段说明:
//   - ID: 唯一标识符，自动生成
//   - Name: 任务名称，用于显示和识别
//   - Command: 要执行的命令
//   - Image: Docker镜像名称
//   - GPURequired: 需要GPU数量
//   - GPUModel: 指定GPU型号（可选，如"V100", "3090"）
//   - Priority: 任务优先级 1-10，数值越大优先级越高
//   - Status: 当前任务状态
//   - GPUAssigned: 已分配的GPU ID列表
//   - ContainerID: 运行的容器ID
//   - ErrorMsg: 错误信息（如果有）
//   - CreatedAt: 创建时间
//   - StartedAt: 开始运行时间
//   - FinishedAt: 结束时间
//   - RayJobID: Ray Job ID（Ray任务专用）
//   - IsRayTask: 是否为Ray任务
type Task struct {
	ID            string     `json:"id" yaml:"id"`                      // 唯一ID
	Name          string     `json:"name" yaml:"name"`                  // 任务名称
	Command       string     `json:"command" yaml:"command"`           // 执行命令
	Image         string     `json:"image" yaml:"image"`               // Docker镜像
	GPURequired   int        `json:"gpu_required" yaml:"gpu_required"`  // 需要GPU数量
	MinGPURequired int       `json:"min_gpu_required" yaml:"min_gpu_required"` // 最低GPU数量（动态任务）
	MaxGPURequired int       `json:"max_gpu_required" yaml:"max_gpu_required"` // 最高GPU数量（动态任务）
	GPUModel      string     `json:"gpu_model" yaml:"gpu_model"`       // 指定GPU型号
	Priority      int        `json:"priority" yaml:"priority"`        // 优先级 1-10
	Status        TaskStatus `json:"status" yaml:"status"`            // 当前状态
	GPUAssigned   []string   `json:"gpu_assigned" yaml:"gpu_assigned"` // 已分配GPU
	ContainerID   string     `json:"container_id" yaml:"container_id"` // 容器ID
	ErrorMsg      string     `json:"error_msg" yaml:"error_msg"`      // 错误信息
	CreatedAt     time.Time  `json:"created_at" yaml:"created_at"`    // 创建时间
	StartedAt     *time.Time `json:"started_at" yaml:"started_at"`   // 开始时间
	FinishedAt    *time.Time `json:"finished_at" yaml:"finished_at"`  // 结束时间
	RayJobID      string     `json:"ray_job_id" yaml:"ray_job_id"`   // Ray Job ID
	IsRayTask     bool       `json:"is_ray_task" yaml:"is_ray_task"` // 是否为Ray任务
	Dynamic       bool       `json:"dynamic" yaml:"dynamic"`          // 是否支持动态调整GPU（true=可动态扩缩容，false=固定GPU）
}

// NewTask 创建新任务的构造函数
//
// 参数:
//   - name: 任务名称
//   - command: 要执行的命令
//   - image: Docker镜像
//   - gpuRequired: 需要GPU数量
//   - gpuModel: 指定GPU型号（空字符串表示任意型号）
//   - priority: 优先级 1-10
//
// 返回:
//   - *Task: 新创建的任务对象，状态为Pending
func NewTask(name, command, image string, gpuRequired int, gpuModel string, priority int) *Task {
	// 确保GPU数量至少为1
	if gpuRequired <= 0 {
		gpuRequired = 1
	}

	// 确保优先级在有效范围内
	if priority <= 0 {
		priority = 5 // 默认优先级
	}

	return &Task{
		ID:            generateID(),
		Name:          name,
		Command:       command,
		Image:         image,
		GPURequired:   gpuRequired,
		MinGPURequired: gpuRequired, // 默认最小GPU等于请求GPU
		MaxGPURequired: gpuRequired, // 默认最大GPU等于请求GPU
		GPUModel:      gpuModel,
		Priority:      priority,
		Status:        TaskStatusPending,
		CreatedAt:     time.Now(),
		Dynamic:       false, // 默认固定GPU
	}
}

// NewRayTask 创建新的Ray任务
//
// 参数:
//   - rayJobID: Ray Job ID
//   - gpuRequired: 需要GPU数量
//   - gpuModel: 指定GPU型号（空字符串表示任意型号）
//   - priority: 优先级 1-10
//
// 返回:
//   - *Task: 新创建的Ray任务对象，状态为Pending
func NewRayTask(rayJobID string, gpuRequired int, gpuModel string, priority int) *Task {
	if gpuRequired <= 0 {
		gpuRequired = 1
	}
	if priority <= 0 {
		priority = 5
	}

	return &Task{
		ID:            generateID(),
		Name:          "ray-" + rayJobID,
		Command:       "",
		Image:         "",
		GPURequired:   gpuRequired,
		MinGPURequired: gpuRequired,
		MaxGPURequired: gpuRequired,
		GPUModel:      gpuModel,
		Priority:      priority,
		Status:        TaskStatusPending,
		RayJobID:      rayJobID,
		IsRayTask:     true,
		Dynamic:       true, // Ray任务默认支持动态调整
		CreatedAt:     time.Now(),
	}
}

// ============================================================================
// GPU状态定义
// ============================================================================

// GPUStatus GPU状态枚举
//   - Idle: 空闲，可分配
//   - Allocated: 已分配给任务
//   - Offline: 离线（故障或不可用）
//   - Blocked: 被阻塞（CLI手动释放，不能用于推理）
type GPUStatus string

const (
	GPUStatusIdle      GPUStatus = "idle"      // 空闲
	GPUStatusAllocated GPUStatus = "allocated" // 已分配
	GPUStatusOffline   GPUStatus = "offline"   // 离线（故障）
	GPUStatusBlocked   GPUStatus = "blocked"   // 被阻塞（CLI释放）
)

// ============================================================================
// GPU设备定义
// ============================================================================

// GPUDevice GPU设备结构体
// 表示一个物理或虚拟GPU设备
//
// 字段说明:
//   - ID: GPU标识符（如gpu0, gpu1）
//   - UUID: GPU唯一序列号
//   - Model: GPU型号（如V100, 3090, 4090）
//   - Memory: 显存大小（MB）
//   - Node: 所属节点
//   - Status: 当前状态
//   - TaskID: 运行的任务ID（如果已分配）
//   - UsedMem: 已使用显存（MB）
//   - Util: 利用率（%）
//   - Temp: 温度（摄氏度）
type GPUDevice struct {
	ID       string   `json:"id" yaml:"id"`                 // GPU ID
	UUID     string   `json:"uuid" yaml:"uuid"`           // GPU UUID
	Model    string   `json:"model" yaml:"model"`        // 型号
	Memory   int      `json:"memory" yaml:"memory"`      // 显存 MB
	Node     string   `json:"node" yaml:"node"`           // 所属节点
	Status   GPUStatus `json:"status" yaml:"status"`      // 状态
	TaskID   string   `json:"task_id" yaml:"task_id"`   // 运行的任务ID
	UsedMem  int      `json:"used_mem" yaml:"used_mem"`  // 已用显存 MB
	Util     int      `json:"util" yaml:"util"`         // 利用率 %
	Temp     int      `json:"temp" yaml:"temp"`         // 温度 Celsius
}

// NewGPUDevice 创建新GPU设备的构造函数
//
// 参数:
//   - id: GPU ID
//   - uuid: GPU UUID
//   - model: GPU型号
//   - memory: 显存大小（MB）
//   - node: 所属节点
//
// 返回:
//   - *GPUDevice: 新创建的GPU设备对象，状态为Idle
func NewGPUDevice(id, uuid, model string, memory int, node string) *GPUDevice {
	return &GPUDevice{
		ID:      id,
		UUID:    uuid,
		Model:   model,
		Memory:  memory,
		Node:    node,
		Status:  GPUStatusIdle,
		UsedMem: 0,
		Util:    0,
		Temp:    0,
	}
}

// ============================================================================
// 工具函数
// ============================================================================

// generateID 生成唯一任务ID
// 格式: 时间戳(YYYYMMDDHHMMSS) + 随机字符串
// 例如: 20260308143015abc123
func generateID() string {
	// 使用时间戳确保大致有序
	timestamp := time.Now().Format("20060102150405")
	// 使用加密安全的随机数
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	return timestamp + randomHex
}

// generateIDLegacy 旧版ID生成方法（保留用于兼容性）
// 注意: 使用时间戳作为随机源不够安全，仅用于遗留兼容
func generateIDLegacy() string {
	return time.Now().Format("20060102150405") + randomString(6)
}

// randomString 生成指定长度的随机字符串
// 注意: 此函数使用时间戳作为随机源，不够安全
// 新代码应使用 crypto/rand
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
