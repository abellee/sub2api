package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	xdraw "golang.org/x/image/draw"
)

const (
	wechatGroupQRMaxUploadBytes = 12 << 20
	wechatGroupQRMaxPixels      = 12_000_000
	wechatGroupQRMaxOutputSide  = 1024
	wechatGroupQRFallbackURL    = "/llmfree/wechat-group-qr.png"
)

type WechatGroupQRHandler struct {
	mu       sync.RWMutex
	filePath string
	metaPath string
}

type wechatGroupQRInfo struct {
	ImageURL  string     `json:"image_url"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Custom    bool       `json:"custom"`
	Expired   bool       `json:"expired"`
}

type wechatGroupQRMetadata struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func NewWechatGroupQRHandler(cfg *config.Config) *WechatGroupQRHandler {
	dataDir := "."
	if cfg != nil && cfg.Pricing.DataDir != "" {
		dataDir = cfg.Pricing.DataDir
	}
	dir := filepath.Join(dataDir, "community")
	_ = os.MkdirAll(dir, 0o755)
	filePath := filepath.Join(dir, "wechat-group-qr.png")
	metaPath := filepath.Join(dir, "wechat-group-qr.json")
	_ = os.Remove(filePath + ".old")
	_ = os.Remove(metaPath + ".old")
	return &WechatGroupQRHandler{filePath: filePath, metaPath: metaPath}
}

func (h *WechatGroupQRHandler) Get(c *gin.Context) {
	response.Success(c, h.currentInfo())
}

func (h *WechatGroupQRHandler) ServeImage(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	info, err := os.Stat(h.filePath)
	if err != nil || info.IsDir() {
		c.Redirect(http.StatusTemporaryRedirect, wechatGroupQRFallbackURL)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(h.filePath)
}

func (h *WechatGroupQRHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wechatGroupQRMaxUploadBytes+(1<<20))
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		response.BadRequest(c, "请选择 PNG 或 JPEG 图片")
		return
	}
	defer func() { _ = file.Close() }()

	raw, err := io.ReadAll(io.LimitReader(file, wechatGroupQRMaxUploadBytes+1))
	if err != nil {
		response.BadRequest(c, "读取图片失败")
		return
	}
	if len(raw) == 0 || len(raw) > wechatGroupQRMaxUploadBytes {
		response.BadRequest(c, "图片大小不能超过 12 MB")
		return
	}
	contentType := http.DetectContentType(raw)
	if contentType != "image/png" && contentType != "image/jpeg" {
		response.BadRequest(c, "仅支持 PNG 或 JPEG 图片")
		return
	}

	source, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		response.BadRequest(c, "无法解析图片")
		return
	}
	bounds := source.Bounds()
	if bounds.Dx() < 120 || bounds.Dy() < 120 || int64(bounds.Dx())*int64(bounds.Dy()) > wechatGroupQRMaxPixels {
		response.BadRequest(c, "图片尺寸无效或过大")
		return
	}

	cropBounds, err := findWechatQRCodeBounds(source)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	processed := cropWechatQRCode(source, cropBounds)
	expiresAt, err := parseWechatQRExpiry(c.PostForm("expires_at"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.save(processed, expiresAt); err != nil {
		response.Error(c, http.StatusInternalServerError, "保存微信群二维码失败")
		return
	}
	response.Success(c, h.currentInfoUnlocked())
}

func (h *WechatGroupQRHandler) currentInfo() wechatGroupQRInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentInfoUnlocked()
}

func (h *WechatGroupQRHandler) currentInfoUnlocked() wechatGroupQRInfo {
	info, err := os.Stat(h.filePath)
	if err != nil || info.IsDir() {
		return wechatGroupQRInfo{ImageURL: wechatGroupQRFallbackURL}
	}
	updatedAt := info.ModTime().UTC()
	metadata := h.readMetadataUnlocked()
	expired := metadata.ExpiresAt != nil && !metadata.ExpiresAt.After(time.Now())
	return wechatGroupQRInfo{
		ImageURL:  fmt.Sprintf("/api/v1/community/wechat-group-qr/image?v=%d", updatedAt.UnixNano()),
		UpdatedAt: &updatedAt,
		ExpiresAt: metadata.ExpiresAt,
		Custom:    true,
		Expired:   expired,
	}
}

func (h *WechatGroupQRHandler) save(img image.Image, expiresAt *time.Time) error {
	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".wechat-group-qr-*.png")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()

	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if err := png.Encode(temp, img); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := replaceWechatQRFile(tempPath, h.filePath); err != nil {
		return err
	}
	return h.saveMetadata(expiresAt)
}

func (h *WechatGroupQRHandler) saveMetadata(expiresAt *time.Time) error {
	if expiresAt == nil {
		if err := os.Remove(h.metaPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(wechatGroupQRMetadata{ExpiresAt: expiresAt})
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(h.metaPath), ".wechat-group-qr-meta-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return replaceWechatQRFile(tempPath, h.metaPath)
}

func (h *WechatGroupQRHandler) readMetadataUnlocked() wechatGroupQRMetadata {
	data, err := os.ReadFile(h.metaPath)
	if err != nil {
		return wechatGroupQRMetadata{}
	}
	var metadata wechatGroupQRMetadata
	if json.Unmarshal(data, &metadata) != nil {
		return wechatGroupQRMetadata{}
	}
	return metadata
}

func parseWechatQRExpiry(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New("二维码失效时间格式无效")
	}
	if !expiresAt.After(time.Now()) {
		return nil, errors.New("二维码失效时间必须晚于当前时间")
	}
	return &expiresAt, nil
}

func replaceWechatQRFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}

	backup := target + ".old"
	_ = os.Remove(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(source, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func findWechatQRCodeBounds(img image.Image) (image.Rectangle, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	pixels := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if isQRBackgroundPixel(img.At(bounds.Min.X+x, bounds.Min.Y+y)) {
				pixels[y*width+x] = 1
			}
		}
	}

	var best image.Rectangle
	bestArea := 0
	queue := make([]int32, 0, width*height/8)
	for start := 0; start < len(pixels); start++ {
		if pixels[start] != 1 {
			continue
		}
		queue = queue[:0]
		queue = append(queue, int32(start))
		pixels[start] = 2
		minX, maxX := start%width, start%width
		minY, maxY := start/width, start/width
		count := 0

		for head := 0; head < len(queue); head++ {
			index := int(queue[head])
			x, y := index%width, index/width
			count++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
			if x > 0 {
				appendQRPixel(&queue, pixels, index-1)
			}
			if x+1 < width {
				appendQRPixel(&queue, pixels, index+1)
			}
			if y > 0 {
				appendQRPixel(&queue, pixels, index-width)
			}
			if y+1 < height {
				appendQRPixel(&queue, pixels, index+width)
			}
		}

		candidate := image.Rect(minX, minY, maxX+1, maxY+1)
		candidateWidth, candidateHeight := candidate.Dx(), candidate.Dy()
		area := candidateWidth * candidateHeight
		minSide := min(candidateWidth, candidateHeight)
		if minSide < 100 || minSide*5 < min(width, height) ||
			candidateWidth*100 < candidateHeight*85 || candidateHeight*100 < candidateWidth*85 ||
			count*4 < area || area <= bestArea {
			continue
		}
		best = candidate
		bestArea = area
	}

	if bestArea == 0 {
		return image.Rectangle{}, errors.New("未识别到完整二维码，请上传包含二维码的原始截图")
	}
	best = squareWechatQRBounds(best, image.Rect(0, 0, width, height))
	absolute := best.Add(bounds.Min)
	if !hasQRCodeCornerPattern(img, absolute) {
		return image.Rectangle{}, errors.New("未识别到二维码定位区域，请确认截图清晰且包含完整二维码")
	}
	return absolute, nil
}

func appendQRPixel(queue *[]int32, pixels []uint8, index int) {
	if pixels[index] != 1 {
		return
	}
	pixels[index] = 2
	*queue = append(*queue, int32(index))
}

func isQRBackgroundPixel(value color.Color) bool {
	r, g, b, _ := value.RGBA()
	r8, g8, b8 := int(r>>8), int(g>>8), int(b>>8)
	return r8 >= 190 && g8 >= 190 && b8 >= 190 && (299*r8+587*g8+114*b8)/1000 >= 220
}

func squareWechatQRBounds(candidate, imageBounds image.Rectangle) image.Rectangle {
	side := max(candidate.Dx(), candidate.Dy())
	centerX := (candidate.Min.X + candidate.Max.X) / 2
	centerY := (candidate.Min.Y + candidate.Max.Y) / 2
	result := image.Rect(centerX-side/2, centerY-side/2, centerX-side/2+side, centerY-side/2+side)
	if result.Min.X < imageBounds.Min.X {
		result = result.Add(image.Pt(imageBounds.Min.X-result.Min.X, 0))
	}
	if result.Max.X > imageBounds.Max.X {
		result = result.Add(image.Pt(imageBounds.Max.X-result.Max.X, 0))
	}
	if result.Min.Y < imageBounds.Min.Y {
		result = result.Add(image.Pt(0, imageBounds.Min.Y-result.Min.Y))
	}
	if result.Max.Y > imageBounds.Max.Y {
		result = result.Add(image.Pt(0, imageBounds.Max.Y-result.Max.Y))
	}
	return result.Intersect(imageBounds)
}

func hasQRCodeCornerPattern(img image.Image, bounds image.Rectangle) bool {
	side := min(bounds.Dx(), bounds.Dy())
	zone := max(24, side/3)
	corners := []image.Rectangle{
		image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+zone, bounds.Min.Y+zone),
		image.Rect(bounds.Max.X-zone, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+zone),
		image.Rect(bounds.Min.X, bounds.Max.Y-zone, bounds.Min.X+zone, bounds.Max.Y),
	}
	for _, corner := range corners {
		dark := 0
		total := corner.Dx() * corner.Dy()
		for y := corner.Min.Y; y < corner.Max.Y; y++ {
			for x := corner.Min.X; x < corner.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if (299*int(r>>8)+587*int(g>>8)+114*int(b>>8))/1000 < 100 {
					dark++
				}
			}
		}
		if dark*100 < total*12 {
			return false
		}
	}
	return true
}

func cropWechatQRCode(source image.Image, bounds image.Rectangle) image.Image {
	side := min(bounds.Dx(), bounds.Dy())
	cropped := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(cropped, cropped.Bounds(), source, bounds.Min, draw.Src)
	if side <= wechatGroupQRMaxOutputSide {
		return cropped
	}
	resized := image.NewRGBA(image.Rect(0, 0, wechatGroupQRMaxOutputSide, wechatGroupQRMaxOutputSide))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)
	return resized
}
