# Channels and Interfaces are already "Pointers"
1. Under the hood, a Go chan is just a pointer to a struct holding a queue. Similarly, net.Conn is an interface, which internally acts as a pointer to the underlying socket implementation.

> Rule: Never use a pointer to a channel (*chan string) or a pointer to an interface (*net.Conn). You are just making a pointer to a pointer for no reason. Keep them as values.

- values stored inside maps are not addressable
