module scaled-server

go 1.23.0

require (
	github.com/google/uuid v1.6.0
	github.com/modelcontextprotocol/go-sdk v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.7.0
)

replace github.com/modelcontextprotocol/go-sdk => ../..

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)
