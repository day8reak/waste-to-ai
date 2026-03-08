package config

import (
	"os"
	"testing"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServerHost != "0.0.0.0" {
		t.Errorf("期望 ServerHost 0.0.0.0, 实际为 %s", cfg.ServerHost)
	}

	if cfg.ServerPort != 8080 {
		t.Errorf("期望 ServerPort 8080, 实际为 %d", cfg.ServerPort)
	}

	if !cfg.MockMode {
		t.Error("MockMode 应默认为 true")
	}

	if cfg.MockGPUCount != 4 {
		t.Errorf("期望 MockGPUCount 4, 实际为 %d", cfg.MockGPUCount)
	}

	if cfg.DefaultPriority != 5 {
		t.Errorf("期望 DefaultPriority 5, 实际为 %d", cfg.DefaultPriority)
	}

	if !cfg.PreemptEnabled {
		t.Error("PreemptEnabled 应默认为 true")
	}
}

// TestLoadConfig_FileNotExist 配置文件不存在
func TestLoadConfig_FileNotExist(t *testing.T) {
	cfg, err := LoadConfig("non_existent_config.json")

	// 应该返回默认配置
	if err != nil {
		t.Logf("返回错误: %v", err)
	}

	if cfg == nil {
		t.Error("应返回默认配置")
	}
}

// TestLoadConfig_ValidFile 测试加载有效配置文件
func TestLoadConfig_ValidFile(t *testing.T) {
	// 创建临时配置文件
	content := `{
		"server_host": "127.0.0.1",
		"server_port": 9090,
		"mock_mode": false,
		"mock_gpu_count": 8,
		"mock_gpus": [
			{"id": "gpu0", "model": "V100", "memory": 32768, "node": "node1"}
		],
		"preempt_enabled": true
	}`

	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置
	if cfg.ServerHost != "127.0.0.1" {
		t.Errorf("期望 ServerHost 127.0.0.1, 实际为 %s", cfg.ServerHost)
	}

	if cfg.ServerPort != 9090 {
		t.Errorf("期望 ServerPort 9090, 实际为 %d", cfg.ServerPort)
	}

	if cfg.MockMode != false {
		t.Errorf("期望 MockMode false, 实际为 %v", cfg.MockMode)
	}

	if len(cfg.MockGPUs) != 1 {
		t.Errorf("期望 1 个Mock GPU, 实际为 %d", len(cfg.MockGPUs))
	}

	if !cfg.PreemptEnabled {
		t.Error("PreemptEnabled 应为 true")
	}
}

// TestSaveConfig 测试保存配置
func TestSaveConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServerPort = 9999

	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 保存配置
	err = SaveConfig(cfg, tmpFile.Name())
	if err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	// 重新加载
	loaded, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("重新加载配置失败: %v", err)
	}

	// 验证
	if loaded.ServerPort != 9999 {
		t.Errorf("期望 ServerPort 9999, 实际为 %d", loaded.ServerPort)
	}
}

// TestLoadConfig_MockGPUsGeneration 测试Mock GPU自动生成
func TestLoadConfig_MockGPUsGeneration(t *testing.T) {
	// 创建没有Mock GPUs的配置
	content := `{
		"server_host": "0.0.0.0",
		"server_port": 8080,
		"mock_mode": true,
		"mock_gpu_count": 2
	}`

	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 应该自动生成Mock GPUs
	if len(cfg.MockGPUs) == 0 {
		t.Error("应该自动生成 Mock GPUs")
	}

	t.Logf("自动生成的 GPUs: %+v", cfg.MockGPUs)
}
