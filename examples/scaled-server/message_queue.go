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

// MessageQueue defines the interface for distributing messages to the correct server
type MessageQueue interface {
	// PublishMessage publishes a message to the queue for a specific session
	PublishMessage(ctx context.Context, sessionID string, message []byte) error
	
	// SubscribeToMessages subscribes to messages for a specific server instance
	SubscribeToMessages(ctx context.Context, serverID string) (<-chan Message, error)
	
	// UnsubscribeFromMessages stops subscription for a server instance
	UnsubscribeFromMessages(ctx context.Context, serverID string) error
}

// Message represents a queued message
type Message struct {
	SessionID string    `json:"session_id"`
	Content   []byte    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// RedisMessageQueue implements MessageQueue using Redis pub/sub
type RedisMessageQueue struct {
	client redis.Cmdable
	prefix string
}

// NewRedisMessageQueue creates a new Redis-backed message queue
func NewRedisMessageQueue(client redis.Cmdable, prefix string) *RedisMessageQueue {
	return &RedisMessageQueue{
		client: client,
		prefix: prefix,
	}
}

func (r *RedisMessageQueue) queueKey(serverID string) string {
	return fmt.Sprintf("%s:queue:%s", r.prefix, serverID)
}

func (r *RedisMessageQueue) PublishMessage(ctx context.Context, sessionID string, message []byte) error {
	// First, we need to find which server owns this session
	sessionStore := NewRedisSessionStore(r.client, r.prefix, time.Hour)
	sessionData, err := sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for message routing: %w", err)
	}
	if sessionData == nil {
		return fmt.Errorf("session %s not found", sessionID)
	}
	
	msg := Message{
		SessionID: sessionID,
		Content:   message,
		Timestamp: time.Now(),
	}
	
	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Push message to the queue for the server that owns this session
	queueKey := r.queueKey(sessionData.ServerID)
	return r.client.LPush(ctx, queueKey, msgData).Err()
}

func (r *RedisMessageQueue) SubscribeToMessages(ctx context.Context, serverID string) (<-chan Message, error) {
	msgChan := make(chan Message, 100)
	queueKey := r.queueKey(serverID)
	
	go func() {
		defer close(msgChan)
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Block for up to 1 second waiting for a message
				result, err := r.client.BRPop(ctx, time.Second, queueKey).Result()
				if err != nil {
					if err == redis.Nil {
						// Timeout, continue polling
						continue
					}
					// Other error, log and continue
					fmt.Printf("Error polling message queue: %v\n", err)
					continue
				}
				
				if len(result) != 2 {
					continue
				}
				
				var msg Message
				if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
					fmt.Printf("Error unmarshaling message: %v\n", err)
					continue
				}
				
				select {
				case msgChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	
	return msgChan, nil
}

func (r *RedisMessageQueue) UnsubscribeFromMessages(ctx context.Context, serverID string) error {
	// In this simple implementation, we just stop the goroutine by canceling the context
	// More sophisticated implementations might track subscriptions
	return nil
}