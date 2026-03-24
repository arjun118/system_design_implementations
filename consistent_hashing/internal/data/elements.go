package data

import (
	"fmt"
	"hash/fnv"
	"slices"
	"sort"
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
}

func GetNewHasher() *ConsistentHasher {
	return &ConsistentHasher{
		HashRing: []uint32{},
		NodeMap:  make(map[uint32]VirtualNode),
		Servers:  make(map[int]Server),
	}
}

func (ch *ConsistentHasher) Get(key string) string {
	// get the server to which this key is mapped to
	keyHash := ch.GetHash(key)
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
	return serverName
}

func (ch *ConsistentHasher) AddServer() bool {
	// add a server to this hasher
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
	return true
}

func (ch *ConsistentHasher) RemoveServer(serverID int) bool {
	// remove a server and its virtual nodes from the ring
	if _, ok := ch.Servers[serverID]; !ok {
		return false
	}
	serverName := fmt.Sprintf("Server_%d", serverID)
	for i := range ch.VirtualNodesPerServer {
		virtualNodeName := fmt.Sprintf("%s#VN_%d", serverName, i+1)
		virtualNodeHash := ch.GetHash(virtualNodeName)
		// remove the virtualNodeHash from nodemap
		delete(ch.NodeMap, virtualNodeHash)
		// remove the hash from hashring
		idx := sort.Search(len(ch.HashRing), func(i int) bool {
			return ch.HashRing[i] >= virtualNodeHash
		})
		// delete from hash ring if it is a valid index
		if idx < len(ch.HashRing) && ch.HashRing[idx] == virtualNodeHash {
			ch.HashRing = slices.Delete(ch.HashRing, idx, idx+1)
		}
	}
	// remove from server map
	delete(ch.Servers, serverID)
	return true
}

func (ch *ConsistentHasher) Visualize() {

}

func (ch *ConsistentHasher) GetHash(key string) uint32 {
	h := fnv.New32()
	h.Write([]byte(key))
	return h.Sum32()
}
