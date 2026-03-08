package gpu

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/models"
	"testing"
)

// TestBlockGPU 测试阻塞 GPU
func TestBlockGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 阻塞 GPU
	err := mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 验证 GPU 状态
	gpu, err := mgr.GetGPUByID("gpu0")
	if err != nil {
		t.Fatalf("GetGPUByID failed: %v", err)
	}

	if gpu.Status != models.GPUStatusBlocked {
		t.Errorf("Expected GPU status 'blocked', got '%s'", gpu.Status)
	}
}

// TestBlockGPU_NotFound 测试阻塞不存在的 GPU
func TestBlockGPU_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	err := mgr.BlockGPU("non-existent-gpu")
	if err == nil {
		t.Error("Expected error for non-existent GPU")
	}
}

// TestBlockAllocatedGPU 测试阻塞已分配的 GPU
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

	// 阻塞已分配的 GPU
	err = mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 验证 GPU 状态
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusBlocked {
		t.Errorf("Expected GPU status 'blocked', got '%s'", gpu.Status)
	}

	// TaskID 应该保留（记录之前属于哪个任务）
	if gpu.TaskID != "task-1" {
		t.Errorf("Expected TaskID 'task-1', got '%s'", gpu.TaskID)
	}
}

// TestUnblockGPU 测试解除阻塞
func TestUnblockGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 先阻塞 GPU
	err := mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 解除阻塞
	err = mgr.UnblockGPU("gpu0")
	if err != nil {
		t.Fatalf("UnblockGPU failed: %v", err)
	}

	// 验证 GPU 状态
	gpu, err := mgr.GetGPUByID("gpu0")
	if err != nil {
		t.Fatalf("GetGPUByID failed: %v", err)
	}

	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU status 'idle', got '%s'", gpu.Status)
	}

	// TaskID 应该被清空
	if gpu.TaskID != "" {
		t.Errorf("Expected TaskID to be empty, got '%s'", gpu.TaskID)
	}
}

// TestUnblockGPU_NotBlocked 测试解除未阻塞的 GPU
func TestUnblockGPU_NotBlocked(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 尝试解除未阻塞的 GPU 应该失败
	err := mgr.UnblockGPU("gpu0")
	if err == nil {
		t.Error("Expected error when unblocking non-blocked GPU")
	}
}

// TestUnblockGPU_NotFound 测试解除不存在的 GPU
func TestUnblockGPU_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	mgr := NewGPUManager(cfg)

	err := mgr.UnblockGPU("non-existent-gpu")
	if err == nil {
		t.Error("Expected error for non-existent GPU")
	}
}

// TestGetBlockedGPUs 测试获取被阻塞的 GPU 列表
func TestGetBlockedGPUs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
	}

	mgr := NewGPUManager(cfg)

	// 初始应该没有阻塞的 GPU
	blocked, err := mgr.GetBlockedGPUs()
	if err != nil {
		t.Fatalf("GetBlockedGPUs failed: %v", err)
	}

	if len(blocked) != 0 {
		t.Errorf("Expected 0 blocked GPUs initially, got %d", len(blocked))
	}

	// 阻塞一些 GPU
	mgr.BlockGPU("gpu0")
	mgr.BlockGPU("gpu2")

	// 验证阻塞列表
	blocked, err = mgr.GetBlockedGPUs()
	if err != nil {
		t.Fatalf("GetBlockedGPUs failed: %v", err)
	}

	if len(blocked) != 2 {
		t.Errorf("Expected 2 blocked GPUs, got %d", len(blocked))
	}

	// 验证具体 GPU
	ids := make(map[string]bool)
	for _, gpu := range blocked {
		ids[gpu.ID] = true
	}

	if !ids["gpu0"] {
		t.Error("gpu0 should be blocked")
	}

	if !ids["gpu2"] {
		t.Error("gpu2 should be blocked")
	}
}

// TestGetAvailableGPUs_ExcludesBlocked 测试获取可用 GPU 时排除阻塞的
func TestGetAvailableGPUs_ExcludesBlocked(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 初始所有 GPU 都可用
	available, _ := mgr.GetAvailableGPUs()
	if len(available) != 2 {
		t.Errorf("Expected 2 available GPUs, got %d", len(available))
	}

	// 阻塞一个 GPU
	mgr.BlockGPU("gpu0")

	// 现在应该只有一个可用
	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 1 {
		t.Errorf("Expected 1 available GPU after blocking, got %d", len(available))
	}

	// 验证可用的是 gpu1
	if available[0].ID != "gpu1" {
		t.Errorf("Expected gpu1 to be available, got %s", available[0].ID)
	}
}

// TestBlockAlreadyBlockedGPU 测试重复阻塞
func TestBlockAlreadyBlockedGPU(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 第一次阻塞
	err := mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("First BlockGPU failed: %v", err)
	}

	// 第二次阻塞同一 GPU 应该成功（幂等）
	err = mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("Second BlockGPU failed: %v", err)
	}

	// 验证状态仍然是 blocked
	gpu, _ := mgr.GetGPUByID("gpu0")
	if gpu.Status != models.GPUStatusBlocked {
		t.Errorf("Expected GPU status 'blocked', got '%s'", gpu.Status)
	}
}

// TestBlockAndUnblockWorkflow 测试完整的阻塞/解除工作流
func TestBlockAndUnblockWorkflow(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	mgr := NewGPUManager(cfg)

	// 1. 初始状态：GPU 可用
	available, _ := mgr.GetAvailableGPUs()
	if len(available) != 1 {
		t.Fatalf("Expected 1 available GPU, got %d", len(available))
	}

	// 2. 阻塞 GPU
	err := mgr.BlockGPU("gpu0")
	if err != nil {
		t.Fatalf("BlockGPU failed: %v", err)
	}

	// 3. 验证 GPU 不可用
	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 0 {
		t.Errorf("Expected 0 available GPUs after block, got %d", len(available))
	}

	// 4. 验证 GPU 在阻塞列表
	blocked, _ := mgr.GetBlockedGPUs()
	if len(blocked) != 1 {
		t.Errorf("Expected 1 blocked GPU, got %d", len(blocked))
	}

	// 5. 解除阻塞
	err = mgr.UnblockGPU("gpu0")
	if err != nil {
		t.Fatalf("UnblockGPU failed: %v", err)
	}

	// 6. 验证 GPU 恢复可用
	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 1 {
		t.Errorf("Expected 1 available GPU after unblock, got %d", len(available))
	}

	// 7. 验证阻塞列表为空
	blocked, _ = mgr.GetBlockedGPUs()
	if len(blocked) != 0 {
		t.Errorf("Expected 0 blocked GPUs after unblock, got %d", len(blocked))
	}
}

// TestMultipleBlockAndUnblock 测试多次阻塞/解除
func TestMultipleBlockAndUnblock(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
	}

	mgr := NewGPUManager(cfg)

	// 阻塞 gpu0, gpu1
	mgr.BlockGPU("gpu0")
	mgr.BlockGPU("gpu1")

	// 验证可用 GPU
	available, _ := mgr.GetAvailableGPUs()
	if len(available) != 1 {
		t.Errorf("Expected 1 available GPU, got %d", len(available))
	}

	// 解除 gpu0
	mgr.UnblockGPU("gpu0")

	// 验证可用 GPU
	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 2 {
		t.Errorf("Expected 2 available GPUs, got %d", len(available))
	}

	// 再次阻塞 gpu0
	mgr.BlockGPU("gpu0")

	// 验证可用 GPU
	available, _ = mgr.GetAvailableGPUs()
	if len(available) != 1 {
		t.Errorf("Expected 1 available GPU, got %d", len(available))
	}
}
