package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestRouter() (*gin.Engine, *ExternalMCPHandler, string) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// English note.
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		panic(err)
	}
	tmpFile.WriteString("server:\n  host: 0.0.0.0\n  port: 8080\n")
	tmpFile.Close()
	configPath := tmpFile.Name()

	logger := zap.NewNop()
	manager := mcp.NewExternalMCPManager(logger)
	cfg := &config.Config{
		ExternalMCP: config.ExternalMCPConfig{
			Servers: make(map[string]config.ExternalMCPServerConfig),
		},
	}

	handler := NewExternalMCPHandler(manager, cfg, configPath, logger)

	api := router.Group("/api")
	api.GET("/external-mcp", handler.GetExternalMCPs)
	api.GET("/external-mcp/stats", handler.GetExternalMCPStats)
	api.GET("/external-mcp/:name", handler.GetExternalMCP)
	api.PUT("/external-mcp/:name", handler.AddOrUpdateExternalMCP)
	api.DELETE("/external-mcp/:name", handler.DeleteExternalMCP)
	api.POST("/external-mcp/:name/start", handler.StartExternalMCP)
	api.POST("/external-mcp/:name/stop", handler.StopExternalMCP)

	return router, handler, configPath
}

func cleanupTestConfig(configPath string) {
	os.Remove(configPath)
	os.Remove(configPath + ".backup")
}

func TestExternalMCPHandler_AddOrUpdateExternalMCP_Stdio(t *testing.T) {
	router, _, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	// English note.
	configJSON := `{
		"command": "python3",
		"args": ["/path/to/script.py", "--server", "http://example.com"],
		"description": "Test stdio MCP",
		"timeout": 300,
		"enabled": true
	}`

	var configObj config.ExternalMCPServerConfig
	if err := json.Unmarshal([]byte(configJSON), &configObj); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	reqBody := AddOrUpdateExternalMCPRequest{
		Config: configObj,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/external-mcp/test-stdio", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w.Code, w.Body.String())
	}

	// English note.
	req2 := httptest.NewRequest("GET", "/api/external-mcp/test-stdio", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w2.Code, w2.Body.String())
	}

	var response ExternalMCPResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &response); err != nil {
		t.Fatalf(": %v", err)
	}

	if response.Config.Command != "python3" {
		t.Errorf("commandpython3，%s", response.Config.Command)
	}
	if len(response.Config.Args) != 3 {
		t.Errorf("args3，%d", len(response.Config.Args))
	}
	if response.Config.Description != "Test stdio MCP" {
		t.Errorf("description'Test stdio MCP'，%s", response.Config.Description)
	}
	if response.Config.Timeout != 300 {
		t.Errorf("timeout300，%d", response.Config.Timeout)
	}
	if !response.Config.Enabled {
		t.Error("enabledtrue")
	}
}

func TestExternalMCPHandler_AddOrUpdateExternalMCP_HTTP(t *testing.T) {
	router, _, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	// English note.
	configJSON := `{
		"transport": "http",
		"url": "http://127.0.0.1:8081/mcp",
		"enabled": true
	}`

	var configObj config.ExternalMCPServerConfig
	if err := json.Unmarshal([]byte(configJSON), &configObj); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	reqBody := AddOrUpdateExternalMCPRequest{
		Config: configObj,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/external-mcp/test-http", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w.Code, w.Body.String())
	}

	// English note.
	req2 := httptest.NewRequest("GET", "/api/external-mcp/test-http", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w2.Code, w2.Body.String())
	}

	var response ExternalMCPResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &response); err != nil {
		t.Fatalf(": %v", err)
	}

	if response.Config.Transport != "http" {
		t.Errorf("transporthttp，%s", response.Config.Transport)
	}
	if response.Config.URL != "http://127.0.0.1:8081/mcp" {
		t.Errorf("url'http://127.0.0.1:8081/mcp'，%s", response.Config.URL)
	}
	if !response.Config.Enabled {
		t.Error("enabledtrue")
	}
}

func TestExternalMCPHandler_AddOrUpdateExternalMCP_InvalidConfig(t *testing.T) {
	router, _, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	testCases := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name:        "commandurl",
			configJSON:  `{"enabled": true}`,
			expectedErr: "command（stdio）url（http/sse）",
		},
		{
			name:        "stdiocommand",
			configJSON:  `{"args": ["test"], "enabled": true}`,
			expectedErr: "stdiocommand",
		},
		{
			name:        "httpurl",
			configJSON:  `{"transport": "http", "enabled": true}`,
			expectedErr: "HTTPURL",
		},
		{
			name:        "transport",
			configJSON:  `{"transport": "invalid", "enabled": true}`,
			expectedErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var configObj config.ExternalMCPServerConfig
			if err := json.Unmarshal([]byte(tc.configJSON), &configObj); err != nil {
				t.Fatalf("JSON: %v", err)
			}

			reqBody := AddOrUpdateExternalMCPRequest{
				Config: configObj,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest("PUT", "/api/external-mcp/test-invalid", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("400，%d: %s", w.Code, w.Body.String())
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf(": %v", err)
			}

			errorMsg := response["error"].(string)
			// English note.
			if tc.name == "stdiocommand" {
				if !strings.Contains(errorMsg, "stdio") && !strings.Contains(errorMsg, "command") {
					t.Errorf("'stdio''command'，'%s'", errorMsg)
				}
			} else if !strings.Contains(errorMsg, tc.expectedErr) {
				t.Errorf("'%s'，'%s'", tc.expectedErr, errorMsg)
			}
		})
	}
}

func TestExternalMCPHandler_DeleteExternalMCP(t *testing.T) {
	router, handler, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	// English note.
	configObj := config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	}
	handler.manager.AddOrUpdateConfig("test-delete", configObj)

	// English note.
	req := httptest.NewRequest("DELETE", "/api/external-mcp/test-delete", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w.Code, w.Body.String())
	}

	// English note.
	req2 := httptest.NewRequest("GET", "/api/external-mcp/test-delete", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("404，%d: %s", w2.Code, w2.Body.String())
	}
}

func TestExternalMCPHandler_GetExternalMCPs(t *testing.T) {
	router, handler, _ := setupTestRouter()

	// English note.
	handler.manager.AddOrUpdateConfig("test1", config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	})
	handler.manager.AddOrUpdateConfig("test2", config.ExternalMCPServerConfig{
		URL:     "http://127.0.0.1:8081/mcp",
		Enabled: false,
	})

	req := httptest.NewRequest("GET", "/api/external-mcp", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf(": %v", err)
	}

	servers := response["servers"].(map[string]interface{})
	if len(servers) != 2 {
		t.Errorf("2，%d", len(servers))
	}
	if _, ok := servers["test1"]; !ok {
		t.Error("test1")
	}
	if _, ok := servers["test2"]; !ok {
		t.Error("test2")
	}

	stats := response["stats"].(map[string]interface{})
	if int(stats["total"].(float64)) != 2 {
		t.Errorf("2，%d", int(stats["total"].(float64)))
	}
}

func TestExternalMCPHandler_GetExternalMCPStats(t *testing.T) {
	router, handler, _ := setupTestRouter()

	// English note.
	handler.manager.AddOrUpdateConfig("enabled1", config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	})
	handler.manager.AddOrUpdateConfig("enabled2", config.ExternalMCPServerConfig{
		URL:     "http://127.0.0.1:8081/mcp",
		Enabled: true,
	})
	handler.manager.AddOrUpdateConfig("disabled1", config.ExternalMCPServerConfig{
		Command:  "python3",
		Enabled:  false,
		Disabled: true,
	})

	req := httptest.NewRequest("GET", "/api/external-mcp/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf(": %v", err)
	}

	if int(stats["total"].(float64)) != 3 {
		t.Errorf("3，%d", int(stats["total"].(float64)))
	}
	if int(stats["enabled"].(float64)) != 2 {
		t.Errorf("2，%d", int(stats["enabled"].(float64)))
	}
	if int(stats["disabled"].(float64)) != 1 {
		t.Errorf("1，%d", int(stats["disabled"].(float64)))
	}
}

func TestExternalMCPHandler_StartStopExternalMCP(t *testing.T) {
	router, handler, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	// English note.
	handler.manager.AddOrUpdateConfig("test-start-stop", config.ExternalMCPServerConfig{
		Command:  "python3",
		Enabled:  false,
		Disabled: true,
	})

	// English note.
	req := httptest.NewRequest("POST", "/api/external-mcp/test-start-stop/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// English note.
	if w.Code != http.StatusOK {
		// English note.
		if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
			t.Errorf("200/400/500，%d: %s", w.Code, w.Body.String())
		}
	}

	// English note.
	req2 := httptest.NewRequest("POST", "/api/external-mcp/test-start-stop/stop", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("200，%d: %s", w2.Code, w2.Body.String())
	}
}

func TestExternalMCPHandler_GetExternalMCP_NotFound(t *testing.T) {
	router, _, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/external-mcp/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("404，%d: %s", w.Code, w.Body.String())
	}
}

func TestExternalMCPHandler_DeleteExternalMCP_NotFound(t *testing.T) {
	router, _, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	req := httptest.NewRequest("DELETE", "/api/external-mcp/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// English note.
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Errorf("404200，%d: %s", w.Code, w.Body.String())
	}
}

func TestExternalMCPHandler_AddOrUpdateExternalMCP_EmptyName(t *testing.T) {
	router, _, _ := setupTestRouter()

	configObj := config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	}

	reqBody := AddOrUpdateExternalMCPRequest{
		Config: configObj,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/external-mcp/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// English note.
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("404400，%d: %s", w.Code, w.Body.String())
	}
}

func TestExternalMCPHandler_AddOrUpdateExternalMCP_InvalidJSON(t *testing.T) {
	router, _, _ := setupTestRouter()

	// English note.
	body := []byte(`{"config": invalid json}`)
	req := httptest.NewRequest("PUT", "/api/external-mcp/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("400，%d: %s", w.Code, w.Body.String())
	}
}

func TestExternalMCPHandler_UpdateExistingConfig(t *testing.T) {
	router, handler, configPath := setupTestRouter()
	defer cleanupTestConfig(configPath)

	// English note.
	config1 := config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	}
	handler.manager.AddOrUpdateConfig("test-update", config1)

	// English note.
	config2 := config.ExternalMCPServerConfig{
		URL:     "http://127.0.0.1:8081/mcp",
		Enabled: true,
	}

	reqBody := AddOrUpdateExternalMCPRequest{
		Config: config2,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/external-mcp/test-update", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w.Code, w.Body.String())
	}

	// English note.
	req2 := httptest.NewRequest("GET", "/api/external-mcp/test-update", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("200，%d: %s", w2.Code, w2.Body.String())
	}

	var response ExternalMCPResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &response); err != nil {
		t.Fatalf(": %v", err)
	}

	if response.Config.URL != "http://127.0.0.1:8081/mcp" {
		t.Errorf("url'http://127.0.0.1:8081/mcp'，%s", response.Config.URL)
	}
	if response.Config.Command != "" {
		t.Errorf("command，%s", response.Config.Command)
	}
}
