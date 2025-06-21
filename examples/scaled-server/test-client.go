// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var (
	serverAddr = flag.String("server", "http://localhost:8080", "MCP server address")
	user       = flag.String("user", "testuser", "User ID for testing")
)

type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      interface{}    `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      interface{}    `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	flag.Parse()

	// Test basic connectivity
	if err := testHealthCheck(); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Println("✓ Health check passed")

	// Test MCP initialize
	sessionID, err := testInitialize()
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	fmt.Printf("✓ Initialize successful, session ID: %s\n", sessionID)

	// Test tools/list
	if err := testListTools(sessionID); err != nil {
		log.Fatalf("List tools failed: %v", err)
	}
	fmt.Println("✓ List tools successful")

	// Test tools/call - greet
	if err := testCallGreet(sessionID); err != nil {
		log.Fatalf("Call greet tool failed: %v", err)
	}
	fmt.Println("✓ Call greet tool successful")

	// Test tools/call - calculate
	if err := testCallCalculate(sessionID); err != nil {
		log.Fatalf("Call calculate tool failed: %v", err)
	}
	fmt.Println("✓ Call calculate tool successful")

	// Test prompts/list
	if err := testListPrompts(sessionID); err != nil {
		log.Fatalf("List prompts failed: %v", err)
	}
	fmt.Println("✓ List prompts successful")

	// Test resources/list
	if err := testListResources(sessionID); err != nil {
		log.Fatalf("List resources failed: %v", err)
	}
	fmt.Println("✓ List resources successful")

	fmt.Println("\n🎉 All tests passed!")
}

func testHealthCheck() error {
	resp, err := http.Get(*serverAddr + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

func testInitialize() (string, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	resp, sessionID, err := sendMCPRequest(req, "")
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	return sessionID, nil
}

func testListTools(sessionID string) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  map[string]any{},
	}

	resp, _, err := sendMCPRequest(req, sessionID)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	tools, ok := resp.Result["tools"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid tools response format")
	}

	fmt.Printf("  Found %d tools\n", len(tools))
	return nil
}

func testCallGreet(sessionID string) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "greet",
			"arguments": map[string]any{
				"name": "World",
			},
		},
	}

	resp, _, err := sendMCPRequest(req, sessionID)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("tools/call greet error: %s", resp.Error.Message)
	}

	fmt.Printf("  Greet response: %v\n", resp.Result)
	return nil
}

func testCallCalculate(sessionID string) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "calculate",
			"arguments": map[string]any{
				"operation": "add",
				"a":         10.0,
				"b":         5.0,
			},
		},
	}

	resp, _, err := sendMCPRequest(req, sessionID)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("tools/call calculate error: %s", resp.Error.Message)
	}

	fmt.Printf("  Calculate response: %v\n", resp.Result)
	return nil
}

func testListPrompts(sessionID string) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "prompts/list",
		Params:  map[string]any{},
	}

	resp, _, err := sendMCPRequest(req, sessionID)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("prompts/list error: %s", resp.Error.Message)
	}

	prompts, ok := resp.Result["prompts"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid prompts response format")
	}

	fmt.Printf("  Found %d prompts\n", len(prompts))
	return nil
}

func testListResources(sessionID string) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "resources/list",
		Params:  map[string]any{},
	}

	resp, _, err := sendMCPRequest(req, sessionID)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("resources/list error: %s", resp.Error.Message)
	}

	resources, ok := resp.Result["resources"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid resources response format")
	}

	fmt.Printf("  Found %d resources\n", len(resources))
	return nil
}

func sendMCPRequest(req JSONRPCRequest, sessionID string) (*JSONRPCResponse, string, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", *serverAddr+"/mcp", bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer user:%s", *user))

	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Get session ID from response if this is a new session
	newSessionID := resp.Header.Get("Mcp-Session-Id")
	if newSessionID == "" {
		newSessionID = sessionID
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &jsonResp, newSessionID, nil
}