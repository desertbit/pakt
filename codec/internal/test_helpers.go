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

package internal

import (
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/desertbit/pakt/codec"
)

type testStruct struct {
	Name string
}

// RoundtripTester is a test helper to test a Codec
func RoundtripTester(t *testing.T, c codec.Codec, vals ...interface{}) {
	var val, to interface{}
	if len(vals) > 0 {
		if len(vals) != 2 {
			panic("Wrong number of vals, expected 2")
		}
		val = vals[0]
		to = vals[1]
	} else {
		val = &testStruct{Name: "test"}
		to = &testStruct{}
	}

	encoded, err := c.Encode(val)
	if err != nil {
		t.Fatal("Encode error:", err)
	}
	err = c.Decode(encoded, to)
	if err != nil {
		t.Fatal("Decode error:", err)
	}
	if !reflect.DeepEqual(val, to) {
		t.Fatalf("Roundtrip codec mismatch, expected\n%#v\ngot\n%#v", val, to)
	}
}

func init() {
	gob.Register(&testStruct{})
}
