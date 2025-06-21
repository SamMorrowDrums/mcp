// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DistributedStreamableHTTPHandler is a horizontally scalable version of StreamableHTTPHandler
// that uses Redis for session storage and message queuing
type DistributedStreamableHTTPHandler struct {
	getServer      func(*http.Request, *AuthenticatedUser) *mcp.Server
	sessionStore   SessionStore
	messageQueue   MessageQueue
	authProvider   AuthProvider
	sessionIDGen   *SessionIDGenerator
	serverID       string
	
	// Local cache of active transports for this server instance
	localSessions   map[string]*mcp.StreamableServerTransport
	localSessionsMu sync.RWMutex
	
	// Message queue subscription
	messageSubscription <-chan Message
	subscriptionCancel  context.CancelFunc
}

// DistributedStreamableHTTPOptions configures the distributed handler
type DistributedStreamableHTTPOptions struct {
	SessionTTL time.Duration
}

// NewDistributedStreamableHTTPHandler creates a new distributed streamable HTTP handler
func NewDistributedStreamableHTTPHandler(
	getServer func(*http.Request, *AuthenticatedUser) *mcp.Server,
	sessionStore SessionStore,
	messageQueue MessageQueue,
	authProvider AuthProvider,
	opts *DistributedStreamableHTTPOptions,
) (*DistributedStreamableHTTPHandler, error) {
	if opts == nil {
		opts = &DistributedStreamableHTTPOptions{
			SessionTTL: time.Hour, // Default 1 hour session TTL
		}
	}
	
	serverID := uuid.New().String()
	
	handler := &DistributedStreamableHTTPHandler{
		getServer:     getServer,
		sessionStore:  sessionStore,
		messageQueue:  messageQueue,
		authProvider:  authProvider,
		sessionIDGen:  NewSessionIDGenerator(),
		serverID:      serverID,
		localSessions: make(map[string]*mcp.StreamableServerTransport),
	}
	
	// Start message queue subscription
	if err := handler.startMessageSubscription(); err != nil {
		return nil, fmt.Errorf("failed to start message subscription: %w", err)
	}
	
	return handler, nil
}

// startMessageSubscription begins listening for messages from Redis
func (h *DistributedStreamableHTTPHandler) startMessageSubscription() error {
	ctx, cancel := context.WithCancel(context.Background())
	h.subscriptionCancel = cancel
	
	msgChan, err := h.messageQueue.SubscribeToMessages(ctx, h.serverID)
	if err != nil {
		cancel()
		return err
	}
	h.messageSubscription = msgChan
	
	// Start goroutine to handle incoming messages
	go h.handleIncomingMessages(ctx)
	
	return nil
}

// handleIncomingMessages processes messages from the Redis queue
func (h *DistributedStreamableHTTPHandler) handleIncomingMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-h.messageSubscription:
			if !ok {
				return
			}
			h.routeMessageToSession(msg)
		}
	}
}

// routeMessageToSession delivers a message to the appropriate local session
func (h *DistributedStreamableHTTPHandler) routeMessageToSession(msg Message) {
	h.localSessionsMu.RLock()
	_, exists := h.localSessions[msg.SessionID]
	h.localSessionsMu.RUnlock()
	
	if !exists {
		// Session not active on this server instance
		fmt.Printf("Received message for inactive session %s, ignoring\n", msg.SessionID)
		return
	}
	
	// TODO: Forward the message to the transport
	// This would require modifying the StreamableServerTransport to accept external messages
	// For now, we'll just log that we received the message
	fmt.Printf("Would forward message to session %s: %s\n", msg.SessionID, string(msg.Content))
}

// ServeHTTP handles incoming HTTP requests with distributed session management
func (h *DistributedStreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Authenticate the user
	user, err := h.authProvider.AuthenticateRequest(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusUnauthorized)
		return
	}
	
	// Check for content type requirements (similar to original handler)
	accept := strings.Split(strings.Join(req.Header.Values("Accept"), ","), ",")
	var jsonOK, streamOK bool
	for _, c := range accept {
		switch strings.TrimSpace(c) {
		case "application/json":
			jsonOK = true
		case "text/event-stream":
			streamOK = true
		}
	}
	
	if req.Method == http.MethodGet {
		if !streamOK {
			http.Error(w, "Accept must contain 'text/event-stream' for GET requests", http.StatusBadRequest)
			return
		}
	} else if !jsonOK || !streamOK {
		http.Error(w, "Accept must contain both 'application/json' and 'text/event-stream'", http.StatusBadRequest)
		return
	}
	
	// Handle session lookup/creation
	sessionID := req.Header.Get("Mcp-Session-Id")
	var session *mcp.StreamableServerTransport
	var sessionData *SessionData
	
	if sessionID != "" {
		// Look up existing session
		sessionData, err = h.sessionStore.GetSession(req.Context(), sessionID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Session lookup failed: %v", err), http.StatusInternalServerError)
			return
		}
		if sessionData == nil {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		
		// Verify session belongs to authenticated user
		userID, _, err := h.sessionIDGen.ParseSessionID(sessionID)
		if err != nil || userID != user.ID {
			http.Error(w, "Session access denied", http.StatusForbidden)
			return
		}
		
		// Check if session is active locally
		h.localSessionsMu.RLock()
		session, _ = h.localSessions[sessionID]
		h.localSessionsMu.RUnlock()
	}
	
	// Handle DELETE requests
	if req.Method == http.MethodDelete {
		if sessionID == "" {
			http.Error(w, "DELETE requires an Mcp-Session-Id header", http.StatusBadRequest)
			return
		}
		
		// Remove from local cache
		h.localSessionsMu.Lock()
		if localSession, exists := h.localSessions[sessionID]; exists {
			delete(h.localSessions, sessionID)
			localSession.Close()
		}
		h.localSessionsMu.Unlock()
		
		// Remove from Redis
		h.sessionStore.DeleteSession(req.Context(), sessionID)
		
		w.WriteHeader(http.StatusNoContent)
		return
	}
	
	// Validate HTTP methods
	switch req.Method {
	case http.MethodPost, http.MethodGet:
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		return
	}
	
	// Create new session if needed
	if session == nil {
		if sessionID == "" {
			sessionID = h.sessionIDGen.GenerateSessionID(user.ID)
		}
		
		session = mcp.NewStreamableServerTransport(sessionID)
		server := h.getServer(req, user)
		if server == nil {
			http.Error(w, "No server available", http.StatusInternalServerError)
			return
		}
		
		// Connect the session
		if _, err := server.Connect(req.Context(), session); err != nil {
			http.Error(w, "Failed to connect session", http.StatusInternalServerError)
			return
		}
		
		// Store session data in Redis
		sessionData = &SessionData{
			UserID:     user.ID,
			SessionID:  sessionID,
			ServerID:   h.serverID,
			CreatedAt:  time.Now(),
			LastAccess: time.Now(),
		}
		
		if err := h.sessionStore.StoreSession(req.Context(), sessionID, *sessionData); err != nil {
			session.Close()
			http.Error(w, "Failed to store session", http.StatusInternalServerError)
			return
		}
		
		// Cache locally
		h.localSessionsMu.Lock()
		h.localSessions[sessionID] = session
		h.localSessionsMu.Unlock()
	}
	
	// Delegate to the session transport
	session.ServeHTTP(w, req)
}

// Close shuts down the handler and its resources
func (h *DistributedStreamableHTTPHandler) Close() error {
	if h.subscriptionCancel != nil {
		h.subscriptionCancel()
	}
	
	h.localSessionsMu.Lock()
	for _, session := range h.localSessions {
		session.Close()
	}
	h.localSessions = nil
	h.localSessionsMu.Unlock()
	
	return nil
}