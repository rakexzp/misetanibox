// Package ipc defines the bounded wire format shared with the native client.
// Transport authentication, deadlines and request dispatch belong to the server;
// framing alone is not an authorization boundary.
package ipc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

const Version = 1
const MaxMessageBytes uint32 = 1 << 20

var ErrFrameSize = errors.New("invalid IPC frame length")

type Request struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r Request) Validate() error {
	if r.Version != Version {
		return errors.New("unsupported_protocol_version")
	}
	if len(r.ID) == 0 || len(r.ID) > 128 {
		return errors.New("invalid_request_id")
	}
	if len(r.Method) == 0 || len(r.Method) > 64 {
		return errors.New("invalid_method")
	}
	return nil
}

type Response struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error codes must be stable and messages must not contain raw network errors,
// subscription URLs, request parameters or headers.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Event is an invalidation, not a raw appcore event payload. On reconnect or a
// sequence gap the client must request a fresh snapshot (no event replay).
type Event struct {
	Version  int    `json:"version"`
	Sequence uint64 `json:"sequence"`
	Event    string `json:"event"`
}

func ReadFrame(r io.Reader, dst any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size == 0 || size > MaxMessageBytes {
		return ErrFrameSize
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func WriteFrame(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > int(MaxMessageBytes) {
		return ErrFrameSize
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		n, err := w.Write(part)
		if err != nil {
			return err
		}
		if n != len(part) {
			return io.ErrShortWrite
		}
	}
	return nil
}
