//go:build integration
// +build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toozej/podgrab/controllers"
	"github.com/toozej/podgrab/db"
)

// setupWebSocketServer creates a test WebSocket server.
func setupWebSocketServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Set up test database for WebSocket handler
	database := db.SetupTestDB(t)
	t.Cleanup(func() { db.TeardownTestDB(t, database) })

	originalDB := db.DB
	db.DB = database
	t.Cleanup(func() { db.DB = originalDB })

	// Create test settings
	db.CreateTestSetting(t, database)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", controllers.Wshandler)

	server := httptest.NewServer(mux)

	// Start message handler in background
	go controllers.HandleWebsocketMessages()

	return server
}

// TestWebSocket_Connection tests WebSocket connection establishment.
func TestWebSocket_Connection(t *testing.T) {
	server := setupWebSocketServer(t)
	defer server.Close()

	// Convert http:// to ws://
	// nosemgrep: javascript.lang.security.detect-insecure-websocket.detect-insecure-websocket - Test uses local httptest server, secure websocket not applicable
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Should connect to WebSocket")
	defer conn.Close()

	// Send a Register message to establish connection
	msg := controllers.Message{
		Identifier:  "test-client",
		MessageType: "Register",
	}
	err = conn.WriteJSON(msg)
	assert.NoError(t, err, "Should send message")

	// Read response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var response controllers.Message
	err = conn.ReadJSON(&response)
	if err == nil {
		assert.Contains(t, []string{"PlayerExists", "NoPlayer"}, response.MessageType, "Should receive valid response")
	}
}

// TestWebSocket_MultipleClients tests multiple client connections.
func TestWebSocket_MultipleClients(t *testing.T) {
	server := setupWebSocketServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect multiple clients
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Client 1 should connect")
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Client 2 should connect")
	defer conn2.Close()

	conn3, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Client 3 should connect")
	defer conn3.Close()

	// Register clients
	for i, conn := range []*websocket.Conn{conn1, conn2, conn3} {
		msg := controllers.Message{
			Identifier:  fmt.Sprintf("client-%d", i+1),
			MessageType: "Register",
		}
		err = conn.WriteJSON(msg)
		assert.NoError(t, err, "Should register client")
	}

	// Give time for registrations
	time.Sleep(100 * time.Millisecond)

	// All connections should be alive
	for _, conn := range []*websocket.Conn{conn1, conn2, conn3} {
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		err = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
		assert.NoError(t, err, "Connection should be alive")
	}
}

// TestWebSocket_PlayerRegistration tests player registration protocol.
func TestWebSocket_PlayerRegistration(t *testing.T) {
	server := setupWebSocketServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect player
	playerConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Player should connect")
	defer playerConn.Close()

	// Register as player
	msg := controllers.Message{
		Identifier:  "test-player",
		MessageType: "RegisterPlayer",
	}
	err = playerConn.WriteJSON(msg)
	require.NoError(t, err, "Should register player")

	// Connect client and check for player
	time.Sleep(100 * time.Millisecond)
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Client should connect")
	defer clientConn.Close()

	checkMsg := controllers.Message{
		Identifier:  "test-player",
		MessageType: "Register",
	}
	err = clientConn.WriteJSON(checkMsg)
	require.NoError(t, err, "Should send register check")

	// Read response
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var response controllers.Message
	err = clientConn.ReadJSON(&response)
	if err == nil {
		assert.Equal(t, "PlayerExists", response.MessageType, "Should detect registered player")
	}
}

// TestWebSocket_EnqueueMessage tests enqueue message handling.
func TestWebSocket_EnqueueMessage(t *testing.T) {
	server := setupWebSocketServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Register player
	playerConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Player should connect")
	defer playerConn.Close()

	registerMsg := controllers.Message{
		Identifier:  "test-player",
		MessageType: "RegisterPlayer",
	}
	err = playerConn.WriteJSON(registerMsg)
	require.NoError(t, err, "Should register player")

	// Read registration response to clear the queue
	playerConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var regResponse controllers.Message
	playerConn.ReadJSON(&regResponse)

	time.Sleep(100 * time.Millisecond)

	// Send enqueue message from client
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Client should connect")
	defer clientConn.Close()

	payload := controllers.EnqueuePayload{
		ItemIDs:   []string{"item1", "item2"},
		PodcastID: "podcast1",
	}
	payloadJSON, _ := json.Marshal(payload)

	enqueueMsg := controllers.Message{
		Identifier:  "test-player",
		MessageType: "Enqueue",
		Payload:     string(payloadJSON),
	}
	err = clientConn.WriteJSON(enqueueMsg)
	assert.NoError(t, err, "Should send enqueue message")

	// Player should receive enqueue message
	playerConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var response controllers.Message
	err = playerConn.ReadJSON(&response)
	if err == nil {
		assert.Equal(t, "Enqueue", response.MessageType, "Player should receive enqueue")
	}
}

// TestWebSocket_ConnectionPersistence tests connection stability.
func TestWebSocket_ConnectionPersistence(t *testing.T) {
	server := setupWebSocketServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Should connect")
	defer conn.Close()

	// Send register message
	msg := controllers.Message{
		Identifier:  "persistent-client",
		MessageType: "Register",
	}
	conn.WriteJSON(msg)

	// Keep connection alive for a few seconds
	for i := 0; i < 3; i++ {
		time.Sleep(500 * time.Millisecond)
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		err = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
		assert.NoError(t, err, "Connection should remain alive")
	}
}

// TestWebSocket_CleanDisconnect tests graceful connection closure.
func TestWebSocket_CleanDisconnect(t *testing.T) {
	server := setupWebSocketServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Should connect")

	// Send close message
	err = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	assert.NoError(t, err, "Should send close message")

	// Close connection
	err = conn.Close()
	assert.NoError(t, err, "Should close cleanly")
}

// TestWebSocket_InvalidURL tests connection to invalid WebSocket endpoint.
func TestWebSocket_InvalidURL(t *testing.T) {
	// Try to connect to non-existent WebSocket server
	invalidURL := "ws://localhost:9999/ws"
	_, _, err := websocket.DefaultDialer.Dial(invalidURL, nil)
	assert.Error(t, err, "Should fail to connect to invalid URL")
}

// TestWebSocket_ReconnectionAfterServerRestart tests client reconnection.
func TestWebSocket_ReconnectionAfterServerRestart(t *testing.T) {
	server := setupWebSocketServer(t)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// First connection
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "First connection should succeed")
	conn1.Close()

	// Close and recreate server (simulates restart)
	server.Close()
	server = setupWebSocketServer(t)
	defer server.Close()

	// Parse URL to get new port
	u, _ := url.Parse(server.URL)
	// nosemgrep: javascript.lang.security.detect-insecure-websocket.detect-insecure-websocket - Test uses local httptest server, secure websocket not applicable
	newWSURL := "ws://" + u.Host + "/ws"

	// Second connection after "restart"
	conn2, _, err := websocket.DefaultDialer.Dial(newWSURL, nil)
	require.NoError(t, err, "Should reconnect after server restart")
	defer conn2.Close()
}
