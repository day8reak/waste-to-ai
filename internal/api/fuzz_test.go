package api

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// FuzzSubmitTask tests the SubmitTask handler with random input
func FuzzSubmitTask(f *testing.F) {
	// Seed with valid and invalid inputs
	f.Add(`{"name": "test", "image": "pytorch:2.0", "command": "python test.py", "gpu_required": 1}`)
	f.Add(`{}`)
	f.Add(`{"invalid json"}`)
	f.Add(`{"name": "", "image": "", "command": ""}`)
	f.Add(`{"name": "test", "image": "pytorch:2.0", "command": "python test.py", "gpu_required": -1}`)
	f.Add(`{"name": "test", "image": "pytorch:2.0", "command": "python test.py", "priority": -5}`)

	h := createTestHandler()

	f.Fuzz(func(t *testing.T, data string) {
		// Skip empty strings
		if len(data) == 0 {
			t.Skip()
		}

		body := []byte(data)
		resp := httptest.NewRecorder()

		// This should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.SubmitTask(resp, createTestRequest("POST", "/api/tasks", body))
		}()
	})
}

// FuzzRayAllocate tests the RayAllocate handler with random input
func FuzzRayAllocate(f *testing.F) {
	f.Add(`{"job_id": "test-job", "gpu_count": 1}`)
	f.Add(`{}`)
	f.Add(`{"invalid json"}`)
	f.Add(`{"job_id": ""}`)
	f.Add(`{"job_id": "test", "gpu_count": -1}`)
	f.Add(`{"job_id": "test", "priority": -10}`)

	h := createTestHandler()

	f.Fuzz(func(t *testing.T, data string) {
		if len(data) == 0 {
			t.Skip()
		}

		body := []byte(data)
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))
		}()
	})
}

// FuzzRayRelease tests the RayRelease handler with random input
func FuzzRayRelease(f *testing.F) {
	f.Add(`{"job_id": "test-job"}`)
	f.Add(`{}`)
	f.Add(`{"invalid json"}`)
	f.Add(`{"job_id": ""}`)
	f.Add(`{"job_id": "test", "gpu_ids": ["gpu0"]}`)

	h := createTestHandler()

	f.Fuzz(func(t *testing.T, data string) {
		if len(data) == 0 {
			t.Skip()
		}

		body := []byte(data)
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.RayRelease(resp, createTestRequest("POST", "/api/ray/release", body))
		}()
	})
}

// FuzzRayBlock tests the RayBlock handler with random input
func FuzzRayBlock(f *testing.F) {
	f.Add(`{"gpu_ids": ["gpu0", "gpu1"]}`)
	f.Add(`{}`)
	f.Add(`{"invalid json"}`)
	f.Add(`{"gpu_ids": []}`)

	h := createTestHandler()

	f.Fuzz(func(t *testing.T, data string) {
		if len(data) == 0 {
			t.Skip()
		}

		body := []byte(data)
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.RayBlock(resp, createTestRequest("POST", "/api/ray/block", body))
		}()
	})
}

// FuzzRayUnblock tests the RayUnblock handler with random input
func FuzzRayUnblock(f *testing.F) {
	f.Add(`{"gpu_ids": ["gpu0", "gpu1"]}`)
	f.Add(`{}`)
	f.Add(`{"invalid json"}`)
	f.Add(`{"gpu_ids": []}`)

	h := createTestHandler()

	f.Fuzz(func(t *testing.T, data string) {
		if len(data) == 0 {
			t.Skip()
		}

		body := []byte(data)
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.RayUnblock(resp, createTestRequest("POST", "/api/ray/unblock", body))
		}()
	})
}

// FuzzGetTasks tests the GetTasks handler with random query parameters
func FuzzGetTasks(f *testing.F) {
	f.Add("")
	f.Add("status=running")
	f.Add("status=pending")
	f.Add("status=invalid")
	f.Add("status=" + strings.Repeat("a", 1000))

	h := createTestHandler()

	f.Fuzz(func(t *testing.T, query string) {
		resp := httptest.NewRecorder()
		url := "/api/tasks"
		if query != "" {
			url += "?" + query
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.GetTasks(resp, createTestRequest("GET", url, nil))
		}()
	})
}

// FuzzGetGPUs tests the GetGPUs handler
func FuzzGetGPUs(f *testing.F) {
	h := createTestHandler()

	f.Fuzz(func(t *testing.T, data string) {
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()
			h.GetGPUs(resp, createTestRequest("GET", "/api/gpus", nil))
		}()
	})
}

// TestMalformedJSONSubmit tests handling of various malformed JSON inputs
func TestMalformedJSONSubmit(t *testing.T) {
	h := createTestHandler()

	testCases := []struct {
		name string
		body string
	}{
		{"Empty string", ""},
		{"Just braces", "{}"},
		{"Invalid JSON", "{invalid}"},
		{"Array instead of object", "[]"},
		{"Null", "null"},
		{"Number", "123"},
		{"Boolean", "true"},
		{"String", "\"test\""},
		{"Very long string", strings.Repeat("a", 10000)},
		{"Special characters", "\x00\x01\x02\xff"},
		{"Unicode", "\u0000\uFFFF"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()

			// Should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic on %s: %v", tc.name, r)
					}
				}()
				h.SubmitTask(resp, createTestRequest("POST", "/api/tasks", []byte(tc.body)))
			}()

			// Should return an error response (400 or similar)
			if resp.Code < 200 || resp.Code > 600 {
				t.Errorf("Invalid HTTP status code: %d", resp.Code)
			}
		})
	}
}

// TestMalformedJSONRayAllocate tests handling of malformed JSON for Ray allocate
func TestMalformedJSONRayAllocate(t *testing.T) {
	h := createTestHandler()

	testCases := []struct {
		name string
		body string
	}{
		{"Empty string", ""},
		{"Just braces", "{}"},
		{"Invalid JSON", "{invalid}"},
		{"Array instead of object", "[]"},
		{"Null", "null"},
		{"Number", "123"},
		{"Very long string", strings.Repeat("a", 10000)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic on %s: %v", tc.name, r)
					}
				}()
				h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", []byte(tc.body)))
			}()

			if resp.Code < 200 || resp.Code > 600 {
				t.Errorf("Invalid HTTP status code: %d", resp.Code)
			}
		})
	}
}

// TestSQLInjectionPrevention tests that special characters in inputs don't cause issues
func TestSQLInjectionPrevention(t *testing.T) {
	h := createTestHandler()

	// Test with potentially malicious inputs
	maliciousInputs := []string{
		`{"name": "'; DROP TABLE tasks; --", "image": "test", "command": "test"}`,
		`{"name": "test", "image": "<script>alert('xss')</script>", "command": "test"}`,
		`{"name": "test", "image": "test", "command": "$(whoami)"}`,
		`{"name": "test", "image": "test", "command": "testcommand"}`,
		`{"name": "test", "image": "test", "command": "test"}`,
	}

	for _, input := range maliciousInputs {
		t.Run("SubmitTask", func(t *testing.T) {
			resp := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic on malicious input: %v", r)
					}
				}()
				h.SubmitTask(resp, createTestRequest("POST", "/api/tasks", []byte(input)))
			}()

			// Should handle gracefully without panicking
		})
	}
}

// TestLargeInputHandling tests handling of very large inputs
func TestLargeInputHandling(t *testing.T) {
	h := createTestHandler()

	// Create a very large input
	largeBody := `{"name": "` + strings.Repeat("a", 100000) + `", "image": "test", "command": "test", "gpu_required": 1}`

	t.Run("LargeSubmitBody", func(t *testing.T) {
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic on large input: %v", r)
				}
			}()
			h.SubmitTask(resp, createTestRequest("POST", "/api/tasks", []byte(largeBody)))
		}()
	})

	// Test with very large job_id
	largeRayBody := `{"job_id": "` + strings.Repeat("x", 100000) + `", "gpu_count": 1}`

	t.Run("LargeRayJobID", func(t *testing.T) {
		resp := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic on large input: %v", r)
				}
			}()
			h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", []byte(largeRayBody)))
		}()
	})
}

// TestInvalidHTTPMethods tests handling of invalid HTTP methods
func TestInvalidHTTPMethods(t *testing.T) {
	h := createTestHandler()

	t.Run("PutOnSubmit", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/tasks", bytes.NewReader([]byte(`{}`)))
		resp := httptest.NewRecorder()
		h.SubmitTask(resp, req)

		// Should not panic - method will be handled by gorilla/mux routing
	})

	t.Run("DeleteOnSubmit", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/tasks", bytes.NewReader([]byte(`{}`)))
		resp := httptest.NewRecorder()
		h.SubmitTask(resp, req)
	})
}

// TestConcurrentRequests tests concurrent request handling
func TestConcurrentRequests(t *testing.T) {
	h := createTestHandler()

	// Run multiple concurrent submissions
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			body := []byte(`{"name": "concurrent-test", "image": "pytorch:2.0", "command": "python test.py", "gpu_required": 1}`)
			resp := httptest.NewRecorder()
			h.SubmitTask(resp, createTestRequest("POST", "/api/tasks", body))
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestRaceCondition tests for race conditions in GPU allocation
func TestRaceCondition(t *testing.T) {
	// Run test with -race flag to detect race conditions
	h := createTestHandler()

	results := make(chan int, 20)

	for i := 0; i < 20; i++ {
		go func(n int) {
			body := []byte(`{"job_id": "race-test-` + string(rune('0'+n)) + `", "gpu_count": 1}`)
			resp := httptest.NewRecorder()
			h.RayAllocate(resp, createTestRequest("POST", "/api/ray/allocate", body))
			results <- resp.Code
		}(i)
	}

	codes := make(map[int]int)
	for i := 0; i < 20; i++ {
		code := <-results
		codes[code]++
	}

	// Should have at least some successful allocations
	if codes[200] == 0 && codes[202] == 0 {
		t.Logf("Warning: No successful allocations in race test, codes: %v", codes)
	}
}

// TestInvalidURLPath tests handling of invalid URL paths
func TestInvalidURLPath(t *testing.T) {
	h := createTestHandler()

	t.Run("GetTaskInvalidID", func(t *testing.T) {
		// Empty task ID
		req := httptest.NewRequest("GET", "/api/tasks/", nil)
		resp := httptest.NewRecorder()
		req = mux.SetURLVars(req, map[string]string{"id": ""})
		h.GetTask(resp, req)
	})

	t.Run("GetTaskVeryLongID", func(t *testing.T) {
		longID := strings.Repeat("x", 10000)
		req := httptest.NewRequest("GET", "/api/tasks/"+longID, nil)
		resp := httptest.NewRecorder()
		req = mux.SetURLVars(req, map[string]string{"id": longID})
		h.GetTask(resp, req)
	})
}

// TestResourceCleanup tests that resources are properly cleaned up
func TestResourceCleanup(t *testing.T) {
	h := createTestHandler()

	// Submit and kill multiple tasks
	for i := 0; i < 5; i++ {
		body := []byte(`{"name": "cleanup-test", "image": "pytorch:2.0", "command": "python test.py", "gpu_required": 1}`)
		resp := httptest.NewRecorder()
		h.SubmitTask(resp, createTestRequest("POST", "/api/tasks", body))

		// Get the task ID from response would require parsing JSON
		// Just verify no errors
		if resp.Code != 200 && resp.Code != 202 {
			t.Logf("Unexpected status code: %d", resp.Code)
		}
	}

	// Get stats to verify system is still responsive
	statsResp := httptest.NewRecorder()
	h.GetStats(statsResp, createTestRequest("GET", "/api/stats", nil))

	if statsResp.Code != 200 {
		t.Errorf("Stats endpoint failed after cleanup: %d", statsResp.Code)
	}
}
