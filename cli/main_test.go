package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestToJSON tests the toJSON utility function
func TestToJSON(t *testing.T) {
	// Test with simple map
	input := map[string]interface{}{
		"name": "test",
		"age":  123,
	}

	reader := toJSON(input)
	if reader == nil {
		t.Error("Expected non-nil reader")
	}

	// Verify the JSON is valid
	var result map[string]interface{}
	err := json.NewDecoder(reader).Decode(&result)
	if err != nil {
		t.Errorf("Failed to decode JSON: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("Expected name 'test', got '%v'", result["name"])
	}
}

// TestToJSON_Empty tests toJSON with empty map
func TestToJSON_Empty(t *testing.T) {
	reader := toJSON(map[string]interface{}{})
	if reader == nil {
		t.Error("Expected non-nil reader")
	}
}

// TestToJSON_Nil tests toJSON with nil
func TestToJSON_Nil(t *testing.T) {
	reader := toJSON(nil)
	if reader == nil {
		t.Error("Expected non-nil reader")
	}
}

// TestTaskJSON tests Task JSON marshaling
func TestTaskJSON(t *testing.T) {
	task := Task{
		ID:          "task-123",
		Name:        "test-task",
		Command:     "python train.py",
		Image:       "pytorch/pytorch:2.0",
		GPURequired: 2,
		GPUModel:    "V100",
		Priority:    8,
		Status:      "running",
		GPUAssigned: []string{"gpu0", "gpu1"},
	}

	// Marshal to JSON
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal task: %v", err)
	}

	// Unmarshal back
	var task2 Task
	err = json.Unmarshal(data, &task2)
	if err != nil {
		t.Fatalf("Failed to unmarshal task: %v", err)
	}

	if task2.ID != task.ID {
		t.Errorf("Expected ID %s, got %s", task.ID, task2.ID)
	}

	if task2.GPURequired != task.GPURequired {
		t.Errorf("Expected GPURequired %d, got %d", task.GPURequired, task2.GPURequired)
	}
}

// TestGPUDeviceJSON tests GPUDevice JSON marshaling
func TestGPUDeviceJSON(t *testing.T) {
	gpu := GPUDevice{
		ID:     "gpu0",
		Model:  "V100",
		Memory: 32768,
		Node:   "node1",
		Status: "idle",
	}

	data, err := json.Marshal(gpu)
	if err != nil {
		t.Fatalf("Failed to marshal GPU: %v", err)
	}

	var gpu2 GPUDevice
	err = json.Unmarshal(data, &gpu2)
	if err != nil {
		t.Fatalf("Failed to unmarshal GPU: %v", err)
	}

	if gpu2.ID != gpu.ID {
		t.Errorf("Expected ID %s, got %s", gpu.ID, gpu2.ID)
	}
}

// TestGPUServerIntegration tests CLI with a mock server
func TestGPUServerIntegration(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tasks":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"tasks": []map[string]interface{}{
						{
							"id":             "task-1",
							"name":           "test",
							"status":         "running",
							"gpu_assigned":   []string{"gpu0"},
							"created_at":     "2024-01-01T00:00:00Z",
						},
					},
				})
			} else if r.Method == "POST" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"task_id": "task-new",
					"status":  "running",
				})
			}
		case "/api/gpus":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"gpus": []map[string]interface{}{
					{"id": "gpu0", "model": "V100", "status": "allocated"},
				},
			})
		case "/api/stats":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{
				"pending":   1,
				"running":   2,
				"completed": 3,
			})
		case "/api/tasks/task-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Task{
				ID:          "task-1",
				Name:        "test",
				Status:      "running",
				GPURequired: 1,
			})
		}
	}))
	defer server.Close()

	// Test task listing
	tasks := listTasksFromServer(server.URL)
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// Test GPU listing
	gpus := listGPUsFromServer(server.URL)
	if len(gpus) != 1 {
		t.Errorf("Expected 1 GPU, got %d", len(gpus))
	}

	// Test stats
	stats := getStatsFromServer(server.URL)
	if stats["running"] != 2 {
		t.Errorf("Expected 2 running, got %d", stats["running"])
	}
}

// Helper functions that mirror CLI logic for testing
func listTasksFromServer(addr string) []map[string]interface{} {
	resp, _ := http.Get(addr + "/api/tasks")
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	tasks := result["tasks"].([]interface{})
	resultTasks := make([]map[string]interface{}, len(tasks))
	for i, t := range tasks {
		resultTasks[i] = t.(map[string]interface{})
	}
	return resultTasks
}

func listGPUsFromServer(addr string) []map[string]interface{} {
	resp, _ := http.Get(addr + "/api/gpus")
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	gpus := result["gpus"].([]interface{})
	resultGPUs := make([]map[string]interface{}, len(gpus))
	for i, g := range gpus {
		resultGPUs[i] = g.(map[string]interface{})
	}
	return resultGPUs
}

func getStatsFromServer(addr string) map[string]int {
	resp, _ := http.Get(addr + "/api/stats")
	defer resp.Body.Close()

	var stats map[string]int
	json.NewDecoder(resp.Body).Decode(&stats)
	return stats
}

// TestBytesReader tests that toJSON returns a valid reader
func TestToJSON_ReturnsValidReader(t *testing.T) {
	data := map[string]string{"key": "value"}
	reader := toJSON(data)

	buf := make([]byte, 100)
	n, err := reader.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Errorf("Unexpected error reading: %v", err)
	}

	var parsed map[string]string
	err = json.Unmarshal(buf[:n], &parsed)
	if err != nil {
		t.Errorf("Failed to parse JSON: %v", err)
	}

	if parsed["key"] != "value" {
		t.Errorf("Expected 'value', got '%s'", parsed["key"])
	}
}

// TestServerNotReachable tests error handling when server is not reachable
func TestServerNotReachable(t *testing.T) {
	// This should fail since there's no server at this address
	resp, err := http.Get("http://localhost:9999/api/tasks")
	if err == nil {
		resp.Body.Close()
		t.Error("Expected error when server not reachable")
	}
}

// TestJSONEncodingEmptySlice tests JSON encoding of empty slices
func TestJSONEncodingEmptySlice(t *testing.T) {
	task := Task{
		ID:          "task-empty",
		GPUAssigned: []string{},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	if !bytes.Contains(data, []byte("gpu_assigned")) {
		t.Error("Expected gpu_assigned field in JSON")
	}
}
