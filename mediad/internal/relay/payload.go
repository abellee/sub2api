package relay

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Payloads are immutable. Metadata updates never rewrite or clone image bytes.
type payloadRef struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func (s *MemoryStore) payloadDir(id string) string { return filepath.Join(s.root, "payloads", id) }
func safeID(id string) bool {
	return id != "" && id != "." && id != ".." && filepath.Base(id) == id && !strings.ContainsAny(id, "/\\")
}
func (s *MemoryStore) savePayload(id, kind string, write func(io.Writer) error) (*payloadRef, error) {
	if !safeID(id) {
		return nil, fmt.Errorf("invalid task id")
	}
	dir := s.payloadDir(id)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	f, e := os.CreateTemp(dir, kind+"-*.json")
	if e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(f.Name())
		}
	}()
	w := bufio.NewWriterSize(f, 64<<10)
	if e = write(w); e == nil {
		e = w.Flush()
	}
	if e == nil {
		e = f.Sync()
	}
	if e != nil {
		return nil, e
	}
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	if e = SyncDirectory(dir); e != nil {
		return nil, e
	}
	ok = true
	return &payloadRef{Name: filepath.Base(f.Name()), Size: st.Size()}, nil
}
func writeBytes(w io.Writer, b []byte) error {
	if _, e := io.WriteString(w, "\""); e != nil {
		return e
	}
	enc := base64.NewEncoder(base64.StdEncoding, w)
	if _, e := enc.Write(b); e != nil {
		return e
	}
	if e := enc.Close(); e != nil {
		return e
	}
	_, e := io.WriteString(w, "\"")
	return e
}
func (s *MemoryStore) offload(task *Task) error {
	if s.root == "" {
		return nil
	}
	var e error
	if task.Request != nil && task.Request.Body != nil {
		task.RequestData, e = s.savePayload(task.ID, "request", func(w io.Writer) error { return writeBytes(w, task.Request.Body) })
		if e != nil {
			return e
		}
		task.Request.Body = nil
	}
	if len(task.Response) > 0 {
		task.ResponseData, e = s.savePayload(task.ID, "response", func(w io.Writer) error { return writeBytes(w, task.Response) })
		if e != nil {
			return e
		}
		task.ResponseSummary = compactResponse(task.Response)
		task.Response = nil
	}
	if task.Result != nil {
		task.ResultData, e = s.savePayload(task.ID, "result", func(w io.Writer) error { return json.NewEncoder(w).Encode(task.Result) })
		if e != nil {
			return e
		}
		task.Result = nil
	}
	return nil
}
func (s *MemoryStore) openPayload(id string, ref *payloadRef) (*os.File, error) {
	if !safeID(id) || ref == nil || !safeID(ref.Name) {
		return nil, fmt.Errorf("invalid payload reference")
	}
	f, e := os.Open(filepath.Join(s.payloadDir(id), ref.Name))
	if e != nil {
		return nil, e
	}
	st, e := f.Stat()
	if e != nil || st.Size() != ref.Size {
		f.Close()
		return nil, fmt.Errorf("payload size mismatch")
	}
	return f, nil
}

// Request bodies are decoded directly from disk into the HTTP connection.
func (s *MemoryStore) requestReader(task *Task) (io.ReadCloser, int64, error) {
	if task.RequestData == nil {
		return io.NopCloser(bytes.NewReader(task.Request.Body)), int64(len(task.Request.Body)), nil
	}
	f, e := s.openPayload(task.ID, task.RequestData)
	if e != nil {
		return nil, 0, e
	}
	size := task.RequestData.Size
	if size < 2 {
		f.Close()
		return nil, 0, fmt.Errorf("invalid body payload")
	}
	var tail [3]byte
	_, e = f.ReadAt(tail[:], max(0, size-3))
	if e != nil && e != io.EOF {
		f.Close()
		return nil, 0, e
	}
	padding := int64(0)
	if tail[1] == '=' {
		padding++
	}
	if tail[0] == '=' {
		padding++
	}
	if _, e = f.Seek(1, io.SeekStart); e != nil {
		f.Close()
		return nil, 0, e
	}
	return &payloadReader{Reader: base64.NewDecoder(base64.StdEncoding, io.LimitReader(f, size-2)), Closer: f}, (size-2)/4*3 - padding, nil
}

type payloadReader struct {
	io.Reader
	io.Closer
}

// Only workers with a memory reservation may decode result payloads.
func (s *MemoryStore) loadResults(task *Task) error { return s.loadPayloadFields(task, true, true) }
func (s *MemoryStore) loadPayloadFields(task *Task, response, result bool) error {
	for _, v := range []struct {
		ref    *payloadRef
		dst    any
		needed bool
	}{{task.ResponseData, &task.Response, response && len(task.Response) == 0}, {task.ResultData, &task.Result, result && task.Result == nil}} {
		if v.ref == nil || !v.needed {
			continue
		}
		if v.ref.Size > 128<<20 {
			return fmt.Errorf("saved response exceeds memory-safe decode limit")
		}
		f, e := s.openPayload(task.ID, v.ref)
		if e != nil {
			return e
		}
		e = json.NewDecoder(io.LimitReader(f, v.ref.Size)).Decode(v.dst)
		f.Close()
		if e != nil {
			return e
		}
	}
	return nil
}
func compactResponse(raw []byte) []byte { return compactResponseReader(bytes.NewReader(raw)) }
func compactResponseReader(reader io.Reader) []byte {
	return compactResponseReaderLimit(reader, 64<<10)
}
func compactResponseReaderLimit(reader io.Reader, limit int) []byte {
	out := &limitedJSON{limit: limit}
	parser := jsonStream{r: bufio.NewReaderSize(reader, 64<<10)}
	parser.visit = func(path []string, p *jsonStream, w io.Writer) (bool, error) {
		if len(path) == 0 {
			return false, nil
		}
		key := path[len(path)-1]
		skip := key == "b64_json" || (key == "data" && len(path) > 1 && (path[len(path)-2] == "inlineData" || path[len(path)-2] == "inline_data"))
		if !skip {
			return false, nil
		}
		if e := p.raw(path, io.Discard, len(path)); e != nil {
			return true, e
		}
		_, e := io.WriteString(w, "\"\"")
		return true, e
	}
	if parser.value(nil, out, 0) != nil || !json.Valid(out.Bytes()) {
		return nil
	}
	return out.Bytes()
}

// Legacy task snapshots are split by streaming, including multi-megabyte JSON strings.
func (s *MemoryStore) readTask(path, id string) (*Task, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	out := &limitedJSON{limit: 4 << 20}
	refs := map[string]*payloadRef{}
	parser := jsonStream{r: bufio.NewReaderSize(f, 64<<10)}
	parser.visit = func(parts []string, p *jsonStream, w io.Writer) (bool, error) {
		key := strings.Join(parts, ".")
		kind := ""
		switch key {
		case "request.Body":
			kind = "request"
		case "response":
			kind = "response"
		case "result":
			kind = "result"
		}
		if kind == "" {
			return false, nil
		}
		peek, e := p.r.Peek(1)
		if e != nil {
			return true, e
		}
		if peek[0] == 'n' {
			return false, nil
		}
		ref, e := s.savePayload(id, kind, func(dst io.Writer) error { return p.raw(parts, dst, len(parts)) })
		if e != nil {
			return true, e
		}
		refs[kind] = ref
		_, e = io.WriteString(w, "null")
		return true, e
	}
	if e = parser.value(nil, out, 0); e != nil {
		return nil, e
	}
	if e = parser.space(); e != io.EOF {
		return nil, fmt.Errorf("trailing task data")
	}
	var task Task
	if e = json.Unmarshal(out.Bytes(), &task); e != nil {
		return nil, e
	}
	if task.ID != id {
		return nil, fmt.Errorf("invalid task filename")
	}
	// A failed migration is retried from the original snapshot; metadata is replaced only last.
	if v := refs["request"]; v != nil {
		task.RequestData = v
	}
	if v := refs["response"]; v != nil {
		task.ResponseData = v
	}
	if v := refs["result"]; v != nil {
		task.ResultData = v
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	released := task.releaseCompletedRequest()
	if len(refs) > 0 || released {
		if e = s.persist(&task); e != nil {
			return nil, e
		}
	}
	return &task, nil
}

func (s *MemoryStore) cleanPayloads(task *Task) error {
	if s.root == "" {
		return nil
	}
	keep := map[string]bool{}
	for _, ref := range []*payloadRef{task.RequestData, task.ResponseData, task.ResultData} {
		if ref != nil {
			keep[ref.Name] = true
		}
	}
	dir := s.payloadDir(task.ID)
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if !entry.IsDir() && !keep[entry.Name()] {
			if e = os.Remove(filepath.Join(dir, entry.Name())); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	return nil
}

func (s *MemoryStore) responseMetadata(task *Task) []byte {
	return s.responseMetadataLimit(task, 64<<10)
}
func (s *MemoryStore) responseMetadataLimit(task *Task, limit int) []byte {
	if len(task.ResponseSummary) > 0 {
		return task.ResponseSummary
	}
	if len(task.Response) > 0 {
		return compactResponseReaderLimit(bytes.NewReader(task.Response), limit)
	}
	if task.ResponseData == nil {
		return nil
	}
	f, e := s.openPayload(task.ID, task.ResponseData)
	if e != nil {
		return nil
	}
	defer f.Close()
	if _, e = f.Seek(1, io.SeekStart); e != nil {
		return nil
	}
	return compactResponseReaderLimit(base64.NewDecoder(base64.StdEncoding, io.LimitReader(f, task.ResponseData.Size-2)), limit)
}
func (s *MemoryStore) errorResponse(task *Task) (func(), error) {
	if len(task.Response) > 0 || task.ResponseData == nil {
		return func() {}, nil
	}
	ref := task.ResponseData
	if ref.Size > 128<<20 {
		return nil, fmt.Errorf("saved response exceeds limit")
	}
	release, ok := mediaMemory.try(ref.Size*8 + (16 << 20))
	if !ok {
		return nil, fmt.Errorf("not enough available memory; retry later")
	}
	f, err := s.openPayload(task.ID, ref)
	if err == nil {
		err = json.NewDecoder(f).Decode(&task.Response)
		f.Close()
	}
	if err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func compactRequestParameters(raw []byte) []byte {
	return compactRequestParametersReader(bytes.NewReader(raw))
}
func compactRequestParametersReader(reader io.Reader) []byte {
	out := &limitedJSON{limit: 64 << 10}
	parser := jsonStream{r: bufio.NewReaderSize(reader, 64<<10)}
	parser.visit = func(path []string, p *jsonStream, w io.Writer) (bool, error) {
		if len(path) != 1 {
			return false, nil
		}
		switch path[0] {
		case "duration", "resolution", "size", "n", "generationConfig":
			return false, nil
		}
		if e := p.raw(path, io.Discard, len(path)); e != nil {
			return true, e
		}
		_, e := io.WriteString(w, "null")
		return true, e
	}
	if parser.value(nil, out, 0) != nil || !json.Valid(out.Bytes()) {
		return nil
	}
	return out.Bytes()
}
