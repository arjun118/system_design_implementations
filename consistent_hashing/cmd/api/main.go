package main

import (
	"flag"
	"fmt"
	"log"

	"arjun118.github.io/consistent_hashing/internal/data"
)

var (
	CacheServers int
	VirtualNodes int
)

func main() {
	consistentHasher := data.GetNewHasher()
	flag.IntVar(&CacheServers, "servers", 4, "initial number of cache servers")
	flag.IntVar(&VirtualNodes, "virtual", 100, "number of virtual nodes per server")
	flag.Parse()
	consistentHasher.VirtualNodesPerServer = VirtualNodes
	fmt.Printf("Initilaizing the hash ring with %d servers...\n", CacheServers)
	fmt.Printf("Adding %d virtual nodes per each server...\n", consistentHasher.VirtualNodesPerServer)
	for range CacheServers {
		_, _, err := consistentHasher.AddServer()
		if err != nil {
			log.Fatalf("Failed to initialize server: %v", err)
		}
	}
	fmt.Println(`
Select one of these
1. get the server name of the key
2. add a server to the ring
3. remove a server from the ring
4. see the distribution of the keys.
	`)
	for {
		var action int
		_, err := fmt.Scanf("%d", &action)
		if err != nil {
			log.Fatal(err)
		}
		switch {
		case action == 1:
			fmt.Printf("Enter the key you want to get: ")
			var ReqKey string
			_, err := fmt.Scanf("%s", &ReqKey)
			if err != nil {
				log.Fatal(err)
			}
			serverName, err := consistentHasher.Get(ReqKey)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("The given key is mapped to %s.\n", serverName)
		case action == 2:
			newServerCount, success, err := consistentHasher.AddServer()
			if !success {
				fmt.Println("Error adding a new server: ", err.Error())
			} else {
				fmt.Printf("Server added successfully, total servers on the ring: %d\n", newServerCount)
			}
		case action == 3:
			fmt.Printf("Enter the server id  you want to remove: ")
			var serverID int
			_, err := fmt.Scanf("%d", &serverID)
			newServerCount, success, err := consistentHasher.RemoveServer(serverID)
			if !success {
				fmt.Println("Error removing a new server: ", err.Error())
			} else {
				fmt.Printf("Server deleted successfully, total servers on the ring: %d\n", newServerCount)
			}
		case action == 4:
			err := consistentHasher.Visualize("./test_data/keys.txt")
			if err != nil {
				log.Fatal(err)
			}
		default:
			fmt.Println("please select from 1,2,3,4.")
		}
	}

}
