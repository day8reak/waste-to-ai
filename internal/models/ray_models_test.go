package models

import (
	"testing"
)

// TestNewRayTask 测试创建 Ray 任务
func TestNewRayTask(t *testing.T) {
	task := NewRayTask(
		"ray-job-123",
		2,    // 需要 2 张 GPU
		"V100", // 只要 V100
		8,   // 高优先级
	)

	// Name 会被添加 "ray-" 前缀
	if task.Name != "ray-ray-job-123" {
		t.Errorf("期望名称 ray-ray-job-123, 实际为 %s", task.Name)
	}

	if task.RayJobID != "ray-job-123" {
		t.Errorf("期望 RayJobID ray-job-123, 实际为 %s", task.RayJobID)
	}

	if !task.IsRayTask {
		t.Error("IsRayTask 应为 true")
	}

	if task.GPURequired != 2 {
		t.Errorf("期望 GPU 数量 2, 实际为 %d", task.GPURequired)
	}

	if task.GPUModel != "V100" {
		t.Errorf("期望 GPU 型号 V100, 实际为 %s", task.GPUModel)
	}

	if task.Priority != 8 {
		t.Errorf("期望优先级 8, 实际为 %d", task.Priority)
	}

	if task.Status != TaskStatusPending {
		t.Errorf("期望状态 pending, 实际为 %s", task.Status)
	}

	if task.ID == "" {
		t.Error("任务 ID 不应为空")
	}

	if task.CreatedAt.IsZero() {
		t.Error("创建时间不应为零")
	}

	// 验证 Command 和 Image 为空
	if task.Command != "" {
		t.Errorf("Ray 任务 Command 应为空, 实际为 %s", task.Command)
	}

	if task.Image != "" {
		t.Errorf("Ray 任务 Image 应为空, 实际为 %s", task.Image)
	}
}

// TestNewRayTask_DefaultValues 测试 Ray 任务默认值
func TestNewRayTask_DefaultValues(t *testing.T) {
	// 使用 0 值创建任务
	task := NewRayTask("ray-job-default", 0, "", 0)

	// 验证 ID 不为空
	if task.ID == "" {
		t.Error("ID 应该自动生成")
	}

	// 验证 RayJobID
	if task.RayJobID != "ray-job-default" {
		t.Errorf("期望 RayJobID ray-job-default, 实际为 %s", task.RayJobID)
	}

	// 验证 IsRayTask
	if !task.IsRayTask {
		t.Error("IsRayTask 应为 true")
	}

	// GPU 数量应被调整为 1
	if task.GPURequired != 1 {
		t.Errorf("GPURequired 应被调整为 1, 实际为 %d", task.GPURequired)
	}

	// 优先级应被调整为 5
	if task.Priority != 5 {
		t.Errorf("Priority 应被调整为 5, 实际为 %d", task.Priority)
	}

	// 创建时间
	if task.CreatedAt.IsZero() {
		t.Error("创建时间应该自动设置")
	}
}

// TestGPUStatusBlocked 测试 GPU blocked 状态
func TestGPUStatusBlocked(t *testing.T) {
	tests := []struct {
		status   GPUStatus
		expected string
	}{
		{GPUStatusIdle, "idle"},
		{GPUStatusAllocated, "allocated"},
		{GPUStatusOffline, "offline"},
		{GPUStatusBlocked, "blocked"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("期望 %s, 实际为 %s", tt.expected, tt.status)
		}
	}
}

// TestTaskRayFields 测试任务的 Ray 相关字段
func TestTaskRayFields(t *testing.T) {
	task := NewRayTask("test-ray-job", 1, "", 5)

	// 验证 Ray 特定字段初始状态 - GPUAssigned 应为 nil 或空切片
	if task.GPUAssigned != nil && len(task.GPUAssigned) != 0 {
		t.Error("GPUAssigned 初始应为空切片或 nil")
	}

	if task.ContainerID != "" {
		t.Error("ContainerID 初始应为空")
	}

	if task.ErrorMsg != "" {
		t.Error("ErrorMsg 初始应为空")
	}

	if task.StartedAt != nil {
		t.Error("StartedAt 初始应为 nil")
	}

	if task.FinishedAt != nil {
		t.Error("FinishedAt 初始应为 nil")
	}
}

// TestNewTask_WithRayFields 测试普通任务也支持 Ray 字段
func TestNewTask_WithRayFields(t *testing.T) {
	task := NewTask("normal-task", "echo hello", "ubuntu:latest", 1, "", 5)

	// 普通任务的 Ray 字段应该为空/false
	if task.RayJobID != "" {
		t.Error("普通任务 RayJobID 应为空")
	}

	if task.IsRayTask != false {
		t.Error("普通任务 IsRayTask 应为 false")
	}
}
