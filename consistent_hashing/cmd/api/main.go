package main

import (
	"flag"
	"fmt"
	"log"
	"slices"
	"sort"

	"arjun118.github.io/consistent_hashing/internal/data"
)

var (
	CacheServers int
	VirtualNodes int //per server
)

func HandleKeyAddition(key string, hashring *data.Ring) {
	// get the hash
	hash := data.GetHash(key)
	// add the key's hash to the hash space i.e hash ring
	*hashring = append(*hashring, hash)
	// sort after addition of the new key so that we can do search easily
	slices.Sort(*hashring)
	fmt.Println("key added")

}

func FetchKey(key string, hashring *data.Ring, nodemap *data.NodeMap) {
	// get the hash
	hash := data.GetHash(key)
	idx := sort.Search(len(*hashring), func(i int) bool {
		return (*hashring)[i] >= hash
	})
	if idx == len(*hashring) {
		idx = 0 // wrap around (consistent hashing ring)
	}
	fmt.Println("idx: ", idx)
	virtualNodeHash := (*hashring)[idx]
	virtualNodeName := (*nodemap)[virtualNodeHash]
	fmt.Println(virtualNodeHash, virtualNodeName)
	fmt.Printf("the key %s is present on %s server", key, virtualNodeName)
}

func main() {
	HashRing := make(data.Ring, 0)
	NodeMap := make(data.NodeMap)

	flag.IntVar(&CacheServers, "servers", 4, "initial number of cache servers")
	flag.IntVar(&VirtualNodes, "virtual_nodes", 50, "number of virtual nodes per server")
	Servers := []string{}
	for i := range CacheServers {
		Servers = append(Servers, string(i+65))
	}
	fmt.Println("initial servers: ", Servers)

	// virtualnodes := []string{}
	for _, cacheserver := range Servers {
		for vn := range VirtualNodes {
			VirtualNodeName := fmt.Sprintf("Server_%s_%d", cacheserver, vn+1)
			VirtualNodeHash := data.GetHash(VirtualNodeName)
			// add the hash of this virtual node to the hash ring
			HashRing = append(HashRing, VirtualNodeHash)
			// keep track of the node hash to node name
			NodeMap[VirtualNodeHash] = VirtualNodeName
		}
	}
	slices.Sort(HashRing)
	fmt.Println(`
	Select one of these
	1. get the details of a key
	2. see the distribution of the ring
		`)
	for {
		var action int
		_, err := fmt.Scanf("%d", &action)
		if err != nil {
			log.Fatal(err)
		}
		switch {
		case action == 1:
			fmt.Println("get")
			fmt.Printf("enter the key you want to get: ")
			var ReqKey string
			_, err := fmt.Scanf("%s", &ReqKey)
			if err != nil {
				log.Fatal(err)
			}
			FetchKey(ReqKey, &HashRing, &NodeMap)
		case action == 2:
			fmt.Println("dist")
		default:
			fmt.Println("please select from 1,2,3")
		}
	}

}
