package gpu

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/models"
	"testing"
)

// TestBlockGPU 测试释放 GPU（使其变为idle）
func TestBlockGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// BlockGPU 现在是释放GPU（使其变为idle）
	err := mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 验证 GPU 状态变为 idle
	gpu, err := mgr.GetGPUByID("gpu0")
	if err != nil {
		t.Fatalf("GetGPUByID failed: %v", err)
	}

	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU status 'idle', got '%s'", gpu.Status)
	}
}

// TestBlockGPU_NotFound 测试释放不存在的 GPU
func TestBlockGPU_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	err := mgr.BlockGPU("non-existent-gpu")
	if err == nil {
		t.Error("Expected error for non-existent GPU")
	}
}

// TestBlockAllocatedGPU 测试释放已分配的 GPU
func TestBlockAllocatedGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 先分配 GPU
	err := mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	if err != nil {
		t.Fatalf("AllocateGPU failed: %v", err)
	}

	// 释放已分配的 GPU
	err = mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 验证 GPU 状态变为 idle
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU status 'idle', got '%s'", gpu.Status)
	}

	// TaskID 应该被清空（GPU已释放）
	if gpu.TaskID != "" {
		t.Errorf("Expected TaskID to be empty, got '%s'", gpu.TaskID)
	}
}

// TestBlockReleasedGPU 测试释放已空闲的GPU（幂等操作）
func TestBlockReleasedGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// GPU初始是idle，再次释放应该成功（幂等）
	err := mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 验证仍然是 idle
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU status 'idle', got '%s'", gpu.Status)
	}
}

// TestGetAvailableGPUs_AfterBlock 测试释放GPU后可用GPU数量增加
func TestGetAvailableGPUs_AfterBlock(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 初始所有GPU都idle
	available, _ := mgr.GetAvailableGPUs()
	if len(available) != 2 {
		t.Errorf("Expected 2 available GPUs initially, got %d", len(available))
	}

	// 分配一个GPU
	mgr.AllocateGPU([]string{"gpu0"}, "task-1")

	// 现在应该只有一个可用
	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 1 {
		t.Errorf("Expected 1 available GPU after allocation, got %d", len(available))
	}

	// 释放GPU后，应该又变成2个可用
	mgr.BlockGPU("gpu0")

	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 2 {
		t.Errorf("Expected 2 available GPUs after block, got %d", len(available))
	}

	// 验证可用的是gpu1和gpu0
	ids := make(map[string]bool)
	for _, gpu := range available {
		ids[gpu.ID] = true
	}

	if !ids["gpu0"] {
		t.Error("gpu0 should be available after block")
	}
	if !ids["gpu1"] {
		t.Error("gpu1 should be available")
	}
}

// TestBlockMultipleGPUs 测试释放多个GPU
func TestBlockMultipleGPUs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
	}

	mgr := NewGPUManager(cfg)

	// 分配多个GPU
	mgr.AllocateGPU([]string{"gpu0", "gpu1", "gpu2"}, "task-1")

	// 释放其中的gpu0和gpu1
	mgr.BlockGPU("gpu0")
	mgr.BlockGPU("gpu1")

	// 验证gpu0和gpu1变为idle
	gpu0, _ := mgr.GetGPUByID("gpu0")
	gpu1, _ := mgr.GetGPUByID("gpu1")
	gpu2, _ := mgr.GetGPUByID("gpu2")

	if gpu0.Status != models.GPUStatusIdle {
		t.Errorf("gpu0 should be idle, got %s", gpu0.Status)
	}
	if gpu1.Status != models.GPUStatusIdle {
		t.Errorf("gpu1 should be idle, got %s", gpu1.Status)
	}
	if gpu2.Status != models.GPUStatusAllocated {
		t.Errorf("gpu2 should still be allocated, got %s", gpu2.Status)
	}

	// 验证可用GPU列表
	available, _ := mgr.GetAvailableGPUs()
	if len(available) != 2 {
		t.Errorf("Expected 2 available GPUs, got %d", len(available))
	}
}

// TestBlockGPU_ClearResources 测试释放GPU后资源被清除
func TestBlockGPU_ClearResources(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 分配GPU并设置一些资源使用
	mgr.AllocateGPU([]string{"gpu0"}, "task-1")
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.UsedMem == 0 {
		gpu.UsedMem = 10000 // 模拟占用显存
		gpu.Util = 80 // 模拟利用率
	}

	// 释放GPU
	mgr.BlockGPU("gpu0")

	// 验证资源被清除
	gpu, _ = mgr.GetGPUByID("gpu0")
	if gpu.UsedMem != 0 {
		t.Errorf("Expected UsedMem to be 0, got %d", gpu.UsedMem)
	}
	if gpu.Util != 0 {
		t.Errorf("Expected Util to be 0, got %d", gpu.Util)
	}
	if gpu.TaskID != "" {
		t.Errorf("Expected TaskID to be empty, got %s", gpu.TaskID)
	}
}
