# Horizontally Scaled MCP Server

This example demonstrates a horizontally scalable Model Context Protocol (MCP) server that uses Redis for session storage and message queuing. Multiple server instances can run simultaneously without requiring session affinity.

## Features

- **Distributed Session Storage**: Uses Redis to store session data, allowing any server instance to handle requests for any session
- **User-based Session IDs**: Session IDs are prefixed with user IDs for security (`user_id:uuid`)
- **Bearer Token Authentication**: Simple mock authentication system (easily replaceable)
- **Message Queuing**: Redis-based message queuing for server-sent events
- **Per-User Server Instances**: Each user gets their own MCP server instance for isolation
- **Graceful Shutdown**: Proper cleanup of resources and active sessions

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Server 1  │    │   Server 2  │    │   Server N  │
│  (Port 8080)│    │  (Port 8081)│    │  (Port 808N)│
└─────┬───────┘    └─────┬───────┘    └─────┬───────┘
      │                  │                  │
      │                  │                  │
      └──────────────────┼──────────────────┘
                         │
              ┌──────────▼──────────┐
              │       Redis         │
              │ ┌─────────────────┐ │
              │ │ Session Store   │ │
              │ │ Message Queue   │ │
              │ └─────────────────┘ │
              └─────────────────────┘
```

## Prerequisites

- Go 1.23+
- Redis server running locally or accessible over network

## Setup

1. **Start Redis** (if not already running):
   ```bash
   # Using Docker
   docker run -d -p 6379:6379 redis:7-alpine
   
   # Or install locally (macOS)
   brew install redis
   redis-server
   
   # Or install locally (Ubuntu/Debian)
   sudo apt-get install redis-server
   sudo systemctl start redis-server
   ```

2. **Build the server**:
   ```bash
   cd examples/scaled-server
   go mod download
   go build -o scaled-server .
   ```

## Running

### Single Server Instance

```bash
./scaled-server -http=:8080 -redis=localhost:6379
```

### Multiple Server Instances

Start multiple instances on different ports:

```bash
# Terminal 1
./scaled-server -http=:8080 -redis=localhost:6379

# Terminal 2  
./scaled-server -http=:8081 -redis=localhost:6379

# Terminal 3
./scaled-server -http=:8082 -redis=localhost:6379
```

## Usage

### Authentication

The server uses a simple mock authentication system. Include an Authorization header with your requests:

```
Authorization: Bearer user:alice
```

This creates a user with ID "alice". The session ID will be `alice:uuid`.

### API Endpoints

- **MCP Endpoint**: `http://localhost:8080/mcp`
- **Health Check**: `http://localhost:8080/health`
- **Status**: `http://localhost:8080/status`

### Example Client Interaction

1. **Start a session** (GET request with SSE):
   ```bash
   curl -H "Authorization: Bearer user:alice" \
        -H "Accept: text/event-stream" \
        http://localhost:8080/mcp
   ```

2. **Send a message** (POST request):
   ```bash
   curl -X POST \
        -H "Authorization: Bearer user:alice" \
        -H "Accept: application/json, text/event-stream" \
        -H "Content-Type: application/json" \
        -H "Mcp-Session-Id: alice:your-session-uuid" \
        -d '{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}' \
        http://localhost:8080/mcp
   ```

3. **Call a tool**:
   ```bash
   curl -X POST \
        -H "Authorization: Bearer user:alice" \
        -H "Accept: application/json, text/event-stream" \
        -H "Content-Type: application/json" \
        -H "Mcp-Session-Id: alice:your-session-uuid" \
        -d '{"jsonrpc":"2.0","id":"2","method":"tools/call","params":{"name":"greet","arguments":{"name":"World"}}}' \
        http://localhost:8080/mcp
   ```

### Available Tools, Prompts, and Resources

**Tools:**
- `greet`: Greets a user by name
- `calculate`: Performs basic arithmetic operations

**Prompts:**
- `code-review`: Provides a code review prompt template

**Resources:**
- `user-info`: Returns user-specific information

## Session Management

- Sessions are automatically created when a user makes their first request
- Session IDs have the format `user_id:uuid` for security
- Sessions are stored in Redis with a default TTL of 1 hour
- Sessions can be accessed from any server instance
- Sessions are cleaned up on DELETE requests or TTL expiration

## Message Queuing

The server implements Redis-based message queuing for distributing server-sent events:

1. When a server needs to send a message to a client, it publishes to the Redis queue for the server instance that owns that session
2. Each server instance subscribes to its own message queue
3. Messages are routed to the correct session transport for delivery

## Configuration Options

- `-http`: HTTP server address (default: `:8080`)
- `-redis`: Redis server address (default: `localhost:6379`)
- `-redis-db`: Redis database number (default: `0`)
- `-redis-prefix`: Redis key prefix (default: `mcp`)

## Testing

The example includes comprehensive tests to validate the distributed functionality:

### Simple Test

```bash
# Build and run the simple test
go build -o simple-test simple-test.go
./simple-test -server=http://localhost:8080 -user=alice
```

This test validates:
- Health check connectivity
- MCP protocol initialization
- Tool calls (greet, calculate)
- Prompt and resource listing

### Distributed Functionality Test

```bash
# Run the comprehensive distributed test
./test-distributed.sh
```

This test validates:
- Multiple server instances running
- User isolation across servers
- Redis session storage and format
- Cross-server session distribution

### Manual Testing

You can also test manually with curl or the provided test scripts.

You can test session isolation by using different user tokens:

```bash
# User Alice
curl -H "Authorization: Bearer user:alice" -H "Accept: text/event-stream" http://localhost:8080/mcp

# User Bob  
curl -H "Authorization: Bearer user:bob" -H "Accept: text/event-stream" http://localhost:8081/mcp
```

Each user will get their own isolated MCP server instance and session.

### Monitoring Redis

You can monitor Redis activity:

```bash
# Connect to Redis CLI
redis-cli

# Monitor all commands
MONITOR

# List all keys
KEYS mcp:*

# Check session data
GET mcp:session:alice:some-uuid-here
```

## Limitations

This is a demonstration implementation with some limitations:

1. **Mock Authentication**: The authentication system is very basic. In production, you would implement proper JWT validation, API key verification, etc.

2. **Message Routing**: The message queuing system is simplified. A full implementation would require more sophisticated message routing and delivery guarantees.

3. **Error Handling**: Error handling could be more comprehensive, especially around Redis failures and network partitions.

4. **Session Cleanup**: While sessions have TTL, there's no active cleanup of local session caches when sessions expire.

## Production Considerations

For production use, consider:

- Implementing proper authentication and authorization
- Adding TLS/SSL support
- Implementing Redis clustering for high availability
- Adding metrics and monitoring
- Implementing rate limiting per user
- Adding proper logging and structured error handling
- Implementing session cleanup and garbage collection
- Adding circuit breakers for Redis failures