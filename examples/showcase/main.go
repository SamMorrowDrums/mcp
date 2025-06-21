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
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var httpAddr = flag.String("http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")

// Global state for tracking tool calls
var (
	documentCount   atomic.Int64
	secondToolAdded atomic.Bool
)

// Document analysis arguments
type AnalyzeDocumentArgs struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
}

// Simple greeting arguments  
type GreetArgs struct {
	Name string `json:"name"`
}

// AnalyzeDocument is a tool that analyzes text documents and demonstrates:
// - ReadOnlyHint annotation
// - Progress notifications
// - Sampling requests
// - Dynamic tool registration
func AnalyzeDocument(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[AnalyzeDocumentArgs]) (*mcp.CallToolResultFor[struct{}], error) {
	// Send initial progress notification
	progressToken := fmt.Sprintf("analyze-%d", time.Now().UnixNano())
	err := ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: progressToken,
		Progress:      0,
		Total:         4,
	})
	if err != nil {
		log.Printf("Failed to send progress notification: %v", err)
	}

	// Step 1: Validate input
	text := strings.TrimSpace(params.Arguments.Text)
	if text == "" {
		return &mcp.CallToolResultFor[struct{}]{
			Content: []*mcp.Content{
				mcp.NewTextContent("Error: Text cannot be empty"),
			},
			IsError: true,
		}, nil
	}

	// Progress update
	ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: progressToken,
		Progress:      1,
		Total:         4,
	})

	// Step 2: Basic analysis
	wordCount := len(strings.Fields(text))
	charCount := len(text)
	language := params.Arguments.Language
	if language == "" {
		language = "unknown"
	}

	// Progress update  
	ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: progressToken,
		Progress:      2,
		Total:         4,
	})

	// Step 3: Use sampling to convert analysis to markdown
	analysisText := fmt.Sprintf("Document Analysis:\n- Word count: %d\n- Character count: %d\n- Language: %s\n- Sample: %s...", 
		wordCount, charCount, language, text[:min(50, len(text))])

	samplingResult, err := ss.CreateMessage(ctx, &mcp.CreateMessageParams{
		Messages: []*mcp.SamplingMessage{
			{
				Role: "user", 
				Content: &mcp.Content{
					Type: "text",
					Text: fmt.Sprintf("Convert this analysis to well-formatted markdown:\n\n%s", analysisText),
				},
			},
		},
		MaxTokens: 200,
	})

	var markdownResult string
	if err != nil {
		log.Printf("Sampling request failed: %v", err)
		markdownResult = analysisText // Fallback to plain text
	} else if samplingResult != nil && samplingResult.Content != nil {
		markdownResult = samplingResult.Content.Text
	} else {
		markdownResult = analysisText // Fallback
	}

	// Progress update
	ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: progressToken,
		Progress:      3,
		Total:         4,
	})

	// Step 4: Register second tool on first call
	if !secondToolAdded.Load() && secondToolAdded.CompareAndSwap(false, true) {
		// Add the second tool dynamically
		greetTool := mcp.NewServerTool("greet", "A simple greeting tool registered dynamically", SimpleGreet,
			mcp.Input(
				mcp.Property("name", mcp.Description("Name to greet"), mcp.Required(false)),
			),
		)
		globalServer.AddTools(greetTool)
		log.Println("Successfully registered second tool: greet")
	}

	// Increment document count
	docNum := documentCount.Add(1)

	// Final progress update
	ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: progressToken,
		Progress:      4,
		Total:         4,
	})

	return &mcp.CallToolResultFor[struct{}]{
		Content: []*mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Document #%d Analysis Complete:\n\n%s", docNum, markdownResult)),
		},
	}, nil
}

// SimpleGreet is a second tool that gets registered dynamically
func SimpleGreet(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GreetArgs]) (*mcp.CallToolResultFor[struct{}], error) {
	name := params.Arguments.Name
	if name == "" {
		name = "World"
	}
	
	return &mcp.CallToolResultFor[struct{}]{
		Content: []*mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Hello, %s! This tool was registered dynamically.", name)),
		},
	}, nil
}

// DocumentPrompt generates analysis prompts
func DocumentPrompt(ctx context.Context, ss *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	analysisType := "general"
	if val, ok := params.Arguments["type"]; ok {
		analysisType = val
	}

	var promptText string
	switch analysisType {
	case "technical":
		promptText = "Analyze this document focusing on technical aspects, terminology, and complexity."
	case "sentiment":
		promptText = "Analyze the sentiment and emotional tone of this document."
	case "summary":
		promptText = "Provide a concise summary of the key points in this document."
	default:
		promptText = "Perform a general analysis of this document including structure, content, and style."
	}

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Document analysis prompt for %s analysis", analysisType),
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.NewTextContent(promptText),
			},
		},
	}, nil
}

// Resource handlers
var analysisResults = map[string]string{
	"welcome": "Welcome to the Document Analysis MCP Server!\n\nThis server demonstrates all major MCP features including tools, prompts, resources, and real-time capabilities.",
	"stats":   "Server Statistics:\n- Documents analyzed: 0\n- Tools registered: 1\n- Resources available: 2",
}

func handleEmbeddedResource(_ context.Context, _ *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	u, err := url.Parse(params.URI)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "analysis" {
		return nil, fmt.Errorf("wrong scheme: %q", u.Scheme)
	}
	
	key := u.Opaque
	text, ok := analysisResults[key]
	if !ok {
		return nil, mcp.ResourceNotFoundError(params.URI)
	}
	
	// Update stats if requested
	if key == "stats" {
		count := documentCount.Load()
		tools := 1
		if secondToolAdded.Load() {
			tools = 2
		}
		text = fmt.Sprintf("Server Statistics:\n- Documents analyzed: %d\n- Tools registered: %d\n- Resources available: 2", count, tools)
	}
	
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			mcp.NewTextResourceContents(params.URI, "text/plain", text),
		},
	}, nil
}

func handleTemplateResource(_ context.Context, _ *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	u, err := url.Parse(params.URI)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "template" {
		return nil, fmt.Errorf("wrong scheme: %q", u.Scheme)
	}
	
	// Extract analysis type from path
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) == 0 {
		return nil, mcp.ResourceNotFoundError(params.URI)
	}
	
	analysisType := pathParts[0]
	
	var template string
	switch analysisType {
	case "technical":
		template = "# Technical Analysis Template\n\n## Overview\n[Document summary]\n\n## Technical Elements\n- Terminology: \n- Complexity Level: \n- Domain: \n\n## Recommendations\n[Analysis recommendations]"
	case "sentiment":
		template = "# Sentiment Analysis Template\n\n## Overall Sentiment\n[Positive/Negative/Neutral]\n\n## Emotional Indicators\n- Tone: \n- Key Emotions: \n- Confidence Level: \n\n## Supporting Evidence\n[Quotes and examples]"
	case "summary":
		template = "# Document Summary Template\n\n## Key Points\n1. \n2. \n3. \n\n## Main Themes\n- \n- \n\n## Conclusion\n[Summary conclusion]"
	default:
		template = "# General Analysis Template\n\n## Document Overview\n[Basic information]\n\n## Content Analysis\n[Content breakdown]\n\n## Structure and Style\n[Writing style and organization]\n\n## Key Insights\n[Important findings]"
	}
	
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			mcp.NewTextResourceContents(params.URI, "text/markdown", template),
		},
	}, nil
}

// Global server variable so we can add tools dynamically
var globalServer *mcp.Server

func main() {
	flag.Parse()

	globalServer = mcp.NewServer("document-analyzer", "v1.0.0", nil)
	
	// Create the main analysis tool with ReadOnlyHint annotation
	analyzeTool := mcp.NewServerTool("analyze_document", "Analyze text documents with progress tracking and markdown output", AnalyzeDocument,
		mcp.Input(
			mcp.Property("text", mcp.Description("The text content to analyze"), mcp.Required(true)),
			mcp.Property("language", mcp.Description("The language of the text (optional)"), mcp.Enum("en", "es", "fr", "de", "auto")),
		),
	)
	
	// Manually set the ReadOnlyHint annotation since it's not exposed as an option
	if analyzeTool.Tool.Annotations == nil {
		analyzeTool.Tool.Annotations = &mcp.ToolAnnotations{}
	}
	analyzeTool.Tool.Annotations.ReadOnlyHint = true
	analyzeTool.Tool.Annotations.Title = "Document Analyzer"
	
	globalServer.AddTools(analyzeTool)

	// Add prompts
	globalServer.AddPrompts(&mcp.ServerPrompt{
		Prompt: &mcp.Prompt{
			Name:        "analysis_prompt",
			Description: "Generate analysis prompts for different types of document analysis",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "type",
					Description: "Type of analysis to perform",
					Required:    false,
				},
			},
		},
		Handler: DocumentPrompt,
	})

	// Add basic resources
	globalServer.AddResources(
		&mcp.ServerResource{
			Resource: &mcp.Resource{
				Name:        "welcome",
				MIMEType:    "text/plain",
				URI:         "analysis:welcome",
				Description: "Welcome message for the document analysis server",
			},
			Handler: handleEmbeddedResource,
		},
		&mcp.ServerResource{
			Resource: &mcp.Resource{
				Name:        "stats",
				MIMEType:    "text/plain", 
				URI:         "analysis:stats",
				Description: "Server statistics and usage information",
			},
			Handler: handleEmbeddedResource,
		},
	)

	// Add resource templates
	globalServer.AddResourceTemplates(&mcp.ServerResourceTemplate{
		ResourceTemplate: &mcp.ResourceTemplate{
			URITemplate: "template:/{type}",
			Name:        "analysis_templates",
			Description: "Analysis templates for different document types",
			MIMEType:    "text/markdown",
		},
		Handler: handleTemplateResource,
	})

	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return globalServer
		}, nil)
		log.Printf("MCP Document Analysis server listening at %s", *httpAddr)
		http.ListenAndServe(*httpAddr, handler)
	} else {
		t := mcp.NewLoggingTransport(mcp.NewStdioTransport(), os.Stderr)
		if err := globalServer.Run(context.Background(), t); err != nil {
			log.Printf("Server failed: %v", err)
		}
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}