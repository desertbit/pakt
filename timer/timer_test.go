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

package timer

import (
	"testing"
	"time"
)

func TestTimer(t *testing.T) {
	start := time.Now()

	// Start a new timer with a timeout of 1 second.
	timer := NewTimer(1 * time.Second)

	// Wait for 2 seconds.
	// Meanwhile the timer fired filled the channel.
	time.Sleep(2 * time.Second)

	// Reset the timer. This should act exactly as creating a new timer.
	timer.Reset(1 * time.Second)

	// However this will fire immediately, because the channel was no drained.
	// See issue: https://github.com/golang/go/issues/11513
	<-timer.C

	if int(time.Since(start).Seconds()) != 3 {
		t.Errorf("took ~%v seconds, should be ~3 seconds\n", int(time.Since(start).Seconds()))
	}
}
