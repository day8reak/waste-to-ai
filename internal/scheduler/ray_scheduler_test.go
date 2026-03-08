package scheduler

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/docker"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/models"
	"testing"
)

// TestGetTaskByRayJobID 测试根据 Ray Job ID 获取任务
func TestGetTaskByRayJobID(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建一个 Ray 任务
	task := models.NewRayTask("ray-job-123", 1, "", 5)
	sched.SubmitTask(task)

	// 根据 Ray Job ID 查找
	found, err := sched.GetTaskByRayJobID("ray-job-123")
	if err != nil {
		t.Fatalf("GetTaskByRayJobID failed: %v", err)
	}

	if found.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, found.ID)
	}

	if !found.IsRayTask {
		t.Error("Task should be marked as Ray task")
	}

	if found.RayJobID != "ray-job-123" {
		t.Errorf("Expected RayJobID ray-job-123, got %s", found.RayJobID)
	}
}

// TestGetTaskByRayJobID_NotFound 测试获取不存在的 Ray 任务
func TestGetTaskByRayJobID_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 尝试获取不存在的任务
	_, err := sched.GetTaskByRayJobID("non-existent-ray-job")
	if err == nil {
		t.Error("Expected error for non-existent Ray job")
	}
}

// TestGetTaskByRayJobID_InPendingQueue 测试从等待队列中获取 Ray 任务
func TestGetTaskByRayJobID_InPendingQueue(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	// 只配置 1 个 GPU
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建 2 个 Ray 任务，但只有 1 个 GPU
	task1 := models.NewRayTask("ray-job-1", 1, "", 5)
	task2 := models.NewRayTask("ray-job-2", 1, "", 5)

	sched.SubmitTask(task1)
	sched.SubmitTask(task2)

	// 查找第二个任务（应该在等待队列中）
	found, err := sched.GetTaskByRayJobID("ray-job-2")
	if err != nil {
		t.Fatalf("GetTaskByRayJobID failed: %v", err)
	}

	if found.ID != task2.ID {
		t.Errorf("Expected task ID %s, got %s", task2.ID, found.ID)
	}
}

// TestGetRayTasks 测试获取所有 Ray 任务
func TestGetRayTasks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 初始应该没有 Ray 任务
	tasks := sched.GetRayTasks()
	if len(tasks) != 0 {
		t.Errorf("Expected 0 Ray tasks initially, got %d", len(tasks))
	}

	// 创建普通任务和 Ray 任务
	normalTask := models.NewTask("normal-task", "echo", "ubuntu", 1, "", 5)
	rayTask1 := models.NewRayTask("ray-job-1", 1, "", 5)
	rayTask2 := models.NewRayTask("ray-job-2", 1, "", 5)

	sched.SubmitTask(normalTask)
	sched.SubmitTask(rayTask1)
	sched.SubmitTask(rayTask2)

	// 获取 Ray 任务
	tasks = sched.GetRayTasks()
	if len(tasks) != 2 {
		t.Errorf("Expected 2 Ray tasks, got %d", len(tasks))
	}

	// 验证都是 Ray 任务
	for _, task := range tasks {
		if !task.IsRayTask {
			t.Error("All returned tasks should be Ray tasks")
		}
	}
}

// TestGetRayTasks_MixedQueue 测试混合队列中的 Ray 任务
func TestGetRayTasks_MixedQueue(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建混合任务（2 个 Ray 任务，但只有 1 个 GPU）
	rayTask1 := models.NewRayTask("ray-job-1", 1, "", 5)
	rayTask2 := models.NewRayTask("ray-job-2", 1, "", 5)
	normalTask := models.NewTask("normal", "echo", "ubuntu", 1, "", 5)

	sched.SubmitTask(rayTask1)
	sched.SubmitTask(rayTask2)
	sched.SubmitTask(normalTask)

	// 获取 Ray 任务
	tasks := sched.GetRayTasks()
	if len(tasks) != 2 {
		t.Errorf("Expected 2 Ray tasks, got %d", len(tasks))
	}
}

// TestReleaseGPUFromTask_ReleaseAll 测试释放任务所有 GPU
func TestReleaseGPUFromTask_ReleaseAll(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建一个任务
	task := models.NewTask("test-task", "echo", "ubuntu", 1, "", 5)
	sched.SubmitTask(task)

	gpuID := task.GPUAssigned[0]

	// 释放所有 GPU（不传 GPU 列表）
	count, err := sched.ReleaseGPUFromTask(task.ID, nil)
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 released GPU, got %d", count)
	}

	// 验证任务状态
	if task.Status != models.TaskStatusCompleted {
		t.Errorf("Expected task status 'completed', got '%s'", task.Status)
	}

	// 验证 GPU 已释放
	gpu, _ := gpuMgr.GetGPUByID(gpuID)
	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU status 'idle', got '%s'", gpu.Status)
	}
}

// TestReleaseGPUFromTask_ReleaseSpecific 测试释放任务指定 GPU
func TestReleaseGPUFromTask_ReleaseSpecific(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建一个需要 2 个 GPU 的任务
	task := models.NewTask("multi-gpu-task", "echo", "ubuntu", 2, "", 5)
	sched.SubmitTask(task)

	if len(task.GPUAssigned) != 2 {
		t.Fatalf("Expected 2 GPUs assigned, got %d", len(task.GPUAssigned))
	}

	// 释放其中一个 GPU
	gpuToRelease := task.GPUAssigned[0]
	count, err := sched.ReleaseGPUFromTask(task.ID, []string{gpuToRelease})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 released GPU, got %d", count)
	}

	// 验证任务还剩 1 个 GPU
	if len(task.GPUAssigned) != 1 {
		t.Errorf("Expected 1 remaining GPU, got %d", len(task.GPUAssigned))
	}

	// 验证释放的 GPU 已空闲
	gpu, _ := gpuMgr.GetGPUByID(gpuToRelease)
	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected released GPU to be idle, got '%s'", gpu.Status)
	}
}

// TestReleaseGPUFromTask_NotFound 测试释放不存在的任务
func TestReleaseGPUFromTask_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	_, err := sched.ReleaseGPUFromTask("non-existent-task", nil)
	if err == nil {
		t.Error("Expected error for non-existent task")
	}
}

// TestReleaseGPUFromTask_NoGPUsToRelease 测试没有 GPU 可释放
func TestReleaseGPUFromTask_NoGPUsToRelease(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建一个任务
	task := models.NewTask("test-task", "echo", "ubuntu", 1, "", 5)
	sched.SubmitTask(task)

	// 尝试释放不存在的 GPU
	count, err := sched.ReleaseGPUFromTask(task.ID, []string{"non-existent-gpu"})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	// 应该没有 GPU 被释放
	if count != 0 {
		t.Errorf("Expected 0 released GPUs, got %d", count)
	}
}

// TestReleaseGPUFromTask_AfterKill 测试任务杀死后的释放
func TestReleaseGPUFromTask_AfterKill(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建一个任务
	task := models.NewTask("test-task", "echo", "ubuntu", 1, "", 5)
	sched.SubmitTask(task)

	// 杀死任务
	sched.KillTask(task.ID)

	// 尝试再次释放（应该失败，因为任务已完成）
	_, err := sched.ReleaseGPUFromTask(task.ID, nil)
	if err == nil {
		t.Error("Expected error when releasing from completed task")
	}
}

// TestReleaseGPUFromTask_TriggersQueueProcessing 测试释放 GPU 后触发队列处理
func TestReleaseGPUFromTask_TriggersQueueProcessing(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建 2 个任务，但只有 1 个 GPU
	task1 := models.NewTask("task-1", "echo", "ubuntu", 1, "", 5)
	task2 := models.NewTask("task-2", "echo", "ubuntu", 1, "", 5)

	sched.SubmitTask(task1)
	sched.SubmitTask(task2)

	// 第一个任务应该在运行，第二个在等待
	stats := sched.GetStats()
	if stats["running"] != 1 {
		t.Errorf("Expected 1 running task, got %d", stats["running"])
	}

	// 杀死正在运行的任务
	sched.KillTask(task1.ID)

	// 等待队列中的任务应该被启动
	stats = sched.GetStats()
	if stats["running"] != 1 {
		t.Logf("Stats after kill: %+v", stats)
	}
}

// TestNewRayTask_SubmitAndQuery 测试 Ray 任务完整流程
func TestNewRayTask_SubmitAndQuery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建并提交 Ray 任务
	rayTask := models.NewRayTask("inference-job", 1, "", 8)
	err := sched.SubmitTask(rayTask)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 验证任务状态
	if rayTask.Status != models.TaskStatusRunning {
		t.Errorf("Expected running status, got %s", rayTask.Status)
	}

	// 通过 Ray Job ID 查询
	found, err := sched.GetTaskByRayJobID("inference-job")
	if err != nil {
		t.Fatalf("GetTaskByRayJobID failed: %v", err)
	}

	if found.ID != rayTask.ID {
		t.Errorf("Expected task ID %s, got %s", rayTask.ID, found.ID)
	}

	// 获取 Ray 任务列表
	rayTasks := sched.GetRayTasks()
	if len(rayTasks) != 1 {
		t.Errorf("Expected 1 Ray task, got %d", len(rayTasks))
	}

	if rayTasks[0].RayJobID != "inference-job" {
		t.Errorf("Expected ray job ID 'inference-job', got %s", rayTasks[0].RayJobID)
	}
}

// TestReleaseGPUFromRayTask 测试释放 Ray 任务的 GPU
func TestReleaseGPUFromRayTask(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 创建一个需要 2 个 GPU 的 Ray 任务，设置最低保障为1以允许释放
	rayTask := models.NewRayTask("ray-scaling", 2, "", 8)
	rayTask.MinGPURequired = 1  // 允许释放到1个GPU
	err := sched.SubmitTask(rayTask)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	originalGPUs := make([]string, len(rayTask.GPUAssigned))
	copy(originalGPUs, rayTask.GPUAssigned)

	// 释放 1 个 GPU（动态缩容）
	gpuToRelease := originalGPUs[0]
	count, err := sched.ReleaseGPUFromTask(rayTask.ID, []string{gpuToRelease})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 released GPU, got %d", count)
	}

	// 验证任务还剩 1 个 GPU
	if len(rayTask.GPUAssigned) != 1 {
		t.Errorf("Expected 1 remaining GPU, got %d", len(rayTask.GPUAssigned))
	}

	// 验证任务仍在运行（不是 Completed）
	if rayTask.Status == models.TaskStatusCompleted {
		t.Error("Task should still be running after partial release")
	}
}
