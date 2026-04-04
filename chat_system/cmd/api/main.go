package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"arjun118.github.io/chat_system/internals/data"
)

var Hub = make(data.Hub)

var (
	ComersChannel   = make(chan data.Client)
	LeaversChannel  = make(chan data.Client)
	MessagesChannel = make(chan data.Message)
	KicksChannel    = make(chan data.Client)
)

// func handleMessages() {
// 	for message := range MessagesChannel {
// 		messageToBroadcast := fmt.Sprintf("%s: %s", message.MessageType, message.Message)
// 		for connectedClient := range Hub {
// 			client := Hub[connectedClient]
// 			if connectedClient != message.ClientName {
// 				// write the incoming message to the personal channel
// 				// we read from this personal channel in a different goroutine
// 				client.PersonalChannel <- fmt.Sprintf("%s", messageToBroadcast)
// 			}
// 		}
// 	}
// }

//	func handleIndividualClient(client data.Client) {
//		// individual handler for client
//		for message := range client.PersonalChannel {
//			fmt.Fprintf(client.Conn, "%s", message)
//		}
//	}
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

// func clientFaninMessages(clientChannels ...chan data.Client) {
// 	out := make(chan data.Client)
// 	var wg sync.WaitGroup
// 	output:= func(channel <- chan data.Client){
// 		for n:= range channel.
// 	}
// 	for _, clientChannel := range clientChannels {

// 	}
// }

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

// this event handler depicts - single monitor goroutine
func EventHandler() {
	// this handles all the messages
	// You must either protect the Hub with a sync.RWMutex,
	// OR (the more idiomatic Go way) handle all map reads and writes inside a single goroutine.
	// You could merge the logic of handleMessages into EventHandler using a select statement so only one thing touches the map at a time.
	for {
		select {

		case deadClientStruct := <-KicksChannel:
			deadClientStruct.Conn.Close()
			delete(Hub, deadClientStruct.Conn.RemoteAddr().String())

		case message := <-MessagesChannel:
			chunkSize := 1000
			currentChunk := make([]data.Client, 0, chunkSize)
			// fan out usage
			for _, client := range Hub {
				currentChunk = append(currentChunk, client)
				if len(currentChunk) == chunkSize {
					go broadcastWorker(currentChunk, message)
					currentChunk = make([]data.Client, 0, chunkSize)
				}
			}
			if len(currentChunk) != 0 {
				go broadcastWorker(currentChunk, message)
			}
		case comer := <-ComersChannel:
			// log.Printf("new comer detected:  %+v", comer)
			clientName := comer.Conn.RemoteAddr().String()
			// add the new comer to the hub
			Hub[clientName] = comer
			log.Printf("Client joined: %s", clientName)
			joinMsg := fmt.Sprintf("%s has joined the chat\n", clientName)
			chunkSize := 1000
			currentChunk := make([]data.Client, 0, chunkSize)
			joinMessage := data.Message{ClientName: clientName, MessageType: "JOIN", Message: joinMsg}
			for _, client := range Hub {
				currentChunk = append(currentChunk, client)
				if len(currentChunk) == chunkSize {
					go broadcastWorker(currentChunk, joinMessage)
					currentChunk = make([]data.Client, 0, chunkSize)
				}
			}
			if len(currentChunk) != 0 {
				go broadcastWorker(currentChunk, joinMessage)
			}
		case leaver := <-LeaversChannel:
			clientName := leaver.Conn.RemoteAddr().String()
			delete(Hub, clientName)

			log.Printf("Client left: %s", clientName)
			leaveMsg := fmt.Sprintf("%s has left the chat\n", clientName)
			chunkSize := 1000
			currentChunk := make([]data.Client, 0, chunkSize)
			leaveMessage := data.Message{ClientName: clientName, MessageType: "LEAVE", Message: leaveMsg}
			for _, client := range Hub {
				currentChunk = append(currentChunk, client)
				if len(currentChunk) == chunkSize {
					go broadcastWorker(currentChunk, leaveMessage)
					currentChunk = make([]data.Client, 0, chunkSize)
				}
			}
			if len(currentChunk) != 0 {
				go broadcastWorker(currentChunk, leaveMessage)
			}
		}
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	clientName := conn.RemoteAddr().String()
	// log.Println("accepted connection from : ", conn.LocalAddr().String())
	reader := bufio.NewReader(conn)
	for {
		// read until a new line, return will be
		// the read string along with the delimiter
		readString, err := reader.ReadString('\n')
		if err != nil {
			log.Println("encourntered error: ", err)
			LeaversChannel <- data.Client{Conn: conn}
			return
		}
		// fmt.Printf("received: %s", readString)
		// conn.Write([]byte("ACK: " + readString))
		MessagesChannel <- data.Message{ClientName: clientName, MessageType: "MSG", Message: fmt.Sprintf("%s said: %s", clientName, readString)}
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
