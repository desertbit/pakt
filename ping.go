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

import "time"

//###############//
//### Private ###//
//###############//

func (s *Socket) resetTimeout() {
	s.resetTimeoutChan <- struct{}{}
	s.resetPingTimeoutChan <- struct{}{}
}

func (s *Socket) timeoutLoop() {
	// Create the timeout.
	timeout := time.NewTimer(socketTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-s.closeChan:
			// Release this goroutine.
			return

		case <-s.resetTimeoutChan:
			// Reset the timeout.
			timeout.Reset(socketTimeout)

		case <-timeout.C:
			Log.Warningf("socket: closed: timeout reached")

			// Close the socket on timeout.
			s.Close()

			return
		}
	}
}

func (s *Socket) pingLoop() {
	// Create the timer.
	timer := time.NewTimer(pingInterval)
	defer timer.Stop()

	for {
		select {
		case <-s.closeChan:
			// Release this goroutine.
			return

		case <-s.resetPingTimeoutChan:
			// Reset the timer.
			timer.Reset(pingInterval)

		case <-timer.C:
			// Send a ping request to the socket peer.
			err := s.write(typePing, nil, nil)
			if err != nil {
				Log.Warningf("socket: failed to send ping request: %v", err)
			}
		}
	}
}
