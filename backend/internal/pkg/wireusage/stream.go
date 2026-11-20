package wireusage

import (
	"bytes"
	"io"
)

// Inspect only envelope keys. Payload strings, tool arguments, images and
// arbitrary nesting pass through as they arrive, even across short writes.
type scope uint8

const (
	opaque scope = iota
	array
	envelope
	message
	response
)

type jsonStream struct {
	depth            int
	scopes           [4]scope // root array -> response envelope -> message/response -> usage
	quoted, escaped  bool
	key              [24]byte
	keyLen           int
	colon            bool
	captureDepth     int
	capture          []byte
	oversized        bool
	gemini, messages bool
}

// A normal usage object is a few hundred bytes. Bound only an unfinished
// usage object, never the event or response. If an exceptional fragmented
// usage exceeds the limit its object becomes empty, leaving content intact.
const maxFragmentedUsage = 128 << 10

func write(dst io.Writer, b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	n, err := dst.Write(b)
	if err == nil && n != len(b) {
		return n, io.ErrShortWrite
	}
	return n, err
}

func (s *jsonStream) keep(b []byte) {
	if s.oversized {
		return
	}
	if len(b) > maxFragmentedUsage-len(s.capture) {
		s.capture = nil
		s.oversized = true
		return
	}
	s.capture = append(s.capture, b...)
}

func (s *jsonStream) parent() scope {
	if s.depth == 0 || s.depth > len(s.scopes) {
		return opaque
	}
	return s.scopes[s.depth-1]
}

func (p *Projector) WriteJSON(dst io.Writer, b []byte) (int, error) {
	s := &p.stream
	start := 0
	for i := 0; i < len(b); i++ {
		c := b[i]
		if s.quoted {
			if s.escaped {
				s.escaped = false
				continue
			}
			switch c {
			case '\\':
				s.escaped = true
				s.keyLen = -1
			case '"':
				s.quoted = false
			default:
				if s.keyLen >= 0 {
					if s.keyLen == len(s.key) {
						s.keyLen = -1
					} else {
						s.key[s.keyLen] = c
						s.keyLen++
					}
				}
				if s.keyLen < 0 {
					j := bytes.IndexAny(b[i+1:], `"\`)
					if j < 0 {
						i = len(b)
					} else {
						i += j
					}
				}
			}
			continue
		}
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		case '"':
			s.quoted = true
			s.keyLen = -1
			if s.captureDepth == 0 && s.parent() >= envelope {
				s.keyLen = 0
			}
			s.colon = false
		case ':':
			s.colon = true
		case '{', '[':
			parent := s.parent()
			next := opaque
			key := ""
			if s.colon && s.keyLen >= 0 {
				key = string(s.key[:s.keyLen])
			}
			if s.depth == 0 || parent == array {
				if c == '[' && s.depth == 0 {
					next = array
				} else if c == '{' {
					next = envelope
				}
			} else if c == '{' && parent >= envelope {
				switch {
				case (key == "usage" || key == "usageMetadata") && (p.ProjectStreamUsage == nil || p.ProjectStreamUsage()):
					if n, err := write(dst, b[start:i]); err != nil {
						return start + n, err
					}
					start = i
					s.captureDepth = s.depth + 1
					s.messages = p.messages || parent == message
					s.gemini = key == "usageMetadata"
				case key == "message" && parent == envelope:
					next = message
				case key == "response" && parent == envelope:
					next = response
				}
			}
			if s.depth < len(s.scopes) {
				s.scopes[s.depth] = next
			}
			s.depth++
			s.colon = false
			s.keyLen = -1
		case '}', ']':
			if s.captureDepth > 0 && s.depth == s.captureDepth {
				part := b[start : i+1]
				if s.oversized || (len(s.capture) > 0 && len(part) > maxFragmentedUsage-len(s.capture)) {
					part = []byte("{}")
				} else {
					if len(s.capture) > 0 {
						s.keep(part)
						part = s.capture
					}
					part = p.object(part, s.messages, s.gemini)
				}
				if n, err := write(dst, part); err != nil {
					// A rewritten usage object is consumed as one logical unit.
					if n == len(part) {
						return i + 1, err
					}
					return start, err
				}
				s.capture = s.capture[:0]
				s.captureDepth = 0
				s.oversized = false
				start = i + 1
			}
			if s.depth > 0 {
				s.depth--
			}
			s.colon = false
			s.keyLen = -1
		default:
			s.colon = false
			s.keyLen = -1
		}
	}
	if s.captureDepth > 0 {
		s.keep(b[start:])
		return len(b), nil
	}
	n, err := write(dst, b[start:])
	return start + n, err
}

type sseStream struct {
	prefix               int
	data, other, afterCR bool
}

// SSE field names and opaque lines (including heartbeats and [DONE]) are
// forwarded immediately. Only data field values enter the JSON scanner.
func (p *Projector) WriteSSE(dst io.Writer, b []byte) (int, error) {
	size := len(b)
	const prefix = "data:"
	s := &p.sse
	for len(b) > 0 {
		if s.afterCR {
			s.afterCR = false
			if b[0] == '\n' {
				if n, err := write(dst, b[:1]); err != nil {
					return size - len(b) + n, err
				}
				b = b[1:]
				continue
			}
		}
		end := bytes.IndexAny(b, "\r\n")
		if end < 0 {
			end = len(b)
		}
		line := b[:end]
		if !s.other && !s.data && len(line) > 0 {
			n := min(len(line), len(prefix)-s.prefix)
			if string(line[:n]) != prefix[s.prefix:s.prefix+n] {
				s.other = true
			} else {
				if written, err := write(dst, line[:n]); err != nil {
					return size - len(b) + written, err
				}
				line = line[n:]
				s.prefix += n
				s.data = s.prefix == len(prefix)
			}
		}
		var err error
		var n int
		if s.data {
			n, err = p.WriteJSON(dst, line)
		} else {
			n, err = write(dst, line)
		}
		if err != nil {
			return size - len(b) + end - len(line) + n, err
		}
		b = b[end:]
		if len(b) == 0 {
			break
		}
		if !s.data && !s.other && s.prefix == 0 {
			// A blank line dispatches the event. Discard lexical state even for
			// non-JSON events; keep cumulative counters across events.
			if p.stream.captureDepth != 0 && !p.stream.oversized {
				if _, err := write(dst, p.stream.capture); err != nil {
					return size - len(b), err
				}
			}
			p.stream = jsonStream{}
		} else if s.data && p.stream.captureDepth > 0 {
			// SSE joins consecutive data fields with a newline. It is whitespace
			// between JSON tokens. The physical line framing is already forwarded.
			p.stream.keep([]byte(" "))
		}
		s.prefix = 0
		s.data = false
		s.other = false
		s.afterCR = b[0] == '\r'
		if n, err := write(dst, b[:1]); err != nil {
			return size - len(b) + n, err
		}
		b = b[1:]
	}
	return size, nil
}
