package scheduler

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/docker"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/models"
	"testing"
)

// TestSchedulerEdgeCases 测试调度器边界情况
func TestSchedulerEdgeCases(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 测试空名称任务
	task := models.NewTask("", "echo hello", "ubuntu:latest", 1, "", 5)
	err := sched.SubmitTask(task)
	if err != nil {
		t.Logf("Empty name task error: %v", err)
	}

	// 测试超长命令
	longCmd := ""
	for i := 0; i < 10000; i++ {
		longCmd += "a"
	}
	task2 := models.NewTask("long-cmd", longCmd, "ubuntu:latest", 1, "", 5)
	err = sched.SubmitTask(task2)
	if err != nil {
		t.Logf("Long command task error: %v", err)
	}
}

// TestSchedulerGPUSelection 测试GPU选择逻辑
func TestSchedulerGPUSelection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 请求V100
	task1 := models.NewTask("v100", "echo", "ubuntu:latest", 2, "V100", 5)
	sched.SubmitTask(task1)

	// 请求3090
	task2 := models.NewTask("3090", "echo", "ubuntu:latest", 1, "3090", 5)
	sched.SubmitTask(task2)

	// 请求任意型号
	task3 := models.NewTask("any", "echo", "ubuntu:latest", 1, "", 5)
	sched.SubmitTask(task3)
}

// TestSchedulerPriorityQueuing 测试优先级队列
func TestSchedulerPriorityQueuing(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交低优先级任务
	sched.SubmitTask(models.NewTask("low", "echo", "ubuntu:latest", 1, "", 1))

	// 提交高优先级任务
	sched.SubmitTask(models.NewTask("high", "echo", "ubuntu:latest", 1, "", 10))

	stats := sched.GetStats()
	if stats["running"] > 0 {
		t.Logf("Running tasks: %d", stats["running"])
	}
}

// TestSchedulerTaskLifecycle 测试任务生命周期
func TestSchedulerTaskLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交 -> 运行 -> 完成
	task := models.NewTask("lifecycle", "echo hello", "ubuntu:latest", 1, "", 5)
	sched.SubmitTask(task)

	// 验证状态
	if task.Status != models.TaskStatusRunning && task.Status != models.TaskStatusPending {
		t.Errorf("Unexpected status: %s", task.Status)
	}

	// 杀死
	sched.KillTask(task.ID)

	// 验证最终状态
	t.Logf("Task final status: %s", task.Status)
}

// TestSchedulerGPUExhaustion 测试GPU耗尽
func TestSchedulerGPUExhaustion(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 耗尽所有GPU
	for i := 0; i < 10; i++ {
		task := models.NewTask("exhaust", "echo", "ubuntu:latest", 1, "", 5)
		err := sched.SubmitTask(task)
		if err != nil {
			t.Logf("Task %d queued: %v", i, err)
		}
	}

	stats := sched.GetStats()
	t.Logf("Stats after exhaustion: running=%d, pending=%d", stats["running"], stats["pending"])
}

// TestSchedulerPreemption 测试抢占
func TestSchedulerPreemption(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}
	cfg.PreemptEnabled = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交低优先级任务
	lowTask := models.NewTask("low-prio", "echo", "ubuntu:latest", 1, "", 1)
	sched.SubmitTask(lowTask)

	// 提交高优先级任务
	highTask := models.NewTask("high-prio", "echo", "ubuntu:latest", 1, "", 10)
	sched.SubmitTask(highTask)

	t.Logf("Low task status: %s", lowTask.Status)
	t.Logf("High task status: %s", highTask.Status)
}

// TestSchedulerMixedWorkload 测试混合工作负载
func TestSchedulerMixedWorkload(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
		{ID: "gpu3", Model: "4090", Memory: 24576, Node: "node3"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交多种任务
	tasks := []struct {
		name  string
		gpus  int
		model string
		prio  int
	}{
		{"small-v100", 1, "V100", 5},
		{"small-3090", 1, "3090", 3},
		{"multi-v100", 2, "V100", 7},
		{"any-model", 1, "", 4},
		{"large", 4, "", 2},
	}

	for _, tt := range tasks {
		task := models.NewTask(tt.name, "echo", "ubuntu:latest", tt.gpus, tt.model, tt.prio)
		err := sched.SubmitTask(task)
		if err != nil {
			t.Logf("Task %s: %v", tt.name, err)
		}
	}

	stats := sched.GetStats()
	t.Logf("Mixed workload stats: %+v", stats)
}

// TestSchedulerGPUMemory 测试GPU内存考虑
func TestSchedulerGPUMemory(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 测试任务提交
	task := models.NewTask("mem-test", "echo", "ubuntu:latest", 1, "", 5)
	err := sched.SubmitTask(task)

	if err != nil {
		t.Logf("Submit error: %v", err)
	}
}

// TestSchedulerNodeAffinity 测试节点亲和性
func TestSchedulerNodeAffinity(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
		{ID: "gpu3", Model: "3090", Memory: 24576, Node: "node2"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交多任务验证调度
	for i := 0; i < 4; i++ {
		task := models.NewTask("node-test", "echo", "ubuntu:latest", 1, "", 5)
		sched.SubmitTask(task)
	}
}

// TestSchedulerFailureRecovery 测试故障恢复
func TestSchedulerFailureRecovery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交任务
	task := models.NewTask("recovery-test", "echo", "ubuntu:latest", 1, "", 5)
	sched.SubmitTask(task)

	// 模拟GPU故障
	gpuMgr.SimulateGPUFailure(task.GPUAssigned[0])

	// 恢复检查
	sched.CheckAndRecoverFromFailures()

	t.Logf("Task status after recovery: %s", task.Status)
}

// TestSchedulerBatchSubmission 测试批量提交
func TestSchedulerBatchSubmission(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
		{ID: "gpu3", Model: "4090", Memory: 24576, Node: "node3"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 批量提交20个任务
	for i := 0; i < 20; i++ {
		task := models.NewTask("batch", "echo", "ubuntu:latest", 1, "", 5)
		err := sched.SubmitTask(task)
		if err != nil {
			t.Logf("Batch task %d queued: %v", i, err)
		}
	}

	stats := sched.GetStats()
	t.Logf("Batch submission stats: running=%d, pending=%d", stats["running"], stats["pending"])
}

// TestSchedulerQueueOrdering 测试队列顺序
func TestSchedulerQueueOrdering(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 按不同优先级提交
	priorities := []int{3, 1, 5, 2, 4}
	for i, p := range priorities {
		task := models.NewTask("ordering", "echo", "ubuntu:latest", 1, "", p)
		err := sched.SubmitTask(task)
		if err != nil {
			t.Logf("Task %d (prio %d): %v", i, p, err)
		}
	}

	tasks := sched.GetTasks("pending")
	t.Logf("Pending tasks count: %d", len(tasks))
}

// TestSchedulerGPUTypes 测试不同GPU类型
func TestSchedulerGPUTypes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 65536, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
		{ID: "gpu3", Model: "4090", Memory: 24576, Node: "node3"},
		{ID: "gpu4", Model: "A100", Memory: 81920, Node: "node4"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 测试各种GPU型号请求
	gpuModels := []string{"V100", "3090", "4090", "A100", ""}
	for i, model := range gpuModels {
		task := models.NewTask("gpu-type-test", "echo", "ubuntu:latest", 1, model, 5)
		err := sched.SubmitTask(task)
		if err != nil {
			t.Logf("Task %d (model %s): %v", i, model, err)
		}
	}
}

// TestSchedulerReassignment 测试GPU重分配
func TestSchedulerReassignment(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = []config.MockGPUConfig{
		{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
		{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
	}

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 任务1占用gpu0
	task1 := models.NewTask("reassign1", "echo", "ubuntu:latest", 1, "", 5)
	sched.SubmitTask(task1)

	// 任务2占用gpu1
	task2 := models.NewTask("reassign2", "echo", "ubuntu:latest", 1, "", 5)
	sched.SubmitTask(task2)

	// 杀死任务1
	sched.KillTask(task1.ID)

	// 再提交新任务，应该能用到gpu0
	task3 := models.NewTask("reassign3", "echo", "ubuntu:latest", 1, "", 5)
	sched.SubmitTask(task3)

	t.Logf("Task3 assigned GPUs: %v", task3.GPUAssigned)
}

// TestSchedulerMetrics 测试指标收集
func TestSchedulerMetrics(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交一些任务
	for i := 0; i < 5; i++ {
		task := models.NewTask("metrics", "echo", "ubuntu:latest", 1, "", 5)
		sched.SubmitTask(task)
	}

	// 杀死一些
	tasks := sched.GetTasks("running")
	if len(tasks) > 0 {
		sched.KillTask(tasks[0].ID)
	}

	stats := sched.GetStats()
	if stats == nil {
		t.Error("GetStats returned nil")
	}

	t.Logf("Final stats: %+v", stats)
}

// TestSchedulerConcurrentKill 测试并发杀死
func TestSchedulerConcurrentKill(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	// 提交多个任务
	taskIDs := []string{}
	for i := 0; i < 5; i++ {
		task := models.NewTask("concurrent-kill", "echo", "ubuntu:latest", 1, "", 5)
		sched.SubmitTask(task)
		taskIDs = append(taskIDs, task.ID)
	}

	// 并发杀死
	for _, id := range taskIDs {
		go func(tid string) {
			sched.KillTask(tid)
		}(id)
	}

	// 验证
	stats := sched.GetStats()
	t.Logf("After concurrent kill: running=%d", stats["running"])
}

// TestSchedulerTaskInfo 测试任务信息完整性
func TestSchedulerTaskInfo(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	sched := NewScheduler(gpuMgr, dockerMgr, true)

	task := models.NewTask("info-test", "python train.py", "pytorch:2.0", 2, "V100", 8)
	sched.SubmitTask(task)

	// 验证所有字段
	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.Name != "info-test" {
		t.Errorf("Expected name 'info-test', got '%s'", task.Name)
	}
	if task.Command != "python train.py" {
		t.Errorf("Expected command 'python train.py', got '%s'", task.Command)
	}
	if task.Image != "pytorch:2.0" {
		t.Errorf("Expected image 'pytorch:2.0', got '%s'", task.Image)
	}
	if task.Priority != 8 {
		t.Errorf("Expected priority 8, got %d", task.Priority)
	}
}
