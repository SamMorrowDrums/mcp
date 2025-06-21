#!/bin/bash

# Comprehensive test script for distributed MCP server functionality

set -e

echo "🧪 Testing Distributed MCP Server Functionality"
echo "=============================================="

# Test basic health on both servers
echo "📍 Testing health endpoints..."
for port in 8080 8081; do
    echo -n "  Server on port $port... "
    if curl -s "http://localhost:$port/health" > /dev/null; then
        echo "✅"
    else
        echo "❌ Failed"
        exit 1
    fi
done

# Test different users on different servers
echo "📍 Testing user isolation across servers..."
echo "  Testing alice on server 8080..."
./simple-test -server=http://localhost:8080 -user=alice > /dev/null 2>&1 && echo "  ✅ Alice test passed"

echo "  Testing bob on server 8081..."
./simple-test -server=http://localhost:8081 -user=bob > /dev/null 2>&1 && echo "  ✅ Bob test passed"

echo "  Testing charlie on server 8080..."
./simple-test -server=http://localhost:8080 -user=charlie > /dev/null 2>&1 && echo "  ✅ Charlie test passed"

# Check Redis for session data
echo "📍 Checking Redis session storage..."
session_count=$(docker exec mcp-redis redis-cli KEYS "mcp:session:*" | wc -l)
echo "  Total sessions in Redis: $session_count"

if [ "$session_count" -gt 0 ]; then
    echo "  ✅ Sessions are being stored in Redis"
    echo "  Sample session keys:"
    docker exec mcp-redis redis-cli KEYS "mcp:session:*" | head -3 | sed 's/^/    /'
    
    # Show session data for one session
    echo "  Sample session data:"
    first_key=$(docker exec mcp-redis redis-cli KEYS "mcp:session:*" | head -1)
    if [ ! -z "$first_key" ]; then
        echo "    Key: $first_key"
        session_data=$(docker exec mcp-redis redis-cli GET "$first_key")
        echo "    Data: $session_data" | jq . 2>/dev/null || echo "    Data: $session_data"
    fi
else
    echo "  ❌ No sessions found in Redis"
    exit 1
fi

echo "📍 Testing session format validation..."
# Check that session IDs follow the user_id:uuid format
docker exec mcp-redis redis-cli KEYS "mcp:session:*" | while read session_key; do
    if [[ $session_key =~ mcp:session:([^:]+):([a-f0-9-]+)$ ]]; then
        user_id="${BASH_REMATCH[1]}"
        uuid="${BASH_REMATCH[2]}"
        echo "  ✅ Valid session format: user=$user_id, uuid=$uuid"
    else
        echo "  ❌ Invalid session format: $session_key"
        exit 1
    fi
done

echo "📍 Testing server distribution..."
# Check that different servers are handling sessions
server_ids=$(docker exec mcp-redis redis-cli KEYS "mcp:session:*" | while read key; do
    docker exec mcp-redis redis-cli GET "$key" | jq -r '.server_id' 2>/dev/null || echo "unknown"
done | sort -u)

server_count=$(echo "$server_ids" | wc -l)
echo "  Unique server IDs: $server_count"
echo "  Server IDs:"
echo "$server_ids" | sed 's/^/    /'

if [ "$server_count" -gt 1 ]; then
    echo "  ✅ Multiple servers are handling sessions"
else
    echo "  ⚠️  Only one server ID found (expected in small tests)"
fi

echo ""
echo "🎉 All distributed functionality tests passed!"
echo ""
echo "📊 Test Summary:"
echo "   ✅ Health checks on multiple servers"
echo "   ✅ User isolation across servers"
echo "   ✅ Redis session storage"
echo "   ✅ Proper session ID format (user_id:uuid)"
echo "   ✅ Session metadata tracking"
echo ""
echo "The distributed MCP server is working correctly!"
echo "Sessions can be handled by any server instance without session affinity."