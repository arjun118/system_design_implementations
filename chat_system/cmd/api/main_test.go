// package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"net"
// 	"testing"
// 	"time"
// )

// func TestSlowClientGetsKilled(t *testing.T) {
// 	ctx, cancel := context.WithCancel(context.Background())

// 	// 2. Guarantee the kill switch is flipped when the test ends
// 	defer cancel()
// 	go func() {
// 		err := StartServer(ctx, "localhost:8080")
// 		if err != nil {
// 			log.Printf("Server exited with error: %v", err)
// 		}
// 	}()

// 	// Brief pause to let the listener bind before we dial
// 	time.Sleep(50 * time.Millisecond)

// 	badConn, err := net.Dial("tcp", "localhost:8080")
// 	if err != nil {
// 		log.Fatalln("failed to connect bad client: ", err)
// 	}
// 	defer badConn.Close()

// 	// simulate crash or frozen client
// 	// do not write any goroutine to consume from conn
// 	// below is a good client
// 	spammerConn, err := net.Dial("tcp", "localhost:8080")
// 	if err != nil {
// 		log.Fatalln("failed to connect good client: ", err)
// 	}
// 	defer spammerConn.Close()

// 	// spam messages from first client
// 	// Create a massively bloated message (approx 100 KB per message)
// 	hugeString := "SPAM"
// 	for i := 0; i < 25000; i++ {
// 		hugeString += "SPAM"
// 	}
// 	hugeString += "\n" // Don't forget the delimiter!

// 	// Send it 150 times. This is 15 Megabytes of data instantly dumped into the socket.
// 	// This will absolutely crush the OS TCP buffer and force the Go channel to back up.
// 	for i := 0; i < 150; i++ {
// 		_, err := fmt.Fprint(spammerConn, hugeString)
// 		if err != nil {
// 			t.Logf("Spammer got blocked sending message %d (this is expected)", i)
// 			break
// 		}
// 	}
// 	time.Sleep(1 * time.Second)
// 	badConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

// 	buf := make([]byte, 1)
// 	_, err = badConn.Read(buf)

// 	if err == nil {
// 		t.Fatalf("FAILED: badConn is still alive! The server did not kick them.")
// 	}

// 	// We want to make sure the error wasn't just our 500ms timeout.
// 	// If it was closed by the server, err should be io.EOF or a connection reset.
// 	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
// 		t.Fatalf("FAILED: badConn read timed out. They are still connected.")
// 	} else {
// 		t.Logf("SUCCESS: badConn was successfully kicked. (Error received: %v)", err)
// 	}

// 	// 2. PROVE THE SPAMMER IS STILL ALIVE
// 	// If the spammer is alive, we should still be able to write to their socket without error.
// 	_, err = fmt.Fprintf(spammerConn, "Am I still here?\n")
// 	if err != nil {
// 		t.Fatalf("FAILED: spammerConn was accidentally kicked too! Err: %v", err)
// 	}
// 	t.Log("SUCCESS: spammerConn is still active and healthy.")
// }
