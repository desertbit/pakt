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

import "sync"

const (
	chainIDLength = 10
)

//##################//
//### Chain Type ###//
//##################//

type chainChan chan interface{}

type chain struct {
	chanMap        map[string]chainChan
	chainChanMutex sync.Mutex
}

func newChain() *chain {
	return &chain{
		chanMap: make(map[string]chainChan),
	}
}

func (c *chain) New() (id string, cc chainChan, err error) {
	// Create a new channel.
	cc = make(chainChan)

	// Create a new ID and ensure it is unqiue.
	var added bool
	for {
		id, err = randomString(chainIDLength)
		if err != nil {
			return
		}

		added = func() bool {
			c.chainChanMutex.Lock()
			defer c.chainChanMutex.Unlock()

			if _, ok := c.chanMap[id]; ok {
				return false
			}

			c.chanMap[id] = cc
			return true
		}()
		if added {
			break
		}
	}

	return
}

// Returns nil if not found.
func (c *chain) Get(id string) (cc chainChan) {
	c.chainChanMutex.Lock()
	cc = c.chanMap[id]
	c.chainChanMutex.Unlock()
	return
}

func (c *chain) Delete(id string) {
	c.chainChanMutex.Lock()
	delete(c.chanMap, id)
	c.chainChanMutex.Unlock()
}
