package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	want := Request{Version: Version, ID: "request-1", Method: "snapshot", Params: json.RawMessage(`{}`)}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, want); err != nil {
		t.Fatal(err)
	}
	var got Request
	if err := ReadFrame(&buf, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.Method != want.Method || got.Version != want.Version {
		t.Fatalf("got %+v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFrameRejectsLengthsBeforeAllocation(t *testing.T) {
	for _, size := range []uint32{0, MaxMessageBytes + 1, ^uint32(0)} {
		var header [4]byte
		binary.LittleEndian.PutUint32(header[:], size)
		var got Request
		if err := ReadFrame(bytes.NewReader(header[:]), &got); !errors.Is(err, ErrFrameSize) {
			t.Fatalf("size %d: %v", size, err)
		}
	}
}

func TestFrameTruncated(t *testing.T) {
	var got Request
	if err := ReadFrame(bytes.NewReader([]byte{8, 0, 0, 0, '{'}), &got); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v", err)
	}
}

func TestFrameRejectsTrailingJSON(t *testing.T) {
	var got Request
	payload := []byte(`{} {}`)
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.Write(payload)
	if err := ReadFrame(&buf, &got); err == nil {
		t.Fatal("accepted two messages")
	}
}

func TestRequestValidation(t *testing.T) {
	for _, req := range []Request{
		{Version: 2, ID: "a", Method: "snapshot"},
		{Version: Version, Method: "snapshot"},
		{Version: Version, ID: "a"},
	} {
		if req.Validate() == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
}

func TestSnapshotFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	var request Request
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	var framed bytes.Buffer
	if err := WriteFrame(&framed, request); err != nil {
		t.Fatal(err)
	}
	var decoded Request
	if err := ReadFrame(&framed, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "fixture-1" || decoded.Method != "snapshot" {
		t.Fatalf("fixture drift: %+v", decoded)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestFrameShortWrite(t *testing.T) {
	if err := WriteFrame(shortWriter{}, Request{Version: Version, ID: "a", Method: "snapshot"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got %v", err)
	}
}
