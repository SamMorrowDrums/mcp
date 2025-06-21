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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
)

var (
	httpAddr    = flag.String("http", ":8080", "HTTP server address")
	redisAddr   = flag.String("redis", "localhost:6379", "Redis server address")
	redisDB     = flag.Int("redis-db", 0, "Redis database number")
	redisPrefix = flag.String("redis-prefix", "mcp", "Redis key prefix")
)

// Tool handlers
type GreetArgs struct {
	Name string `json:"name"`
}

func Greet(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GreetArgs]) (*mcp.CallToolResultFor[any], error) {
	return &mcp.CallToolResultFor[any]{
		Content: []*mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Hello %s from distributed server!", params.Arguments.Name)),
		},
	}, nil
}

type CalculateArgs struct {
	Operation string  `json:"operation"`
	A         float64 `json:"a"`
	B         float64 `json:"b"`
}

func Calculate(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[CalculateArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	var result float64
	switch args.Operation {
	case "add":
		result = args.A + args.B
	case "subtract":
		result = args.A - args.B
	case "multiply":
		result = args.A * args.B
	case "divide":
		if args.B == 0 {
			return &mcp.CallToolResultFor[any]{
				Content: []*mcp.Content{
					mcp.NewTextContent("Error: Division by zero"),
				},
				IsError: true,
			}, nil
		}
		result = args.A / args.B
	default:
		return &mcp.CallToolResultFor[any]{
			Content: []*mcp.Content{
				mcp.NewTextContent("Error: Unknown operation"),
			},
			IsError: true,
		}, nil
	}
	
	return &mcp.CallToolResultFor[any]{
		Content: []*mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Result: %g", result)),
		},
	}, nil
}

// Resource handler
func GetUserInfo(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	// In a real implementation, you could use the session context to get user info
	info := fmt.Sprintf("User info resource accessed at %s", time.Now().Format(time.RFC3339))
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			mcp.NewTextResourceContents(params.URI, "text/plain", info),
		},
	}, nil
}

// Prompt handler
func GetCodeReviewPrompt(ctx context.Context, ss *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	language := "Go"
	if langArg, ok := params.Arguments["language"]; ok {
		language = langArg
	}
	
	return &mcp.GetPromptResult{
		Description: "Code review prompt for distributed server",
		Messages: []*mcp.PromptMessage{
			{
				Role: "system",
				Content: mcp.NewTextContent("You are a code reviewer. Review the following code carefully."),
			},
			{
				Role: "user",
				Content: mcp.NewTextContent(fmt.Sprintf("Please review this %s code for best practices and potential issues.", language)),
			},
		},
	}, nil
}

// createServer creates an MCP server instance for a user
func createServer(req *http.Request, user *AuthenticatedUser) *mcp.Server {
	// Create a new server instance per user for isolation
	serverName := fmt.Sprintf("scaled-server-%s", user.ID)
	server := mcp.NewServer(serverName, "v1.0.0", nil)
	
	// Add tools
	server.AddTools(
		mcp.NewServerTool("greet", "Greet a user", Greet, mcp.Input(
			mcp.Property("name", mcp.Description("the name to greet")),
		)),
		mcp.NewServerTool("calculate", "Perform calculations", Calculate, mcp.Input(
			mcp.Property("operation", mcp.Description("operation: add, subtract, multiply, divide")),
			mcp.Property("a", mcp.Description("first number")),
			mcp.Property("b", mcp.Description("second number")),
		)),
	)
	
	// Add prompts
	server.AddPrompts(&mcp.ServerPrompt{
		Prompt: &mcp.Prompt{
			Name:        "code-review",
			Description: "Get a code review prompt",
		},
		Handler: GetCodeReviewPrompt,
	})
	
	// Add resources
	server.AddResources(&mcp.ServerResource{
		Resource: &mcp.Resource{
			Name:        "user-info",
			Description: "Get user information",
			URI:         fmt.Sprintf("user://%s/info", user.ID),
			MIMEType:    "text/plain",
		},
		Handler: GetUserInfo,
	})
	
	return server
}

func main() {
	flag.Parse()
	
	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
		DB:   *redisDB,
	})
	
	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Connected to Redis at %s", *redisAddr)
	
	// Create session store and message queue
	sessionStore := NewRedisSessionStore(rdb, *redisPrefix, time.Hour)
	messageQueue := NewRedisMessageQueue(rdb, *redisPrefix)
	authProvider := NewMockAuthProvider()
	
	// Create distributed handler
	handler, err := NewDistributedStreamableHTTPHandler(
		createServer,
		sessionStore,
		messageQueue,
		authProvider,
		&DistributedStreamableHTTPOptions{
			SessionTTL: time.Hour,
		},
	)
	if err != nil {
		log.Fatalf("Failed to create distributed handler: %v", err)
	}
	defer handler.Close()
	
	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	
	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK - Server healthy\n")
	})
	
	// Add status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"running","redis_addr":"%s","server_addr":"%s"}`, *redisAddr, *httpAddr)
	})
	
	server := &http.Server{
		Addr:    *httpAddr,
		Handler: mux,
	}
	
	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()
	
	log.Printf("Starting MCP distributed server on %s", *httpAddr)
	log.Printf("MCP endpoint: http://%s/mcp", *httpAddr)
	log.Printf("Health check: http://%s/health", *httpAddr)
	log.Printf("Status: http://%s/status", *httpAddr)
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
	
	log.Println("Server stopped")
}