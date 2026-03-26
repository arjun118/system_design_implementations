package data

import (
	"bufio"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"sync"

	"github.com/pterm/pterm"
)

type Server struct {
	ID   int
	Name string
}

type VirtualNode struct {
	ServerID int
	VNID     int
}

type ConsistentHasher struct {
	HashRing []uint32
	NodeMap  map[uint32]VirtualNode
	Servers  map[int]Server // server id -> server

	VirtualNodesPerServer int
	nextServerId          int

	mu sync.RWMutex
}

func GetNewHasher() *ConsistentHasher {
	return &ConsistentHasher{
		HashRing: []uint32{},
		NodeMap:  make(map[uint32]VirtualNode),
		Servers:  make(map[int]Server),
	}
}

func (ch *ConsistentHasher) Get(key string) (string, error) {
	// get the server to which this key is mapped to
	if len(ch.HashRing) == 0 {
		return "", errors.New("no servers exist")
	}
	keyHash := ch.GetHash(key)
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	idx := sort.Search(len(ch.HashRing), func(i int) bool {
		// this performs binary search on the sorted slice
		return ch.HashRing[i] >= keyHash
	})
	if idx == len(ch.HashRing) {
		idx = 0 // wrap around (consistent hashing ring)
	}
	virtualNodeHash := ch.HashRing[idx]
	virtualNode := ch.NodeMap[virtualNodeHash]
	serverId := virtualNode.ServerID
	serverName := ch.Servers[serverId].Name
	return serverName, nil
}

func (ch *ConsistentHasher) AddServer() (int, bool, error) {
	// returns the current number of servers, operation status and error if any
	// add a server to this hasher
	// can also use time of check and time of use fix  approach to locking here
	ch.mu.Lock()
	defer ch.mu.Unlock()

	nextServerid := ch.nextServerId

	serverName := fmt.Sprintf("Server_%d", nextServerid)
	virtualNodeHashes := []uint32{}
	server := Server{
		ID:   nextServerid,
		Name: serverName,
	}
	//update servers
	ch.Servers[nextServerid] = server
	for i := range ch.VirtualNodesPerServer {
		virtualNodeName := fmt.Sprintf("%s#VN_%d", serverName, i+1)
		virtualNodeHash := ch.GetHash(virtualNodeName)
		virtualNodeHashes = append(virtualNodeHashes, virtualNodeHash)
		virtualNode := VirtualNode{
			ServerID: nextServerid,
			VNID:     i + 1,
		}
		//update node NodeMap
		ch.NodeMap[virtualNodeHash] = virtualNode
	}
	// add virtual nodes
	ch.HashRing = append(ch.HashRing, virtualNodeHashes...)
	// sort after server addition
	slices.Sort(ch.HashRing)
	// update the next server id
	ch.nextServerId = nextServerid + 1
	return len(ch.Servers), true, nil
}

func (ch *ConsistentHasher) RemoveServer(serverID int) (int, bool, error) {
	// returns the number of servers left and the status of removal
	// remove a server and its virtual nodes from the ring
	// serverName := fmt.Sprintf("Server_%d", serverID)
	// hashesToRemove := make([]uint32, 0, ch.VirtualNodesPerServer)

	// for i := 1; i <= ch.VirtualNodesPerServer; i++ {
	// 	vnName := fmt.Sprintf("%s#VN_%d", serverName, i)
	// 	hashesToRemove = append(hashesToRemove, ch.GetHash(vnName))
	// }

	// // verify and commit (inside the lock)
	// ch.mu.Lock()
	// defer ch.mu.Unlock()
	// // time of check and time of use verification
	// if _, ok := ch.Servers[serverID]; !ok {
	// 	// some one beat us to removing the serverID and we just return that it doesnot exist
	// 	return len(ch.Servers), false, errors.New("server doesnot exist")
	// }
	// // if not,commit the deletions
	// for _, virtualNodeHash := range hashesToRemove {
	// 	delete(ch.NodeMap, virtualNodeHash)

	// 	idx := sort.Search(len(ch.HashRing), func(i int) bool {
	// 		return ch.HashRing[i] >= virtualNodeHash
	// 	})

	// 	if idx < len(ch.HashRing) && ch.HashRing[idx] == virtualNodeHash {
	// 		ch.HashRing = slices.Delete(ch.HashRing, idx, idx+1)
	// 	}
	// }

	// delete(ch.Servers, serverID)
	// return len(ch.Servers), true, nil
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if _, ok := ch.Servers[serverID]; !ok {
		return len(ch.Servers), false, errors.New("server does not exist")

	}
	serverName := ch.Servers[serverID].Name
	hashesToRemove := make(map[uint32]struct{}, ch.VirtualNodesPerServer)
	for i := 1; i <= ch.VirtualNodesPerServer; i++ {
		vnName := fmt.Sprintf("%s#VN_%d", serverName, i)
		h := ch.GetHash(vnName)
		hashesToRemove[h] = struct{}{}

		// Clean up the NodeMap while we are already looping
		delete(ch.NodeMap, h)
	}
	// rebuild, in place filtering
	// cut the slice but keep the capacity
	filteredRing := ch.HashRing[:0]
	for _, h := range ch.HashRing {
		if _, exists := hashesToRemove[h]; !exists {
			filteredRing = append(filteredRing, h)
		}
	}
	ch.HashRing = filteredRing
	delete(ch.Servers, serverID)
	return len(ch.Servers), true, nil
}

func (ch *ConsistentHasher) Visualize(keysPath string) error {
	// Check if the file exists
	_, err := os.Stat(keysPath)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("file doesnot exist")
	}
	if err != nil {
		return err
	}

	distribution := make(map[string]int)
	file, err := os.Open(keysPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		serverName, err := ch.Get(line)
		if err != nil {
			log.Fatal(err)
		}
		distribution[serverName]++
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// --- pterm specific implementation ---

	// 1. Convert the distribution map into a slice of pterm.Bar
	var bars pterm.Bars
	for key, value := range distribution {
		bars = append(bars, pterm.Bar{
			Label: key,
			Value: value,
		})
	}

	// 2. Sort the bars alphabetically by server name to ensure the output
	// doesn't scramble randomly every time you run the script.
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Label < bars[j].Label
	})

	// 3. Render the horizontal bar chart
	pterm.DefaultBarChart.
		WithHorizontal(). // Makes the chart horizontal
		WithShowValue().  // Displays the numerical value at the end of the bar
		WithBars(bars).   // Injects our data
		Render()

	return nil
}

func (ch *ConsistentHasher) GetHash(key string) uint32 {
	// smooth
	// h := fnv.New32a()
	// h.Write([]byte(key))
	// return h.Sum32()
	// smoother
	// return crc32.ChecksumIEEE([]byte(key))
	hash := md5.Sum([]byte(key))
	//smoothest
	// Grab the first 4 bytes of the MD5 hash and convert to uint32
	return binary.BigEndian.Uint32(hash[0:4])
}
