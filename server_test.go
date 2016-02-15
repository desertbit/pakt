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

package pakt_test

import (
	"testing"
	"time"

	"github.com/desertbit/pakt/tcp"
	"github.com/stretchr/testify/require"
)

func TestServerClose(t *testing.T) {
	server, err := tcp.NewServer("127.0.0.1:34349")
	require.NoError(t, err)
	require.NotNil(t, server)

	// Timeout.
	go func() {
		time.Sleep(3 * time.Second)

		if !server.IsClosed() {
			t.Fatal("failed to close server within timeout")
		}
	}()

	go func() {
		time.Sleep(time.Second)
		server.Close()
	}()

	server.Listen()
}
