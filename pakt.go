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

// Package pakt provides access to exported methods across a network or
// other I/O connections similar to RPC. It handles any I/O connection
// which implements the golang net.Conn interface.
package pakt

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/desertbit/pakt/codec"
	"github.com/desertbit/pakt/codec/msgpack"
)

//#################//
//### Constants ###//
//#################//

const (
	// ProtocolVersion defines the protocol version defined in the specifications.
	ProtocolVersion byte = 0

	// DefaultMaxMessageSize specifies the default maximum message payload size in KiloBytes.
	DefaultMaxMessageSize = 100 * 1024

	// DefaultCallTimeout specifies the default timeout for a call request.
	DefaultCallTimeout = 30 * time.Second
)

const (
	maxHeaderBufferSize = 10 * 1024 // 10 KB

	socketTimeout = 45 * time.Second
	pingInterval  = 30 * time.Second // Should be smaller than the socket timeout.
	readTimeout   = 40 * time.Second // Should be bigger than ping interval.
	writeTimeout  = 30 * time.Second
)

const (
	typeClose      byte = 0
	typePing       byte = 1
	typePong       byte = 2
	typeCall       byte = 3
	typeCallReturn byte = 4
)

//#################//
//### Variables ###//
//#################//

var (
	// ErrClosed defines the error if the socket connection is closed.
	ErrClosed = errors.New("socket closed")

	// ErrTimeout defines the error if the call timeout is reached.
	ErrTimeout = errors.New("timeout")

	// ErrMaxMsgSizeExceeded if the maximum message size is exceeded for a call request.
	ErrMaxMsgSizeExceeded = errors.New("maximum message size exceeded")
)

//###################//
//### Socket Type ###//
//###################//

// Func defines a callable PAKT function.
type Func func(c *Context) (data interface{}, err error)

// Funcs defines a set of functions.
type Funcs map[string]Func

// CallHook defines the callback function.
type CallHook func(s *Socket, funcID string, c *Context)

// ErrorHook defines the error callback function.
type ErrorHook func(s *Socket, funcID string, err error)

// ClosedChan defines a channel which is closed as soon as the socket is closed.
type ClosedChan <-chan struct{}

// OnCloseFunc defines the callback function which is triggered as soon as the socket closes.
type OnCloseFunc func(s *Socket)

// Socket defines the PAKT socket implementation.
type Socket struct {
	// Value is a custom value which can be set.
	Value interface{}

	// Codec holds the encoding and decoding interface.
	Codec codec.Codec

	id             string
	conn           net.Conn
	writeMutex     sync.Mutex
	callTimeout    time.Duration
	maxMessageSize int

	resetTimeoutChan     chan struct{}
	resetPingTimeoutChan chan struct{}

	closeMutex sync.Mutex
	closeChan  chan struct{}

	funcMap      map[string]Func
	funcMapMutex sync.Mutex
	funcChain    *chain

	callHook  CallHook
	errorHook ErrorHook
}

// NewSocket creates a new PAKT socket using the passed connection.
// One variadic argument specifies the socket ID.
// Ready() must be called to start the socket read routine.
func NewSocket(conn net.Conn, vars ...string) *Socket {
	// Create a new socket.
	s := &Socket{
		Codec:                msgpack.Codec,
		conn:                 conn,
		callTimeout:          DefaultCallTimeout,
		maxMessageSize:       DefaultMaxMessageSize,
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
// Only set this during initialization.
func (s *Socket) SetMaxMessageSize(size int) {
	s.maxMessageSize = size
}

// SetCallHook sets the call hook function which is triggered, if a local
// remote callable function will be called. This hook can be used for logging purpose.
// Only set this hook during initialization.
func (s *Socket) SetCallHook(h CallHook) {
	s.callHook = h
}

// SetErrorHook sets the error hook function which is triggered, if a local
// remote callable function returns an error. This hook can be used for logging purpose.
// Only set this hook during initialization.
func (s *Socket) SetErrorHook(h ErrorHook) {
	s.errorHook = h
}

// SetCallTimeout sets the timeout for call requests.
// Only set this during initialization.
func (s *Socket) SetCallTimeout(t time.Duration) {
	s.callTimeout = t
}

// IsClosed returns a boolean indicating if the socket connection is closed.
// This method is thread-safe.
func (s *Socket) IsClosed() bool {
	select {
	case <-s.closeChan:
		return true
	default:
		return false
	}
}

// OnClose triggers the function as soon as the connection closes.
// This method can be called multiple times to bind multiple functions.
// This method is thread-safe.
func (s *Socket) OnClose(f OnCloseFunc) {
	go func() {
		<-s.closeChan
		f(s)
	}()
}

// ClosedChan returns a channel which is closed as soon as the socket is closed.
// This method is thread-safe.
func (s *Socket) ClosedChan() ClosedChan {
	return s.closeChan
}

// Close the socket connection.
// This method is thread-safe.
func (s *Socket) Close() {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()

	// Check if already closed.
	if s.IsClosed() {
		return
	}

	// Close the close channel.
	close(s.closeChan)

	// Call this in a new goroutine to not block (mutex lock).
	// Due to the nested write method call.
	go func() {
		// Tell the other peer, that the connection was closed.
		// Ignore errors. The connection might be closed already.
		_ = s.write(typeClose, nil, nil)

		// Close the socket connection.
		err := s.conn.Close()
		if err != nil {
			Log.Warningf("socket: failed to close the socket: %v", err)
		}
	}()
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

// Call a remote function and wait for its result.
// This method blocks until the remote socket function returns.
// The first variadic argument specifies an optional data value [interface{}].
// The second variadic argument specifies an optional call timeout [time.Duration].
// Returns ErrTimeout on a timeout.
// Returns ErrClosed if the connection is closed.
// This method is thread-safe.
func (s *Socket) Call(id string, args ...interface{}) (*Context, error) {
	// Create a new channel with its key.
	key, channel := s.funcChain.New()
	defer s.funcChain.Delete(key)

	// Create the header.
	header := &headerCall{
		FuncID:    id,
		ReturnKey: key,
	}

	// Obtain the data if present.
	var data interface{}
	if len(args) > 0 {
		data = args[0]
	}

	// Write to the client.
	err := s.write(typeCall, header, data)
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
	case <-s.closeChan:
		return nil, ErrClosed

	case <-timeout.C:
		return nil, ErrTimeout

	case rDataI := <-channel:
		// Assert the return data.
		rData, ok := rDataI.(retChainData)
		if !ok {
			return nil, fmt.Errorf("failed to assert return data")
		}

		return rData.Context, rData.Err
	}
}

//###############//
//### Private ###//
//###############//

type retChainData struct {
	Context *Context
	Err     error
}

func (s *Socket) write(reqType byte, headerI interface{}, dataI interface{}) (err error) {
	var payload, header []byte

	// Marshal the payload data if present.
	if dataI != nil {
		payload, err = s.Codec.Encode(dataI)
		if err != nil {
			return fmt.Errorf("encode: %v", err)
		}
	}

	// Check if the maximum message size is exceeded (Only the payload size without the header).
	if len(payload) > s.maxMessageSize {
		return ErrMaxMsgSizeExceeded
	}

	// Get the length of the payload data in bytes.
	payloadLen, err := uint32ToBytes(uint32(len(payload)))
	if err != nil {
		return err
	}

	// Marshal the header data if present.
	if headerI != nil {
		header, err = s.Codec.Encode(headerI)
		if err != nil {
			return fmt.Errorf("encode header: %v", err)
		}
	}

	// Check if the maximum header size is exceeded.
	if len(header) > maxHeaderBufferSize {
		return fmt.Errorf("maximum header size exceeded")
	}

	// Get the length of the header in bytes.
	headerLen, err := uint16ToBytes(uint16(len(header)))
	if err != nil {
		return err
	}

	// Create the head of the message.
	head := []byte{ProtocolVersion, reqType}

	// Calulcate the total bytes of this message.
	totalLen := len(head) + len(headerLen) + len(payloadLen) + len(header) + len(payload)

	// Ensure that all bytes were written to the connection.
	// Otherwise immediately close the socket to prevent out-of-sync.
	var bytesWritten int
	defer func() {
		if bytesWritten != totalLen {
			err = fmt.Errorf("write: not all message bytes have been written: closing socket")
			s.Close()
		}
	}()

	// Calculate the write deadline.
	writeDeadline := time.Now().Add(writeTimeout)

	// Lock the mutex.
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	// Reset the read deadline.
	s.conn.SetWriteDeadline(writeDeadline)

	// Write to the socket.
	bytesWritten, err = s.conn.Write(head)
	if err != nil {
		return err
	}

	n, err := s.conn.Write(headerLen)
	bytesWritten += n
	if err != nil {
		return err
	}

	n, err = s.conn.Write(payloadLen)
	bytesWritten += n
	if err != nil {
		return err
	}

	if len(header) > 0 {
		n, err = s.conn.Write(header)
		bytesWritten += n
		if err != nil {
			return err
		}
	}

	if len(payload) > 0 {
		n, err = s.conn.Write(payload)
		bytesWritten += n
		if err != nil {
			return err
		}
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
	var n, bytesRead int

	// Message Head.
	headBuf := make([]byte, 8)
	var headerLen16 uint16
	var headerLen int
	var payloadLen32 uint32
	var payloadLen int

	// Read loop.
	for {
		// Read the head from the stream.
		bytesRead = 0
		for bytesRead < 8 {
			n, err = s.read(headBuf[bytesRead:])
			if err != nil {
				// Log only if not closed.
				if err != io.EOF && !s.IsClosed() {
					Log.Warningf("socket: read: %v", err)
				}
				return
			}
			bytesRead += n
		}

		// The first byte is the version field.
		// Check if this protocol version matches.
		if headBuf[0] != ProtocolVersion {
			Log.Warningf("socket: read: invalid protocol version: %v != %v", ProtocolVersion, headBuf[0])
			return
		}

		// Extract the head fields.
		reqType := headBuf[1]

		// Extract the header length.
		headerLen16, err = bytesToUint16(headBuf[2:4])
		if err != nil {
			Log.Warningf("socket: read: failed to extract header length: %v", err)
			return
		}
		headerLen = int(headerLen16)

		// Check if the maximum header size is exceeded.
		if headerLen > maxHeaderBufferSize {
			Log.Warningf("socket: read: maximum header size exceeded")
			return
		}

		// Extract the payload length.
		payloadLen32, err = bytesToUint32(headBuf[4:8])
		if err != nil {
			Log.Warningf("socket: read: failed to extract payload length: %v", err)
			return
		}
		payloadLen = int(payloadLen32)

		// Check if the maximum payload size is exceeded.
		if payloadLen > s.maxMessageSize {
			Log.Warningf("socket: read: maximum message size exceeded")
			return
		}

		// Read the header bytes from the stream.
		var headerBuf []byte
		if headerLen > 0 {
			headerBuf = make([]byte, headerLen)
			bytesRead = 0
			for bytesRead < headerLen {
				n, err = s.read(headerBuf[bytesRead:])
				if err != nil {
					// Log only if not closed.
					if err != io.EOF && !s.IsClosed() {
						Log.Warningf("socket: read: %v", err)
					}
					return
				}
				bytesRead += n
			}
		}

		// Read the payload bytes from the stream.
		var payloadBuf []byte
		if payloadLen > 0 {
			payloadBuf = make([]byte, payloadLen)
			bytesRead = 0
			for bytesRead < payloadLen {
				n, err = s.read(payloadBuf[bytesRead:])
				if err != nil {
					// Log only if not closed.
					if err != io.EOF && !s.IsClosed() {
						Log.Warningf("socket: read: %v", err)
					}
					return
				}
				bytesRead += n
			}
		}

		// Handle the received message in a new goroutine.
		go func() {
			err := s.handleReceivedMessage(reqType, headerBuf, payloadBuf)
			if err != nil {
				Log.Warningf("socket: %v", err)
			}
		}()
	}
}

func (s *Socket) handleReceivedMessage(reqType byte, headerBuf, payloadBuf []byte) (err error) {
	// Catch panics.
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("catched panic: %v", e)
		}
	}()

	// Reset the timeout, because data was successful read from the socket.
	s.resetTimeout()

	// Check the request type.
	switch reqType {
	case typeClose:
		// The socket peer has closed the connection.
		s.Close()

	case typePing:
		// The socket peer has requested a pong response.
		err = s.write(typePong, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to send pong response: %v", err)
		}

	case typePong:
		// Don't do anything. The socket timeouts have already been reset.

	case typeCall:
		return s.handleCallRequest(headerBuf, payloadBuf)

	case typeCallReturn:
		return s.handleCallReturnRequest(headerBuf, payloadBuf)

	default:
		return fmt.Errorf("invalid request type: %v", reqType)
	}

	return nil
}

func (s *Socket) handleCallRequest(headerBuf, payloadBuf []byte) (err error) {
	// Decode the header.
	var header headerCall
	err = s.Codec.Decode(headerBuf, &header)
	if err != nil {
		return fmt.Errorf("decode call header: %v", err)
	}

	// Obtain the function defined by the ID.
	f, ok := func() (Func, bool) {
		// Lock the mutex.
		s.funcMapMutex.Lock()
		defer s.funcMapMutex.Unlock()

		f, ok := s.funcMap[header.FuncID]
		return f, ok
	}()
	if !ok {
		return fmt.Errorf("call request: requested function does not exists: id=%v", header.FuncID)
	}

	// Create a new context.
	context := newContext(s, payloadBuf)

	// Call the call hook if defined.
	if s.callHook != nil {
		s.callHook(s, header.FuncID, context)
	}

	// Call the function.
	retData, retErr := f(context)

	// Get the string representation of the error if present.
	var retErrString string
	if retErr != nil {
		retErrString = retErr.Error()
	}

	// Create the return header.
	retHeader := &headerCallReturn{
		ReturnKey: header.ReturnKey,
		ReturnErr: retErrString,
	}

	// Write to the client.
	err = s.write(typeCallReturn, retHeader, retData)
	if err != nil {
		return fmt.Errorf("call request: send return request: %v", err)
	}

	// Call the error hook if defined.
	if retErr != nil && s.errorHook != nil {
		s.errorHook(s, header.FuncID, retErr)
	}

	return nil
}

func (s *Socket) handleCallReturnRequest(headerBuf, payloadBuf []byte) (err error) {
	// Decode the header.
	var header headerCallReturn
	err = s.Codec.Decode(headerBuf, &header)
	if err != nil {
		return fmt.Errorf("decode call return header: %v", err)
	}

	// Get the channel by the return key.
	channel := s.funcChain.Get(header.ReturnKey)
	if channel == nil {
		return fmt.Errorf("call return request failed (call timeout exceeded?)")
	}

	// Create the error if present.
	var retErr error
	if len(header.ReturnErr) > 0 {
		retErr = errors.New(header.ReturnErr)
	}

	// Create a new context.
	context := newContext(s, payloadBuf)

	// Create the channel data.
	rData := retChainData{
		Context: context,
		Err:     retErr,
	}

	// Send the return data to the channel.
	// Ensure that there is a receiving endpoint.
	// Otherwise we would have a lost blocking goroutine.
	select {
	case channel <- rData:
		return nil

	default:
		// Retry with a timeout.
		timeout := time.NewTimer(time.Second)
		defer timeout.Stop()

		select {
		case channel <- rData:
			return nil
		case <-timeout.C:
			return fmt.Errorf("call return request failed (call timeout exceeded?)")
		}
	}
}
