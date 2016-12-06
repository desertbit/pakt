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

import "time"

type Timer struct {
	*time.Timer
}

// Reset changes the timer to expire after duration d.
// Any previous active timer is reset and the channel is drained.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
// This should not be done concurrent to other receives from the Timer's channel.
// This fixes the issue: https://github.com/golang/go/issues/14038
func (t *Timer) Reset(d time.Duration) (active bool) {
	if active = t.Stop(); !active {
		<-t.C
	}
	t.Timer.Reset(d)
	return
}

func NewTimer(d time.Duration) *Timer {
	return &Timer{
		Timer: time.NewTimer(d),
	}
}
