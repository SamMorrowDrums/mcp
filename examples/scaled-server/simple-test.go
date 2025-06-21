// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	serverAddr = flag.String("server", "http://localhost:8080", "MCP server address")
	user       = flag.String("user", "testuser", "User ID for testing")
)

func main() {
	flag.Parse()

	// Test basic connectivity
	if err := testHealthCheck(); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Println("✓ Health check passed")

	// Test MCP connection using the proper client transport
	if err := testMCPConnection(); err != nil {
		log.Fatalf("MCP connection test failed: %v", err)
	}
	fmt.Println("✓ MCP connection test passed")

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

func testMCPConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create HTTP client with authentication
	httpClient := &http.Client{}
	
	// Add authentication to all requests
	originalTransport := httpClient.Transport
	if originalTransport == nil {
		originalTransport = http.DefaultTransport
	}
	
	httpClient.Transport = &AuthTransport{
		Transport: originalTransport,
		UserID:    *user,
	}

	// Create MCP client with streamable transport
	transport := mcp.NewStreamableClientTransport(*serverAddr+"/mcp", &mcp.StreamableClientTransportOptions{
		HTTPClient: httpClient,
	})
	
	client := mcp.NewClient("test-client", "v1.0.0", nil)
	session, err := client.Connect(ctx, transport)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer session.Close()

	fmt.Println("  ✓ Connected to MCP server")

	// Test tools/list
	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	fmt.Printf("  ✓ Listed %d tools\n", len(tools.Tools))

	// Test tools/call - greet
	greetResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "greet",
		Arguments: map[string]any{
			"name": "World",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to call greet tool: %w", err)
	}
	fmt.Printf("  ✓ Greet tool result: %s\n", greetResult.Content[0].Text)

	// Test tools/call - calculate
	calcResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "calculate",
		Arguments: map[string]any{
			"operation": "add",
			"a":         10.0,
			"b":         5.0,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to call calculate tool: %w", err)
	}
	fmt.Printf("  ✓ Calculate tool result: %s\n", calcResult.Content[0].Text)

	// Test prompts/list
	prompts, err := session.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		return fmt.Errorf("failed to list prompts: %w", err)
	}
	fmt.Printf("  ✓ Listed %d prompts\n", len(prompts.Prompts))

	// Test resources/list
	resources, err := session.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}
	fmt.Printf("  ✓ Listed %d resources\n", len(resources.Resources))

	return nil
}

// AuthTransport wraps an http.RoundTripper to add authentication headers
type AuthTransport struct {
	Transport http.RoundTripper
	UserID    string
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	newReq := req.Clone(req.Context())
	
	// Add authentication header
	newReq.Header.Set("Authorization", fmt.Sprintf("Bearer user:%s", t.UserID))
	
	return t.Transport.RoundTrip(newReq)
}