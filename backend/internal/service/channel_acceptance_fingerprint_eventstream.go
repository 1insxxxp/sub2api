package service

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
)

// Inspect a bounded prefix of an AWS EventStream already being read by the
// forwarder. CRC checks frame integrity only. Retain labels, never payloads.
const acceptanceFingerprintStreamLimit = 64 << 10

type acceptanceFingerprintStream struct {
	remaining int
	frame     []byte
}

func (s *acceptanceFingerprintStream) observe(data []byte, f *acceptanceFingerprint) {
	data = data[:min(len(data), s.remaining)]
	s.remaining -= len(data)
	for len(data) > 0 {
		if len(s.frame) < 4 {
			n := min(4-len(s.frame), len(data))
			s.frame = append(s.frame, data[:n]...)
			data = data[n:]
			if len(s.frame) < 4 {
				break
			}
		}
		size := binary.BigEndian.Uint32(s.frame)
		if size < 16 || size > acceptanceFingerprintStreamLimit {
			s.frame, s.remaining = nil, 0
			return
		}
		n := min(int(size)-len(s.frame), len(data))
		s.frame = append(s.frame, data[:n]...)
		data = data[n:]
		if len(s.frame) != int(size) {
			break
		}
		f.observeAWSFrame(s.frame)
		s.frame = nil
	}
	if s.remaining == 0 {
		s.frame = nil
	}
}

func (f *acceptanceFingerprint) observeAWSFrame(frame []byte) {
	end := len(frame) - 4
	headerLen := binary.BigEndian.Uint32(frame[4:8])
	if headerLen > uint32(end-12) || crc32.ChecksumIEEE(frame[:8]) != binary.BigEndian.Uint32(frame[8:12]) ||
		crc32.ChecksumIEEE(frame[:end]) != binary.BigEndian.Uint32(frame[end:]) {
		return
	}
	payload := 12 + int(headerLen)
	headers := frame[12:payload]
	if extractEventStreamHeaderValue(headers, ":message-type") != "event" {
		return
	}
	event := extractEventStreamHeaderValue(headers, ":event-type")
	switch event {
	case "assistantResponseEvent", "contextUsageEvent", "meteringEvent", "toolUseEvent":
		var m map[string]any
		d := json.NewDecoder(bytes.NewReader(frame[payload:end]))
		d.UseNumber()
		if d.Decode(&m) == nil && d.Decode(new(any)) == io.EOF {
			f.observeAssistantEvent(event, m)
		}
	}
}
