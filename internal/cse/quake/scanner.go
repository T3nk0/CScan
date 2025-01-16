package quake

import (
	"bytes"
	"cscan/internal/common/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Scanner struct {
	apiKey string
}

func NewScanner(apiKey string) *Scanner {
	return &Scanner{apiKey: apiKey}
}

func (s *Scanner) Name() string {
	return "Quake"
}

func (s *Scanner) Search(query string, page, size int) ([]model.Asset, error) {
	baseURL := "https://quake.360.net/api/v3/search/quake_service"

	requestData := map[string]interface{}{
		"query":  query,
		"start":  (page - 1) * size,
		"size":   size,
		"fields": "ip,port,domain,service,title,status_code,location,icp",
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-QuakeToken", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

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
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    []interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API错误: %v", result.Message)
	}

	var assets []model.Asset
	for _, item := range result.Data {
		if m, ok := item.(map[string]interface{}); ok {
			asset := model.Asset{
				Source: s.Name(),
			}

			if v, ok := m["ip"].(string); ok {
				asset.IP = v
			}
			if v, ok := m["domain"].(string); ok {
				asset.Domain = v
			}
			if v, ok := m["port"].(float64); ok {
				asset.Port = fmt.Sprintf("%d", int(v))
			}

			// 处理service字段
			if service, ok := m["service"].(map[string]interface{}); ok {
				if name, ok := service["name"].(string); ok {
					asset.Service = name
				}

				// 处理http信息
				if http, ok := service["http"].(map[string]interface{}); ok {
					if title, ok := http["title"].(string); ok {
						asset.Title = title
					}
					if status, ok := http["status_code"].(float64); ok {
						asset.StatusCode = fmt.Sprintf("%d", int(status))
					}
				}
			}

			// 处理location字段
			if location, ok := m["location"].(map[string]interface{}); ok {
				var parts []string
				if v, ok := location["country_cn"].(string); ok && v != "" {
					parts = append(parts, v)
				}
				if v, ok := location["province_cn"].(string); ok && v != "" {
					parts = append(parts, v)
				}
				if v, ok := location["city_cn"].(string); ok && v != "" {
					parts = append(parts, v)
				}
				asset.Location = joinNonEmpty(parts)
			}

			assets = append(assets, asset)
		}
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
