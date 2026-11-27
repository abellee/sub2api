package server

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

const testItemSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><defs><linearGradient id="g"><stop offset="0" stop-color="red"/></linearGradient></defs><circle cx="12" cy="12" r="10" fill="url(#g)"><animate attributeName="r" values="8;10;8" dur="1s" repeatCount="indefinite"/></circle></svg>`
const testItemLottie = `{"v":"5.7.0","fr":30,"w":100,"h":100,"ip":0,"op":60,"layers":[]}`

func TestPointsAssetsValidateSVGAndLottie(t *testing.T) {
	for _, slot := range []string{"cover", "active", "expired", "animation", "icon"} {
		asset, err := parsePointsAsset("art.svg", slot, []byte(testItemSVG))
		require.NoError(t, err)
		require.Equal(t, "svg", asset.Kind)
		require.Len(t, asset.ID, 64)
	}
	for _, svg := range []string{
		`<svg><script>alert(1)</script></svg>`, `<svg onload="alert(1)"/>`,
		`<svg><foreignObject><div/></foreignObject></svg>`, `<svg><image href="https://example.com/tracker"/></svg>`,
		`<!DOCTYPE svg><svg/>`, `<svg><use href="javascript:alert(1)"/></svg>`,
		`<svg><set attributeName="href" to="https://example.com"/></svg>`, `<svg style="filter:url(https://example.com/x)"/>`,
		`<svg/><svg/>`, `<html/>`,
	} {
		_, err := parsePointsAsset("art.svg", "cover", []byte(svg))
		require.Error(t, err, svg)
	}
	asset, err := parsePointsAsset("animation.json", "animation", []byte(testItemLottie))
	require.NoError(t, err)
	require.Equal(t, "lottie", asset.Kind)
	_, err = parsePointsAsset("animation.json", "cover", []byte(testItemLottie))
	require.Error(t, err)
	for _, body := range []string{`{}`, `{"layers":[]}`, `{"v":"5","fr":30,"w":100,"h":100,"ip":0,"op":30,"layers":[{"x":"alert(1)"}]}`, `{"fr":30,"w":100,"h":100,"ip":0,"op":30,"layers":[],"assets":[{"p":"https://example.com/a.png"}]}`} {
		_, err = parsePointsAsset("animation.json", "animation", []byte(body))
		require.Error(t, err)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifest, err := writer.Create("manifest.json")
	require.NoError(t, err)
	_, err = manifest.Write([]byte(`{"animations":[{"id":"main"}]}`))
	require.NoError(t, err)
	animation, err := writer.Create("animations/main.json")
	require.NoError(t, err)
	_, err = animation.Write([]byte(testItemLottie))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	asset, err = parsePointsAsset("animation.lottie", "animation", buffer.Bytes())
	require.NoError(t, err)
	require.Equal(t, "lottie", asset.Kind)
	_, err = parsePointsAsset("bad.png", "icon", []byte("not an image"))
	require.Error(t, err)
	_, err = parsePointsAsset("big.svg", "cover", bytes.Repeat([]byte(" "), (2<<20)+1))
	require.Error(t, err)
}
