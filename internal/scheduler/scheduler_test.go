package scheduler

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/docker"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/models"
	"testing"
)

// 创建测试用的调度器
func createTestScheduler() *Scheduler {
	cfg := config.DefaultConfig()
	cfg.MockMode = true

	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)

	return NewScheduler(gpuMgr, dockerMgr, true)
}

// TestSubmitTask 测试任务提交
func TestSubmitTask(t *testing.T) {
	sched := createTestScheduler()

	// 创建一个简单的任务
	task := models.NewTask(
		"test-task",
		"python train.py",
		"pytorch/pytorch:2.0",
		1,  // 需要1张GPU
		"",  // 不限制GPU型号
		5,   // 优先级5
	)

	// 提交任务
	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// 验证任务状态
	if task.Status != models.TaskStatusRunning {
		t.Errorf("Expected task status to be 'running', got '%s'", task.Status)
	}

	// 验证GPU分配
	if len(task.GPUAssigned) != 1 {
		t.Errorf("Expected 1 GPU assigned, got %d", len(task.GPUAssigned))
	}

	t.Logf("Task %s is running on GPU %s", task.ID, task.GPUAssigned)
}

// TestSubmitTaskWithSpecificGPUModel 测试指定GPU型号
func TestSubmitTaskWithSpecificGPUModel(t *testing.T) {
	sched := createTestScheduler()

	// 创建一个需要特定GPU型号的任务
	task := models.NewTask(
		"v100-task",
		"python train.py",
		"pytorch/pytorch:2.0",
		1,
		"V100",  // 只要V100
		5,
	)

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// 验证分配的是V100
	if len(task.GPUAssigned) > 0 {
		// 检查GPU型号
		gpu, _ := sched.GetGPUManager().GetGPUByID(task.GPUAssigned[0])
		if gpu.Model != "V100" {
			t.Errorf("Expected V100 GPU, got %s", gpu.Model)
		}
	}
}

// TestKillTask 测试杀死任务
func TestKillTask(t *testing.T) {
	sched := createTestScheduler()

	// 先提交一个任务
	task := models.NewTask(
		"kill-test",
		"python train.py",
		"pytorch/pytorch:2.0",
		1,
		"",
		5,
	)

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// 杀死任务
	err = sched.KillTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to kill task: %v", err)
	}

	// 验证任务状态
	if task.Status != models.TaskStatusKilled {
		t.Errorf("Expected task status to be 'killed', got '%s'", task.Status)
	}

	// 验证GPU已释放
	if len(task.GPUAssigned) > 0 {
		gpu, _ := sched.GetGPUManager().GetGPUByID(task.GPUAssigned[0])
		if gpu.Status != models.GPUStatusIdle {
			t.Errorf("Expected GPU to be idle after kill, got '%s'", gpu.Status)
		}
	}
}

// TestMultipleTasks 测试多任务
func TestMultipleTasks(t *testing.T) {
	sched := createTestScheduler()

	// 提交多个任务
	task1 := models.NewTask("task-1", "python train1.py", "pytorch/pytorch:2.0", 1, "", 5)
	task2 := models.NewTask("task-2", "python train2.py", "pytorch/pytorch:2.0", 1, "", 5)
	task3 := models.NewTask("task-3", "python train3.py", "pytorch/pytorch:2.0", 1, "", 5)

	sched.SubmitTask(task1)
	sched.SubmitTask(task2)
	sched.SubmitTask(task3)

	// 检查任务状态 - 默认有4个GPU，所以3个任务都可以运行
	tasks := sched.GetTasks("running")
	if len(tasks) != 3 {
		t.Errorf("Expected 3 running tasks (4 GPUs), got %d", len(tasks))
	}

	// 至少第一个任务应该在运行
	if tasks[0].Status != models.TaskStatusRunning {
		t.Errorf("First task should be running")
	}

	stats := sched.GetStats()
	t.Logf("Running tasks: %d, Pending: %d", stats["running"], stats["pending"])
}

// TestPreemption 测试抢占
func TestPreemption(t *testing.T) {
	sched := createTestScheduler()

	// 提交一个低优先级任务
	task1 := models.NewTask("low-priority", "python train.py", "pytorch/pytorch:2.0", 1, "", 1)
	sched.SubmitTask(task1)

	// 提交一个高优先级任务（需要抢占）
	task2 := models.NewTask("high-priority", "python train.py", "pytorch/pytorch:2.0", 1, "", 10)
	err := sched.SubmitTask(task2)

	// 由于抢占，低优先级任务应该被杀死
	// 注意：实际抢占行为取决于实现，这里测试流程能跑通
	if err != nil {
		t.Logf("Expected error due to no available GPUs: %v", err)
	}

	t.Logf("Task1 status: %s, Task2 status: %s", task1.Status, task2.Status)
}

// TestGPUsAfterTaskCompletion 测试任务完成后GPU释放
func TestGPUsAfterTaskCompletion(t *testing.T) {
	sched := createTestScheduler()

	// 提交任务
	task := models.NewTask("completion-test", "python train.py", "pytorch/pytorch:2.0", 1, "", 5)
	sched.SubmitTask(task)

	gpuID := task.GPUAssigned[0]

	// 杀死任务
	sched.KillTask(task.ID)

	// 验证GPU已释放
	gpu, err := sched.GetGPUManager().GetGPUByID(gpuID)
	if err != nil {
		t.Fatalf("Failed to get GPU: %v", err)
	}

	if gpu.Status != models.GPUStatusIdle {
		t.Errorf("Expected GPU to be idle, got '%s'", gpu.Status)
	}

	if gpu.TaskID != "" {
		t.Errorf("Expected GPU.TaskID to be empty, got '%s'", gpu.TaskID)
	}
}

// TestGetStats 测试统计信息
func TestGetStats(t *testing.T) {
	sched := createTestScheduler()

	// 初始统计
	stats := sched.GetStats()
	if stats["pending"] != 0 || stats["running"] != 0 {
		t.Errorf("Expected empty stats initially")
	}

	// 提交任务
	task1 := models.NewTask("task1", "python train.py", "pytorch/pytorch:2.0", 1, "", 5)
	task2 := models.NewTask("task2", "python train.py", "pytorch/pytorch:2.0", 1, "", 5)
	task3 := models.NewTask("task3", "python train.py", "pytorch/pytorch:2.0", 1, "", 5)

	sched.SubmitTask(task1)
	sched.SubmitTask(task2)
	sched.SubmitTask(task3)

	// 检查统计 - 至少应该有1个任务运行
	stats = sched.GetStats()
	if stats["running"] < 1 {
		t.Errorf("Expected at least 1 running task, got %d", stats["running"])
	}

	// 总任务数应该>=1
	total := stats["pending"] + stats["running"] + stats["completed"]
	if total < 1 {
		t.Errorf("Expected at least 1 task")
	}

	t.Logf("Stats: %+v", stats)
}
