package server

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"lotteryd/internal/store"
)

const maxPointsAsset = 8 << 20

var assetIDPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validateSVG(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	root := false
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid SVG")
		}
		switch node := token.(type) {
		case xml.Directive:
			return fmt.Errorf("SVG declarations are not allowed")
		case xml.ProcInst:
			if node.Target != "xml" {
				return fmt.Errorf("SVG processing instructions are not allowed")
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return fmt.Errorf("invalid SVG root")
			}
		case xml.StartElement:
			if depth == 0 {
				if root || node.Name.Local != "svg" {
					return fmt.Errorf("file must contain one SVG root")
				}
				root = true
			}
			depth++
			name := strings.ToLower(node.Name.Local)
			if node.Name.Space != "" && node.Name.Space != "http://www.w3.org/2000/svg" {
				return fmt.Errorf("unsupported SVG namespace")
			}
			switch name {
			case "script", "foreignobject", "iframe", "object", "embed", "a", "style":
				return fmt.Errorf("unsupported SVG element: %s", name)
			}
			for _, a := range node.Attr {
				key := strings.ToLower(a.Name.Local)
				value := strings.ToLower(strings.TrimSpace(a.Value))
				compact := strings.Join(strings.Fields(value), "")
				if strings.HasPrefix(key, "on") || strings.Contains(compact, "javascript:") || strings.Contains(compact, "@import") || strings.Contains(compact, "expression(") {
					return fmt.Errorf("unsafe SVG attribute")
				}
				if key == "href" && !strings.HasPrefix(value, "#") {
					return fmt.Errorf("external SVG assets are not supported")
				}
				if key == "attributename" && (strings.Contains(value, "href") || value == "style" || strings.HasPrefix(value, "on")) {
					return fmt.Errorf("unsafe SVG animation")
				}
				if strings.Contains(value, "url(") { // Local filter/gradient references only.
					for _, part := range strings.Split(value, "url(")[1:] {
						ref := strings.TrimLeft(part, " \t\r\n\"'")
						if !strings.HasPrefix(ref, "#") {
							return fmt.Errorf("external SVG references are not supported")
						}
					}
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if !root || depth != 0 {
		return fmt.Errorf("invalid SVG")
	}
	return nil
}

func normalizeLottie(data []byte, extension string) ([]byte, error) {
	if extension == ".lottie" {
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("invalid .lottie archive")
		}
		var manifest struct {
			Animations []struct {
				ID string `json:"id"`
			} `json:"animations"`
		}
		files := map[string]*zip.File{}
		for _, file := range reader.File {
			files[file.Name] = file
		}
		read := func(name string) ([]byte, error) {
			file := files[name]
			if file == nil || file.UncompressedSize64 > maxPointsAsset {
				return nil, fmt.Errorf("missing or oversized Lottie content")
			}
			r, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			b, err := io.ReadAll(io.LimitReader(r, maxPointsAsset+1))
			if len(b) > maxPointsAsset {
				return nil, fmt.Errorf("Lottie is too large")
			}
			return b, err
		}
		manifestData, err := read("manifest.json")
		if err != nil {
			return nil, err
		}
		if json.Unmarshal(manifestData, &manifest) != nil || len(manifest.Animations) != 1 {
			return nil, fmt.Errorf("upload a .lottie file containing one animation")
		}
		id := manifest.Animations[0].ID
		if strings.ContainsAny(id, "/\\") || id == "" {
			return nil, fmt.Errorf("invalid Lottie animation ID")
		}
		data, err = read("animations/" + id + ".json")
		if err != nil {
			return nil, err
		}
	}
	var animation map[string]any
	if json.Unmarshal(data, &animation) != nil {
		return nil, fmt.Errorf("invalid Lottie JSON")
	}
	numeric := func(key string) float64 { v, _ := animation[key].(float64); return v }
	_, hasLayers := animation["layers"].([]any)
	if !hasLayers || numeric("fr") <= 0 || numeric("fr") > 240 || numeric("w") <= 0 || numeric("h") <= 0 || numeric("w") > 8192 || numeric("h") > 8192 || numeric("op") <= numeric("ip") {
		return nil, fmt.Errorf("file is not a supported Lottie animation")
	}
	// This player uses vector-only data, without expressions, remote images or fonts.
	var check func(any) error
	check = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if key == "x" {
					if _, ok := child.(string); ok {
						return fmt.Errorf("Lottie expressions are not supported")
					}
				}
				if key == "p" || key == "u" {
					if text, ok := child.(string); ok && text != "" {
						return fmt.Errorf("Lottie image assets are not supported")
					}
				}
				if key == "fPath" {
					if text, ok := child.(string); ok && text != "" {
						return fmt.Errorf("external fonts are not supported")
					}
				}
				if err := check(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := check(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := check(animation); err != nil {
		return nil, err
	}
	return json.Marshal(animation)
}

func parsePointsAsset(name, slot string, data []byte) (store.PointsAsset, error) {
	asset := store.PointsAsset{}
	extension := strings.ToLower(filepath.Ext(name))
	if len(data) == 0 || len(data) > maxPointsAsset {
		return asset, fmt.Errorf("file must be between 1 byte and 8 MB")
	}
	switch extension {
	case ".svg":
		if len(data) > 2<<20 {
			return asset, fmt.Errorf("SVG must not exceed 2 MB")
		}
		if err := validateSVG(data); err != nil {
			return asset, err
		}
		asset.Kind = "svg"
		asset.MIME = "image/svg+xml"
	case ".json", ".lottie":
		if slot != "animation" {
			return asset, fmt.Errorf("only the animation file accepts Lottie")
		}
		var err error
		data, err = normalizeLottie(data, extension)
		if err != nil {
			return asset, err
		}
		asset.Kind = "lottie"
		asset.MIME = "application/json"
	case ".png", ".jpg", ".jpeg":
		if slot != "icon" {
			return asset, fmt.Errorf("item artwork must use SVG")
		}
		info, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || info.Width < 1 || info.Height < 1 || info.Width > 4096 || info.Height > 4096 {
			return asset, fmt.Errorf("invalid icon image or dimensions")
		}
		asset.Kind = "image"
		asset.MIME = "image/" + format
	default:
		return asset, fmt.Errorf("unsupported file format")
	}
	asset.Data = data
	hash := sha256.Sum256(append([]byte(asset.Kind+":"), data...))
	asset.ID = hex.EncodeToString(hash[:])
	return asset, nil
}

func (s *Server) handleUploadPointsAsset(w http.ResponseWriter, r *http.Request, claims *Claims) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPointsAsset+(1<<20))
	if err := r.ParseMultipartForm(maxPointsAsset); err != nil {
		fail(w, 400, 400, "invalid upload or file exceeds 8 MB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	slot := r.FormValue("slot")
	if slot != "cover" && slot != "active" && slot != "expired" && slot != "animation" && slot != "icon" {
		fail(w, 400, 400, "invalid asset slot")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, 400, "file is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPointsAsset+1))
	if err != nil {
		fail(w, 400, 400, "unable to read file")
		return
	}
	asset, err := parsePointsAsset(header.Filename, slot, data)
	if err != nil {
		fail(w, 400, 400, err.Error())
		return
	}
	if err = s.App.Store.SavePointsAsset(asset, claims.UserID); err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]string{"id": asset.ID, "kind": asset.Kind})
}

func (s *Server) handlePointsAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !assetIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	asset, err := s.App.Store.GetPointsAsset(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", asset.MIME)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+id+`"`)
	if r.Header.Get("If-None-Match") == `"`+id+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(asset.Data)
}
