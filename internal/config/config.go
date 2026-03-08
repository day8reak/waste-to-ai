// ============================================================================
// 配置管理模块
// 定义系统配置结构和加载/保存方法
// ============================================================================

package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config 系统配置结构
//
// 配置项说明：
// - Server配置：API服务器监听地址和端口
// - Mock模式：用于测试和无GPU环境
// - Docker配置：Docker守护进程连接地址
// - 调度配置：任务调度相关参数
// - 监控配置：健康检查间隔
type Config struct {
	// Server配置
	// 监听地址，默认"0.0.0.0"
	ServerHost string `json:"server_host" yaml:"server_host"`
	// 监听端口，默认8080
	ServerPort int `json:"server_port" yaml:"server_port"`

	// Mock模式配置
	// 是否启用Mock模式（不调用真实Docker/NVIDIA API）
	MockMode bool `json:"mock_mode" yaml:"mock_mode"`
	// Mock模式下自动创建的GPU数量，默认4
	MockGPUCount int `json:"mock_gpu_count" yaml:"mock_gpu_count"`
	// 自定义Mock GPU配置列表
	MockGPUs []MockGPUConfig `json:"mock_gpus" yaml:"mock_gpus"`

	// Docker配置
	// Docker守护进程地址
	// Unix socket: "unix:///var/run/docker.sock"
	// TCP: "tcp://localhost:2375"
	DockerEndpoint string `json:"docker_endpoint" yaml:"docker_endpoint"`

	// 调度配置
	// 默认任务优先级（1-10），默认5
	DefaultPriority int `json:"default_priority" yaml:"default_priority"`
	// 是否启用任务抢占功能，默认true
	PreemptEnabled bool `json:"preempt_enabled" yaml:"preempt_enabled"`

	// 监控配置
	// GPU健康检查间隔，默认5秒
	MonitorInterval time.Duration `json:"monitor_interval" yaml:"monitor_interval"`
}

// MockGPUConfig 模拟GPU配置
//
// 用于在Mock模式下定义GPU设备
type MockGPUConfig struct {
	ID     string `json:"id" yaml:"id"`         // GPU ID（如gpu0, gpu1）
	Model  string `json:"model" yaml:"model"`    // GPU型号（V100, 3090, 4090等）
	Memory int    `json:"memory" yaml:"memory"`  // 显存大小（MB）
	Node   string `json:"node" yaml:"node"`     // 所属节点名称
}

// DefaultConfig 返回默认配置
//
// 返回适用于开发和测试环境的默认配置：
// - Mock模式：启用（便于无GPU环境测试）
// - GPU数量：4个
// - 调度策略：启用抢占
// - 监控间隔：5秒
func DefaultConfig() *Config {
	return &Config{
		ServerHost:      "0.0.0.0",
		ServerPort:      8080,
		MockMode:        true,
		MockGPUCount:    4,
		DockerEndpoint:  "unix:///var/run/docker.sock",
		DefaultPriority: 5,
		PreemptEnabled:  true,
		MonitorInterval: 5 * time.Second,
	}
}

// LoadConfig 从文件加载配置
//
// 加载流程：
// 1. 尝试读取配置文件
// 2. 解析JSON格式配置
// 3. 如果未配置Mock GPU，使用默认GPU列表
// 4. 返回配置对象
//
// 参数：
//   - path: 配置文件路径
//
// 返回：
//   - *Config: 加载的配置
//   - error: 解析失败时返回错误
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// 文件不存在，返回默认配置
		return DefaultConfig(), nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 如果没有配置Mock GPU，使用默认GPU列表
	if len(cfg.MockGPUs) == 0 {
		cfg.MockGPUs = []MockGPUConfig{
			{ID: "gpu0", Model: "V100", Memory: 32768, Node: "node1"},
			{ID: "gpu1", Model: "V100", Memory: 32768, Node: "node1"},
			{ID: "gpu2", Model: "3090", Memory: 24576, Node: "node2"},
			{ID: "gpu3", Model: "4090", Memory: 24576, Node: "node3"},
		}
	}

	return &cfg, nil
}

// SaveConfig 保存配置到文件
//
// 参数：
//   - cfg: 配置对象
//   - path: 保存路径
//
// 返回：
//   - error: 保存失败时返回错误
func SaveConfig(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
