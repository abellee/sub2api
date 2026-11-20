package relay

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"mediad/internal/signature"
)

func canStreamRequest(s *Schema, r *http.Request, store *MemoryStore) bool {
	return store.root != "" && s.Request.BodyMode == "passthrough" && !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/")
}
func (h *RelayHandler) streamRequest(w http.ResponseWriter, r *http.Request, in AssembleInput, id string) (*UpstreamRequest, *payloadRef, map[string]any, string, error) {
	if err := os.MkdirAll(h.store.root, 0700); err != nil {
		return nil, nil, nil, "", err
	}
	f, err := os.CreateTemp(h.store.root, ".incoming-*")
	if err != nil {
		return nil, nil, nil, "", err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if _, err = io.Copy(f, http.MaxBytesReader(w, r.Body, MaxRelayBody)); err != nil {
		return nil, nil, nil, "", err
	}
	f.Seek(0, io.SeekStart)
	if err = signature.VerifyReader(h.secret, r.Header.Get(HeaderTimestamp), r.Method, r.URL.Path, f, r.Header.Get(HeaderSignature), h.now()); err != nil {
		return nil, nil, nil, "", err
	}
	f.Seek(0, io.SeekStart)
	fingerprint := sha256.New()
	io.WriteString(fingerprint, in.Schema.ID+"\x00"+in.Model+"\x00"+in.UpstreamURL+"\x00")
	if _, err = io.Copy(fingerprint, f); err != nil {
		return nil, nil, nil, "", err
	}
	f.Seek(0, io.SeekStart)
	var params map[string]any
	json.Unmarshal(compactRequestParametersReader(f), &params)
	in.Body = []byte("{}")
	assembled, err := Assemble(in)
	if err != nil {
		return nil, nil, nil, "", err
	}
	assembled.Body = nil
	replace := map[string]any{}
	for k, v := range in.Schema.Request.StaticFields {
		replace[k] = v
	}
	if protocol := r.Header.Get(HeaderProtocol); protocol != "" && protocol != "gemini" {
		if _, static := replace["model"]; !static {
			replace["model"] = in.Model
		}
	}
	f.Seek(0, io.SeekStart)
	ref, err := h.store.savePayload(id, "request", func(w io.Writer) error {
		if _, e := io.WriteString(w, "\""); e != nil {
			return e
		}
		enc := base64.NewEncoder(base64.StdEncoding, w)
		if e := rewriteRequestObject(f, enc, replace); e != nil {
			return e
		}
		if e := enc.Close(); e != nil {
			return e
		}
		_, e := io.WriteString(w, "\"")
		return e
	})
	return assembled, ref, params, hex.EncodeToString(fingerprint.Sum(nil)), err
}

// Validate and rewrite only top-level fields; image strings go directly to disk.
func rewriteRequestObject(reader io.Reader, w io.Writer, replace map[string]any) error {
	s := jsonStream{r: bufio.NewReaderSize(reader, 64<<10)}
	if e := s.space(); e != nil {
		return e
	}
	b, e := s.r.ReadByte()
	if e != nil || b != '{' {
		return fmt.Errorf("image request must be a JSON object")
	}
	if e = put(w, '{'); e != nil {
		return e
	}
	first := true
	seen := map[string]bool{}
	for {
		if e = s.space(); e != nil {
			return e
		}
		peek, e := s.r.Peek(1)
		if e != nil {
			return e
		}
		if peek[0] == '}' {
			s.r.ReadByte()
			break
		}
		key := &limitedJSON{limit: 4096}
		if e = s.str(key); e != nil {
			return e
		}
		var name string
		if e = json.Unmarshal(key.Bytes(), &name); e != nil {
			return e
		}
		if !first {
			if e = put(w, ','); e != nil {
				return e
			}
		}
		first = false
		if _, e = w.Write(key.Bytes()); e != nil {
			return e
		}
		if e = s.space(); e != nil {
			return e
		}
		colon, e := s.r.ReadByte()
		if e != nil || colon != ':' {
			return fmt.Errorf("expected colon")
		}
		if e = put(w, ':'); e != nil {
			return e
		}
		if value, ok := replace[name]; ok {
			if e = s.value(nil, io.Discard, 0); e != nil {
				return e
			}
			raw, e := json.Marshal(value)
			if e != nil {
				return e
			}
			if _, e = w.Write(raw); e != nil {
				return e
			}
		} else {
			if e = s.value(nil, w, 0); e != nil {
				return e
			}
		}
		seen[name] = true
		if e = s.space(); e != nil {
			return e
		}
		sep, e := s.r.ReadByte()
		if e != nil {
			return e
		}
		if sep == '}' {
			break
		}
		if sep != ',' {
			return fmt.Errorf("expected comma")
		}
		if e = s.space(); e != nil {
			return e
		}
		peek, e = s.r.Peek(1)
		if e != nil || peek[0] == '}' {
			return fmt.Errorf("trailing comma")
		}
	}
	keys := []string{}
	for k := range replace {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !first {
			if e = put(w, ','); e != nil {
				return e
			}
		}
		first = false
		key, _ := json.Marshal(k)
		value, e := json.Marshal(replace[k])
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(w, "%s:%s", key, value); e != nil {
			return e
		}
	}
	if e = put(w, '}'); e != nil {
		return e
	}
	if e = s.space(); e != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}
