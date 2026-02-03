package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

// EnqueuePayload represents enqueue payload data.
type EnqueuePayload struct {
	ItemIDs   []string `json:"itemIDs"`
	PodcastID string   `json:"podcastID"`
	TagIDs    []string `json:"tagIDs"`
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var activePlayers = make(map[*websocket.Conn]string)
var allConnections = make(map[*websocket.Conn]string)

var broadcast = make(chan Message) // broadcast channel

// Message represents message data.
type Message struct {
	Connection  *websocket.Conn `json:"-"`
	Identifier  string          `json:"identifier"`
	MessageType string          `json:"messageType"`
	Payload     string          `json:"payload"`
}

// Wshandler handles the wshandler request.
func Wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Printf("Error closing websocket connection: %v\n", err)
		}
	}()
	for {
		var mess Message
		err := conn.ReadJSON(&mess)
		if err != nil {
			isPlayer := activePlayers[conn] != ""
			if isPlayer {
				delete(activePlayers, conn)
				broadcast <- Message{
					MessageType: "PlayerRemoved",
					Identifier:  mess.Identifier,
				}
			}
			delete(allConnections, conn)
			break
		}
		mess.Connection = conn
		allConnections[conn] = mess.Identifier
		broadcast <- mess
	}
}

// HandleWebsocketMessages handles the handle websocket messages request.
func HandleWebsocketMessages() {
	for {
		// Grab the next message from the broadcast channel
		msg := <-broadcast

		switch msg.MessageType {
		case "RegisterPlayer":
			activePlayers[msg.Connection] = msg.Identifier
			for connection := range allConnections {
				if err := connection.WriteJSON(Message{
					Identifier:  msg.Identifier,
					MessageType: "PlayerExists",
				}); err != nil {
					fmt.Printf("Error writing JSON to connection: %v\n", err)
				}
			}
			fmt.Println("Player Registered")
		case "PlayerRemoved":
			for connection := range allConnections {
				if err := connection.WriteJSON(Message{
					Identifier:  msg.Identifier,
					MessageType: "NoPlayer",
				}); err != nil {
					fmt.Printf("Error writing JSON to connection: %v\n", err)
				}
			}
			fmt.Println("Player Registered")
		case "Enqueue":
			var payload EnqueuePayload
			fmt.Println(msg.Payload)
			err := json.Unmarshal([]byte(msg.Payload), &payload)
			if err == nil {
				items := getItemsToPlay(payload.ItemIDs, payload.PodcastID, payload.TagIDs)
				var player *websocket.Conn
				for connection, id := range activePlayers {
					if msg.Identifier == id {
						player = connection
						break
					}
				}
				if player != nil {
					payloadStr, marshalErr := json.Marshal(items)
					if marshalErr == nil {
						if writeErr := player.WriteJSON(Message{
							Identifier:  msg.Identifier,
							MessageType: "Enqueue",
							Payload:     string(payloadStr),
						}); writeErr != nil {
							fmt.Printf("Error writing JSON to connection: %v\n", writeErr)
						}
					}
				}
			} else {
				fmt.Println(err.Error())
			}
		case "Register":
			var player *websocket.Conn
			for connection, id := range activePlayers {
				if msg.Identifier == id {
					player = connection
					break
				}
			}

			if player == nil {
				fmt.Println("Player Not Exists")
				if err := msg.Connection.WriteJSON(Message{
					Identifier:  msg.Identifier,
					MessageType: "NoPlayer",
				}); err != nil {
					fmt.Printf("Error writing JSON to connection: %v\n", err)
				}
			} else {
				if err := msg.Connection.WriteJSON(Message{
					Identifier:  msg.Identifier,
					MessageType: "PlayerExists",
				}); err != nil {
					fmt.Printf("Error writing JSON to connection: %v\n", err)
				}
			}
		}
	}
}
