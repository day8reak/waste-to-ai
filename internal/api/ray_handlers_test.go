package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ==================== Ray API Tests ====================

// TestRayAllocate 测试 Ray 分配 GPU
func TestRayAllocate(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"job_id": "ray-job-123", "gpu_count": 2, "priority": 8}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Body.Bytes(), &result)

	if result["job_id"] == nil {
		t.Error("Expected job_id in response")
	}

	if result["gpu_ids"] == nil {
		t.Error("Expected gpu_ids in response")
	}

	if result["status"] == nil {
		t.Error("Expected status in response")
	}
}

// TestRayAllocate_DefaultValues 测试 Ray 分配默认值
func TestRayAllocate_DefaultValues(t *testing.T) {
	h := createTestHandler()

	// 只提供 job_id，其他使用默认值
	body := []byte(`{"job_id": "ray-job-default"}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
}

// TestRayAllocate_MissingJobID 测试缺少 job_id
func TestRayAllocate_MissingJobID(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"gpu_count": 2}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayAllocate_InvalidJSON 测试无效 JSON
func TestRayAllocate_InvalidJSON(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{invalid json}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayAllocate_GPUModel 测试指定 GPU 型号
func TestRayAllocate_GPUModel(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"job_id": "ray-job-v100", "gpu_count": 1, "gpu_model": "V100", "priority": 8}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

// TestRayAllocate_NotEnoughGPUs 测试 GPU 不足
func TestRayAllocate_NotEnoughGPUs(t *testing.T) {
	h := createTestHandler()

	// 请求超过可用数量的 GPU
	body := []byte(`{"job_id": "ray-job-many", "gpu_count": 100, "priority": 8}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 应该返回 202（任务加入队列）
	if resp.Code != http.StatusAccepted && resp.Code != http.StatusOK {
		t.Errorf("Expected status 200 or 202, got %d", resp.Code)
	}
}

// TestRayRelease 测试 Ray 释放 GPU
func TestRayRelease(t *testing.T) {
	h := createTestHandler()

	// 先分配 GPU
	body := []byte(`{"job_id": "ray-job-release", "gpu_count": 1}`)
	resp := httptest.NewRecorder()
	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 释放 GPU
	releaseBody := []byte(`{"job_id": "ray-job-release"}`)
	releaseResp := httptest.NewRecorder()

	h.RayRelease(releaseResp, createTestRequest("POST", "/api/ray/release", releaseBody))

	if releaseResp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", releaseResp.Code, releaseResp.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(releaseResp.Body.Bytes(), &result)

	if result["status"] == nil {
		t.Error("Expected status in response")
	}
}

// TestRayRelease_WithGPUIDs 测试释放指定 GPU
func TestRayRelease_WithGPUIDs(t *testing.T) {
	h := createTestHandler()

	// 先分配 2 个 GPU
	body := []byte(`{"job_id": "ray-job-partial", "gpu_count": 2}`)
	resp := httptest.NewRecorder()
	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 解析返回的 GPU IDs
	var allocResult map[string]interface{}
	json.Unmarshal(resp.Body.Bytes(), &allocResult)
	gpuIDs, ok := allocResult["gpu_ids"].([]interface{})
	if !ok || len(gpuIDs) < 2 {
		t.Skip("Not enough GPUs allocated for partial release test")
	}

	// 释放第一个 GPU
	gpuID := gpuIDs[0].(string)
	releaseBody := []byte(`{"job_id": "ray-job-partial", "gpu_ids": ["` + gpuID + `"]}`)
	releaseResp := httptest.NewRecorder()

	h.RayRelease(releaseResp, createTestRequest("POST", "/api/ray/release", releaseBody))

	if releaseResp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", releaseResp.Code)
	}
}

// TestRayRelease_MissingJobID 测试缺少 job_id
func TestRayRelease_MissingJobID(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"gpu_ids": ["gpu0"]}`)
	resp := httptest.NewRecorder()

	h.RayRelease(resp, createTestRequest("POST", "/api/ray/release", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayRelease_InvalidJSON 测试无效 JSON
func TestRayRelease_InvalidJSON(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{invalid json}`)
	resp := httptest.NewRecorder()

	h.RayRelease(resp, createTestRequest("POST", "/api/ray/release", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayRelease_NotFound 测试释放不存在的任务
func TestRayRelease_NotFound(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"job_id": "non-existent-ray-job"}`)
	resp := httptest.NewRecorder()

	h.RayRelease(resp, createTestRequest("POST", "/api/ray/release", body))

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

// TestRayStatus 测试 Ray 状态查询
func TestRayStatus(t *testing.T) {
	h := createTestHandler()

	// 先分配一些 GPU
	body := []byte(`{"job_id": "ray-job-status", "gpu_count": 1}`)
	resp := httptest.NewRecorder()
	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 查询状态
	w := httptest.NewRecorder()
	h.RayStatus(w, createTestRequest("GET", "/api/ray/status", nil))

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	if result["total_gpus"] == nil {
		t.Error("Expected total_gpus in response")
	}

	if result["available_gpus"] == nil {
		t.Error("Expected available_gpus in response")
	}

	if result["allocated_gpus"] == nil {
		t.Error("Expected allocated_gpus in response")
	}

	if result["ray_tasks"] == nil {
		t.Error("Expected ray_tasks in response")
	}

	if result["total_ray_tasks"] == nil {
		t.Error("Expected total_ray_tasks in response")
	}
}

// TestRayStatus_WithBlockedGPUs 测试有阻塞 GPU 的状态
func TestRayStatus_WithBlockedGPUs(t *testing.T) {
	h := createTestHandler()

	// 先分配 GPU
	body := []byte(`{"job_id": "ray-job-blocked", "gpu_count": 1}`)
	resp := httptest.NewRecorder()
	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 阻塞一个 GPU（不是当前任务使用的）
	blockBody := []byte(`{"gpu_ids": ["gpu1"]}`)
	blockResp := httptest.NewRecorder()
	h.RayBlock(blockResp, createTestRequest("POST", "/api/ray/block", blockBody))

	// 查询状态
	w := httptest.NewRecorder()
	h.RayStatus(w, createTestRequest("GET", "/api/ray/status", nil))

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	// 应该有 blocked_gpus 字段
	if result["blocked_gpus"] == nil {
		t.Error("Expected blocked_gpus in response")
	}
}

// TestRayBlock 测试阻塞 GPU
func TestRayBlock(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"gpu_ids": ["gpu0", "gpu1"]}`)
	resp := httptest.NewRecorder()

	h.RayBlock(resp, createTestRequest("POST", "/api/ray/block", body))

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Body.Bytes(), &result)

	if result["status"] == nil {
		t.Error("Expected status in response")
	}

	if result["blocked"] == nil {
		t.Error("Expected blocked list in response")
	}
}

// TestRayBlock_MissingGPUIDs 测试缺少 gpu_ids
func TestRayBlock_MissingGPUIDs(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{}`)
	resp := httptest.NewRecorder()

	h.RayBlock(resp, createTestRequest("POST", "/api/ray/block", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayBlock_InvalidJSON 测试无效 JSON
func TestRayBlock_InvalidJSON(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{invalid json}`)
	resp := httptest.NewRecorder()

	h.RayBlock(resp, createTestRequest("POST", "/api/ray/block", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayBlock_NotFound 测试阻塞不存在的 GPU
func TestRayBlock_NotFound(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"gpu_ids": ["non-existent-gpu"]}`)
	resp := httptest.NewRecorder()

	h.RayBlock(resp, createTestRequest("POST", "/api/ray/block", body))

	// 应该返回错误
	if resp.Code != http.StatusBadRequest && resp.Code != http.StatusOK {
		t.Errorf("Expected status 400 or 200, got %d", resp.Code)
	}
}

// TestRayUnblock 测试解除阻塞
func TestRayUnblock(t *testing.T) {
	h := createTestHandler()

	// 先阻塞 GPU
	blockBody := []byte(`{"gpu_ids": ["gpu0"]}`)
	blockResp := httptest.NewRecorder()
	h.RayBlock(blockResp, createTestRequest("POST", "/api/ray/block", blockBody))

	// 解除阻塞
	unblockBody := []byte(`{"gpu_ids": ["gpu0"]}`)
	unblockResp := httptest.NewRecorder()

	h.RayUnblock(unblockResp, createTestRequest("POST", "/api/ray/unblock", unblockBody))

	if unblockResp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", unblockResp.Code, unblockResp.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(unblockResp.Body.Bytes(), &result)

	if result["status"] == nil {
		t.Error("Expected status in response")
	}

	if result["unblocked"] == nil {
		t.Error("Expected unblocked list in response")
	}
}

// TestRayUnblock_MissingGPUIDs 测试缺少 gpu_ids
func TestRayUnblock_MissingGPUIDs(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{}`)
	resp := httptest.NewRecorder()

	h.RayUnblock(resp, createTestRequest("POST", "/api/ray/unblock", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayUnblock_InvalidJSON 测试无效 JSON
func TestRayUnblock_InvalidJSON(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{invalid json}`)
	resp := httptest.NewRecorder()

	h.RayUnblock(resp, createTestRequest("POST", "/api/ray/unblock", body))

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayUnblock_NotBlocked 测试解除未阻塞的 GPU
func TestRayUnblock_NotBlocked(t *testing.T) {
	h := createTestHandler()

	// 尝试解除未阻塞的 GPU
	body := []byte(`{"gpu_ids": ["gpu0"]}`)
	resp := httptest.NewRecorder()

	h.RayUnblock(resp, createTestRequest("POST", "/api/ray/unblock", body))

	// 应该返回错误
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayUnblock_NotFound 测试解除不存在的 GPU
func TestRayUnblock_NotFound(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"gpu_ids": ["non-existent-gpu"]}`)
	resp := httptest.NewRecorder()

	h.RayUnblock(resp, createTestRequest("POST", "/api/ray/unblock", body))

	// 应该返回错误
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestRayAllocateAndReleaseWorkflow 测试完整的分配释放流程
func TestRayAllocateAndReleaseWorkflow(t *testing.T) {
	h := createTestHandler()

	// 1. 分配 GPU
	allocBody := []byte(`{"job_id": "workflow-job", "gpu_count": 2}`)
	allocResp := httptest.NewRecorder()
	h.RayAllocate(allocResp, createTestRequest("POST", "/api/ray/allocate", allocBody))

	if allocResp.Code != http.StatusOK {
		t.Fatalf("Allocation failed: %d - %s", allocResp.Code, allocResp.Body.String())
	}

	// 2. 查询状态
	statusResp := httptest.NewRecorder()
	h.RayStatus(statusResp, createTestRequest("GET", "/api/ray/status", nil))

	var status map[string]interface{}
	json.Unmarshal(statusResp.Body.Bytes(), &status)

	initialRayTasks := status["total_ray_tasks"].(float64)
	if initialRayTasks != 1 {
		t.Errorf("Expected 1 ray task, got %f", initialRayTasks)
	}

	// 3. 释放 GPU
	releaseBody := []byte(`{"job_id": "workflow-job"}`)
	releaseResp := httptest.NewRecorder()
	h.RayRelease(releaseResp, createTestRequest("POST", "/api/ray/release", releaseBody))

	if releaseResp.Code != http.StatusOK {
		t.Errorf("Release failed: %d - %s", releaseResp.Code, releaseResp.Body.String())
	}
}

// TestRayBlockAndUnblockWorkflow 测试阻塞解除流程
func TestRayBlockAndUnblockWorkflow(t *testing.T) {
	h := createTestHandler()

	// 1. 阻塞 GPU
	blockBody := []byte(`{"gpu_ids": ["gpu0", "gpu1"]}`)
	blockResp := httptest.NewRecorder()
	h.RayBlock(blockResp, createTestRequest("POST", "/api/ray/block", blockBody))

	if blockResp.Code != http.StatusOK {
		t.Fatalf("Block failed: %d - %s", blockResp.Code, blockResp.Body.String())
	}

	// 2. 查询状态确认阻塞
	statusResp := httptest.NewRecorder()
	h.RayStatus(statusResp, createTestRequest("GET", "/api/ray/status", nil))

	var status map[string]interface{}
	json.Unmarshal(statusResp.Body.Bytes(), &status)

	blockedCount := status["blocked_gpus"].(float64)
	if blockedCount != 2 {
		t.Errorf("Expected 2 blocked GPUs, got %f", blockedCount)
	}

	// 3. 解除部分阻塞
	unblockBody := []byte(`{"gpu_ids": ["gpu0"]}`)
	unblockResp := httptest.NewRecorder()
	h.RayUnblock(unblockResp, createTestRequest("POST", "/api/ray/unblock", unblockBody))

	if unblockResp.Code != http.StatusOK {
		t.Fatalf("Unblock failed: %d - %s", unblockResp.Code, unblockResp.Body.String())
	}

	// 4. 确认还有一个阻塞
	statusResp2 := httptest.NewRecorder()
	h.RayStatus(statusResp2, createTestRequest("GET", "/api/ray/status", nil))

	var status2 map[string]interface{}
	json.Unmarshal(statusResp2.Body.Bytes(), &status2)

	blockedCount2 := status2["blocked_gpus"].(float64)
	if blockedCount2 != 1 {
		t.Errorf("Expected 1 blocked GPU after partial unblock, got %f", blockedCount2)
	}

	// 5. 解除剩余阻塞
	unblockBody2 := []byte(`{"gpu_ids": ["gpu1"]}`)
	unblockResp2 := httptest.NewRecorder()
	h.RayUnblock(unblockResp2, createTestRequest("POST", "/api/ray/unblock", unblockBody2))

	// 6. 确认没有阻塞
	statusResp3 := httptest.NewRecorder()
	h.RayStatus(statusResp3, createTestRequest("GET", "/api/ray/status", nil))

	var status3 map[string]interface{}
	json.Unmarshal(statusResp3.Body.Bytes(), &status3)

	blockedCount3 := status3["blocked_gpus"].(float64)
	if blockedCount3 != 0 {
		t.Errorf("Expected 0 blocked GPUs after full unblock, got %f", blockedCount3)
	}
}

// TestRayStatus_EmptyCluster 测试空集群状态
func TestRayStatus_EmptyCluster(t *testing.T) {
	h := createTestHandler()

	w := httptest.NewRecorder()
	h.RayStatus(w, createTestRequest("GET", "/api/ray/status", nil))

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	// 应该有 4 个 GPU（默认配置）
	if result["total_gpus"].(float64) != 4 {
		t.Errorf("Expected 4 total GPUs, got %f", result["total_gpus"].(float64))
	}

	// 应该有 4 个可用
	if result["available_gpus"].(float64) != 4 {
		t.Errorf("Expected 4 available GPUs, got %f", result["available_gpus"].(float64))
	}

	// 应该有 0 个分配
	if result["allocated_gpus"].(float64) != 0 {
		t.Errorf("Expected 0 allocated GPUs, got %f", result["allocated_gpus"].(float64))
	}

	// 应该有 0 个 Ray 任务
	if result["total_ray_tasks"].(float64) != 0 {
		t.Errorf("Expected 0 ray tasks, got %f", result["total_ray_tasks"].(float64))
	}
}

// TestRayAllocate_ZeroGPUCount 测试 GPU 数量为 0
func TestRayAllocate_ZeroGPUCount(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"job_id": "ray-job-zero", "gpu_count": 0}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 应该使用默认值 1
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
}

// TestRayAllocate_NegativePriority 测试负数优先级
func TestRayAllocate_NegativePriority(t *testing.T) {
	h := createTestHandler()

	body := []byte(`{"job_id": "ray-job-neg", "gpu_count": 1, "priority": -5}`)
	resp := httptest.NewRecorder()

	h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))

	// 应该使用默认值
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
}
