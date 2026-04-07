package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"arjun118.github.io/chat_system/internals/data"
)

// var Hub = make(data.Hub)

var Rooms = make(data.Rooms)
var ClientLocations = make(data.ClientLocations)
var (
	ComersChannel   = make(chan data.Client)
	LeaversChannel  = make(chan data.Client)
	MessagesChannel = make(chan data.Message)
	KicksChannel    = make(chan data.Client)
)

func handleIndividualClient(client data.Client) {
	for message := range client.PersonalChannel {
		// If the OS buffer is full, and we can't write within 5 seconds, kill it.
		client.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

		_, err := fmt.Fprintf(client.Conn, "%s", message)
		if err != nil {
			// They are too slow! Close the connection and break the loop.
			client.Conn.Close()
			return
		}
	}
}

func broadcastWorker(clientList []data.Client, message data.Message) {
	messageToBroadcast := fmt.Sprintf("%s: %s", message.MessageType, message.Message)
	for _, connectedClient := range clientList {
		toClientName := connectedClient.Conn.RemoteAddr().String()
		if toClientName != message.ClientName {
			select {
			case connectedClient.PersonalChannel <- messageToBroadcast:
				// Message queued successfully
			default:
				// THE PRODUCTION KICK:
				// The buffer is full. This client is dead weight.
				log.Printf("CRITICAL: Client %s buffer full. Kicking them out.", toClientName)

				KicksChannel <- connectedClient
				// 1. Slam the TCP connection shut.
				// (net.Conn is thread-safe in Go, so this is perfectly legal)
				// connectedClient.Conn.Close()

				// 2. Delete them from the Hub instantly so we don't
				// waste CPU trying to send them the next message.
				// delete(Hub, toClientName)

				// Note: We don't need to manually broadcast a LEAVE message here.
				// Closing the connection forces handleConn to error out, which
				// will naturally push a LEAVE event into our loop!
			}
		}
	}
}

func getRoomChunks(roomName string, ignoreClientName string, chunkSize int) [][]data.Client {
	allChunks := make([][]data.Client, 0)
	currentChunk := make([]data.Client, 0, chunkSize)
	for _, client := range Rooms[roomName] {
		if client.Name == ignoreClientName {
			continue
		}
		currentChunk = append(currentChunk, client)
		if len(currentChunk) == chunkSize {
			// go broadcastWorker(currentChunk, joinMessage)
			allChunks = append(allChunks, currentChunk)
			currentChunk = make([]data.Client, 0, chunkSize)
		}
	}
	if len(currentChunk) != 0 {
		allChunks = append(allChunks, currentChunk)
	}
	return allChunks
}

// this event handler depicts - single monitor goroutine
func EventHandler() {
	// this handles all the messages
	// You must either protect the Hub with a sync.RWMutex,
	// OR (the more idiomatic Go way) handle all map reads and writes inside a single goroutine.
	// You could merge the logic of handleMessages into EventHandler using a select statement so only one thing touches the map at a time.
	Rooms["default"] = make(data.Hub)
	for {
		select {
		case comer := <-ComersChannel:
			// log.Printf("new comer detected:  %+v", comer)
			clientName := comer.Conn.RemoteAddr().String()
			// add the new comer to the hub
			Rooms["default"][clientName] = comer
			ClientLocations[clientName] = "default"
			log.Printf("Client joined: %s", clientName)
			joinMsg := fmt.Sprintf("%s has joined the chat\n", clientName)
			// chunkSize := 1000
			// currentChunk := make([]data.Client, 0, chunkSize)
			joinMessage := data.Message{ClientName: clientName, MessageType: "JOIN", Message: joinMsg}
			// slice of slices (where slices is a chunk of size 1000 - each will be
			// a new goroutine broadCastWorker)
			roomChunks := getRoomChunks("default", clientName, 1000)

			for _, roomChunk := range roomChunks {
				go broadcastWorker(roomChunk, joinMessage)

			}
		case leaver := <-LeaversChannel:
			clientName := leaver.Name
			roomName := ClientLocations[clientName]
			delete(Rooms[roomName], clientName)
			delete(ClientLocations, clientName)

			log.Printf("Client left: %s", clientName)
			leaveMsg := fmt.Sprintf("%s has left the chat\n", clientName)
			// chunkSize := 1000
			// currentChunk := make([]data.Client, 0, chunkSize)
			leaveMessage := data.Message{ClientName: clientName, MessageType: "LEAVE", Message: leaveMsg}
			roomChunks := getRoomChunks(roomName, clientName, 1000)

			for _, roomChunk := range roomChunks {
				go broadcastWorker(roomChunk, leaveMessage)

			}
		case deadClientStruct := <-KicksChannel:
			clientName := deadClientStruct.Name
			roomName := ClientLocations[clientName]
			deadClientStruct.Conn.Close()
			delete(Rooms[roomName], clientName)
			delete(ClientLocations, clientName)

		case message := <-MessagesChannel:
			clientName := message.ClientName
			currentRoom := ClientLocations[clientName]

			switch message.MessageType {
			case "MSG":
				formattedMsg := fmt.Sprintf("[%s] %s: %s\n", currentRoom, clientName, message.Message)
				msgToSend := data.Message{ClientName: clientName, MessageType: "MSG", Message: formattedMsg}

				// go broadcastWorker(getRoomChunk(currentRoom, clientName), msgToSend)

				roomChunks := getRoomChunks(currentRoom, clientName, 1000)

				for _, roomChunk := range roomChunks {
					go broadcastWorker(roomChunk, msgToSend)

				}
				// Give the sender their prompt back
				if client, ok := Rooms[currentRoom][clientName]; ok {
					client.PersonalChannel <- fmt.Sprintf("[%s]> ", currentRoom)
				}
			case "CMD_JOIN":
				newRoom := message.Message
				client := Rooms[currentRoom][clientName]

				if currentRoom != "default" {
					roomChunks := getRoomChunks(currentRoom, clientName, 1000)
					for _, roomChunk := range roomChunks {
						go broadcastWorker(roomChunk, data.Message{MessageType: "LEAVE", Message: fmt.Sprintf("%s left for %s\n", clientName, newRoom)})

					}
				}
				delete(Rooms[currentRoom], clientName)

				// 3. Create new room if it doesn't exist
				if Rooms[newRoom] == nil {
					Rooms[newRoom] = make(data.Hub)
				}
				// 4. Add to new room and update location tracker
				Rooms[newRoom][clientName] = client
				ClientLocations[clientName] = newRoom

				roomChunks := getRoomChunks(newRoom, clientName, 1000)
				for _, roomChunk := range roomChunks {
					go broadcastWorker(roomChunk, data.Message{MessageType: "JOIN", Message: fmt.Sprintf("%s joined\n", clientName)})

				}

				client.PersonalChannel <- fmt.Sprintf("Joined %s!\n[%s]> ", newRoom, newRoom)

			case "CMD_LEAVE":
				if currentRoom == "default" {
					Rooms[currentRoom][clientName].PersonalChannel <- "You are already in the default room.\n[default]> "
					continue
				}
				// (You can implement the exact same logic as CMD_JOIN here, just hardcode "default" as the target
				// )
				client := Rooms[currentRoom][clientName]
				roomChunks := getRoomChunks(currentRoom, clientName, 1000)
				for _, roomChunk := range roomChunks {
					go broadcastWorker(roomChunk, data.Message{MessageType: "JOIN", Message: fmt.Sprintf("%s joined\n", clientName)})

				}
				delete(Rooms[currentRoom], clientName)
				// enter default
				newRoom := "default"
				ClientLocations[clientName] = newRoom
				Rooms[newRoom][clientName] = client
				client.PersonalChannel <- fmt.Sprintf("Joined %s!\n[%s]> ", newRoom, newRoom)
			case "CMD_GETROOMS":
				var roomList string
				for roomName := range Rooms {
					roomList += fmt.Sprintf("- %s (%d users)\n", roomName, len(Rooms[roomName]))
				}
				Rooms[currentRoom][clientName].PersonalChannel <- fmt.Sprintf("Available Rooms:\n%s[%s]> ", roomList, currentRoom)

			}
		}
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	clientName := conn.RemoteAddr().String()
	// log.Println("accepted connection from : ", conn.LocalAddr().String())
	reader := bufio.NewReader(conn)
	fmt.Fprint(conn, "[default]> ")
	for {
		// read until a new line, return will be
		// the read string along with the delimiter
		readString, err := reader.ReadString('\n')
		if err != nil {
			log.Println("encourntered error: ", err)
			LeaversChannel <- data.Client{Conn: conn, Name: conn.RemoteAddr().String()}
			return
		}
		text := strings.TrimSpace(readString)

		if strings.HasPrefix(text, "#join ") {
			newRoom := strings.TrimPrefix(text, "#join ")
			MessagesChannel <- data.Message{ClientName: clientName, MessageType: "CMD_JOIN", Message: newRoom}
		} else if text == "#leave" {
			MessagesChannel <- data.Message{ClientName: clientName, MessageType: "CMD_LEAVE"}
		} else if text == "#getrooms" {
			MessagesChannel <- data.Message{ClientName: clientName, MessageType: "CMD_GETROOMS"}
		} else if text != "" {
			MessagesChannel <- data.Message{ClientName: clientName, MessageType: "MSG", Message: text}
		}
		// fmt.Printf("received: %s", readString)
		// conn.Write([]byte("ACK: " + readString))
		// MessagesChannel <- data.Message{ClientName: clientName, MessageType: "MSG", Message: fmt.Sprintf("%s said: %s", clientName, readString)}
	}

}

func StartServer(ctx context.Context, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	// A dedicated goroutine just waiting for the kill switch.
	go func() {
		<-ctx.Done()     // Blocks until cancel() is called in the test
		listener.Close() // Slam the listener shut
		log.Println("Server shutting down...")
	}()

	go EventHandler()
	log.Println("started listening on 8080...")
	// Accept loop runs in the foreground (or background, depending on your main func setup)
	for {
		conn, err := listener.Accept()
		if err != nil {
			// If context is done, this error is expected. Just return.
			if ctx.Err() != nil {
				return nil
			}
			log.Println("accept error:", err)
			continue
		}
		// / add this to the new comers channel
		// log.Println("accepted the connection: ", conn.RemoteAddr().String())
		clientStruct := data.Client{
			Conn: conn, PersonalChannel: make(chan string, 100),
			Name: conn.RemoteAddr().String(),
		}
		go handleIndividualClient(clientStruct)
		ComersChannel <- clientStruct
		// handle conneciton in a seperate goroutine
		go handleConn(conn)
	}
}

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartServer(ctx, "localhost:8080")
}
