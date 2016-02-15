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
	"errors"
	"fmt"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"
	msgpackCodes "gopkg.in/vmihailenco/msgpack.v2/codes"
)

var (
	ErrNoContextData = errors.New("no context data available to decode")
)

//####################//
//### Conext Type ####//
//####################//

type Context struct {
	socket *Socket
	data   []byte
}

func newContext(s *Socket, data []byte) *Context {
	return &Context{
		socket: s,
		data:   data,
	}
}

// Socket returns the socket of the context.
func (c *Context) Socket() *Socket {
	return c.socket
}

// Decode the context data to a custom value.
// The value has to be passed as pointer.
// Returns ErrNoContextData if there is no context data available to decode.
func (c *Context) Decode(v interface{}) error {
	// Check if no data was passed.
	if len(c.data) == 0 || (len(c.data) == 1 && c.data[0] == msgpackCodes.Nil) {
		return ErrNoContextData
	}

	// Unmarshal the data.
	err := msgpack.Unmarshal(c.data, v)
	if err != nil {
		return fmt.Errorf("msgpack unmarshal: %v", err)
	}

	return nil
}
