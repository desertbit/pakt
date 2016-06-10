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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/desertbit/pakt"
	"github.com/desertbit/pakt/tcp"
	"github.com/stretchr/testify/require"
)

func TestServerMultipleSockets(t *testing.T) {
	type data struct {
		A string
		B string
		C int
	}

	var wg sync.WaitGroup

	server, err := tcp.NewServer("127.0.0.1:45354")
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
		wg.Add(1)

		s.SetCallTimeout(5 * time.Second)

		s.RegisterFunc("greet", func(c *pakt.Context) (interface{}, error) {
			var s string
			err := c.Decode(&s)
			must(err == nil, "greet")
			must(s == "Greet", "greet")
			return "Roger", nil
		})

		s.Ready()

		ss := server.GetSocket(s.ID())
		must(ss != nil, "GetSocket")
		must(len(ss.ID()) > 0, "GetSocket")
		must(ss.ID() == s.ID(), "GetSocket")

		d := data{
			A: "Hallo",
			B: "Welt",
			C: 2408234082374023,
		}

		for i := 0; i < 100; i++ {
			c, err := s.Call("call", d)
			must(err == nil, "call request: ", err)

			err = c.Decode(&d)
			must(err == nil, "call request: decode")
			must(d.A == "Hallo", "call request: decode")
			must(d.B == "Welt", "call request: decode")
			must(d.C == 2408234082374023, "call request: decode")

			c, err = s.Call("err")
			must(err != nil, "call err: err != nil:", err)
			must(err.Error() == "ERROR", "call err: err != 'ERROR'", err.Error())
			err = c.Decode(&d)
			must(err != nil, "call err: decode:", err)
		}

		wg.Done()
	})

	go func() {
		server.Listen()
	}()

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			c, err := tcp.NewClient("127.0.0.1:45354")
			must(err == nil, "client")
			must(c != nil, "client")

			c.SetCallTimeout(5 * time.Second)

			c.RegisterFunc("call", func(c *pakt.Context) (interface{}, error) {
				var d data
				err := c.Decode(&d)
				must(err == nil, "client: call")
				must(d.A == "Hallo", "client: call")
				must(d.B == "Welt", "client: call")
				must(d.C == 2408234082374023, "client: call")

				return d, nil
			})

			c.RegisterFunc("err", func(c *pakt.Context) (interface{}, error) {
				return nil, fmt.Errorf("ERROR")
			})

			c.Ready()

			var s string
			for i := 0; i < 100; i++ {
				cc, err := c.Call("greet", "Greet")
				must(err == nil, "client: call greet: ", err)
				err = cc.Decode(&s)
				must(err == nil, "client: call greet")
				must(s == "Roger", "client: call greet")
			}

			wg.Done()
		}()
	}

	wg.Wait()

	server.Close()
	time.Sleep(time.Second)
	require.Len(t, server.Sockets(), 0)
}

func TestSocketMultipleRoutines(t *testing.T) {
	type data struct {
		A string
		B string
		C int
	}

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
		wg.Add(1)

		s.SetCallTimeout(3 * time.Second)

		s.RegisterFunc("greet", func(c *pakt.Context) (interface{}, error) {
			var s string
			err := c.Decode(&s)
			must(err == nil, "greet")
			must(s == "Greet", "greet")
			return "Roger", nil
		})

		s.Ready()

		d := data{
			A: "Hallo",
			B: "Welt",
			C: 2408234082374023,
		}

		for i := 0; i < 10000; i++ {
			wg.Add(1)

			go func() {
				c, err := s.Call("call", d)
				must(err == nil, "call request: ", err)

				var dd data
				err = c.Decode(&dd)
				must(err == nil, "call request: decode")
				must(dd.A == "Hallo", "call request: decode")
				must(dd.B == "Welt", "call request: decode")
				must(dd.C == 2408234082374023, "call request: decode")

				wg.Done()
			}()
		}

		wg.Done()
	})

	go func() {
		server.Listen()
	}()

	wg.Add(1)

	go func() {
		c, err := tcp.NewClient("127.0.0.1:45356")
		must(err == nil, "client")
		must(c != nil, "client")

		c.SetCallTimeout(3 * time.Second)

		c.RegisterFunc("call", func(c *pakt.Context) (interface{}, error) {
			var d data
			err := c.Decode(&d)
			must(err == nil, "client: call")
			must(d.A == "Hallo", "client: call")
			must(d.B == "Welt", "client: call")
			must(d.C == 2408234082374023, "client: call")

			return d, nil
		})

		c.Ready()

		for i := 0; i < 10000; i++ {
			wg.Add(1)

			go func() {
				cc, err := c.Call("greet", "Greet")
				must(err == nil, "client: call greet: ", err)

				var s string
				err = cc.Decode(&s)
				must(err == nil, "client: call greet")
				must(s == "Roger", "client: call greet")

				wg.Done()
			}()
		}

		wg.Done()
	}()

	wg.Wait()

	server.Close()
}

func TestSocketTimeout(t *testing.T) {
	var wg sync.WaitGroup

	server, err := tcp.NewServer("127.0.0.1:45355")
	require.NoError(t, err)
	require.NotNil(t, server)

	must := func(ok bool, args ...interface{}) {
		if ok {
			return
		}

		wg.Done()
		t.Fatal(args...)
	}

	wg.Add(1)

	server.OnNewSocket(func(s *pakt.Socket) {
		s.SetCallTimeout(2 * time.Second)
		s.Ready()

		_, err := s.Call("timeout")
		must(err == pakt.ErrTimeout, err)

		_, err = s.Call("timeout", nil, 5*time.Second)
		must(err == nil, err)

		wg.Done()
	})

	go func() {
		server.Listen()
	}()

	go func() {
		c, err := tcp.NewClient("127.0.0.1:45355")
		must(err == nil, "client")
		must(c != nil, "client")

		c.RegisterFunc("timeout", func(c *pakt.Context) (interface{}, error) {
			time.Sleep(3 * time.Second)
			return nil, nil
		})

		c.Ready()
	}()

	wg.Wait()

	server.Close()
}

func TestSocketClose(t *testing.T) {
	var wg sync.WaitGroup

	server, err := tcp.NewServer("127.0.0.1:45355")
	require.NoError(t, err)
	require.NotNil(t, server)

	must := func(ok bool, args ...interface{}) {
		if ok {
			return
		}

		wg.Done()
		t.Fatal(args...)
	}

	wg.Add(3)

	go func() {
		server.Listen()
	}()

	go func() {
		c, err := tcp.NewClient("127.0.0.1:45355")
		must(err == nil, "client")
		must(c != nil, "client")

		c.OnClose(func(s *pakt.Socket) {
			must(c == s)
			wg.Done()
		})

		c.OnClose(func(s *pakt.Socket) {
			must(c == s)
			wg.Done()
		})

		// Start a goroutine for race test.
		go func() {
			for {
				if c.IsClosed() {
					return
				}
			}
		}()

		must(!c.IsClosed())
		c.Close()
		must(c.IsClosed())

		wg.Done()
	}()

	wg.Wait()
	server.Close()
}
