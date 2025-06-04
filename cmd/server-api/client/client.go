package client

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient 是 HTTP API 客户端
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient 创建一个新的API客户端
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// Get 发送GET请求
func (c *APIClient) Get(path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

// Post 发送POST请求
func (c *APIClient) Post(path string, requestBody interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("JSON编码失败: %w", err)
	}

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

// handleResponse 处理HTTP响应
func handleResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d，响应: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// APIResponse API统一响应结构
type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// HexData 十六进制数据转换辅助函数
func HexData(hexString string) []byte {
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return []byte{}
	}
	return data
}
