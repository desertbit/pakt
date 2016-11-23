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

package pakt

import (
	"net"
	"sync"
)

const (
	socketIDLength       = 20
	newConnChanSize      = 10
	defaultServerWorkers = 20
)

//######################//
//### Server Options ###//
//######################//

// ServerOptions defines server settings.
type ServerOptions struct {
	// Workers defines the amount of routines handling new connections.
	Workers int
}

func (o *ServerOptions) SetDefaults() {
	if o.Workers <= 0 {
		o.Workers = defaultServerWorkers
	}
}

//##############//
//### Server ###//
//###############//

// Server defines the PAKT server implementation.
type Server struct {
	ln net.Listener

	sockets      map[string]*Socket
	socketsMutex sync.RWMutex

	onNewSocket func(*Socket)
	newConnChan chan net.Conn

	closeMutex sync.Mutex
	closeChan  chan struct{}
}

// NewServer creates a new PAKT server.
// One variadic argument defines optional server options.
func NewServer(ln net.Listener, opts ...*ServerOptions) *Server {
	var opt *ServerOptions
	if len(opts) > 0 {
		opt = opts[0]
	} else {
		opt = new(ServerOptions)
	}
	opt.SetDefaults()

	s := &Server{
		ln:          ln,
		sockets:     make(map[string]*Socket),
		newConnChan: make(chan net.Conn, newConnChanSize),
		closeChan:   make(chan struct{}),
	}

	for w := 0; w < opt.Workers; w++ {
		go s.handleConnectionLoop()
	}

	return s
}

// Listen for new socket connections.
// This method is blocking.
func (s *Server) Listen() {
	// Call close on exit.
	defer s.Close()

	// Wait for new client connections.
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			// Check if the listener was closed.
			if s.IsClosed() {
				return
			}

			// Log.
			Log.Warningf("server: accept connection: %v", err)

			// Continue accepting clients.
			continue
		}

		// Pass the new connection to the channel.
		s.newConnChan <- conn
	}
}

// IsClosed returns a boolean indicating if the network listener is closed.
func (s *Server) IsClosed() bool {
	select {
	case <-s.closeChan:
		return true
	default:
		return false
	}
}

// ClosedChan returns a channel which is closed as soon as the network listener is closed.
func (s *Server) ClosedChan() ClosedChan {
	return s.closeChan
}

// OnClose triggers the function as soon as the network listener closes.
// This method can be called multiple times to bind multiple functions.
func (s *Server) OnClose(f func()) {
	go func() {
		<-s.closeChan
		f()
	}()
}

// Close the server and disconnect all connected sockets.
func (s *Server) Close() {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()

	// Check if already closed.
	if s.IsClosed() {
		return
	}

	// Close the close channel.
	close(s.closeChan)

	// Close the network listener.
	err := s.ln.Close()
	if err != nil {
		Log.Warningf("server: failed to close network listener: %v", err)
	}

	// Close all connected sockets.
	for _, s := range s.Sockets() {
		s.Close()
	}
}

// OnNewSocket sets the function which is
// triggered if a new socket connection was made.
// Only set this during initialization.
func (s *Server) OnNewSocket(f func(*Socket)) {
	s.onNewSocket = f
}

// GetSocket obtains a socket by its ID.
// Returns nil if not found.
func (s *Server) GetSocket(id string) (so *Socket) {
	s.socketsMutex.RLock()
	so = s.sockets[id]
	s.socketsMutex.RUnlock()
	return
}

// Sockets returns a list of all current connected sockets.
func (s *Server) Sockets() []*Socket {
	// Lock the mutex.
	s.socketsMutex.RLock()
	defer s.socketsMutex.RUnlock()

	// Create the slice.
	list := make([]*Socket, len(s.sockets))

	// Add all sockets from the map.
	i := 0
	for _, s := range s.sockets {
		list[i] = s
		i++
	}

	return list
}

//###############//
//### Private ###//
//###############//

func (s *Server) handleConnectionLoop() {
	for {
		select {
		case <-s.closeChan:
			return

		case conn := <-s.newConnChan:
			s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	// Catch panics.
	defer func() {
		if e := recover(); e != nil {
			Log.Errorf("server catched panic: %v", e)
		}
	}()

	// Create a new socket.
	socket := NewSocket(conn)

	// Add the new socket to the active sockets map.
	// If the ID is already present, then generate a new one.
	err := func() (err error) {
		socket.id, err = randomString(socketIDLength)
		if err != nil {
			return
		}

		// Lock the mutex.
		s.socketsMutex.Lock()
		defer s.socketsMutex.Unlock()

		// Be sure that the ID is unique.
		for {
			if _, ok := s.sockets[socket.id]; !ok {
				break
			}

			socket.id, err = randomString(socketIDLength)
			if err != nil {
				return
			}
		}

		// Add the socket to the map.
		s.sockets[socket.id] = socket
		return
	}()
	if err != nil {
		Log.Errorf("server: new socket failed: %v", err)
		return
	}

	// Remove the socket from the active sockets map on close.
	go func() {
		// Wait for the socket to close.
		<-socket.closeChan

		s.socketsMutex.Lock()
		delete(s.sockets, socket.id)
		s.socketsMutex.Unlock()
	}()

	// Call the function if defined.
	if s.onNewSocket != nil {
		s.onNewSocket(socket)
	}
}
