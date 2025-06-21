# MCP Showcase Example

This example demonstrates all the major features of the MCP (Model Context Protocol) Go SDK in a comprehensive "Document Analyzer" server.

## Features Demonstrated

### ✅ Tools with Annotations
- **analyze_document**: A tool that analyzes text documents
  - Uses `ReadOnlyHint` annotation to indicate it doesn't modify the environment
  - Takes typed arguments with validation (text content and optional language)
  - Demonstrates proper error handling

### ✅ Sampling Requests  
- The analyze tool makes sampling requests to the client to convert analysis results to markdown format
- Uses `ServerSession.CreateMessage()` to request LLM processing
- Gracefully falls back to plain text if sampling fails

### ✅ Progress Notifications
- Real-time progress updates during document analysis
- Uses `ServerSession.NotifyProgress()` with progress tokens
- Shows completion percentage (0/4 to 4/4 steps)

### ✅ Dynamic Tool Registration
- The server starts with one tool and dynamically registers a second tool (`greet`) when the analyze tool is first called
- Demonstrates `AddTools()` and automatic tool list change notifications

### ✅ Prompts with Arguments
- **analysis_prompt**: A prompt that generates analysis instructions based on type
- Supports different analysis types: general, technical, sentiment, summary
- Uses prompt arguments for customization

### ✅ Resources
- **welcome**: Static welcome message resource
- **stats**: Dynamic statistics resource that updates based on server state
- Uses custom URI scheme (`analysis:`) for embedded resources

### ✅ Resource Templates  
- **analysis_templates**: URI template that provides analysis templates
- Pattern: `template:/{type}` matches URLs like `template:/technical`
- Returns different markdown templates based on the analysis type
- Shares the same argument structure as the prompt for consistency

### ✅ Real-time Capabilities
- Progress notifications during long-running operations
- Tool list change notifications when new tools are registered
- Dynamic resource content (statistics update in real-time)

## Usage

### Stdio Mode (default)
```bash
go run ./examples/showcase
```

### HTTP Mode
```bash
go run ./examples/showcase -http :8080
```

## Example Interactions

### 1. Analyze a Document
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "analyze_document",
    "arguments": {
      "text": "This is a sample document about artificial intelligence and machine learning.",
      "language": "en"
    }
  }
}
```

### 2. Get Analysis Prompt
```json
{
  "jsonrpc": "2.0", 
  "id": 2,
  "method": "prompts/get",
  "params": {
    "name": "analysis_prompt",
    "arguments": {
      "type": "technical"
    }
  }
}
```

### 3. Read Welcome Resource
```json
{
  "jsonrpc": "2.0",
  "id": 3, 
  "method": "resources/read",
  "params": {
    "uri": "analysis:welcome"
  }
}
```

### 4. Get Template Resource
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/read", 
  "params": {
    "uri": "template:/sentiment"
  }
}
```

## Architecture Highlights

### Idiomatic Go Design
- Uses typed handlers with reflection-based schema generation
- Follows Go naming conventions and error handling patterns
- Leverages Go's concurrency features for real-time updates

### MCP Protocol Compliance
- Implements all major MCP primitives correctly
- Handles progress tokens and notifications properly
- Uses appropriate MIME types and URI schemes

### Production Readiness
- Proper error handling and fallbacks
- Thread-safe operations using atomic operations
- Structured logging for debugging

## Testing

Run the included tests:
```bash
cd examples/showcase
go test -v
```

## Notes

- **Completion**: The MCP completion feature is mentioned in the protocol but not yet fully implemented in the current SDK version, so it's not included in this example.
- **Sampling**: Requires a client that supports sampling requests. The server gracefully falls back if sampling fails.
- **Progress**: Progress notifications demonstrate the capability but require a client that handles them.

This example serves as both a learning resource and a validation of the MCP Go SDK's capabilities.