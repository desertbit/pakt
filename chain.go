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

func (c *chain) New() (string, chainChan) {
	// Create a new channel.
	channel := make(chainChan)

	// Create a new ID.
	id := randomString(chainIDLength)
	func() {
		// Lock the mutex.
		c.chainChanMutex.Lock()
		defer c.chainChanMutex.Unlock()

		// Be sure that the ID is unique.
		for {
			if _, ok := c.chanMap[id]; !ok {
				break
			}

			id = randomString(chainIDLength)
		}

		// Add the channel to the map.
		c.chanMap[id] = channel
	}()

	return id, channel
}

// Returns nil if not found.
func (c *chain) Get(id string) chainChan {
	// Lock the mutex.
	c.chainChanMutex.Lock()
	defer c.chainChanMutex.Unlock()

	// Obtain the channel.
	return c.chanMap[id]
}

func (c *chain) Delete(id string) {
	// Lock the mutex.
	c.chainChanMutex.Lock()
	defer c.chainChanMutex.Unlock()

	// Delete the channel from the map.
	delete(c.chanMap, id)
}
