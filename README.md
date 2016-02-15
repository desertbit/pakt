# PAKT - Interlink Remote Applications

PAKT provides access to exported methods across a network or other I/O connections similar to RPC.
It handles any I/O connection which implements the golang net.Conn interface.

## Project Name

"Pakt" is the German word for pact (deal). Server and client make a pact and start communicating.

## Sample

Check the sample directory for a simple [server](sample/server) and [client](sample/client) example.

## Introduction

Register a function callable from remote peers:
```go
// Register the function.
s.RegisterFunc("foo", foo)

// ...

func foo(c *pakt.Context) (interface{}, error) {
    data := ... // A struct, string, ...

	// Decode the received data from the peer.
	err := c.Decode(&data)
	if err != nil {
		// The remote peer would get this error.
		return nil, err
	}

	// Just send the data back to the client.
	return data, nil
}
```

Call the remote function from a remote peer.
```go
c, err := s.Call("foo", data)
if err != nil {
    // Handle error. This includes errors returned from the remote function.
}

// Decode the received data from the peer if it returns a data value.
err = c.Decode(&data)
if err != nil {
    // Handle error.
}
```
