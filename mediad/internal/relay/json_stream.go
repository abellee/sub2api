package relay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type jsonStream struct {
	r     *bufio.Reader
	visit func([]string, *jsonStream, io.Writer) (bool, error)
}

func (s *jsonStream) space() error {
	for {
		b, e := s.r.ReadByte()
		if e != nil {
			return e
		}
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			return s.r.UnreadByte()
		}
	}
}
func put(w io.Writer, b byte) error {
	if bw, ok := w.(interface{ WriteByte(byte) error }); ok {
		return bw.WriteByte(b)
	}
	_, e := w.Write([]byte{b})
	return e
}
func (s *jsonStream) str(w io.Writer) error {
	b, e := s.r.ReadByte()
	if e != nil {
		return e
	}
	if b != '"' {
		return fmt.Errorf("expected JSON string")
	}
	if e = put(w, b); e != nil {
		return e
	}
	slashes := 0
	escaped := false
	unicodeDigits := 0
	for {
		chunk, err := s.r.ReadSlice('"')
		end := len(chunk)
		if err == nil {
			end--
		}
		for i, v := range chunk {
			if err == nil && i == len(chunk)-1 && !escaped && unicodeDigits == 0 {
				continue
			}
			if v < 32 {
				return fmt.Errorf("invalid JSON string")
			}
			if unicodeDigits > 0 {
				if !(v >= '0' && v <= '9' || v >= 'a' && v <= 'f' || v >= 'A' && v <= 'F') {
					return fmt.Errorf("invalid unicode escape")
				}
				unicodeDigits--
				continue
			}
			if escaped {
				escaped = false
				switch v {
				case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				case 'u':
					unicodeDigits = 4
				default:
					return fmt.Errorf("invalid JSON escape")
				}
				continue
			}
			if v == '\\' {
				escaped = true
			}
		}
		n := 0
		for i := end - 1; i >= 0 && chunk[i] == '\\'; i-- {
			n++
		}
		if n == end {
			n += slashes
		}
		if _, e = w.Write(chunk); e != nil {
			return e
		}
		if err == nil {
			if n%2 == 0 {
				return nil
			}
			slashes = 0
			continue
		}
		if err != bufio.ErrBufferFull {
			return err
		}
		slashes = n
	}
}
func (s *jsonStream) value(path []string, w io.Writer, depth int) error {
	if depth > 100 {
		return fmt.Errorf("JSON nesting too deep")
	}
	if e := s.space(); e != nil {
		return e
	}
	if s.visit != nil {
		handled, e := s.visit(path, s, w)
		if handled || e != nil {
			return e
		}
	}
	return s.raw(path, w, depth)
}
func (s *jsonStream) raw(path []string, w io.Writer, depth int) error {
	b, e := s.r.Peek(1)
	if e != nil {
		return e
	}
	switch b[0] {
	case '"':
		return s.str(w)
	case '{', '[':
		open, _ := s.r.ReadByte()
		close := byte('}')
		if open == '[' {
			close = ']'
		}
		if e = put(w, open); e != nil {
			return e
		}
		if e = s.space(); e != nil {
			return e
		}
		next, _ := s.r.Peek(1)
		if len(next) > 0 && next[0] == close {
			s.r.ReadByte()
			return put(w, close)
		}
		for {
			child := append(append([]string(nil), path...), "[]")
			if open == '{' {
				key := limitedJSON{limit: 4096}
				if e = s.str(&key); e != nil {
					return e
				}
				if key.Len() > 4096 {
					return fmt.Errorf("JSON key too long")
				}
				var name string
				if e = json.Unmarshal(key.Bytes(), &name); e != nil {
					return e
				}
				child[len(child)-1] = name
				if _, e = w.Write(key.Bytes()); e != nil {
					return e
				}
				if e = s.space(); e != nil {
					return e
				}
				colon, e := s.r.ReadByte()
				if e != nil || colon != ':' {
					return fmt.Errorf("expected JSON colon")
				}
				if e = put(w, ':'); e != nil {
					return e
				}
			}
			if e = s.value(child, w, depth+1); e != nil {
				return e
			}
			if e = s.space(); e != nil {
				return e
			}
			sep, e := s.r.ReadByte()
			if e != nil {
				return e
			}
			if e = put(w, sep); e != nil {
				return e
			}
			if sep == close {
				return nil
			}
			if sep != ',' {
				return fmt.Errorf("invalid JSON delimiter")
			}
			if e = s.space(); e != nil {
				return e
			}
		}
	default:
		token := limitedJSON{limit: 128}
		for {
			b, e := s.r.ReadByte()
			if e == io.EOF {
				break
			}
			if e != nil {
				return e
			}
			if b == ',' || b == ']' || b == '}' || b == ' ' || b == '\n' || b == '\r' || b == '\t' {
				s.r.UnreadByte()
				break
			}
			if e = token.WriteByte(b); e != nil {
				return e
			}
		}
		if !json.Valid(token.Bytes()) {
			return fmt.Errorf("invalid JSON value")
		}
		_, e = w.Write(token.Bytes())
		return e
	}
}

type limitedJSON struct {
	bytes.Buffer
	limit int
}

func (b *limitedJSON) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("task metadata exceeds limit")
	}
	return b.Buffer.Write(p)
}

func (b *limitedJSON) WriteByte(v byte) error {
	if b.Len() >= b.limit {
		return fmt.Errorf("task metadata exceeds limit")
	}
	return b.Buffer.WriteByte(v)
}
