package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// ========== WebSocket Connection Management ==========
// These methods handle WebSocket connections for real-time updates between
// the server and UI clients. Supports state updates, plan execution progress,
// and recovery coordination.

// websocketHandler handles WebSocket connections for real-time updates
func (ws *WebServer) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to upgrade WebSocket")
		return
	}

	// Generate unique connection ID
	connID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

	// Store connection
	ws.connMutex.Lock()
	ws.connections[connID] = &wsConnection{
		conn:     conn,
		lastPong: time.Now(),
	}
	ws.connMutex.Unlock()

	ws.aiAgent.Logger.WithField("conn_id", connID).Info("WebSocket connection established")

	// Setup connection cleanup
	defer func() {
		ws.connMutex.Lock()
		delete(ws.connections, connID)
		ws.connMutex.Unlock()
		conn.Close()
		ws.aiAgent.Logger.WithField("conn_id", connID).Info("WebSocket connection closed")
	}()

	// Configure connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Setup ping/pong for connection health
	conn.SetPongHandler(func(string) error {
		ws.connMutex.Lock()
		if wsConn, exists := ws.connections[connID]; exists {
			wsConn.lastPong = time.Now()
		}
		ws.connMutex.Unlock()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send periodic updates and ping
	ticker := time.NewTicker(30 * time.Second)
	pingTicker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	defer pingTicker.Stop()

	// Send initial state
	ws.sendStateUpdate(connID)

	// Handle connection lifecycle
	done := make(chan struct{})

	// Handle incoming messages (if any)
	go func() {
		defer close(done)
		for {
			_, msgData, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("WebSocket read error")
				}
				return
			}

			// Process incoming message
			ws.handleIncomingMessage(connID, msgData)
		}
	}()

	// Main event loop
	for {
		select {
		case <-ticker.C:
			// Send state update
			ws.sendStateUpdate(connID)

		case <-pingTicker.C:
			// Send ping to check connection health
			ws.connMutex.Lock()
			wsConn, exists := ws.connections[connID]
			ws.connMutex.Unlock()

			if !exists {
				return
			}

			// Lock write mutex to prevent concurrent writes
			wsConn.writeMutex.Lock()
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			wsConn.writeMutex.Unlock()

			if err != nil {
				ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("Failed to send ping")
				return
			}

			// Check if we've received a recent pong
			if time.Since(wsConn.lastPong) > 90*time.Second {
				ws.aiAgent.Logger.WithField("conn_id", connID).Warn("Connection seems stale, closing")
				return
			}

		case <-done:
			return

		case <-r.Context().Done():
			return
		}
	}
}

// sendStateUpdate sends a state update to a specific connection
func (ws *WebServer) sendStateUpdate(connID string) {
	ws.connMutex.RLock()
	wsConn, exists := ws.connections[connID]
	ws.connMutex.RUnlock()

	if !exists {
		return
	}

	// Use MCP server to get current state with fresh discovery
	stateJSON, err := ws.aiAgent.ExportInfrastructureStateWithOptions(context.Background(), true, true)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to get state for WebSocket update")
		return
	}

	update := map[string]interface{}{
		"type":      "state_update",
		"data":      stateJSON,
		"timestamp": time.Now(),
	}

	// Lock the write mutex to prevent concurrent writes
	wsConn.writeMutex.Lock()
	wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err = wsConn.conn.WriteJSON(update)
	wsConn.writeMutex.Unlock()

	if err != nil {
		ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Debug("Failed to send state update, connection likely closed")
		// Connection is broken, it will be cleaned up by the main handler
	}
}

// broadcastUpdate sends an update to all active WebSocket connections
func (ws *WebServer) broadcastUpdate(update map[string]interface{}) {
	ws.connMutex.RLock()
	connections := make(map[string]*wsConnection)
	for id, conn := range ws.connections {
		connections[id] = conn
	}
	ws.connMutex.RUnlock()

	for connID, wsConn := range connections {
		// Lock the write mutex for this connection to prevent concurrent writes
		wsConn.writeMutex.Lock()

		wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := wsConn.conn.WriteJSON(update)

		wsConn.writeMutex.Unlock()

		if err != nil {
			ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Debug("Failed to broadcast update, removing connection")
			// Remove broken connection
			ws.connMutex.Lock()
			delete(ws.connections, connID)
			ws.connMutex.Unlock()
			wsConn.conn.Close()
		}
	}
}
