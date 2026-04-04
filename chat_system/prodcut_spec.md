## 🏗️ Project Spec: `go-chat-hub`

### 1. The Core Architecture (The "Hub")
Instead of letting every user talk to every other user directly (which is $O(N^2)$ and a nightmare), you will implement a **Central Broadcast Hub**.
* **The Hub** is a single long-running goroutine that manages the state of all connected "Clients."
* **The Client** is a struct representing a single WebSocket/TCP connection. It has its own egress (outbound) channel to prevent slow consumers from blocking the Hub.



### 2. Functional Requirements
* **Dynamic Rooms**: Users shouldn't just be in one big "lobby." They should be able to join and leave specific named rooms (e.g., `#golang`, `#general`).
* **Message Types**: Support at least three types of events:
    1. `JOIN`: Notify a room that a new user arrived.
    2. `MSG`: A standard text message sent to everyone in that specific room.
    3. `LEAVE`: Clean up the connection and notify the room.
* **Concurrency-Safe Registry**: The Hub must track which users are in which rooms using a map of maps (`map[string]map[*Client]bool`).

### 3. Technical Requirements (The "Complex" Part)
* **The Non-Blocking Broadcast**: If one user has a terrible internet connection, their "write" buffer will fill up. The Hub **must not** wait for them. If a client's channel is full, the Hub should drop the message for that specific client and move on.
* **Graceful Disconnects**: If a user closes their tab (or the network clips), the Hub must detect the broken pipe, close the client's internal channels, and unregister them to prevent memory leaks.
* **Heartbeats (Ping/Pong)**: Implement a side-goroutine for each client that sends a "ping" every 30 seconds. If the client doesn't "pong" back, kill the connection.

### 4. Component Breakdown
| Component | Responsibility | Concurrency Tool |
| :--- | :--- | :--- |
| **Listener** | Accepts new TCP/WS connections. | `net.Listen` / `accept` loop |
| **Hub** | The "Brain." Routes messages to rooms. | `select` + `channels` |
| **Reader** | Per-client: Reads from the socket. | `goroutine` + `scanner.Scan` |
| **Writer** | Per-client: Pushes channel data to socket. | `goroutine` + `chan` |

---

## 🛠️ The "Pro" Challenge: The Fan-Out Buffer
In a standard implementation, the Hub iterates through a slice of clients and sends messages. To make it complex, implement **Buffered Broadcasts**.

If the Hub tries to send to a `client.send` channel and it's blocked, use a `select` with a `default` case:
```go
select {
case client.send <- message:
    // Success
default:
    // Client is too slow! Drop message or close connection.
    close(client.send)
    delete(h.clients, client)
}
```
