package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/kabili207/matrixemoji/pkg/models"
)

type imageInfo struct {
	Path       string
	Name       string
	MimeType   string
	IsAnimated bool
	Width      int
	Height     int
	Bytes      []byte
}

type MatrixClient interface {
	EncodePackId(packName string) string
	GetEmotePack(roomId string, packId string) (*models.Pack, error)
	PutEmotePack(roomId string, packId string, emotePack *models.Pack) (*models.Pack, error)
	UploadFile(fileName string, mimeType string, data []byte) (string, error)
}

type matrixClient struct {
	baseUrl   string
	authToken string
}

func NewMatrixClient(baseUrl string, authToken string) MatrixClient {
	return &matrixClient{
		baseUrl:   baseUrl,
		authToken: authToken,
	}
}

func (c *matrixClient) makeAuthedRequest(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.authToken)
	return req, nil
}

func (c *matrixClient) makePostRequest(url string, mimeType string, data []byte) (*http.Request, error) {

	reader := bytes.NewReader(data)

	req, err := c.makeAuthedRequest("POST", url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", mimeType)

	return req, nil
}

func (c *matrixClient) EncodePackId(packName string) string {
	validUrlRegex := regexp.MustCompile(`([^0-9a-zA-Z\-_\.+!*'(),]+)`)
	packUrlSlug := validUrlRegex.ReplaceAllString(packName, "-")
	return packUrlSlug
}

func (c *matrixClient) PutEmotePack(roomId string, packId string, emotePack *models.Pack) (*models.Pack, error) {

	packUrl := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/state/im.ponies.room_emotes/%s",
		c.baseUrl, url.QueryEscape(roomId), url.PathEscape(packId))

	jsonStr, err := json.Marshal(emotePack)
	if err != nil {
		return nil, err
	}

	req, err := c.makeAuthedRequest("PUT", packUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&emotePack)
	}
	return emotePack, err
}

func (c *matrixClient) GetEmotePack(roomId string, packId string) (*models.Pack, error) {

	packUrl := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/state/im.ponies.room_emotes/%s",
		c.baseUrl, url.QueryEscape(roomId), url.PathEscape(packId))

	req, err := c.makeAuthedRequest("GET", packUrl, nil)
	if err != nil {
		return nil, err
	}

	var emotePack models.Pack

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&emotePack)
	}
	return &emotePack, err
}

type uploadResponse struct {
	ContentUrl   *string `json:"content_uri,omitempty"`
	ErrorCode    *string `json:"errcode,omitempty"`
	RetryAfterMs *int    `json:"retry_after_ms,omitempty"`
}

func (c *matrixClient) UploadFile(fileName string, mimeType string, data []byte) (string, error) {

	uploadUrl := fmt.Sprintf("%s/_matrix/media/v3/upload?filename=%s", c.baseUrl, url.QueryEscape(fileName))
	req, err := c.makePostRequest(uploadUrl, mimeType, data)

	if err != nil {
		return "", err
	}

	var uploadResp uploadResponse

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
		json.NewDecoder(resp.Body).Decode(&uploadResp)
	}

	if uploadResp.ErrorCode != nil {
		if *uploadResp.ErrorCode == "M_LIMIT_EXCEEDED" {
			// TODO: Add a retry limit to prevent an infinite loop
			time.Sleep(time.Duration(*uploadResp.RetryAfterMs) * time.Millisecond)
			return c.UploadFile(fileName, mimeType, data)
		} else {
			return "", fmt.Errorf("error %s", *uploadResp.ErrorCode)
		}
	}

	return *uploadResp.ContentUrl, nil
}
