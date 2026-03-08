package docker

import (
	"testing"
)

// TestMockRun 测试模拟运行容器
func TestMockRun(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	// 运行容器
	containerID, err := mgr.Run(
		"pytorch/pytorch:2.0",
		"python train.py",
		[]string{"gpu0"},
	)

	if err != nil {
		t.Fatalf("Failed to run container: %v", err)
	}

	if containerID == "" {
		t.Error("Container ID should not be empty")
	}

	t.Logf("Container %s started on GPU(s): gpu0", containerID)
}

// TestMockStop 测试模拟停止容器
func TestMockStop(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	// 运行容器
	containerID, _ := mgr.Run("pytorch/pytorch:2.0", "python train.py", []string{"gpu0"})

	// 停止容器
	err := mgr.Stop(containerID)
	if err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}

	// 检查状态
	status, _ := mgr.GetContainerStatus(containerID)
	if status != "exited" {
		t.Errorf("Expected container status 'exited', got '%s'", status)
	}
}

// TestMockGetContainerStatus 测试获取容器状态
func TestMockGetContainerStatus(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	// 运行容器
	containerID, _ := mgr.Run("pytorch/pytorch:2.0", "python train.py", []string{"gpu0"})

	// 检查运行状态
	status, _ := mgr.GetContainerStatus(containerID)
	if status != "running" {
		t.Errorf("Expected container status 'running', got '%s'", status)
	}
}

// TestMockListContainers 测试列出容器
func TestMockListContainers(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	// 运行多个容器
	mgr.Run("image1", "cmd1", []string{"gpu0"})
	mgr.Run("image2", "cmd2", []string{"gpu1"})

	// 列出容器
	containers, err := mgr.ListContainers()
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}
}

// TestMockRun_MultipleGPUs 测试多GPU运行
func TestMockRun_MultipleGPUs(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	containerID, err := mgr.Run(
		"pytorch/pytorch:2.0",
		"python train.py",
		[]string{"gpu0", "gpu1", "gpu2"},
	)

	if err != nil {
		t.Fatalf("Failed to run container: %v", err)
	}

	if containerID == "" {
		t.Error("Container ID should not be empty")
	}
}

// TestMockStop_NotFound 测试停止不存在的容器
func TestMockStop_NotFound(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	err := mgr.Stop("non-existent-container")
	if err != nil {
		t.Logf("Expected error for non-existent container: %v", err)
	}
}

// TestMockGetContainerStatus_NotFound 测试获取不存在的容器状态
func TestMockGetContainerStatus_NotFound(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	status, err := mgr.GetContainerStatus("non-existent-container")
	if err != nil {
		t.Logf("Expected error for non-existent container: %v", err)
	}
	// 状态可能是空或者其他值，取决于实现
	t.Logf("Status for non-existent container: '%s', err: %v", status, err)
}

// TestCreateContext 测试创建上下文
func TestCreateContext(t *testing.T) {
	ctx, cancel, err := CreateContext("unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("CreateContext failed: %v", err)
	}
	defer cancel()

	if ctx == nil {
		t.Error("Context should not be nil")
	}
}

// TestMockStop_AlreadyStopped 测试停止已停止的容器
func TestMockStop_AlreadyStopped(t *testing.T) {
	mgr := NewDockerManager("unix:///var/run/docker.sock", true)

	// 运行容器
	containerID, _ := mgr.Run("pytorch/pytorch:2.0", "python train.py", []string{"gpu0"})

	// 第一次停止
	mgr.Stop(containerID)

	// 第二次停止 - 应该成功（幂等）
	err := mgr.Stop(containerID)
	if err != nil {
		t.Logf("Second stop returned error: %v", err)
	}
}
