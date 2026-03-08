package scheduler

// ============================================================================
// GPU任务调度器
// 负责管理任务队列、GPU分配、任务启动和生命周期管理
// 支持任务抢占、GPU故障自动恢复等高级功能
// ============================================================================

import (
	"container/list"
	"fmt"
	"gpu-scheduler/internal/docker"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/models"
	"sync"
	"time"
)

// Scheduler 调度器
// 核心调度组件，负责协调GPU资源和任务执行
//
// 线程安全性：
// - 所有公共方法都使用互斥锁保护
// - 内部方法调用需注意锁的获取顺序避免死锁
//
// 主要功能：
// - 任务提交和队列管理
// - GPU资源分配和回收
// - 任务优先级抢占
// - GPU故障自动恢复
type Scheduler struct {
	gpuManager  gpu.GPUManager    // GPU管理器接口
	dockerMgr   docker.DockerManager // Docker管理器接口

	// 任务队列
	// pendingTasks: 等待分配GPU的任务队列（FIFO顺序）
	// runningTasks: 正在运行的任务映射（taskID -> Task）
	// completedTasks: 已完成的任务映射（用于历史查询）
	pendingTasks   *list.List
	runningTasks  map[string]*models.Task
	completedTasks map[string]*models.Task

	mu sync.RWMutex // 读写锁，保护所有任务相关数据

	// 配置
	// preemptEnabled: 是否启用任务抢占功能
	// 当新任务无法获取足够GPU时，可抢占低优先级任务
	preemptEnabled bool
}

// NewScheduler 创建调度器
func NewScheduler(gpuMgr gpu.GPUManager, dockerMgr docker.DockerManager, preemptEnabled bool) *Scheduler {
	return &Scheduler{
		gpuManager:     gpuMgr,
		dockerMgr:      dockerMgr,
		pendingTasks:   list.New(),
		runningTasks:   make(map[string]*models.Task),
		completedTasks: make(map[string]*models.Task),
		preemptEnabled: preemptEnabled,
	}
}

// SubmitTask 提交任务
//
// 功能说明：
// 1. 尝试为任务分配所需的GPU资源
// 2. 如果GPU不足且启用抢占，尝试抢占低优先级任务
// 3. 分配成功后启动Docker容器
// 4. 失败则加入等待队列
//
// 参数：
//   - task: 要提交的任务，必须包含Image、Command等必要信息
//
// 返回：
//   - nil: 任务成功提交并启动
//   - error: 任务加入等待队列（返回包含"task queued"的消息）或启动失败
//
// 线程安全：方法内部获取写锁
func (s *Scheduler) SubmitTask(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 步骤1: 先尝试分配GPU
	err := s.assignGPU(task)

	// 步骤2: 如果分配失败且启用抢占，尝试抢占低优先级任务
	for err != nil && s.preemptEnabled {
		// 查找可抢占的最低优先级任务
		preemptTask := s.tryPreempt(task.Priority)
		if preemptTask == nil {
			// 没有可抢占的任务，退出循环
			break
		}

		// 重要：释放锁后再杀死任务，避免死锁
		// 因为KillTask也需要获取锁
		s.mu.Unlock()
		s.KillTask(preemptTask.ID)
		s.mu.Lock()

		// 重新尝试分配GPU
		err = s.assignGPU(task)
	}

	// 步骤3: 如果仍然分配失败，加入等待队列
	if err != nil {
		s.pendingTasks.PushBack(task)
		return fmt.Errorf("task queued: %v", err)
	}

	// 步骤4: 分配成功，启动容器
	if err := s.startTask(task); err != nil {
		// 启动失败，回滚GPU分配
		s.releaseGPU(task)
		return err
	}

	// 步骤5: 加入运行队列
	s.runningTasks[task.ID] = task
	return nil
}

// assignGPU 为任务分配GPU
//
// 分配策略：
// 1. 获取所有空闲GPU列表
// 2. 根据任务指定的GPU型号筛选（如V100、3090等）
// 3. 选择足够数量的GPU分配给任务
//
// 注意：
// - 此方法在持有写锁的情况下调用
// - 不能在此方法内调用tryPreempt（会导致死锁）
// - 抢占逻辑在SubmitTask中处理
//
// 参数：
//   - task: 需要分配GPU的任务
//
// 返回：
//   - nil: 分配成功
//   - error: GPU不足或分配失败
func (s *Scheduler) assignGPU(task *models.Task) error {
	// 步骤1: 获取所有空闲GPU
	availableGPUs, err := s.gpuManager.GetAvailableGPUs()
	if err != nil {
		return err
	}

	// 步骤2: 按型号筛选（如果任务指定了GPU型号）
	var filtered []*models.GPUDevice
	for _, gpu := range availableGPUs {
		// task.GPUModel为空表示接受任意型号
		if task.GPUModel == "" || gpu.Model == task.GPUModel {
			filtered = append(filtered, gpu)
		}
	}

	// 步骤3: 检查GPU数量是否足够
	if len(filtered) < task.GPURequired {
		return fmt.Errorf("not enough available GPUs: need %d, have %d",
			task.GPURequired, len(filtered))
	}

	// 步骤4: 选择GPU（选择前N个）
	selected := filtered[:task.GPURequired]
	gpuIDs := make([]string, len(selected))
	for i, gpu := range selected {
		gpuIDs[i] = gpu.ID
	}

	// 步骤5: 调用GPU管理器分配资源
	if err := s.gpuManager.AllocateGPU(gpuIDs, task.ID); err != nil {
		return err
	}

	// 步骤6: 更新任务的GPU分配信息
	task.GPUAssigned = gpuIDs
	return nil
}

// startTask 启动任务容器
//
// 功能说明：
// 1. 调用Docker管理器创建并启动容器
// 2. 将容器ID关联到任务
// 3. 更新任务状态为Running
// 4. 记录任务开始时间
//
// 参数：
//   - task: 已分配GPU的任务（GPUAssigned字段已设置）
//
// 返回：
//   - nil: 容器启动成功
//   - error: 容器启动失败
func (s *Scheduler) startTask(task *models.Task) error {
	// 步骤1: 启动Docker容器
	containerID, err := s.dockerMgr.Run(task.Image, task.Command, task.GPUAssigned)
	if err != nil {
		return err
	}

	// 步骤2: 更新任务状态
	task.ContainerID = containerID
	task.Status = models.TaskStatusRunning

	// 步骤3: 记录开始时间
	now := time.Now()
	task.StartedAt = &now

	return nil
}

// releaseGPU 释放任务占用的GPU资源
//
// 功能说明：
// - 将GPU状态从Allocated改回Idle
// - 清除GPU与任务的关联关系
// - 重置GPU的显存使用和利用率
//
// 注意：
// - 此方法不获取锁，由调用者负责锁的管理
// - 调用前应确保task.GPUAssigned有效
func (s *Scheduler) releaseGPU(task *models.Task) {
	if len(task.GPUAssigned) > 0 {
		s.gpuManager.ReleaseGPU(task.GPUAssigned)
	}
}

// tryPreempt 尝试抢占低优先级任务
//
// 抢占策略：
// - 查找运行中优先级最低的任务
// - 仅当新任务优先级高于最低优先级任务时才抢占
// - 如果未启用抢占功能，直接返回nil
//
// 注意：
// - 此方法在持有读锁的情况下调用
// - 调用者负责锁的获取和释放
//
// 参数：
//   - priority: 新任务的优先级
//
// 返回：
//   - *models.Task: 需要被抢占的任务（优先级较低）
//   - nil: 不需要抢占（未启用抢占或无可抢占任务）
func (s *Scheduler) tryPreempt(priority int) *models.Task {
	// 检查是否启用抢占功能
	if !s.preemptEnabled {
		return nil
	}

	var lowestTask *models.Task

	// 遍历运行中的任务，找到优先级最低的
	for _, task := range s.runningTasks {
		if lowestTask == nil || task.Priority < lowestTask.Priority {
			lowestTask = task
		}
	}

	// 判断是否可以抢占：当前任务优先级必须高于最低优先级任务
	if lowestTask != nil && lowestTask.Priority < priority {
		return lowestTask
	}

	return nil
}

// KillTask 杀死任务（动态回收）
//
// 功能说明：
// 1. 查找正在运行的任务
// 2. 停止关联的Docker容器
// 3. 释放占用的GPU资源
// 4. 更新任务状态为Killed
// 5. 将任务移入已完成集合
// 6. 尝试处理等待队列中的任务
//
// 参数：
//   - taskID: 要杀死的任务ID
//
// 返回：
//   - nil: 任务成功杀死
//   - error: 任务不存在或不在运行状态
//
// 线程安全：方法内部获取写锁
func (s *Scheduler) KillTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 步骤1: 查找运行中的任务
	task, ok := s.runningTasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found or not running", taskID)
	}

	// 步骤2: 停止Docker容器
	if task.ContainerID != "" {
		s.dockerMgr.Stop(task.ContainerID)
	}

	// 步骤3: 释放GPU资源
	s.releaseGPU(task)

	// 步骤4: 更新任务状态
	task.Status = models.TaskStatusKilled
	now := time.Now()
	task.FinishedAt = &now

	// 步骤5: 从运行中移除，加入已完成集合
	delete(s.runningTasks, taskID)
	s.completedTasks[taskID] = task

	// 步骤6: 尝试处理等待队列
	s.processPendingQueue()

	return nil
}

// processPendingQueue 处理等待队列
//
// 处理策略：
// - 按FIFO顺序尝试为等待队列中的任务分配资源
// - 成功分配则启动容器并移入运行队列
// - 失败则继续等待（保持顺序）
//
// 改进建议：
// - 可考虑按优先级排序，实现优先级调度
// - 当前实现简单，可能导致高优先级任务等待时间过长
func (s *Scheduler) processPendingQueue() {
	// 遍历等待队列
	for e := s.pendingTasks.Front(); e != nil; e = e.Next() {
		task := e.Value.(*models.Task)

		// 尝试分配GPU
		if err := s.assignGPU(task); err != nil {
			// GPU不足，继续等待
			continue
		}

		// 分配成功，启动容器
		if err := s.startTask(task); err != nil {
			// 启动失败，释放GPU并继续
			s.releaseGPU(task)
			continue
		}

		// 成功启动，移入运行队列
		s.runningTasks[task.ID] = task
		s.pendingTasks.Remove(e)
	}
}

// GetTasks 获取任务列表
//
// 参数：
//   - status: 过滤条件，可选值：
//     - "pending": 等待中的任务
//     - "running": 运行中的任务
//     - "completed": 已完成的任务
//     - "": 或其他值，返回所有任务
//
// 返回：
//   - []*models.Task: 符合条件任务列表
//
// 线程安全：方法内部获取读锁
func (s *Scheduler) GetTasks(status string) []*models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*models.Task

	switch status {
	case "pending":
		// 获取等待队列中的任务
		for e := s.pendingTasks.Front(); e != nil; e = e.Next() {
			result = append(result, e.Value.(*models.Task))
		}
	case "running":
		// 获取运行中的任务
		for _, task := range s.runningTasks {
			result = append(result, task)
		}
	case "completed":
		// 获取已完成的任务
		for _, task := range s.completedTasks {
			result = append(result, task)
		}
	default:
		// 返回所有任务
		for e := s.pendingTasks.Front(); e != nil; e = e.Next() {
			result = append(result, e.Value.(*models.Task))
		}
		for _, task := range s.runningTasks {
			result = append(result, task)
		}
		for _, task := range s.completedTasks {
			result = append(result, task)
		}
	}

	return result
}

// GetTaskByID 根据ID获取任务
//
// 搜索顺序：
// 1. 等待队列 (pending)
// 2. 运行队列 (running)
// 3. 已完成队列 (completed)
//
// 参数：
//   - taskID: 任务ID
//
// 返回：
//   - *models.Task: 找到的任务
//   - error: 任务不存在
//
// 线程安全：方法内部获取读锁
func (s *Scheduler) GetTaskByID(taskID string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 步骤1: 在等待队列中查找
	for e := s.pendingTasks.Front(); e != nil; e = e.Next() {
		task := e.Value.(*models.Task)
		if task.ID == taskID {
			return task, nil
		}
	}

	// 步骤2: 在运行队列中查找
	if task, ok := s.runningTasks[taskID]; ok {
		return task, nil
	}

	// 步骤3: 在已完成队列中查找
	if task, ok := s.completedTasks[taskID]; ok {
		return task, nil
	}

	return nil, fmt.Errorf("task %s not found", taskID)
}

// GetTaskByRayJobID 根据Ray Job ID获取任务
//
// 搜索顺序：
// 1. 运行队列 (running)
// 2. 等待队列 (pending)
//
// 参数：
//   - rayJobID: Ray Job ID
//
// 返回：
//   - *models.Task: 找到的任务
//   - error: 任务不存在
//
// 线程安全：方法内部获取读锁
func (s *Scheduler) GetTaskByRayJobID(rayJobID string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 步骤1: 在运行队列中查找
	for _, task := range s.runningTasks {
		if task.RayJobID == rayJobID {
			return task, nil
		}
	}

	// 步骤2: 在等待队列中查找
	for e := s.pendingTasks.Front(); e != nil; e = e.Next() {
		task := e.Value.(*models.Task)
		if task.RayJobID == rayJobID {
			return task, nil
		}
	}

	return nil, fmt.Errorf("ray job %s not found", rayJobID)
}

// ReleaseGPUFromTask 释放任务占用的指定GPU
//
// 功能说明：
// 1. 根据任务ID查找任务
// 2. 如果未指定GPU列表，释放所有GPU
// 3. 如果指定了GPU列表，只释放指定的GPU
// 4. 更新GPU状态为Idle
// 5. 如果任务不再占用任何GPU，将其从运行队列移除
//
// 参数：
//   - taskID: 任务ID
//   - gpuIDs: 要释放的GPU ID列表（nil表示释放所有）
//
// 返回：
//   - int: 释放的GPU数量
//   - error: 任务不存在或释放失败
//
// 线程安全：方法内部获取写锁
func (s *Scheduler) ReleaseGPUFromTask(taskID string, gpuIDs []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 步骤1: 查找任务
	task, ok := s.runningTasks[taskID]
	if !ok {
		return 0, fmt.Errorf("task %s not found or not running", taskID)
	}

	// 步骤2: 确定要释放的GPU列表
	var gpusToRelease []string
	if len(gpuIDs) == 0 {
		// 未指定，释放所有GPU
		gpusToRelease = task.GPUAssigned
	} else {
		// 指定了GPU列表，只释放指定的
		for _, gpuID := range gpuIDs {
			for _, assignedGPU := range task.GPUAssigned {
				if gpuID == assignedGPU {
					gpusToRelease = append(gpusToRelease, gpuID)
					break
				}
			}
		}
	}

	// 步骤2.5: 检查动态任务的最低GPU保障
	// 如果是动态任务，释放后不能低于最低GPU数量
	// 注意：如果释放所有GPU（remaining将为0），则允许释放（任务会完成）
	if task.Dynamic && task.MinGPURequired > 0 {
		currentGPUs := len(task.GPUAssigned)
		remainingAfterRelease := currentGPUs - len(gpusToRelease)
		// 只有在还有剩余GPU的情况下才检查最低保障
		// 如果释放后GPU为0（任务完成），则允许
		if remainingAfterRelease > 0 && remainingAfterRelease < task.MinGPURequired {
			return 0, fmt.Errorf("cannot release GPU: task %s has minimum %d GPU(s) protected, current %d, release would leave %d",
				taskID, task.MinGPURequired, currentGPUs, remainingAfterRelease)
		}
	}

	// 步骤3: 释放GPU资源
	if len(gpusToRelease) > 0 {
		s.gpuManager.ReleaseGPU(gpusToRelease)
	}

	// 步骤4: 更新任务的GPU分配信息
	// 从GPUAssigned中移除已释放的GPU
	remaining := make([]string, 0)
	for _, assigned := range task.GPUAssigned {
		found := false
		for _, released := range gpusToRelease {
			if assigned == released {
				found = true
				break
			}
		}
		if !found {
			remaining = append(remaining, assigned)
		}
	}
	task.GPUAssigned = remaining

	// 步骤5: 无论任务是否还有GPU，都尝试处理等待队列
	// 这样当GPU释放后，等待队列中的任务可以及时获取资源运行
	s.processPendingQueue()

	// 步骤6: 如果任务不再占用任何GPU，将其移出运行队列
	if len(task.GPUAssigned) == 0 {
		// 停止容器
		if task.ContainerID != "" {
			s.dockerMgr.Stop(task.ContainerID)
		}
		// 更新状态
		task.Status = models.TaskStatusCompleted
		now := time.Now()
		task.FinishedAt = &now
		// 移入已完成集合
		delete(s.runningTasks, taskID)
		s.completedTasks[taskID] = task
	}

	return len(gpusToRelease), nil
}

// GetRayTasks 获取所有Ray任务
//
// 返回：
//   - []*models.Task: 所有Ray任务列表
//
// 线程安全：方法内部获取读锁
func (s *Scheduler) GetRayTasks() []*models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*models.Task

	// 遍历运行队列，收集Ray任务
	for _, task := range s.runningTasks {
		if task.IsRayTask {
			result = append(result, task)
		}
	}

	// 遍历等待队列，收集Ray任务
	for e := s.pendingTasks.Front(); e != nil; e = e.Next() {
		task := e.Value.(*models.Task)
		if task.IsRayTask {
			result = append(result, task)
		}
	}

	return result
}

// GetStats 获取调度统计信息
//
// 返回统计信息：
// - pending: 等待分配GPU的任务数量
// - running: 正在运行的任务数量
// - completed: 已完成的任务数量（含成功、失败、被杀死）
//
// 返回：
//   - map[string]int: 各状态任务数量
//
// 线程安全：方法内部获取读锁
func (s *Scheduler) GetStats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]int{
		"pending":   s.pendingTasks.Len(),
		"running":   len(s.runningTasks),
		"completed":  len(s.completedTasks),
	}
}

// GetGPUManager 获取GPU管理器（用于测试）
func (s *Scheduler) GetGPUManager() gpu.GPUManager {
	return s.gpuManager
}

// GetTasksByGPUID 根据GPU ID查找使用该GPU的任务
//
// 参数:
//   - gpuID: GPU ID
//
// 返回:
//   - []*models.Task: 使用该GPU的任务列表
func (s *Scheduler) GetTasksByGPUID(gpuID string) []*models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*models.Task
	for _, task := range s.runningTasks {
		for _, assignedGPU := range task.GPUAssigned {
			if assignedGPU == gpuID {
				tasks = append(tasks, task)
				break
			}
		}
	}
	return tasks
}

// HandleGPUFailure 处理GPU故障（自动恢复）
//
// 故障处理流程：
// 1. 检查GPU健康状态，获取离线GPU列表
// 2. 将离线GPU标记为Offline状态，获取受影响的任务ID
// 3. 停止受影响任务关联的容器
// 4. 将任务状态重置为Pending，重新加入等待队列
// 5. 尝试处理等待队列，重新分配资源
//
// 使用场景：
// - GPU硬件故障导致离线
// - 网络问题导致GPU不可访问
// - 定期健康检查发现异常
//
// 返回：
//   - int: 被重新入队的任务数量
//   - error: 健康检查或状态更新失败
//
// 注意：
// - 推理任务通常无状态，可安全重试
// - 训练任务需谨慎使用，可能丢失检查点
func (s *Scheduler) HandleGPUFailure() (int, error) {
	// 步骤1: 检查GPU健康状态
	offlineGPUs, err := s.gpuManager.CheckHealth()
	if err != nil {
		return 0, fmt.Errorf("检查GPU健康状态失败: %v", err)
	}

	// 无离线GPU，无需处理
	if len(offlineGPUs) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 步骤2: 标记离线GPU，获取受影响的任务ID
	affectedTaskIDs, err := s.gpuManager.MarkGPUsOffline(offlineGPUs)
	if err != nil {
		return 0, fmt.Errorf("标记GPU离线失败: %v", err)
	}

	// 步骤3: 处理受影响的任务
	requeuedCount := 0
	for _, taskID := range affectedTaskIDs {
		task, ok := s.runningTasks[taskID]
		if !ok {
			continue
		}

		// 停止Docker容器
		if task.ContainerID != "" {
			s.dockerMgr.Stop(task.ContainerID)
		}

		// 重置任务状态为等待重新分配
		task.Status = models.TaskStatusPending
		task.GPUAssigned = nil
		task.ContainerID = ""

		// 从运行队列移除，加入等待队列
		delete(s.runningTasks, taskID)
		s.pendingTasks.PushBack(task)
		requeuedCount++
	}

	// 步骤4: 尝试为等待队列中的任务重新分配资源
	s.processPendingQueue()

	return requeuedCount, nil
}

// CheckAndRecoverFromFailures 定时检查并从故障中恢复
//
// 使用建议：
// - 可设置定时器定期调用（如每秒或每分钟）
// - 在生产环境中建议集成到监控系统中
// - 结合GPU健康检测实现自动化故障恢复
//
// 参数：无
//
// 返回：
//   - int: 被重新入队的任务数量
//   - error: 处理过程中的错误
func (s *Scheduler) CheckAndRecoverFromFailures() (int, error) {
	return s.HandleGPUFailure()
}
