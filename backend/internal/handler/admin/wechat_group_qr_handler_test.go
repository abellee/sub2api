package admin

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFindWechatQRCodeBoundsCropsScreenshotPanel(t *testing.T) {
	img := makeWechatQRScreenshot(360, 544, image.Rect(48, 184, 312, 448), color.Black)
	bounds, err := findWechatQRCodeBounds(img)
	require.NoError(t, err)
	require.Equal(t, image.Rect(48, 184, 312, 448), bounds)

	cropped := cropWechatQRCode(img, bounds)
	require.Equal(t, 264, cropped.Bounds().Dx())
	require.Equal(t, 264, cropped.Bounds().Dy())
}

func TestWechatGroupQRUploadReplacesOldImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	handler := &WechatGroupQRHandler{
		filePath: filepath.Join(dir, "wechat-group-qr.png"),
		metaPath: filepath.Join(dir, "wechat-group-qr.json"),
	}

	first := makeWechatQRScreenshot(360, 544, image.Rect(48, 184, 312, 448), color.Black)
	firstResponse := uploadWechatQRForTest(t, handler, first)
	require.Equal(t, http.StatusOK, firstResponse.Code)
	storedFirst, err := os.ReadFile(handler.filePath)
	require.NoError(t, err)

	second := makeWechatQRScreenshot(420, 640, image.Rect(90, 220, 330, 460), color.RGBA{R: 30, G: 30, B: 30, A: 255})
	secondResponse := uploadWechatQRForTest(t, handler, second)
	require.Equal(t, http.StatusOK, secondResponse.Code)
	storedSecond, err := os.ReadFile(handler.filePath)
	require.NoError(t, err)
	require.NotEqual(t, storedFirst, storedSecond)

	decoded, err := png.Decode(bytes.NewReader(storedSecond))
	require.NoError(t, err)
	require.Equal(t, 240, decoded.Bounds().Dx())
	require.Equal(t, 240, decoded.Bounds().Dy())
	_, err = os.Stat(handler.filePath + ".old")
	require.True(t, os.IsNotExist(err))
	temporaryFiles, err := filepath.Glob(filepath.Join(dir, ".wechat-group-qr-*.png"))
	require.NoError(t, err)
	require.Empty(t, temporaryFiles)
}

func TestWechatGroupQRUploadStoresExpiryAndFallsBackAfterExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	handler := &WechatGroupQRHandler{
		filePath: filepath.Join(dir, "wechat-group-qr.png"),
		metaPath: filepath.Join(dir, "wechat-group-qr.json"),
	}
	img := makeWechatQRScreenshot(360, 544, image.Rect(48, 184, 312, 448), color.Black)
	expiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	response := uploadWechatQRForTest(t, handler, img, expiresAt)
	require.Equal(t, http.StatusOK, response.Code)
	info := handler.currentInfo()
	require.NotNil(t, info.ExpiresAt)
	require.False(t, info.Expired)
	require.Contains(t, info.ImageURL, "/api/v1/community/wechat-group-qr/image")

	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, handler.save(img, &expiredAt))
	expiredInfo := handler.currentInfo()
	require.True(t, expiredInfo.Expired)
	require.Contains(t, expiredInfo.ImageURL, "/api/v1/community/wechat-group-qr/image")
}

func TestParseWechatQRExpiryRequiresFutureRFC3339(t *testing.T) {
	_, err := parseWechatQRExpiry("not-a-time")
	require.ErrorContains(t, err, "格式无效")
	_, err = parseWechatQRExpiry(time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))
	require.ErrorContains(t, err, "必须晚于当前时间")
}

func TestFindWechatQRCodeBoundsRejectsImageWithoutQR(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 360, 544))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 25, G: 25, B: 25, A: 255}}, image.Point{}, draw.Src)
	_, err := findWechatQRCodeBounds(img)
	require.ErrorContains(t, err, "未识别到完整二维码")
}

func uploadWechatQRForTest(t *testing.T, handler *WechatGroupQRHandler, img image.Image, expiresAt ...string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "group.png")
	require.NoError(t, err)
	require.NoError(t, png.Encode(part, img))
	if len(expiresAt) > 0 && expiresAt[0] != "" {
		require.NoError(t, writer.WriteField("expires_at", expiresAt[0]))
	}
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/wechat-group-qr", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	handler.Upload(context)
	return recorder
}

func makeWechatQRScreenshot(width, height int, panel image.Rectangle, moduleColor color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 28, G: 28, B: 28, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, panel, &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	side := panel.Dx()
	module := max(3, side/66)
	margin := max(12, module*4)
	qr := image.Rect(panel.Min.X+margin, panel.Min.Y+margin, panel.Max.X-margin, panel.Max.Y-margin)
	for y := qr.Min.Y; y < qr.Max.Y; y += module * 2 {
		for x := qr.Min.X; x < qr.Max.X; x += module * 2 {
			draw.Draw(img, image.Rect(x, y, min(x+module, qr.Max.X), min(y+module, qr.Max.Y)), &image.Uniform{C: moduleColor}, image.Point{}, draw.Src)
		}
	}
	finderSide := module * 9
	drawFinderPattern(img, image.Rect(qr.Min.X, qr.Min.Y, qr.Min.X+finderSide, qr.Min.Y+finderSide), moduleColor)
	drawFinderPattern(img, image.Rect(qr.Max.X-finderSide, qr.Min.Y, qr.Max.X, qr.Min.Y+finderSide), moduleColor)
	drawFinderPattern(img, image.Rect(qr.Min.X, qr.Max.Y-finderSide, qr.Min.X+finderSide, qr.Max.Y), moduleColor)
	return img
}

func drawFinderPattern(img draw.Image, bounds image.Rectangle, moduleColor color.Color) {
	draw.Draw(img, bounds, &image.Uniform{C: moduleColor}, image.Point{}, draw.Src)
	inset := max(2, bounds.Dx()/7)
	middle := image.Rect(bounds.Min.X+inset, bounds.Min.Y+inset, bounds.Max.X-inset, bounds.Max.Y-inset)
	draw.Draw(img, middle, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	center := image.Rect(middle.Min.X+inset, middle.Min.Y+inset, middle.Max.X-inset, middle.Max.Y-inset)
	draw.Draw(img, center, &image.Uniform{C: moduleColor}, image.Point{}, draw.Src)
}
