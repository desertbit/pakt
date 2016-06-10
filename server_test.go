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
	"sync"
	"testing"
	"time"

	"github.com/desertbit/pakt"
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

func TestServerSocketsMap(t *testing.T) {
	var wg sync.WaitGroup

	server, err := tcp.NewServer("127.0.0.1:45356")
	require.NoError(t, err)
	require.NotNil(t, server)

	must := func(ok bool, args ...interface{}) {
		if ok {
			return
		}

		wg.Done()
		t.Fatal(args...)
	}

	server.OnNewSocket(func(s *pakt.Socket) {
		s.SetCallTimeout(2 * time.Second)

		s.OnClose(func(s *pakt.Socket) {
			wg.Done()
		})

		s.Ready()

		_, err := s.Call("exit")
		must(err == nil, err)
	})

	go func() {
		server.Listen()
	}()

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			cl, err := tcp.NewClient("127.0.0.1:45356")
			must(err == nil, "client")
			must(cl != nil, "client")

			cl.RegisterFunc("exit", func(c *pakt.Context) (interface{}, error) {
				go func() {
					time.Sleep(500 * time.Millisecond)
					c.Socket().Close()
				}()
				return nil, nil
			})

			cl.Ready()
		}()
	}

	wg.Wait()

	time.Sleep(time.Second)

	require.Len(t, server.Sockets(), 0)

	server.Close()
}
