/*
 *  PAKT - Interlink Remote Applications
 *  Copyright (C) 2016  Roland Singer <roland.singer[at]desertbit.com>
 *
 *  This program is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"log"

	"github.com/desertbit/pakt"
	"github.com/desertbit/pakt/tcp"
)

func main() {
	// Create a new server.
	server, err := tcp.NewServer("127.0.0.1:42193")
	if err != nil {
		log.Fatalln(err)
	}

	// Set the handler function.
	server.OnNewSocket(onNewSocket)

	// Log.
	log.Println("Server listening...")

	// Start the server.
	server.Listen()
}

func onNewSocket(s *pakt.Socket) {
	// Optional set the timeout for a call request.
	// s.SetCallTimeout(time.Minute)

	// Optional set the maximum message size.
	// s.SetMaxMessageSize(100 * 1024)

	// Set a function which is triggered as soon as the socket closed.
	// Optionally use the s.ClosedChan channel.
	s.OnClose(func(s *pakt.Socket) {
		log.Printf("client socket closed with id: %s", s.ID())
	})

	// Register a remote callable function.
	// Optionally use s.RegisterFuncs to register multiple functions at once.
	s.RegisterFunc("foo", foo)

	// Signalize the socket that initialization is done.
	// Start accepting remote requests.
	s.Ready()

	// Log.
	log.Printf("new client socket with id: %s", s.ID())

	// Call the remote function and obtain the return value.
	c, err := s.Call("bar", "Hello World")
	if err != nil {
		log.Fatalln(err)
	}

	// Decode the return value.
	var data string
	err = c.Decode(&data)
	if err != nil {
		log.Fatalln(err)
	}

	// Log.
	log.Printf("received return data from client: %+v", data)
}

func foo(c *pakt.Context) (interface{}, error) {
	// Create a dummy value.
	data := struct {
		A, B string
		C    int
	}{}

	// Decode the received data from the peer to the dummy value.
	err := c.Decode(&data)
	if err != nil {
		// The remote peer would get this error.
		return nil, err
	}

	// Log.
	log.Printf("received data from client: %+v", data)

	// Just send the data back to the client.
	return data, nil
}
