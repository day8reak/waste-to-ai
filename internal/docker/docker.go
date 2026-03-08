// ============================================================================
// Docker容器管理器
// 负责容器的创建、停止、状态查询等生命周期管理
// 支持Mock模式和真实Docker API模式
// ============================================================================

package docker

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// DockerManager Docker容器管理器接口
//
// 该接口定义了容器管理的基本操作：
// - 容器创建和启动
// - 容器停止
// - 容器状态查询
// - 容器列表获取
//
// 实现说明：
// - Mock模式：用于测试和无GPU环境
// - 真实模式：需要集成docker/docker-go库
type DockerManager interface {
	// Run 运行容器
	// 参数：
	//   - image: Docker镜像名称
	//   - command: 要执行的命令
	//   - gpus: 分配的GPU列表（如["gpu0", "gpu1"]）
	// 返回：
	//   - string: 容器ID
	//   - error: 启动失败时返回错误
	Run(image, command string, gpus []string) (string, error)

	// Stop 停止容器
	// 参数：
	//   - containerID: 容器ID
	// 返回：
	//   - error: 停止失败时返回错误
	Stop(containerID string) error

	// GetContainerStatus 获取容器状态
	// 参数：
	//   - containerID: 容器ID
	// 返回：
	//   - string: 容器状态（running, exited, paused等）
	//   - error: 查询失败时返回错误
	GetContainerStatus(containerID string) (string, error)

	// ListContainers 列出所有容器
	// 返回：
	//   - []string: 容器ID列表
	//   - error: 查询失败时返回错误
	ListContainers() ([]string, error)
}

// BaseDockerManager 基础Docker管理器
//
// 字段说明：
// - endpoint: Docker守护进程地址（Unix socket或TCP）
// - mockMode: 是否使用模拟模式（不实际创建容器）
// - running: 模拟模式下运行中的容器映射
type BaseDockerManager struct {
	endpoint string
	mockMode bool
	running  map[string]*MockContainer
}

// MockContainer 模拟容器结构
//
// 用于Mock模式下的容器模拟，不涉及真实Docker操作
type MockContainer struct {
	ID        string    // 容器唯一标识
	Image     string    // 使用的镜像
	Command   string    // 执行的命令
	GPUs      []string  // 分配的GPU列表
	Status    string    // 容器状态（running, exited）
	StartedAt time.Time // 启动时间
}

// NewDockerManager 创建Docker管理器
//
// 参数：
//   - endpoint: Docker守护进程地址
//     - Unix socket: "unix:///var/run/docker.sock"
//     - TCP: "tcp://localhost:2375"
//   - mockMode: 是否使用Mock模式
//
// 返回：
//   - DockerManager: Docker管理器实例
func NewDockerManager(endpoint string, mockMode bool) DockerManager {
	return &BaseDockerManager{
		endpoint: endpoint,
		mockMode: mockMode,
		running:  make(map[string]*MockContainer),
	}
}

// Run 运行容器
//
// 功能说明：
// 1. Mock模式下调用mockRun创建模拟容器
// 2. 真实模式下调用Docker API创建容器（待实现）
//
// 注意：
// - 真实模式需要使用docker/docker-go库
// - 需要配置nvidia-runtime支持GPU容器
func (d *BaseDockerManager) Run(image, command string, gpus []string) (string, error) {
	if d.mockMode {
		return d.mockRun(image, command, gpus)
	}

	// TODO: 实现真实的Docker API调用
	// 这里需要使用docker/docker-go库
	return "", fmt.Errorf("real Docker not implemented yet, use mock mode")
}

// mockRun 模拟运行容器
//
// 功能说明：
// 1. 生成唯一的容器ID
// 2. 创建模拟容器对象并加入运行映射
// 3. 设置容器状态为running
func (d *BaseDockerManager) mockRun(image, command string, gpus []string) (string, error) {
	containerID := generateContainerID()
	container := &MockContainer{
		ID:        containerID,
		Image:     image,
		Command:   command,
		GPUs:      gpus,
		Status:    "running",
		StartedAt: time.Now(),
	}
	d.running[containerID] = container
	return containerID, nil
}

// Stop 停止容器
//
// 参数：
//   - containerID: 要停止的容器ID
//
// 返回：
//   - nil: 停止成功
//   - error: 容器不存在或停止失败
func (d *BaseDockerManager) Stop(containerID string) error {
	if d.mockMode {
		return d.mockStop(containerID)
	}

	// TODO: 实现真实的Docker API调用
	return fmt.Errorf("real Docker not implemented yet, use mock mode")
}

// mockStop 模拟停止容器
//
// 功能说明：
// 1. 查找指定容器
// 2. 更新容器状态为exited
// 3. 从运行映射中移除
//
// 注意：
// - 如果容器不存在，返回错误
// - 幂等操作：重复停止已停止的容器会返回错误
func (d *BaseDockerManager) mockStop(containerID string) error {
	container, ok := d.running[containerID]
	if !ok {
		return fmt.Errorf("container %s not found", containerID)
	}
	container.Status = "exited"
	delete(d.running, containerID)
	return nil
}

// GetContainerStatus 获取容器状态
//
// 参数：
//   - containerID: 容器ID
//
// 返回：
//   - string: 容器状态（running: 运行中, exited: 已停止）
//   - error: 真实模式下未实现
//
// Mock模式说明：
// - 容器存在且运行中：返回"running"
// - 容器不存在或已停止：返回"exited"
func (d *BaseDockerManager) GetContainerStatus(containerID string) (string, error) {
	if d.mockMode {
		container, ok := d.running[containerID]
		if !ok {
			return "exited", nil
		}
		return container.Status, nil
	}

	return "", fmt.Errorf("real Docker not implemented yet")
}

// ListContainers 列出所有容器
//
// 返回：
//   - []string: 当前运行中的容器ID列表
//   - error: 真实模式下未实现
func (d *BaseDockerManager) ListContainers() ([]string, error) {
	if d.mockMode {
		ids := make([]string, 0, len(d.running))
		for id := range d.running {
			ids = append(ids, id)
		}
		return ids, nil
	}

	return nil, fmt.Errorf("real Docker not implemented yet")
}

// generateContainerID 生成容器ID
//
// 生成策略：
// - 使用原子计数器保证并发安全
// - 格式：时间戳-计数器-随机字符串
// - 示例：20260308103045-1-a3b2
var containerIDCounter int64

func generateContainerID() string {
	// 使用原子操作确保并发安全
	id := atomic.AddInt64(&containerIDCounter, 1)
	return fmt.Sprintf("%s-%d-%s", time.Now().Format("20060102150405"), id, randomString(4))
}

// randomString 生成指定长度的随机字符串
//
// 用途：作为容器ID的后缀，增加唯一性
//
// 注意：使用时间戳作为随机源，不够安全
//       实际生产环境应使用crypto/rand
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// CreateContext 创建Docker上下文（预留）
//
// 用途：为Docker API调用创建超时上下文
//
// 参数：
//   - endpoint: Docker守护进程地址
//
// 返回：
//   - context.Context: 超时上下文
//   - context.CancelFunc: 取消函数
//   - error: 创建失败时返回错误
func CreateContext(endpoint string) (context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	return ctx, cancel, nil
}
