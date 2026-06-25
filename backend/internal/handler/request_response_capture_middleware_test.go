package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestShouldCaptureGatewayBody_AllowsMultipartForm(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")

	require.True(t, shouldCaptureGatewayBody(req))
}

func TestCaptureRequestBody_MultipartSummarizesWithoutFileContent(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("model", "gpt-image-1"))
	require.NoError(t, mw.WriteField("prompt", "make a cat"))
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="cat.png"`)
	h.Set("Content-Type", "image/png")
	fw, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = fw.Write([]byte("PNG_BINARY_SHOULD_NOT_BE_SAVED"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	body, truncated, bodyBytes := captureRequestBody(req, 4096)
	require.False(t, truncated)
	require.Equal(t, buf.Len(), bodyBytes)
	require.True(t, gjson.Get(body, "multipart").Bool())
	require.Equal(t, "gpt-image-1", gjson.Get(body, "model").String())
	require.Equal(t, "make a cat", gjson.Get(body, "fields.prompt.0").String())
	require.Equal(t, "image", gjson.Get(body, "files.0.field").String())
	require.Equal(t, "cat.png", gjson.Get(body, "files.0.filename").String())
	require.Equal(t, "image/png", gjson.Get(body, "files.0.content_type").String())
	require.Equal(t, int64(len("PNG_BINARY_SHOULD_NOT_BE_SAVED")), gjson.Get(body, "files.0.size").Int())
	require.Contains(t, gjson.Get(body, "files.0.data_url").String(), "data:image/png;base64,")
	require.NotContains(t, body, "PNG_BINARY_SHOULD_NOT_BE_SAVED")

	restored, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, buf.Bytes(), restored)
}
