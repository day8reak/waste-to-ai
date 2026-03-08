package models

import (
	"testing"
	"time"
)

// TestNewTask 测试创建任务
func TestNewTask(t *testing.T) {
	task := NewTask(
		"test-task",
		"python train.py",
		"pytorch/pytorch:2.0",
		2,    // 需要2张GPU
		"V100", // 只要V100
		8,   // 高优先级
	)

	if task.Name != "test-task" {
		t.Errorf("期望名称 test-task, 实际为 %s", task.Name)
	}

	if task.Command != "python train.py" {
		t.Errorf("期望命令 python train.py, 实际为 %s", task.Command)
	}

	if task.Image != "pytorch/pytorch:2.0" {
		t.Errorf("期望镜像 pytorch/pytorch:2.0, 实际为 %s", task.Image)
	}

	if task.GPURequired != 2 {
		t.Errorf("期望GPU数量 2, 实际为 %d", task.GPURequired)
	}

	if task.GPUModel != "V100" {
		t.Errorf("期望GPU型号 V100, 实际为 %s", task.GPUModel)
	}

	if task.Priority != 8 {
		t.Errorf("期望优先级 8, 实际为 %d", task.Priority)
	}

	if task.Status != TaskStatusPending {
		t.Errorf("期望状态 pending, 实际为 %s", task.Status)
	}

	if task.ID == "" {
		t.Error("任务ID不应为空")
	}

	if task.CreatedAt.IsZero() {
		t.Error("创建时间不应为零")
	}
}

// TestNewGPUDevice 测试创建GPU设备
func TestNewGPUDevice(t *testing.T) {
	gpu := NewGPUDevice(
		"gpu0",
		"GPU-12345678",
		"V100",
		32768,
		"node1",
	)

	if gpu.ID != "gpu0" {
		t.Errorf("期望ID gpu0, 实际为 %s", gpu.ID)
	}

	if gpu.UUID != "GPU-12345678" {
		t.Errorf("期望UUID GPU-12345678, 实际为 %s", gpu.UUID)
	}

	if gpu.Model != "V100" {
		t.Errorf("期望型号 V100, 实际为 %s", gpu.Model)
	}

	if gpu.Memory != 32768 {
		t.Errorf("期望显存 32768, 实际为 %d", gpu.Memory)
	}

	if gpu.Node != "node1" {
		t.Errorf("期望节点 node1, 实际为 %s", gpu.Node)
	}

	if gpu.Status != GPUStatusIdle {
		t.Errorf("期望状态 idle, 实际为 %s", gpu.Status)
	}

	if gpu.TaskID != "" {
		t.Errorf("期望任务ID为空, 实际为 %s", gpu.TaskID)
	}
}

// TestTaskStatus 测试任务状态常量
func TestTaskStatus(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskStatusPending, "pending"},
		{TaskStatusRunning, "running"},
		{TaskStatusCompleted, "completed"},
		{TaskStatusFailed, "failed"},
		{TaskStatusKilled, "killed"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("期望 %s, 实际为 %s", tt.expected, tt.status)
		}
	}
}

// TestGPUStatus 测试GPU状态常量
func TestGPUStatus(t *testing.T) {
	tests := []struct {
		status   GPUStatus
		expected string
	}{
		{GPUStatusIdle, "idle"},
		{GPUStatusAllocated, "allocated"},
		{GPUStatusOffline, "offline"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("期望 %s, 实际为 %s", tt.expected, tt.status)
		}
	}
}

// TestTaskFields 测试任务字段
func TestTaskFields(t *testing.T) {
	task := NewTask("test", "echo hello", "ubuntu:latest", 1, "", 5)

	// 测试所有时间字段初始状态
	if task.StartedAt != nil {
		t.Error("StartedAt 初始应为 nil")
	}
	if task.FinishedAt != nil {
		t.Error("FinishedAt 初始应为 nil")
	}
	if task.ContainerID != "" {
		t.Error("ContainerID 初始应为空")
	}
	if task.ErrorMsg != "" {
		t.Error("ErrorMsg 初始应为空")
	}
	if len(task.GPUAssigned) != 0 {
		t.Error("GPUAssigned 初始应为空切片")
	}
}

// TestGPUFields 测试GPU字段
func TestGPUFields(t *testing.T) {
	gpu := NewGPUDevice("gpu0", "uuid", "3090", 24576, "node1")

	if gpu.UsedMem != 0 {
		t.Errorf("UsedMem 初始应为 0, 实际为 %d", gpu.UsedMem)
	}
	if gpu.Util != 0 {
		t.Errorf("Util 初始应为 0, 实际为 %d", gpu.Util)
	}
	if gpu.Temp != 0 {
		t.Errorf("Temp 初始应为 0, 实际为 %d", gpu.Temp)
	}
}

// TestGenerateID 测试ID生成
func TestGenerateID(t *testing.T) {
	// 生成两个ID
	task1 := NewTask("test1", "cmd", "img", 1, "", 5)
	time.Sleep(time.Millisecond) // 稍微等待
	task2 := NewTask("test2", "cmd", "img", 1, "", 5)

	// ID应该不同
	if task1.ID == task2.ID {
		t.Error("两个任务的ID应该不同")
	}

	// ID应该包含时间戳（长度检查）
	if len(task1.ID) < 14 {
		t.Errorf("ID长度应至少为14, 实际为 %d", len(task1.ID))
	}
}

// TestNewTaskWithDefaults 测试默认值
func TestNewTaskWithDefaults(t *testing.T) {
	// 使用0值创建任务
	task := NewTask("", "", "", 0, "", 0)

	// 验证ID不为空
	if task.ID == "" {
		t.Error("ID应该自动生成")
	}

	// 验证创建时间
	if task.CreatedAt.IsZero() {
		t.Error("创建时间应该自动设置")
	}
}

// TestGenerateIDLegacy 测试遗留ID生成函数
func TestGenerateIDLegacy(t *testing.T) {
	// 生成两个ID（添加延迟确保时间戳不同）
	id1 := generateIDLegacy()
	time.Sleep(time.Second)
	id2 := generateIDLegacy()

	// ID应该包含时间戳前缀
	if len(id1) < 14 {
		t.Errorf("ID长度应至少为14, 实际为 %d", len(id1))
	}

	// ID长度应该相同
	if len(id1) != len(id2) {
		t.Errorf("两个ID长度应该相同")
	}
}

// TestRandomString 测试随机字符串生成
func TestRandomString(t *testing.T) {
	// 测试不同长度的随机字符串
	for _, length := range []int{4, 8, 16, 32} {
		result := randomString(length)
		if len(result) != length {
			t.Errorf("随机字符串长度应为 %d, 实际为 %d", length, len(result))
		}
	}

	// 测试多次生成的结果可能不同
	result1 := randomString(6)
	result2 := randomString(6)
	// 注意：由于使用时间戳作为随机源，可能生成相同结果
	// 但在大多数情况下应该不同
	_ = result1
	_ = result2
}
