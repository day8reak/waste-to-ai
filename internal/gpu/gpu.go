package gpu

import (
	"fmt"
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/models"
	"sync"
)

// GPUManager GPU管理器接口
type GPUManager interface {
	// GetGPUs 获取所有GPU列表
	GetGPUs() ([]*models.GPUDevice, error)
	// GetAvailableGPUs 获取空闲GPU列表（排除被阻塞的GPU）
	GetAvailableGPUs() ([]*models.GPUDevice, error)
	// GetAllocatedGPUs 获取已分配的GPU列表
	GetAllocatedGPUs() ([]*models.GPUDevice, error)
	// AllocateGPU 分配GPU给任务
	AllocateGPU(gpuIDs []string, taskID string) error
	// ReleaseGPU 释放GPU
	ReleaseGPU(gpuIDs []string) error
	// GetGPUByID 根据ID获取GPU
	GetGPUByID(id string) (*models.GPUDevice, error)
	// UpdateGPUStatus 更新GPU状态
	UpdateGPUStatus() error
	// CheckHealth 检查GPU健康状态，返回离线的GPU列表
	CheckHealth() ([]string, error)
	// MarkGPUsOffline 将GPU标记为离线，返回受影响的任务ID列表
	MarkGPUsOffline(gpuIDs []string) ([]string, error)
	// SimulateGPUFailure 模拟GPU故障（用于测试）
	SimulateGPUFailure(gpuID string) error
	// BlockGPU 阻塞GPU（CLI手动释放后进入阻塞状态，不能用于推理）
	BlockGPU(gpuID string) error
	// UnblockGPU 解除GPU阻塞（恢复可用）
	UnblockGPU(gpuID string) error
	// GetBlockedGPUs 获取被阻塞的GPU列表
	GetBlockedGPUs() ([]*models.GPUDevice, error)
}

// BaseGPUManager 基础GPU管理器
type BaseGPUManager struct {
	config *config.Config
	gpus   map[string]*models.GPUDevice
	mu     sync.RWMutex
}

// NewGPUManager 创建GPU管理器
func NewGPUManager(cfg *config.Config) GPUManager {
	mgr := &BaseGPUManager{
		config: cfg,
		gpus:   make(map[string]*models.GPUDevice),
	}

	// 如果是Mock模式，初始化模拟GPU
	if cfg.MockMode {
		mgr.initMockGPUs()
	}

	return mgr
}

// initMockGPUs 初始化模拟GPU
func (m *BaseGPUManager) initMockGPUs() {
	// 如果有明确的MockGPUs配置，使用配置
	if len(m.config.MockGPUs) > 0 {
		for _, mockCfg := range m.config.MockGPUs {
			gpu := models.NewGPUDevice(
				mockCfg.ID,
				fmt.Sprintf("GPU-%s-%s", mockCfg.Model, mockCfg.ID),
				mockCfg.Model,
				mockCfg.Memory,
				mockCfg.Node,
			)
			m.gpus[gpu.ID] = gpu
		}
		return
	}

	// 否则根据MockGPUCount自动生成
	gpuModels := []string{"V100", "V100", "3090", "4090"}
	memorySizes := []int{32768, 32768, 24576, 24576}
	nodes := []string{"node1", "node1", "node2", "node3"}

	for i := 0; i < m.config.MockGPUCount && i < len(gpuModels); i++ {
		gpu := models.NewGPUDevice(
			fmt.Sprintf("gpu%d", i),
			fmt.Sprintf("GPU-%s-%d", gpuModels[i], i),
			gpuModels[i],
			memorySizes[i],
			nodes[i],
		)
		m.gpus[gpu.ID] = gpu
	}
}

// GetGPUs 获取所有GPU
func (m *BaseGPUManager) GetGPUs() ([]*models.GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.GPUDevice, 0, len(m.gpus))
	for _, gpu := range m.gpus {
		result = append(result, gpu)
	}
	return result, nil
}

// GetAvailableGPUs 获取空闲GPU（排除被阻塞的GPU）
func (m *BaseGPUManager) GetAvailableGPUs() ([]*models.GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.GPUDevice
	for _, gpu := range m.gpus {
		// 只返回idle状态的GPU
		if gpu.Status == models.GPUStatusIdle {
			result = append(result, gpu)
		}
	}
	return result, nil
}

// AllocateGPU 分配GPU
func (m *BaseGPUManager) AllocateGPU(gpuIDs []string, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range gpuIDs {
		gpu, ok := m.gpus[id]
		if !ok {
			return fmt.Errorf("GPU %s not found", id)
		}
		if gpu.Status != models.GPUStatusIdle {
			return fmt.Errorf("GPU %s is not idle", id)
		}
		gpu.Status = models.GPUStatusAllocated
		gpu.TaskID = taskID
		gpu.UsedMem = gpu.Memory / 2 // 模拟占用一半显存
		gpu.Util = 50                 // 模拟50%利用率
	}
	return nil
}

// ReleaseGPU 释放GPU
func (m *BaseGPUManager) ReleaseGPU(gpuIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range gpuIDs {
		gpu, ok := m.gpus[id]
		if !ok {
			return fmt.Errorf("GPU %s not found", id)
		}
		gpu.Status = models.GPUStatusIdle
		gpu.TaskID = ""
		gpu.UsedMem = 0
		gpu.Util = 0
	}
	return nil
}

// GetGPUByID 根据ID获取GPU
func (m *BaseGPUManager) GetGPUByID(id string) (*models.GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gpu, ok := m.gpus[id]
	if !ok {
		return nil, fmt.Errorf("GPU %s not found", id)
	}
	return gpu, nil
}

// UpdateGPUStatus 更新GPU状态（在Mock模式下模拟状态变化）
func (m *BaseGPUManager) UpdateGPUStatus() error {
	// 在Mock模式下，定期更新GPU状态模拟真实情况
	// 这里可以添加随机变化逻辑
	return nil
}

// CheckHealth 检查GPU健康状态
// 在非Mock模式下，会调用nvidia-smi检查真实GPU状态
// 返回离线的GPU ID列表
// 注意：此方法不修改任何状态，只返回检测结果
func (m *BaseGPUManager) CheckHealth() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var offlineGPUs []string

	// 遍历所有GPU，检查健康状态
	for id, gpu := range m.gpus {
		if gpu.Status == models.GPUStatusOffline {
			offlineGPUs = append(offlineGPUs, id)
			continue
		}

		// 在非Mock模式下，这里会调用nvidia-smi检查真实GPU
		// 如果GPU不可用，标记为离线
		if !m.config.MockMode {
			// TODO: 实现真实的nvidia-smi调用
			// 示例: nvidia-smi --query-gpu=index,health --format=csv,noheader
		} else {
			// Mock模式下，GPU始终健康
			// 可以通过MarkGPUsOffline手动标记GPU离线来模拟故障
		}
	}

	return offlineGPUs, nil
}

// SimulateGPUFailure 模拟GPU故障（用于测试）
// 在Mock模式下，可以调用此方法手动标记GPU为离线
func (m *BaseGPUManager) SimulateGPUFailure(gpuID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gpu, ok := m.gpus[gpuID]
	if !ok {
		return fmt.Errorf("GPU %s not found", gpuID)
	}

	// 标记为离线
	gpu.Status = models.GPUStatusOffline
	return nil
}

// MarkGPUsOffline 将GPU标记为离线，返回受影响的任务ID列表
func (m *BaseGPUManager) MarkGPUsOffline(gpuIDs []string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var affectedTaskIDs []string

	for _, id := range gpuIDs {
		gpu, ok := m.gpus[id]
		if !ok {
			continue
		}

		// 如果GPU上有任务，记录任务ID
		if gpu.TaskID != "" {
			affectedTaskIDs = append(affectedTaskIDs, gpu.TaskID)
		}

		// 标记GPU为离线，释放任务关联
		gpu.Status = models.GPUStatusOffline
		gpu.TaskID = ""
	}

	return affectedTaskIDs, nil
}

// GetAllocatedGPUs 获取已分配的GPU列表（用于查找运行任务的GPU）
func (m *BaseGPUManager) GetAllocatedGPUs() ([]*models.GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.GPUDevice
	for _, gpu := range m.gpus {
		if gpu.Status == models.GPUStatusAllocated {
			result = append(result, gpu)
		}
	}
	return result, nil
}

// BlockGPU 阻塞GPU（CLI手动释放后进入阻塞状态，不能用于新的推理任务）
// 只设置Blocked标记，不改变Status（保持allocated状态，正在运行的任务可以继续使用）
// GetAvailableGPUs会排除此类GPU
// BlockGPU 释放GPU（从推理服务中真正释放）
// 将GPU从当前任务中释放，GPU变为idle，可给其他任务使用
// 推理服务检测到GPU减少后会自动调整（降低吞吐）
func (m *BaseGPUManager) BlockGPU(gpuID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gpu, ok := m.gpus[gpuID]
	if !ok {
		return fmt.Errorf("GPU %s not found", gpuID)
	}

	// 释放GPU：将GPU状态设为idle，清除任务关联
	gpu.Status = models.GPUStatusIdle
	gpu.TaskID = ""
	gpu.UsedMem = 0
	gpu.Util = 0

	return nil
}

// UnblockGPU 解除GPU阻塞（恢复可用于推理）- 已废弃，使用BlockGPU直接释放
// UnblockGPU 已废弃 - 现在使用BlockGPU直接释放GPU
// 保留此函数仅用于API兼容
func (m *BaseGPUManager) UnblockGPU(gpuID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.gpus[gpuID]
	if !ok {
		return fmt.Errorf("GPU %s not found", gpuID)
	}

	// 已废弃：现在释放GPU使用BlockGPU，解除阻塞已无意义
	// 如果GPU是idle状态，什么都不做
	// 如果GPU是allocated状态，保持不变

	return nil
}

// GetBlockedGPUs 获取被阻塞的GPU列表
func (m *BaseGPUManager) GetBlockedGPUs() ([]*models.GPUDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.GPUDevice
	for _, gpu := range m.gpus {
		if gpu.Status == models.GPUStatusBlocked {
			result = append(result, gpu)
		}
	}
	return result, nil
}
