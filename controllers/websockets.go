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
			//	fmt.Println("Socket Error")
			// fmt.Println(err.Error())
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
		//	conn.WriteJSON(mess)
	}
}

// HandleWebsocketMessages handles the handle websocket messages request.
func HandleWebsocketMessages() {
	for {
		// Grab the next message from the broadcast channel
		msg := <-broadcast
		// fmt.Println(msg)

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
					payloadStr, err := json.Marshal(items)
					if err == nil {
						if err := player.WriteJSON(Message{
							Identifier:  msg.Identifier,
							MessageType: "Enqueue",
							Payload:     string(payloadStr),
						}); err != nil {
							fmt.Printf("Error writing JSON to connection: %v\n", err)
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
		// Send it out to every client that is currently connected
		// for client := range clients {
		// 	err := client.WriteJSON(msg)
		// 	if err != nil {
		// 		log.Printf("error: %v", err)
		// 		client.Close()
		// 		delete(clients, client)
		// 	}
		// }
	}
}
