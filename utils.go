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
	"bytes"
	"crypto/rand"
	"encoding/binary"

	"github.com/Sirupsen/logrus"
)

const (
	uint32Max = 4294967295
	uint16Max = 65535
)

var (
	// Log is the public logrus value used internally.
	Log *logrus.Logger

	endian binary.ByteOrder = binary.BigEndian
)

func init() {
	// Create a new logger instance.
	Log = logrus.New()
}

func bytesToUint16(data []byte) (ret uint16, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, endian, &ret)
	return
}

func uint16ToBytes(v uint16) (data []byte, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Write(buf, endian, v)
	data = buf.Bytes()
	return
}

func bytesToUint32(data []byte) (ret uint32, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Read(buf, endian, &ret)
	return
}

func uint32ToBytes(v uint32) (data []byte, err error) {
	buf := bytes.NewBuffer(data)
	err = binary.Write(buf, endian, v)
	data = buf.Bytes()
	return
}

// randomString generates a random string.
func randomString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}
