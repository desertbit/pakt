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

// PAKT provides access to exported methods across a network or other I/O connections similar to RPC.
// It handles any I/O connection which implements the golang net.Conn interface.
package pakt

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

//#################//
//### Constants ###//
//#################//

const (
	defaultMaxMessageSize = 4090

	defaultCallTimeout = 30 * time.Second

	socketTimeout = 45 * time.Second
	pingInterval  = 30 * time.Second
	readTimeout   = 40 * time.Second // Should be bigger than ping interval.
	writeTimeout  = 30 * time.Second
)

//#################//
//### Variables ###//
//#################//

var (
	ErrTimeout                = errors.New("timeout")
	ErrMaxMessageSizeExceeded = errors.New("maximum message size exceeded")
)

//###################//
//### Socket Type ###//
//###################//

type Func func(c *Context) (data interface{}, err error)
type Funcs map[string]Func

type ErrorHook func(funcID string, err error)

type ClosedChan <-chan struct{}

type Socket struct {
	// Value is a custom value which can be set.
	Value interface{}

	id             string
	conn           net.Conn
	writeMutex     sync.Mutex
	callTimeout    time.Duration
	maxMessageSize int

	resetTimeoutChan     chan struct{}
	resetPingTimeoutChan chan struct{}

	closeMutex  sync.Mutex
	isClosed    bool
	closeChan   chan struct{}
	onCloseFunc func()

	funcMap      map[string]Func
	funcMapMutex sync.Mutex
	funcChain    *chain

	errorHook ErrorHook
}

// NewSocket creates a new PAKT socket using the passed connection.
// One variadic argument specifies the socket ID.
// Ready() must be called to start the socket read routine.
func NewSocket(conn net.Conn, vars ...string) *Socket {
	// Create a new socket.
	s := &Socket{
		conn:                 conn,
		callTimeout:          defaultCallTimeout,
		maxMessageSize:       defaultMaxMessageSize,
		resetTimeoutChan:     make(chan struct{}, 1),
		resetPingTimeoutChan: make(chan struct{}, 1),
		closeChan:            make(chan struct{}),
		funcMap:              make(map[string]Func),
		funcChain:            newChain(),
	}

	// Set the ID if specified.
	if len(vars) > 0 {
		s.id = vars[0]
	}

	return s
}

// Ready signalizes the Socket that the initialization is done.
// The socket starts reading from the underlying connection.
// This should be only called once per socket.
func (s *Socket) Ready() {
	// Start the service routines.
	go s.readLoop()
	go s.timeoutLoop()
	go s.pingLoop()
}

// ID returns the socket ID.
func (s *Socket) ID() string {
	return s.id
}

// LocalAddr returns the local network address.
func (s *Socket) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (s *Socket) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// SetMaxMessageSize sets the maximum message size in bytes.
func (s *Socket) SetMaxMessageSize(size int) {
	s.maxMessageSize = size
}

// SetErrorHook sets the error hook function which is triggered, if a local
// remote callable function returns an error. This hook can be used for logging purpose.
// Only set this hook during initialization. This method is not thread-safe.
func (s *Socket) SetErrorHook(h ErrorHook) {
	s.errorHook = h
}

// IsClosed returns a boolean indicating if the socket connection is closed.
func (s *Socket) IsClosed() bool {
	return s.isClosed
}

// OnClose triggers the function as soon as the connection closes.
func (s *Socket) OnClose(f func()) {
	s.onCloseFunc = f
}

// Returns a channel which is closed as soon as the socket is closed.
func (s *Socket) ClosedChan() ClosedChan {
	return s.closeChan
}

// Close the socket connection.
func (s *Socket) Close() {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()

	// Check if already closed.
	if s.isClosed {
		return
	}

	// Update the flag.
	s.isClosed = true

	// Close the close channel.
	close(s.closeChan)

	// Close the socket connection.
	err := s.conn.Close()
	if err != nil {
		Log.Warningf("socket: failed to close socket: %v", err)
	}

	// Call the on close function if defined.
	if s.onCloseFunc != nil {
		s.onCloseFunc()
	}
}

// RegisterFunc registers a remote function.
// This method is thread-safe.
func (s *Socket) RegisterFunc(id string, f Func) {
	// Lock the mutex.
	s.funcMapMutex.Lock()
	defer s.funcMapMutex.Unlock()

	// Set the function to the map.
	s.funcMap[id] = f
}

// RegisterFuncs registers a map of remote functions.
// This method is thread-safe.
func (s *Socket) RegisterFuncs(funcs Funcs) {
	// Lock the mutex.
	s.funcMapMutex.Lock()
	defer s.funcMapMutex.Unlock()

	// Iterate through the map and register the functions.
	for id, f := range funcs {
		s.funcMap[id] = f
	}
}

// SetCallTimeout sets the timeout for call requests.
func (s *Socket) SetCallTimeout(t time.Duration) {
	s.callTimeout = t
}

// Call a remote function and wait for its result.
// This method blocks until the remote socket function returns.
// The first variadic argument specifies an optional data value [interface{}].
// The second variadic argument specifies an optional call timeout [time.Duration].
// Returns ErrTimeout on a timeout.
func (s *Socket) Call(id string, args ...interface{}) (*Context, error) {
	// Create a new channel with its key.
	key, channel := s.funcChain.New()
	defer s.funcChain.Delete(key)

	// Create the header.
	header := &header{
		Type:      headerTypeCall,
		FuncID:    id,
		ReturnKey: key,
	}

	// Write to the client.
	err := s.write(header, args...)
	if err != nil {
		return nil, err
	}

	// Get the timeout duration. If no timeout is passed, use the default.
	timeoutDuration := s.callTimeout
	if len(args) >= 2 {
		d, ok := args[1].(time.Duration)
		if !ok {
			return nil, fmt.Errorf("failed to assert optional variadic call timeout to a time.Duration value")
		}

		timeoutDuration = d
	}

	// Create the timeout.
	timeout := time.NewTimer(timeoutDuration)
	defer timeout.Stop()

	// Wait for a response.
	select {
	case <-timeout.C:
		return nil, ErrTimeout

	case rDataI := <-channel:
		// Assert the return data.
		rData, ok := rDataI.(retChainData)
		if !ok {
			return nil, fmt.Errorf("failed to assert return data")
		}

		// Create a new context.
		context := newContext(s, rData.Data)

		return context, rData.Err
	}
}

//###############//
//### Private ###//
//###############//

type headerType byte

const (
	headerTypeCall   = 1 << iota
	headerTypeReturn = 1 << iota
	headerTypePing   = 1 << iota
	headerTypePong   = 1 << iota
)

type header struct {
	Type      headerType
	FuncID    string
	ReturnKey string
	ReturnErr string
}

type retChainData struct {
	Data []byte
	Err  error
}

func (s *Socket) write(h *header, dataArg ...interface{}) error {
	var data []byte
	var err error

	// Obtain and marshal the data if present.
	if len(dataArg) > 0 {
		// Marshal it msgpack.
		data, err = msgpack.Marshal(dataArg[0])
		if err != nil {
			return err
		}
	}

	// Marshal to msgpack.
	headerData, err := msgpack.Marshal(h)
	if err != nil {
		return err
	}

	// Hint: Payload structure:
	// uint32 (total size) + uint16 (header size) + header + data payload.

	// Get the length of the header in bytes.
	headerLen, err := uint16ToBytes(uint16(len(headerData)))
	if err != nil {
		return err
	}

	// Create and add the header and its size to the buffer.
	buf := append(headerLen, headerData...)

	// Append the data to the buffer.
	buf = append(buf, data...)

	// Check if the maximum size is exceeded.
	if len(buf) > s.maxMessageSize {
		return ErrMaxMessageSizeExceeded
	}

	// Get the total length.
	bufLen, err := uint32ToBytes(uint32(len(buf)))
	if err != nil {
		return err
	}

	// Finally prepend the total length.
	buf = append(bufLen, buf...)

	// Calculate the write deadline.
	writeDeadline := time.Now().Add(writeTimeout)

	// Lock the mutex.
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	// Reset the read deadline.
	s.conn.SetWriteDeadline(writeDeadline)

	// Write to the socket.
	n, err := s.conn.Write(buf)
	if err != nil {
		return err
	} else if n != len(buf) {
		return fmt.Errorf("not all data bytes hav been written to the socket: %v written of total %v bytes", n, len(buf))
	}

	return nil
}

func (s *Socket) read(buf []byte) (int, error) {
	// Reset the read deadline.
	s.conn.SetReadDeadline(time.Now().Add(readTimeout))

	// Read from the socket connection.
	n, err := s.conn.Read(buf)
	if err != nil {
		return 0, err
	}

	// Reset the timeout, because data was successful read from the socket.
	s.resetTimeout()

	return n, nil
}

func (s *Socket) readLoop() {
	// Catch panics.
	defer func() {
		if e := recover(); e != nil {
			Log.Warningf("socket: read loop: catched panic: %v", e)
		}
	}()

	// Close the socket on exit.
	defer s.Close()

	var err error

	var totalBytesRead uint32
	var totalDataBytesRead int
	var dataSize uint32
	dataSizeBuf := make([]byte, 4)

	for {
		// Reset the count.
		totalDataBytesRead = 0

		// Read the data size from the stream.
		for totalDataBytesRead < 4 {
			// Read from the socket.
			n, err := s.read(dataSizeBuf[totalDataBytesRead:])
			if err != nil {
				// Log if not EOF and if not closed.
				if err != io.EOF && !s.isClosed {
					Log.Warningf("socket: failed to read from the socket: %v", err)
				}
				return
			}

			// Update the count.
			totalDataBytesRead += n
		}

		// Obtain the data size.
		dataSize, err = bytesToUint32(dataSizeBuf)
		if err != nil {
			Log.Warningf("socket: failed to read data size from stream: %v", err)
			return
		} else if dataSize == 0 {
			Log.Warning("socket: invalid data size received: data size == 0")
			return
		}

		// Check if the size exceeds the maximum message size.
		if dataSize > uint32(s.maxMessageSize) {
			Log.Warning("socket: maximum message size exceed")
			return
		}

		// Reset the count.
		totalBytesRead = 0

		// Create the buffer.
		buf := make([]byte, dataSize)

		// Read the data from the stream.
		for totalBytesRead < dataSize {
			// Read from the socket.
			n, err := s.read(buf[totalBytesRead:])
			if err != nil {
				// Log if not EOF and if not closed.
				if err != io.EOF && !s.isClosed {
					Log.Warningf("socket: failed to read from the socket: %v", err)
				}
				return
			}

			// Update the count.
			totalBytesRead += uint32(n)
		}

		// Handle the received data bytes in a new goroutine.
		go func() {
			if err = s.handleReceivedData(buf); err != nil {
				Log.Warningf("socket: handle received data: %v", err)
			}
		}()
	}
}

func (s *Socket) handleReceivedData(rawData []byte) (err error) {
	// Catch panics.
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("catched panic: %v", e)
		}
	}()

	// Validate.
	if len(rawData) < 2 {
		return fmt.Errorf("invalid data: not enough bytes to extract the header length")
	}

	// Obtain the length of the header.
	buf := rawData[:2]
	rawData = rawData[2:]
	headerLen, err := bytesToUint16(buf)
	if err != nil {
		return fmt.Errorf("failed to extract the header length")
	}

	// Validate.
	if headerLen == 0 {
		return fmt.Errorf("invalid data: header size is zero")
	} else if uint16(len(rawData)) < headerLen {
		return fmt.Errorf("invalid data: not enough bytes to extract the header")
	}

	// Extract the header data.
	buf = rawData[:headerLen]
	rawData = rawData[headerLen:]

	// Unmarshal the header data.
	headerD := &header{}
	err = msgpack.Unmarshal(buf, headerD)
	if err != nil {
		return fmt.Errorf("msgpack unmarshal header: %v", err)
	}

	switch headerD.Type {
	case headerTypePong:
		// Don't do anything. The socket timeouts have
		// already been reset in the read method.

	case headerTypePing:
		// Create the header.
		headerD = &header{
			Type: headerTypePong,
		}

		// Send a pong response.
		err = s.write(headerD)
		if err != nil {
			return fmt.Errorf("failed to send pong response: %v", err)
		}

	case headerTypeReturn:
		// Get the channel by the return key.
		channel := s.funcChain.Get(headerD.ReturnKey)
		if channel == nil {
			return fmt.Errorf("return request: no channel exists with ID=%v (call timeout exceeded?)", headerD.ReturnKey)
		}

		// Create the error if present.
		var retErr error
		if len(headerD.ReturnErr) > 0 {
			retErr = errors.New(headerD.ReturnErr)
		}

		// Create the channel data.
		rData := retChainData{
			Data: rawData,
			Err:  retErr,
		}

		// Send the return data to the channel.
		channel <- rData

	case headerTypeCall:
		// Obtain the function defined by the ID.
		f, ok := func() (Func, bool) {
			// Lock the mutex.
			s.funcMapMutex.Lock()
			defer s.funcMapMutex.Unlock()

			f, ok := s.funcMap[headerD.FuncID]
			return f, ok
		}()
		if !ok {
			return fmt.Errorf("call request: requested function does not exists: id=%v", headerD.FuncID)
		}

		// Create a new context.
		context := newContext(s, rawData)

		// Call the function.
		retData, retErr := f(context)

		// Get the string representation of the error if present.
		var retErrString string
		if retErr != nil {
			retErrString = retErr.Error()
		}

		// Create the header.
		responseHeaderD := &header{
			Type:      headerTypeReturn,
			ReturnKey: headerD.ReturnKey,
			ReturnErr: retErrString,
		}

		// Write to the client.
		err = s.write(responseHeaderD, retData)
		if err != nil {
			return fmt.Errorf("call request: send return data: %v", err)
		}

		// Call the error hook if defined.
		if retErr != nil && s.errorHook != nil {
			s.errorHook(headerD.FuncID, retErr)
		}

	default:
		return fmt.Errorf("invalid header type: %v", headerD.Type)
	}

	return nil
}
