package fofa

import (
	"cscan/internal/common/model"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Scanner struct {
	email  string
	apiKey string
}

func NewScanner(email, apiKey string) *Scanner {
	return &Scanner{
		email:  email,
		apiKey: apiKey,
	}
}

func (s *Scanner) Name() string {
	return "FOFA"
}

func (s *Scanner) Search(query string, page, size int) ([]model.Asset, error) {
	baseURL := "https://fofa.info/api/v1/search/all"
	queryBase64 := base64.StdEncoding.EncodeToString([]byte(query))

	url := fmt.Sprintf("%s?email=%s&key=%s&qbase64=%s&page=%d&size=%d&fields=host,ip,port,protocol,title,icp,country,province,city",
		baseURL, s.email, s.apiKey, queryBase64, page, size)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Error   bool       `json:"error"`
		ErrMsg  string     `json:"errmsg"`
		Results [][]string `json:"results"`
		Fields  []string   `json:"fields"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Error {
		return nil, fmt.Errorf("API错误: %v", result.ErrMsg)
	}

	var assets []model.Asset
	for _, fields := range result.Results {
		if len(fields) < 9 {
			continue
		}

		asset := model.Asset{
			Domain:   fields[0],
			IP:       fields[1],
			Port:     fields[2],
			Service:  fields[3],
			Title:    fields[4],
			ICPOrg:   fields[5],
			Location: joinNonEmpty([]string{fields[6], fields[7], fields[8]}),
			Source:   s.Name(),
		}
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
