// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionStore defines the interface for storing and retrieving MCP sessions.
type SessionStore interface {
	// StoreSession saves a session with the given ID and data
	StoreSession(ctx context.Context, sessionID string, data SessionData) error
	
	// GetSession retrieves a session by ID
	GetSession(ctx context.Context, sessionID string) (*SessionData, error)
	
	// DeleteSession removes a session by ID
	DeleteSession(ctx context.Context, sessionID string) error
	
	// SessionExists checks if a session exists
	SessionExists(ctx context.Context, sessionID string) (bool, error)
}

// SessionData represents the data stored for each session
type SessionData struct {
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	ServerID   string    `json:"server_id"`   // Which server instance owns this session
	CreatedAt  time.Time `json:"created_at"`
	LastAccess time.Time `json:"last_access"`
}

// RedisSessionStore implements SessionStore using Redis
type RedisSessionStore struct {
	client redis.Cmdable
	prefix string
	ttl    time.Duration
}

// NewRedisSessionStore creates a new Redis-backed session store
func NewRedisSessionStore(client redis.Cmdable, prefix string, ttl time.Duration) *RedisSessionStore {
	return &RedisSessionStore{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}
}

func (r *RedisSessionStore) key(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", r.prefix, sessionID)
}

func (r *RedisSessionStore) StoreSession(ctx context.Context, sessionID string, data SessionData) error {
	data.LastAccess = time.Now()
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	return r.client.Set(ctx, r.key(sessionID), jsonData, r.ttl).Err()
}

func (r *RedisSessionStore) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	data, err := r.client.Get(ctx, r.key(sessionID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}
	
	// Update last access time
	sessionData.LastAccess = time.Now()
	r.StoreSession(ctx, sessionID, sessionData)
	
	return &sessionData, nil
}

func (r *RedisSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	return r.client.Del(ctx, r.key(sessionID)).Err()
}

func (r *RedisSessionStore) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	exists, err := r.client.Exists(ctx, r.key(sessionID)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check session existence: %w", err)
	}
	return exists > 0, nil
}