package data

import "net"

type Client struct {
	Conn            net.Conn
	Name            string
	PersonalChannel chan string
}

type Message struct {
	ClientName  string
	MessageType string
	Message     string
}

// room -> then map remote addrs to Clients - need to design this

type Hub map[string]Client

type Rooms map[string]Hub

type ClientLocations map[string]string
