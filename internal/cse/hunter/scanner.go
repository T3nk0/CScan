package hunter

import (
	"cscan/internal/common/model"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Scanner struct {
	apiKey string
}

func NewScanner(apiKey string) *Scanner {
	return &Scanner{apiKey: apiKey}
}

func (s *Scanner) Name() string {
	return "Hunter"
}

func (s *Scanner) Search(query string, page, size int) ([]model.Asset, error) {
	baseURL := "https://hunter.qianxin.com/openApi/search"
	searchBase64 := base64.StdEncoding.EncodeToString([]byte(query))

	params := url.Values{}
	params.Add("api-key", s.apiKey)
	params.Add("search", searchBase64)
	params.Add("page", fmt.Sprintf("%d", page))
	params.Add("page_size", fmt.Sprintf("%d", size))

	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Arr []map[string]interface{} `json:"arr"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("API错误: %v", result.Message)
	}

	var assets []model.Asset
	for _, item := range result.Data.Arr {
		asset := model.Asset{
			Source: s.Name(),
		}

		if v, ok := item["ip"].(string); ok {
			asset.IP = v
		}
		if v, ok := item["domain"].(string); ok {
			asset.Domain = v
		}
		if v, ok := item["port"].(float64); ok {
			asset.Port = fmt.Sprintf("%d", int(v))
		}
		if v, ok := item["protocol"].(string); ok {
			asset.Service = v
		}
		if v, ok := item["web_title"].(string); ok {
			asset.Title = v
		}
		if v, ok := item["status_code"].(float64); ok {
			asset.StatusCode = fmt.Sprintf("%d", int(v))
		}

		// 处理ICP信息
		if icp, ok := item["icp"].(map[string]interface{}); ok {
			if name, ok := icp["name"].(string); ok {
				asset.ICPOrg = name
			}
		}

		// 处理地理位置信息
		var location []string
		if v, ok := item["country"].(string); ok && v != "" {
			location = append(location, v)
		}
		if v, ok := item["province"].(string); ok && v != "" {
			location = append(location, v)
		}
		if v, ok := item["city"].(string); ok && v != "" {
			location = append(location, v)
		}
		asset.Location = joinNonEmpty(location)

		assets = append(assets, asset)
	}

	return assets, nil
}

func joinNonEmpty(parts []string) string {
	var result string
	for i, part := range parts {
		if part != "" {
			if i > 0 && result != "" {
				result += " "
			}
			result += part
		}
	}
	return result
}
