package scheduler

import (
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/docker"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/models"
	"testing"
)

// ============================================================================
// GPU释放功能测试
// 核心需求：
// 1. 释放GPU后，GPU变为idle，可给其他任务使用
// 2. 动态任务有最低GPU保障，不能低于min_gpu_required
// 3. 释放GPU后，等待队列中的任务可以及时获取资源运行
// ============================================================================

func createTestSchedulerForRelease(count int) *Scheduler {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.MockGPUs = make([]config.MockGPUConfig, count)
	for i := 0; i < count; i++ {
		cfg.MockGPUs[i] = config.MockGPUConfig{
			ID:     "gpu" + string(rune('0'+i)),
			Model:  "V100",
			Memory: 32768,
			Node:   "node1",
		}
	}
	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)
	return NewScheduler(gpuMgr, dockerMgr, false)
}

// TestReleaseGPU_Basic 基础测试：释放单个GPU
func TestReleaseGPU_Basic(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个任务
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 1
	task.MaxGPURequired = 4

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	if task.Status != models.TaskStatusRunning {
		t.Fatalf("Task should be running, got %s", task.Status)
	}

	// 释放一个GPU
	released, err := sched.ReleaseGPUFromTask(task.ID, []string{task.GPUAssigned[0]})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 1 {
		t.Fatalf("Expected to release 1 GPU, got %d", released)
	}

	// 验证GPU已释放
	// 初始4个GPU，2个被任务占用，2个idle
	// 释放1个GPU后，应该有3个idle (2个原来idle + 1个新释放)
	mgr := sched.GetGPUManager()
	gpus, _ := mgr.GetGPUs()
	idleCount := 0
	for _, g := range gpus {
		if g.Status == models.GPUStatusIdle {
			idleCount++
		}
	}

	if idleCount != 3 {
		t.Fatalf("Expected 3 idle GPUs, got %d", idleCount)
	}

	// 验证任务仍然在运行，只是GPU减少
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusRunning {
		t.Fatalf("Task should still be running, got %s", taskAfter.Status)
	}

	if len(taskAfter.GPUAssigned) != 1 {
		t.Fatalf("Task should have 1 GPU left, got %d", len(taskAfter.GPUAssigned))
	}
}

// TestReleaseGPU_DynamicMin 动态任务最低GPU保障测试
func TestReleaseGPU_DynamicMin(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个动态任务
	task := models.NewTask("dynamic-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 2
	task.MaxGPURequired = 4

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 尝试释放一个GPU，应该失败（低于最低保障）
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{task.GPUAssigned[0]})
	if err == nil {
		t.Fatalf("Expected error when releasing below min GPU, got nil")
	}

	// 验证任务GPU数量不变
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if len(taskAfter.GPUAssigned) != 2 {
		t.Fatalf("Task should still have 2 GPUs, got %d", len(taskAfter.GPUAssigned))
	}
}

// TestReleaseGPU_CanReleaseToMin 动态任务可以释放到最低GPU
func TestReleaseGPU_CanReleaseToMin(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个动态任务，请求3个
	task := models.NewTask("dynamic-task", "python train.py", "pytorch:2.0", 3, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 2
	task.MaxGPURequired = 4

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 释放一个GPU，应该成功（还剩2个，等于最低保障）
	released, err := sched.ReleaseGPUFromTask(task.ID, []string{task.GPUAssigned[0]})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 1 {
		t.Fatalf("Expected to release 1 GPU, got %d", released)
	}

	// 验证任务还有2个GPU
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if len(taskAfter.GPUAssigned) != 2 {
		t.Fatalf("Task should have 2 GPUs, got %d", len(taskAfter.GPUAssigned))
	}

	// 再次释放，应该失败
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{task.GPUAssigned[0]})
	if err == nil {
		t.Fatalf("Expected error when releasing below min GPU, got nil")
	}
}

// TestReleaseGPU_FixedTaskAll 固定任务可以释放所有GPU
func TestReleaseGPU_FixedTaskAll(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个固定任务
	task := models.NewTask("fixed-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = false

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 释放所有GPU，应该成功
	released, err := sched.ReleaseGPUFromTask(task.ID, nil)
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 2 {
		t.Fatalf("Expected to release 2 GPUs, got %d", released)
	}

	// 验证任务已完成
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusCompleted {
		t.Fatalf("Task should be completed, got %s", taskAfter.Status)
	}
}

// TestReleaseGPU_ReleaseAll 释放所有GPU
func TestReleaseGPU_ReleaseAll(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个任务，占用4个GPU
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 4, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 1

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 释放所有GPU
	released, err := sched.ReleaseGPUFromTask(task.ID, nil)
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 4 {
		t.Fatalf("Expected to release 4 GPUs, got %d", released)
	}

	// 验证任务已完成
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusCompleted {
		t.Fatalf("Task should be completed, got %s", taskAfter.Status)
	}

	// 验证所有GPU都是idle
	mgr := sched.GetGPUManager()
	gpus, _ := mgr.GetGPUs()
	idleCount := 0
	for _, g := range gpus {
		if g.Status == models.GPUStatusIdle {
			idleCount++
		}
	}

	if idleCount != 4 {
		t.Fatalf("Expected 4 idle GPUs, got %d", idleCount)
	}
}

// TestReleaseGPU_SpecificGPU 释放指定的GPU
func TestReleaseGPU_SpecificGPU(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个任务，占用4个GPU
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 4, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 2

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 只释放gpu0和gpu1
	released, err := sched.ReleaseGPUFromTask(task.ID, []string{"gpu0", "gpu1"})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 2 {
		t.Fatalf("Expected to release 2 GPUs, got %d", released)
	}

	// 验证任务还剩2个GPU
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if len(taskAfter.GPUAssigned) != 2 {
		t.Fatalf("Task should have 2 GPUs left, got %d", len(taskAfter.GPUAssigned))
	}
}

// TestReleaseGPU_NotFound 释放不存在的任务
func TestReleaseGPU_NotFound(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 尝试释放不存在的任务
	_, err := sched.ReleaseGPUFromTask("non-existent-task", nil)
	if err == nil {
		t.Fatalf("Expected error for non-existent task, got nil")
	}
}

// TestReleaseGPU_8GPUsMin 8 GPU最低保障测试
func TestReleaseGPU_8GPUsMin(t *testing.T) {
	sched := createTestSchedulerForRelease(8)

	// 提交一个任务，请求4个GPU，最低2个
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 4, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 2
	task.MaxGPURequired = 8

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 获取任务实际分配的GPU
	assignedGPUs := task.GPUAssigned
	if len(assignedGPUs) != 4 {
		t.Fatalf("Task should have 4 GPUs assigned, got %d", len(assignedGPUs))
	}

	// 释放前2个GPU，应该成功（还剩2个）
	gpusToRelease := assignedGPUs[:2]
	_, err = sched.ReleaseGPUFromTask(task.ID, gpusToRelease)
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	// 验证任务还有2个GPU
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if len(taskAfter.GPUAssigned) != 2 {
		t.Fatalf("Task should have 2 GPUs, got %d", len(taskAfter.GPUAssigned))
	}

	// 再释放1个，应该失败
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{taskAfter.GPUAssigned[0]})
	if err == nil {
		t.Fatalf("Expected error when releasing below min GPU, got nil")
	}
}

// TestReleaseGPU_MinZero 动态任务min为0
func TestReleaseGPU_MinZero(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个动态任务，min为0
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 0
	task.MaxGPURequired = 4

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 释放所有GPU，应该成功
	released, err := sched.ReleaseGPUFromTask(task.ID, nil)
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 2 {
		t.Fatalf("Expected to release 2 GPUs, got %d", released)
	}

	// 验证任务已完成
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusCompleted {
		t.Fatalf("Task should be completed, got %s", taskAfter.Status)
	}
}

// TestReleaseGPU_RayTask Ray任务的GPU释放
func TestReleaseGPU_RayTask(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个Ray任务
	rayTask := models.NewRayTask("ray-job-1", 2, "", 5)
	rayTask.Dynamic = true
	rayTask.MinGPURequired = 1
	rayTask.MaxGPURequired = 4

	err := sched.SubmitTask(rayTask)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 释放一个GPU
	released, err := sched.ReleaseGPUFromTask(rayTask.ID, []string{rayTask.GPUAssigned[0]})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	if released != 1 {
		t.Fatalf("Expected to release 1 GPU, got %d", released)
	}

	// 验证Ray任务仍在运行
	taskAfter, _ := sched.GetTaskByID(rayTask.ID)
	if taskAfter.Status != models.TaskStatusRunning {
		t.Fatalf("Ray task should still be running, got %s", taskAfter.Status)
	}

	if len(taskAfter.GPUAssigned) != 1 {
		t.Fatalf("Ray task should have 1 GPU left, got %d", len(taskAfter.GPUAssigned))
	}
}

// TestReleaseGPU_StateConsistency 释放后状态一致性
func TestReleaseGPU_StateConsistency(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个任务
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 1

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 获取实际分配的GPU
	assignedGPUs := task.GPUAssigned
	if len(assignedGPUs) != 2 {
		t.Fatalf("Task should have 2 GPUs assigned, got %d", len(assignedGPUs))
	}

	// 释放第一个GPU
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{assignedGPUs[0]})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	// 验证GPU状态一致性
	mgr := sched.GetGPUManager()
	releasedGPU, _ := mgr.GetGPUByID(assignedGPUs[0])
	remainingGPU, _ := mgr.GetGPUByID(assignedGPUs[1])

	if releasedGPU.Status != models.GPUStatusIdle {
		t.Fatalf("%s should be idle, got %s", assignedGPUs[0], releasedGPU.Status)
	}
	if remainingGPU.Status != models.GPUStatusAllocated {
		t.Fatalf("%s should be allocated, got %s", assignedGPUs[1], remainingGPU.Status)
	}

	// 验证任务状态
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusRunning {
		t.Fatalf("Task should still be running, got %s", taskAfter.Status)
	}
	if len(taskAfter.GPUAssigned) != 1 {
		t.Fatalf("Task should have 1 GPU, got %d", len(taskAfter.GPUAssigned))
	}
}

// TestReleaseGPU_ReleaseTwice 连续释放
func TestReleaseGPU_ReleaseTwice(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个任务，占用2个GPU
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 1

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 获取实际分配的GPU
	assignedGPUs := task.GPUAssigned
	if len(assignedGPUs) != 2 {
		t.Fatalf("Task should have 2 GPUs assigned, got %d", len(assignedGPUs))
	}

	// 第一次释放第一个GPU
	released1, err := sched.ReleaseGPUFromTask(task.ID, []string{assignedGPUs[0]})
	if err != nil {
		t.Fatalf("First ReleaseGPUFromTask failed: %v", err)
	}
	if released1 != 1 {
		t.Fatalf("Expected to release 1 GPU, got %d", released1)
	}

	// 第二次释放剩余的GPU（此时任务还有1个GPU，min=1，不能再释放了）
	// 但因为释放后将为0，所以应该允许
	released2, err := sched.ReleaseGPUFromTask(task.ID, nil) // 释放所有剩余GPU
	if err != nil {
		t.Fatalf("Second ReleaseGPUFromTask failed: %v", err)
	}
	if released2 != 1 {
		t.Fatalf("Expected to release 1 GPU, got %d", released2)
	}

	// 验证任务已完成
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusCompleted {
		t.Fatalf("Task should be completed, got %s", taskAfter.Status)
	}
}

// TestReleaseGPU_MixedMin 混合最低GPU保障值
func TestReleaseGPU_MixedMin(t *testing.T) {
	sched := createTestSchedulerForRelease(8)

	// 任务1: min=2
	task1 := models.NewTask("task1", "python train.py", "pytorch:2.0", 4, "", 5)
	task1.Dynamic = true
	task1.MinGPURequired = 2

	// 任务2: min=1
	task2 := models.NewTask("task2", "python train.py", "pytorch:2.0", 2, "", 5)
	task2.Dynamic = true
	task2.MinGPURequired = 1

	sched.SubmitTask(task1)
	sched.SubmitTask(task2)

	// 获取task1实际分配的GPU
	task1GPUs := task1.GPUAssigned
	// 释放task1的2个GPU（降到min=2）
	_, err := sched.ReleaseGPUFromTask(task1.ID, task1GPUs[:2])
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	// 验证task1还有2个GPU
	task1After, _ := sched.GetTaskByID(task1.ID)
	if len(task1After.GPUAssigned) != 2 {
		t.Fatalf("Task1 should have 2 GPUs, got %d", len(task1After.GPUAssigned))
	}

	// 释放task2的1个GPU（降到min=1）
	_, err = sched.ReleaseGPUFromTask(task2.ID, []string{task2.GPUAssigned[0]})
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}

	// 验证task2还有1个GPU
	task2After, _ := sched.GetTaskByID(task2.ID)
	if len(task2After.GPUAssigned) != 1 {
		t.Fatalf("Task2 should have 1 GPU, got %d", len(task2After.GPUAssigned))
	}
}

// TestReleaseGPU_CannotBelowMinMultiple 多次尝试释放到低于最低保障
func TestReleaseGPU_CannotBelowMinMultiple(t *testing.T) {
	sched := createTestSchedulerForRelease(8)

	// 提交一个任务，占用4个GPU，最低2个
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 4, "", 5)
	task.Dynamic = true
	task.MinGPURequired = 2

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 获取实际分配的GPU
	assignedGPUs := task.GPUAssigned
	if len(assignedGPUs) != 4 {
		t.Fatalf("Task should have 4 GPUs assigned, got %d", len(assignedGPUs))
	}

	// 第一次释放1个GPU - 应该成功（还剩3个）
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{assignedGPUs[0]})
	if err != nil {
		t.Fatalf("First release should succeed, got: %v", err)
	}

	// 第二次释放1个GPU - 应该成功（还剩2个，等于min）
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{assignedGPUs[1]})
	if err != nil {
		t.Fatalf("Second release should succeed, got: %v", err)
	}

	// 第三次释放1个GPU - 应该失败（还剩1个，低于min=2）
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{assignedGPUs[2]})
	if err == nil {
		t.Fatalf("Third release should fail (below min), got nil")
	}

	// 验证任务还有2个GPU
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if len(taskAfter.GPUAssigned) != 2 {
		t.Fatalf("Task should have 2 GPUs, got %d", len(taskAfter.GPUAssigned))
	}
}

// TestReleaseGPU_DefaultMin 默认MinGPURequired
func TestReleaseGPU_DefaultMin(t *testing.T) {
	sched := createTestSchedulerForRelease(4)

	// 提交一个任务，占用2个GPU
	task := models.NewTask("test-task", "python train.py", "pytorch:2.0", 2, "", 5)
	task.Dynamic = true
	// MinGPURequired默认为GPURequired=2

	err := sched.SubmitTask(task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}

	// 尝试释放1个GPU，应该失败（剩余1个 < min=2）
	_, err = sched.ReleaseGPUFromTask(task.ID, []string{task.GPUAssigned[0]})
	if err == nil {
		t.Fatalf("Expected error when releasing below min GPU, got nil")
	}

	// 释放所有GPU，应该成功（任务完成）
	released, err := sched.ReleaseGPUFromTask(task.ID, nil)
	if err != nil {
		t.Fatalf("ReleaseGPUFromTask failed: %v", err)
	}
	if released != 2 {
		t.Fatalf("Expected to release 2 GPUs, got %d", released)
	}

	// 验证任务已完成
	taskAfter, _ := sched.GetTaskByID(task.ID)
	if taskAfter.Status != models.TaskStatusCompleted {
		t.Fatalf("Task should be completed, got %s", taskAfter.Status)
	}
}
