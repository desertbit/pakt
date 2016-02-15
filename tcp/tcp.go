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

package tcp

import (
	"net"

	"github.com/desertbit/pakt"
)

// NewClient create a new tcp client, connects to the remote address
// and returns a new PAKT socket.
func NewClient(remoteAddr string) (*pakt.Socket, error) {
	// Connect to the server.
	conn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		return nil, err
	}

	// Create a new pakt socket.
	s := pakt.NewSocket(conn)

	return s, nil
}

// NewServer create a new tcp server and returns a new PAKT server.
func NewServer(listenAddr string) (*pakt.Server, error) {
	// Connect to the server.
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	// Create a new pakt server.
	s := pakt.NewServer(ln)

	return s, nil
}
