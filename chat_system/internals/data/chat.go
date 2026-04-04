package data

import "net"

type Client struct {
	Conn            net.Conn
	PersonalChannel chan string
}

type Message struct {
	ClientName  string
	MessageType string
	Message     string
}

type Hub map[string]Client
