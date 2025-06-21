// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AuthenticatedUser represents an authenticated user
type AuthenticatedUser struct {
	ID       string
	Username string
}

// AuthProvider defines the interface for user authentication
type AuthProvider interface {
	// AuthenticateRequest extracts and validates user from the request
	AuthenticateRequest(req *http.Request) (*AuthenticatedUser, error)
}

// MockAuthProvider is a simple mock implementation for demonstration
// In a real implementation, this would validate JWT tokens, API keys, etc.
type MockAuthProvider struct{}

// NewMockAuthProvider creates a new mock authentication provider
func NewMockAuthProvider() *MockAuthProvider {
	return &MockAuthProvider{}
}

// AuthenticateRequest extracts user information from the Authorization header
// For this example, we expect "Bearer user:username" format
func (m *MockAuthProvider) AuthenticateRequest(req *http.Request) (*AuthenticatedUser, error) {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}
	
	// Expect "Bearer user:username" format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("invalid Authorization header format")
	}
	
	token := parts[1]
	userParts := strings.SplitN(token, ":", 2)
	if len(userParts) != 2 || userParts[0] != "user" {
		return nil, fmt.Errorf("invalid token format, expected 'user:username'")
	}
	
	username := userParts[1]
	if username == "" {
		return nil, fmt.Errorf("empty username")
	}
	
	// In a real implementation, you would validate the token/username
	// For this example, we'll accept any non-empty username
	return &AuthenticatedUser{
		ID:       username, // Using username as ID for simplicity
		Username: username,
	}, nil
}

// SessionIDGenerator generates secure session IDs with user prefix
type SessionIDGenerator struct{}

// NewSessionIDGenerator creates a new session ID generator
func NewSessionIDGenerator() *SessionIDGenerator {
	return &SessionIDGenerator{}
}

// GenerateSessionID creates a new session ID in the format "userID:uuid"
func (g *SessionIDGenerator) GenerateSessionID(userID string) string {
	sessionUUID := uuid.New().String()
	return fmt.Sprintf("%s:%s", userID, sessionUUID)
}

// ParseSessionID extracts the user ID from a session ID
func (g *SessionIDGenerator) ParseSessionID(sessionID string) (userID string, sessionUUID string, err error) {
	parts := strings.SplitN(sessionID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid session ID format")
	}
	return parts[0], parts[1], nil
}