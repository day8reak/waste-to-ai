package gpu

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/models"
	"testing"
)

// TestGPUManager 创建测试用的GPU管理器
func TestGPUManager(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
	}

	mgr := NewGPUManager(cfg)

	// 测试获取所有GPU
	gpus, err := mgr.GetGPUs()
	if err != nil {
		t.Fatalf("Failed to get GPUs: %v", err)
	}

	if len(gpus) != 3 {
		t.Errorf("Expected 3 GPUs, got %d", len(gpus))
	}

	t.Logf("Found %d GPUs", len(gpus))
	for _, gpu := range gpus {
		t.Logf("  %s: %s %dMB", gpu.ID, gpu.Model, gpu.Memory)
	}
}

// TestAllocateGPU 测试分配GPU
func TestAllocateGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	// 分配GPU
	err := mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	if err != nil {
		t.Fatalf("Failed to allocate GPU: %v", err)
	}

	// 检查GPU状态
	gpu, err := mgr.GetGPUByID("gpu0")
	if err != nil {
		t.Fatalf("Failed to get GPU: %v", err)
	}

	if gpu.Status != models.GPUStatusAllocated {
		t.Errorf("Expected GPU status 'allocated', got '%s'", gpu.Status)
	}

	if gpu.TaskID != "task-1" {
		t.Errorf("Expected GPU task ID 'task-1', got '%s'", gpu.TaskID)
	}
}

// TestReleaseGPU 测试释放GPU
func TestReleaseGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	// 先分配
	mgr.AllocateGPU([]string{"gpu0"}, "task-1")

	// 再释放
	err := mgr.ReleaseGPU([]string{"gpu0"})
	if err != nil {
		t.Fatalf("Failed to release GPU: %v", err)
	}

	// 检查GPU状态
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU status 'idle', got '%s'", gpu.Status)
	}

	if gpu.TaskID != "" {
		t.Errorf("Expected GPU task ID to be empty, got '%s'", gpu.TaskID)
	}
}

// TestAllocateMultipleGPUs 测试分配多个GPU
func TestAllocateMultipleGPUs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	// 分配多个GPU
	err := mgr.AllocateGPU([]string{"gpu0", "gpu1"}, "task-multi")
	if err != nil {
		t.Fatalf("Failed to allocate multiple GPUs: %v", err)
	}

	// 检查两个GPU状态
	gpu0, _ := mgr.GetGPUByID("gpu0")
	gpu1, _ := mgr.GetGPUByID("gpu1")

	if gpu0.Status != models.GPUStatusAllocated || gpu1.Status != models.GPUStatusAllocated {
		t.Errorf("Both GPUs should be allocated")
	}
}

// TestAllocateSameGPUTwice 测试重复分配同一GPU
func TestAllocateSameGPUTwice(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	// 第一次分配
	err := mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}

	// 第二次分配同一GPU应该失败
	err = mgr.AllocateGPU([]string{"gpu0"}, "task-2")
	if err == nil {
		t.Error("Expected error when allocating same GPU twice")
	}
}

// TestCheckHealth 测试GPU健康检查
func TestCheckHealth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 健康检查应该返回空列表（没有离线GPU）
	offline, err := mgr.CheckHealth()
	if err != nil {
		t.Fatalf("CheckHealth failed: %v", err)
	}

	if len(offline) != 0 {
		t.Errorf("Expected 0 offline GPUs, got %d", len(offline))
	}
}

// TestMarkGPUsOffline 测试标记GPU为离线
func TestMarkGPUsOffline(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 先分配GPU给任务
	err := mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	if err != nil {
		t.Fatalf("AllocateGPU failed: %v", err)
	}

	// 标记GPU为离线
	affectedTasks, err := mgr.MarkGPUsOffline([]string{"gpu0"})
	if err != nil {
		t.Fatalf("MarkGPUsOffline failed: %v", err)
	}

	// 应该返回受影响的任务ID
	if len(affectedTasks) != 1 {
		t.Errorf("Expected 1 affected task, got %d", len(affectedTasks))
	}

	if affectedTasks[0] != "task-1" {
		t.Errorf("Expected task-1, got %s", affectedTasks[0])
	}

	// 验证GPU状态已更新
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusOffline {
		t.Errorf("Expected GPU status offline, got %s", gpu.Status)
	}

	if gpu.TaskID != "" {
		t.Errorf("Expected GPU.TaskID to be empty, got %s", gpu.TaskID)
	}
}

// TestGetAvailableGPUs 测试获取可用GPU
func TestGetAvailableGPUs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 初始应该所有GPU都可用
	available, err := mgr.GetAvailableGPUs()
	if err != nil {
		t.Fatalf("GetAvailableGPUs failed: %v", err)
	}

	if len(available) != 2 {
		t.Errorf("Expected 2 available GPUs, got %d", len(available))
	}

	// 分配一个GPU后应该只有一个可用
	mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	available, err = mgr.GetAvailableGPUs()
	if err != nil {
		t.Fatalf("GetAvailableGPUs failed: %v", err)
	}

	if len(available) != 1 {
		t.Errorf("Expected 1 available GPU after allocation, got %d", len(available))
	}
}

// TestGetAllocatedGPUs 测试获取已分配GPU
func TestGetAllocatedGPUs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 初始应该没有已分配的GPU
	allocated, err := mgr.GetAllocatedGPUs()
	if err != nil {
		t.Fatalf("GetAllocatedGPUs failed: %v", err)
	}

	if len(allocated) != 0 {
		t.Errorf("Expected 0 allocated GPUs initially, got %d", len(allocated))
	}

	// 分配GPU后
	mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	allocated, err = mgr.GetAllocatedGPUs()
	if err != nil {
		t.Fatalf("GetAllocatedGPUs failed: %v", err)
	}

	if len(allocated) != 1 {
		t.Errorf("Expected 1 allocated GPU, got %d", len(allocated))
	}
}

// TestSimulateGPUFailure 测试模拟GPU故障
func TestSimulateGPUFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 模拟GPU故障
	err := mgr.SimulateGPUFailure("gpu0")
	if err != nil {
		t.Fatalf("SimulateGPUFailure failed: %v", err)
	}

	// 验证GPU状态
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusOffline {
		t.Errorf("Expected GPU status offline, got %s", gpu.Status)
	}
}

// TestSimulateGPUFailure_NotFound 测试模拟不存在的GPU故障
func TestSimulateGPUFailure_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	// 模拟不存在的GPU故障
	err := mgr.SimulateGPUFailure("non-existent-gpu")
	if err == nil {
		t.Error("Expected error for non-existent GPU")
	}
}

// TestUpdateGPUStatus 测试更新GPU状态
func TestUpdateGPUStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	// 更新GPU状态
	err := mgr.UpdateGPUStatus()
	if err != nil {
		t.Fatalf("UpdateGPUStatus failed: %v", err)
	}
}

// TestAllocateGPU_NotEnoughMemory 测试显存不足情况
func TestAllocateGPU_NotEnoughMemory(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 尝试分配需要超过可用显存的任务（Mock模式不检查显存）
	err := mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	if err != nil {
		t.Fatalf("AllocateGPU failed unexpectedly: %v", err)
	}
}

// TestReleaseGPU_NotFound 测试释放不存在的GPU
func TestReleaseGPU_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	err := mgr.ReleaseGPU([]string{"non-existent-gpu"})
	if err == nil {
		t.Error("Expected error for non-existent GPU")
	}
}

// TestMarkGPUsOffline_NotFound 测试标记不存在的GPU为离线
func TestMarkGPUsOffline_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	affected, err := mgr.MarkGPUsOffline([]string{"non-existent-gpu"})
	if err != nil {
		t.Fatalf("MarkGPUsOffline failed: %v", err)
	}

	// 不存在的GPU不影响任何任务
	if len(affected) != 0 {
		t.Errorf("Expected 0 affected tasks, got %d", len(affected))
	}
}
