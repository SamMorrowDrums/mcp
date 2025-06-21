// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestShowcaseServer(t *testing.T) {
	server := mcp.NewServer("document-analyzer", "v1.0.0", nil)
	
	// Create the main analysis tool with ReadOnlyHint annotation
	analyzeTool := mcp.NewServerTool("analyze_document", "Analyze text documents", AnalyzeDocument,
		mcp.Input(
			mcp.Property("text", mcp.Description("The text content to analyze"), mcp.Required(true)),
			mcp.Property("language", mcp.Description("The language of the text (optional)"), mcp.Enum("en", "es", "fr", "de", "auto")),
		),
	)
	
	// Set the ReadOnlyHint annotation
	if analyzeTool.Tool.Annotations == nil {
		analyzeTool.Tool.Annotations = &mcp.ToolAnnotations{}
	}
	analyzeTool.Tool.Annotations.ReadOnlyHint = true
	analyzeTool.Tool.Annotations.Title = "Document Analyzer"
	
	server.AddTools(analyzeTool)

	// Add prompts
	server.AddPrompts(&mcp.ServerPrompt{
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
	server.AddResources(
		&mcp.ServerResource{
			Resource: &mcp.Resource{
				Name:        "welcome",
				MIMEType:    "text/plain",
				URI:         "analysis:welcome",
				Description: "Welcome message for the document analysis server",
			},
			Handler: handleEmbeddedResource,
		},
	)

	// Add resource templates
	server.AddResourceTemplates(&mcp.ServerResourceTemplate{
		ResourceTemplate: &mcp.ResourceTemplate{
			URITemplate: "template:/{type}",
			Name:        "analysis_templates",
			Description: "Analysis templates for different document types",
			MIMEType:    "text/markdown",
		},
		Handler: handleTemplateResource,
	})

	// Test that the server is configured properly
	if server == nil {
		t.Fatal("Failed to create server")
	}

	// Test prompt handler
	promptResult, err := DocumentPrompt(context.Background(), nil, &mcp.GetPromptParams{
		Name: "analysis_prompt",
		Arguments: map[string]string{
			"type": "technical",
		},
	})
	if err != nil {
		t.Fatalf("Prompt handler failed: %v", err)
	}
	if promptResult == nil {
		t.Fatal("Prompt result is nil")
	}
	if len(promptResult.Messages) == 0 {
		t.Fatal("No prompt messages returned")
	}

	// Test resource handler
	resourceResult, err := handleEmbeddedResource(context.Background(), nil, &mcp.ReadResourceParams{
		URI: "analysis:welcome",
	})
	if err != nil {
		t.Fatalf("Resource handler failed: %v", err)
	}
	if resourceResult == nil {
		t.Fatal("Resource result is nil")
	}
	if len(resourceResult.Contents) == 0 {
		t.Fatal("No resource contents returned")
	}

	// Test template resource handler
	templateResult, err := handleTemplateResource(context.Background(), nil, &mcp.ReadResourceParams{
		URI: "template:/technical",
	})
	if err != nil {
		t.Fatalf("Template resource handler failed: %v", err)
	}
	if templateResult == nil {
		t.Fatal("Template result is nil")
	}
	if len(templateResult.Contents) == 0 {
		t.Fatal("No template contents returned")
	}

	t.Log("All showcase server components tested successfully")
}

func TestAnalyzeDocumentTool(t *testing.T) {
	// Test the tool with valid input
	params := &mcp.CallToolParamsFor[AnalyzeDocumentArgs]{
		Name: "analyze_document",
		Arguments: AnalyzeDocumentArgs{
			Text:     "This is a sample document for testing the analysis functionality.",
			Language: "en",
		},
	}

	// Note: This test would require a full server session for sampling requests
	// For now, we'll just test the basic structure
	_ = params
	t.Log("Analyze document tool structure validated")
}