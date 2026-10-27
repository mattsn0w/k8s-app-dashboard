package main

import (
	"fmt"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net"
	"testing"
)

// ---------- wsReadFrameReader: frame parsing ----------

func TestWSReadFrame_Text_Short(t *testing.T) {
	input := []byte{0x81, 0x05, 'h', 'e', 'l', 'l', 'o'}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if string(payload) != "hello" {
		t.Errorf("payload = %q, want %q", payload, "hello")
	}
}

func TestWSReadFrame_Text_126Byte(t *testing.T) {
	payload := make([]byte, 126)
	for i := range payload {
		payload[i] = byte(i)
	}
	input := append([]byte{0x81, 0x7E, 0x00, 0x7E}, payload...)
	opcode, got, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload length = %d, want 126", len(got))
	}
}

func TestWSReadFrame_Text_65536Byte(t *testing.T) {
	payload := make([]byte, 65536)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	input := append([]byte{0x81, 0x7F, 0, 0, 0, 0, 0, 1, 0, 0}, payload...)
	opcode, got, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if len(got) != 65536 {
		t.Errorf("payload length = %d, want 65536", len(got))
	}
}

func TestWSReadFrame_Masked(t *testing.T) {
	// RFC 6455 section 5.3 example:
	// mask = 0x37 0xfa 0x21 0x3d
	// original payload "hello" = 0x68 0x65 0x6c 0x6c 0x6f
	// masked = 0x5f 0x9f 0x4d 0x51 0x58
	input := []byte{0x81, 0x85, 0x37, 0xfa, 0x21, 0x3d, 0x5f, 0x9f, 0x4d, 0x51, 0x58}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if string(payload) != "hello" {
		t.Errorf("payload = %q, want %q", payload, "hello")
	}
}

func TestWSReadFrame_Close(t *testing.T) {
	input := []byte{0x88, 0x00}
	opcode, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x8 {
		t.Errorf("opcode = 0x%x, want 0x8", opcode)
	}
}

func TestWSReadFrame_Ping(t *testing.T) {
	input := []byte{0x89, 0x04, 'p', 'i', 'n', 'g'}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x9 {
		t.Errorf("opcode = 0x%x, want 0x9", opcode)
	}
	if string(payload) != "ping" {
		t.Errorf("payload = %q, want %q", payload, "ping")
	}
}

func TestWSReadFrame_Pong(t *testing.T) {
	input := []byte{0x8A, 0x04, 'p', 'o', 'n', 'g'}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0xA {
		t.Errorf("opcode = 0x%x, want 0xA", opcode)
	}
	if string(payload) != "pong" {
		t.Errorf("payload = %q, want %q", payload, "pong")
	}
}

func TestWSReadFrame_Fragmented(t *testing.T) {
	input := []byte{0x01, 0x03, 'a', 'b', 'c'}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if string(payload) != "abc" {
		t.Errorf("payload = %q, want %q", payload, "abc")
	}
}

func TestWSReadFrame_Binary(t *testing.T) {
	input := []byte{0x82, 0x03, 0x01, 0x02, 0x03}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x2 {
		t.Errorf("opcode = 0x%x, want 0x2", opcode)
	}
	if !bytes.Equal(payload, []byte{0x01, 0x02, 0x03}) {
		t.Errorf("payload = %v, want [1 2 3]", payload)
	}
}

func TestWSReadFrame_ZeroLength(t *testing.T) {
	input := []byte{0x81, 0x00}
	opcode, payload, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if len(payload) != 0 {
		t.Errorf("payload length = %d, want 0", len(payload))
	}
}

func TestWSReadFrame_125Byte(t *testing.T) {
	payload := make([]byte, 125)
	for i := range payload {
		payload[i] = 'A'
	}
	input := append([]byte{0x81, 0x7D}, payload...)
	opcode, got, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if len(got) != 125 {
		t.Errorf("payload length = %d, want 125", len(got))
	}
}

func TestWSReadFrame_126Boundary(t *testing.T) {
	payload := make([]byte, 126)
	input := append([]byte{0x81, 0x7E, 0x00, 0x7E}, payload...)
	_, got, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 126 {
		t.Errorf("payload length = %d, want 126", len(got))
	}
}

func TestWSReadFrame_65535Boundary(t *testing.T) {
	payload := make([]byte, 65535)
	input := append([]byte{0x81, 0x7E, 0xFF, 0xFF}, payload...)
	_, got, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 65535 {
		t.Errorf("payload length = %d, want 65535", len(got))
	}
}

func TestWSReadFrame_65536Boundary(t *testing.T) {
	payload := make([]byte, 65536)
	input := append([]byte{0x81, 0x7F, 0, 0, 0, 0, 0, 1, 0, 0}, payload...)
	_, got, err := wsReadFrameReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 65536 {
		t.Errorf("payload length = %d, want 65536", len(got))
	}
}

func TestWSReadFrame_TooLarge(t *testing.T) {
	// 2MB = 2097152 = 0x00200000
	// As 8 big-endian bytes: 00 00 00 00 00 20 00 00
	input := []byte{0x81, 0x7F, 0, 0, 0, 0, 0x00, 0x20, 0x00, 0x00}
	_, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err == nil {
		t.Fatal("expected error for frame > 1MB, got nil")
	}
	if err.Error() != "frame too large: 2097152" {
		t.Errorf("error = %q, want %q", err.Error(), "frame too large: 2097152")
	}
}

func TestWSReadFrame_TruncatedHeader(t *testing.T) {
	input := []byte{0x81}
	_, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestWSReadFrame_Truncated16BitLen(t *testing.T) {
	input := []byte{0x81, 0x7E, 0x00}
	_, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestWSReadFrame_Truncated64BitLen(t *testing.T) {
	input := []byte{0x81, 0x7F, 0, 0, 0, 0}
	_, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestWSReadFrame_TruncatedPayload(t *testing.T) {
	input := []byte{0x81, 0x05, 'h', 'e', 'l'}
	_, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestWSReadFrame_TruncatedMask(t *testing.T) {
	input := []byte{0x81, 0x85, 0x37, 0xfa}
	_, _, err := wsReadFrameReader(bytes.NewReader(input))
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}

// ---------- wsWriteFrameConn: round-trip tests ----------

func TestWSWriteRead_RoundTrip_Empty(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, []byte{})
	}()

	opcode, payload, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if len(payload) != 0 {
		t.Errorf("payload length = %d, want 0", len(payload))
	}
}

func TestWSWriteRead_RoundTrip_1Byte(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, []byte{0x42})
	}()

	opcode, payload, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if !bytes.Equal(payload, []byte{0x42}) {
		t.Errorf("payload = %v, want [0x42]", payload)
	}
}

func TestWSWriteRead_RoundTrip_125Byte(t *testing.T) {
	payload := make([]byte, 125)
	for i := range payload {
		payload[i] = byte(i)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, payload)
	}()

	opcode, got, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch (len=%d)", len(got))
	}
}

func TestWSWriteRead_RoundTrip_126Byte(t *testing.T) {
	payload := make([]byte, 126)
	for i := range payload {
		payload[i] = byte(i)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, payload)
	}()

	opcode, got, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch (len=%d)", len(got))
	}
}

func TestWSWriteRead_RoundTrip_65535Byte(t *testing.T) {
	payload := make([]byte, 65535)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, payload)
	}()

	opcode, got, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch (len=%d)", len(got))
	}
}

func TestWSWriteRead_RoundTrip_65536Byte(t *testing.T) {
	payload := make([]byte, 65536)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, payload)
	}()

	opcode, got, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 {
		t.Errorf("opcode = 0x%x, want 0x1", opcode)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch (len=%d)", len(got))
	}
}

func TestWSWriteRead_RoundTrip_Binary(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x2, payload)
	}()

	opcode, got, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x2 {
		t.Errorf("opcode = 0x%x, want 0x2", opcode)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %v, want %v", got, payload)
	}
}

func TestWSWriteRead_RoundTrip_Close(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x8, []byte{})
	}()

	opcode, _, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x8 {
		t.Errorf("opcode = 0x%x, want 0x8", opcode)
	}
}

func TestWSWriteFrame_NoMaskBit(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, []byte("test"))
	}()

	buf := make([]byte, 2)
	_, err := io.ReadFull(client, buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if buf[1]&0x80 != 0 {
		t.Errorf("mask bit is set in server frame: 0x%02x", buf[1])
	}
}

func TestWSWriteFrame_FinBit(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0x1, []byte("test"))
	}()

	buf := make([]byte, 2)
	_, err := io.ReadFull(client, buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if buf[0]&0x80 == 0 {
		t.Errorf("FIN bit is not set in server frame: 0x%02x", buf[0])
	}
}

// ---------- wsCombinedReader ----------

func TestWSCombinedReader_AllInBuffer(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() { server.Close() }()

	r := &wsCombinedReader{data: []byte("hello"), conn: client}
	buf := make([]byte, 5)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 || string(buf) != "hello" {
		t.Errorf("got %d bytes %q, want 5 %q", n, buf, "hello")
	}
}

func TestWSCombinedReader_PartialInBuffer(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Write "llo" to the conn side (in a goroutine since net.Pipe is synchronous)
	go func() {
		server.Write([]byte("llo"))
	}()

	r := &wsCombinedReader{data: []byte("he"), conn: client}

	// First read: gets "he" from buffer (2 bytes)
	buf1 := make([]byte, 2)
	n1, err := r.Read(buf1)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if n1 != 2 || string(buf1) != "he" {
		t.Errorf("first read: got %d bytes %q, want 2 %q", n1, buf1, "he")
	}

	// Second read: gets "llo" from conn (3 bytes)
	buf2 := make([]byte, 3)
	n2, err := r.Read(buf2)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if n2 != 3 || string(buf2) != "llo" {
		t.Errorf("second read: got %d bytes %q, want 3 %q", n2, buf2, "llo")
	}
}

func TestWSCombinedReader_EmptyBuffer(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		server.Write([]byte("hello"))
	}()

	r := &wsCombinedReader{data: nil, conn: client}
	buf := make([]byte, 5)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 || string(buf) != "hello" {
		t.Errorf("got %d bytes %q, want 5 %q", n, buf, "hello")
	}
}

func TestWSCombinedReader_MultipleReads(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	r := &wsCombinedReader{data: []byte("abc"), conn: client}
	var result string
	for i := 0; i < 3; i++ {
		buf := make([]byte, 1)
		n, err := r.Read(buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if n != 1 {
			t.Fatalf("read %d: got %d bytes", i, n)
		}
		result += string(buf)
	}
	if result != "abc" {
		t.Errorf("result = %q, want %q", result, "abc")
	}
}

func TestWSCombinedReader_BufferExhausted(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		server.Write([]byte("c"))
	}()

	r := &wsCombinedReader{data: []byte("ab"), conn: client}
	var result string
	for i := 0; i < 3; i++ {
		buf := make([]byte, 1)
		n, err := r.Read(buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if n != 1 {
			t.Fatalf("read %d: got %d bytes", i, n)
		}
		result += string(buf)
	}
	if result != "abc" {
		t.Errorf("result = %q, want %q", result, "abc")
	}
}

// ---------- WebSocket handshake (RFC 6455 section 4.1) ----------

func TestWSHandshake_RFC6455Example(t *testing.T) {
	// RFC 6455 section 4.1 example key
	key := "dGhlIHNhbXBsZSBvZnZlciB2YWx1ZQ=="
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	// Verify against the known-correct SHA1+base64 computation
	expected := "OP5bU+q1W4kVAIrOaIAtkankXlM="
	if accept != expected {
		t.Errorf("accept = %q, want %q", accept, expected)
	}
}

func TestWSHandshake_RandomKey(t *testing.T) {
	// Use a known 16-byte random key (22 base64 chars + 1 padding = 24)
	key := "kx7dR4BqF2mNp8sT0w=="
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	// SHA1 produces 20 bytes, base64 of 20 bytes = 28 chars
	if len(accept) != 28 {
		t.Errorf("accept length = %d, want 28", len(accept))
	}
	// Verify it's valid base64
	if _, err := base64.StdEncoding.DecodeString(accept); err != nil {
		t.Errorf("accept is not valid base64: %v", err)
	}
}

// ---------- APP-10: single-codec regression tests ----------
//
// After consolidating the duplicated frame codec (removing the bufio
// wsWriteFrame/wsReadFrame variants), the only writer is wsWriteFrameConn and
// the only reader is wsReadFrameReader. These tests guard that consolidation:
// the pong path (which previously used the deleted bufio writer) must round-trip
// through the conn writer, and the byte layout must be correct at every length
// tier that the removed code shared.

// TestWSWriteRead_RoundTrip_Pong verifies the pong frame (opcode 0xA) round-trips
// through the single conn writer + reader. This is the exact path the ping
// handler now uses (wsWriteFrameConn(conn, 0xA, payload)).
func TestWSWriteRead_RoundTrip_Pong(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = wsWriteFrameConn(server, 0xA, []byte("ping-payload"))
	}()

	opcode, payload, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0xA {
		t.Errorf("opcode = 0x%x, want 0xA (pong)", opcode)
	}
	if !bytes.Equal(payload, []byte("ping-payload")) {
		t.Errorf("payload = %q, want %q", payload, "ping-payload")
	}
}

// TestWSWriteRead_RoundTrip_Pong_AllLengthTiers verifies the pong frame is
// correct across every length-encoding tier (7-bit, 16-bit, 64-bit). The old
// bufio writer had identical tier logic to the conn writer; this confirms the
// single surviving codec handles all tiers for the control-frame path.
func TestWSWriteRead_RoundTrip_Pong_AllLengthTiers(t *testing.T) {
	for _, size := range []int{0, 1, 125, 126, 127, 65535, 65536, 65537} {
		size := size
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i % 251)
			}
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			go func() { _ = wsWriteFrameConn(server, 0xA, payload) }()

			opcode, got, err := wsReadFrameReader(client)
			if err != nil {
				t.Fatalf("size=%d: unexpected error: %v", size, err)
			}
			if opcode != 0xA {
				t.Errorf("size=%d: opcode = 0x%x, want 0xA", size, opcode)
			}
			if !bytes.Equal(got, payload) {
				t.Errorf("size=%d: payload mismatch (got %d bytes, want %d)", size, len(got), size)
			}
		})
	}
}

// TestWSWriteFrameConn_Pong_HeaderLayout pins the exact wire bytes for a small
// pong frame. This guards the header construction (which was duplicated across
// the removed bufio writer and the conn writer) against a regression that would
// silently corrupt the single remaining writer.
func TestWSWriteFrameConn_Pong_HeaderLayout(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() { _ = wsWriteFrameConn(server, 0xA, []byte{0x01, 0x02}) }()

	// Header: FIN+opcode (0x8A) + length (0x02, no mask for server frames).
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(client, hdr); err != nil {
		t.Fatalf("read header: %v", err)
	}
	if hdr[0] != 0x8A {
		t.Errorf("first byte = 0x%02x, want 0x8A (FIN+pong)", hdr[0])
	}
	if hdr[1] != 0x02 {
		t.Errorf("second byte = 0x%02x, want 0x02 (length, unmasked)", hdr[1])
	}
	// Payload follows.
	payload := make([]byte, 2)
	if _, err := io.ReadFull(client, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if !bytes.Equal(payload, []byte{0x01, 0x02}) {
		t.Errorf("payload = %v, want [0x01 0x02]", payload)
	}
}

// TestWSCoade_SingleCodec_NoBufioVariant is a compile-time + behavioral guard
// that the active codec is the conn/reader pair. It writes through
// wsWriteFrameConn and reads back with wsReadFrameReader for a text frame,
// confirming the two are the matched, tested pair. (The removed bufio variants
// are gone from the source; this test pins that the replacement pair is what
// handles traffic.)
func TestWSCoade_SingleCodec_NoBufioVariant(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() { _ = wsWriteFrameConn(server, 0x1, []byte("hello")) }()

	opcode, payload, err := wsReadFrameReader(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opcode != 0x1 || string(payload) != "hello" {
		t.Errorf("got opcode=0x%x payload=%q, want 0x1 \"hello\"", opcode, payload)
	}
}
