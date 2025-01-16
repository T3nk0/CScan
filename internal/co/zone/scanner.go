package zone

import (
	"bytes"
	"cscan/internal/common/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type Scanner struct {
	client *http.Client
	key    string
}

type ZoneAsset struct {
	IP       string `json:"ip"`
	Domain   string `json:"domain"`
	Port     string `json:"port"`
	Service  string `json:"service"`
	Title    string `json:"title"`
	Location string `json:"location"`
	Status   string `json:"status"`
	Company  string `json:"company"`
}

func NewScanner(apiKey string) *Scanner {
	return &Scanner{
		client: &http.Client{},
		key:    apiKey,
	}
}

func (s *Scanner) Name() string {
	return "Zone"
}

// 定义支持的搜索类型和对应的表头
var searchTypes = map[string][]string{
	"site": {
		"IP", "Domain", "Port", "Service", "Title", "Status",
		"Location", "Company", "UpdateTime",
	},
	"apk": {
		"Name", "Package", "Version", "Platform", "Size",
		"Developer", "Category", "UpdateTime",
	},
	"domain": {
		"Domain", "Registrar", "RegisterTime", "ExpireTime",
		"Status", "Company", "UpdateTime",
	},
	"email": {
		"Email", "Source", "Company", "UpdateTime",
	},
	"code": {
		"Title", "URL", "Language", "Source", "UpdateTime",
	},
	"member": {
		"Name", "Position", "Department", "Company", "Source", "UpdateTime",
	},
}

func (s *Scanner) SearchByCompany(company string, page, size int) (map[string][]model.Asset, error) {
	results := make(map[string][]model.Asset)
	fmt.Printf("正在搜索公司: %s\n", company)

	// 遍历所有搜索类型
	for searchType := range searchTypes {
		typeResults, err := s.searchByType(company, searchType, page, size)
		if err != nil {
			fmt.Printf("- %s搜索失败: %v\n", searchType, err)
			// 如果是权限错误，添加一个特殊的资产来标记
			if strings.Contains(err.Error(), "无权限") || strings.Contains(err.Error(), "未授权") {
				results[searchType] = []model.Asset{{
					Title:   "API访问受限",
					Source:  "0.zone",
					Service: fmt.Sprintf("当前API Key无%s数据访问权限", searchType),
				}}
			}
			continue
		}
		if len(typeResults) > 0 {
			fmt.Printf("- 找到%d个%s资产\n", len(typeResults), searchType)
			results[searchType] = typeResults
		}
	}

	return results, nil
}

func (s *Scanner) searchByType(company, queryType string, page, size int) ([]model.Asset, error) {
	baseURL := "https://0.zone/api/data/" + queryType
	requestBody := map[string]interface{}{
		"query":       buildQuery(company),
		"query_type":  queryType,
		"page":        page,
		"pagesize":    size,
		"zone_key_id": s.key,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("构建请求体失败: %v", err)
	}

	req, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Sort    struct {
			Timestamp struct {
				Order string `json:"order"`
			} `json:"timestamp"`
		} `json:"sort"`
		Page     int         `json:"page"`
		Next     interface{} `json:"next"`
		Pagesize int         `json:"pagesize"`
		Total    string      `json:"total"`
		Data     []struct {
			ID        string      `json:"_id"`
			Sort      []int64     `json:"sort"`
			IP        string      `json:"ip"`
			IPAddr    string      `json:"ip_addr"`
			Port      string      `json:"port"`
			URL       string      `json:"url"`
			Title     string      `json:"title"`
			Service   string      `json:"service"`
			Status    interface{} `json:"status"`  // 可能是多种类型
			Domain    interface{} `json:"domain"`  // 可能是字符串或数组
			Company   interface{} `json:"company"` // 可能是字符串或数组
			Country   string      `json:"country"`
			Province  string      `json:"province"`
			City      string      `json:"city"`
			Platform  string      `json:"platform"`
			Name      string      `json:"name"`
			Component string      `json:"component"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("%s", response.Message)
	}

	var results []model.Asset
	for _, item := range response.Data {
		asset := model.Asset{Source: "0.zone"}

		switch queryType {
		case "site":
			asset.IP = item.IP
			asset.Port = item.Port
			asset.Service = item.Service
			if item.Component != "" {
				if asset.Service != "" {
					asset.Service += "/"
				}
				asset.Service += item.Component
			}
			asset.Title = item.Title
			if item.URL != "" {
				if u, err := url.Parse(item.URL); err == nil {
					asset.Domain = u.Hostname()
				}
			}
			asset.Location = formatLocation(item.Country, item.Province, item.City)

		case "domain", "org":
			// 处理 domain 字段
			if item.Domain != nil {
				switch d := item.Domain.(type) {
				case string:
					asset.Domain = d
				case []interface{}:
					if len(d) > 0 {
						if str, ok := d[0].(string); ok {
							asset.Domain = str
						}
					}
				}
			}

			// 处理 company 字段
			if item.Company != nil {
				switch c := item.Company.(type) {
				case string:
					asset.ICPOrg = c
				case []interface{}:
					if len(c) > 0 {
						if str, ok := c[0].(string); ok {
							asset.ICPOrg = str
						}
					}
				}
			}
			asset.Location = formatLocation(item.Country, item.Province, item.City)
		}

		if isValidAsset(asset, queryType) {
			results = append(results, asset)
		}
	}

	return results, nil
}

// formatLocation 格式化位置信息
func formatLocation(country, province, city string) string {
	var parts []string
	for _, part := range []string{country, province, city} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "/")
}

// isValidAsset 检查资产是否有效
func isValidAsset(asset model.Asset, queryType string) bool {
	switch queryType {
	case "site":
		return asset.IP != "" || asset.Domain != "" || asset.Port != ""
	case "domain":
		return asset.Domain != ""
	case "org":
		return asset.ICPOrg != ""
	default:
		return true
	}
}

// SaveResults 将结果保存到Excel文件的不同sheet中
func (s *Scanner) SaveResults(results map[string][]model.Asset, filename string) error {
	f := excelize.NewFile()

	// 删除默认的Sheet1
	f.DeleteSheet("Sheet1")

	// 确保所有支持的类型都有对应的sheet
	for searchType := range searchTypes {
		// 创建新的sheet
		sheetName := strings.ToUpper(searchType)
		_, err := f.NewSheet(sheetName)
		if err != nil {
			return fmt.Errorf("创建sheet失败: %v", err)
		}

		// 写入表头
		headers := searchTypes[searchType]
		for i, header := range headers {
			cell := fmt.Sprintf("%c1", 'A'+i)
			f.SetCellValue(sheetName, cell, header)
		}

		// 获取当前类型的资产
		assets := results[searchType]

		// 如果没有数据，添加说明行
		if len(assets) == 0 {
			cell := "A2"
			message := "当前API Key无此类型数据访问权限或未找到相关数据"
			f.SetCellValue(sheetName, cell, message)
			f.MergeCell(sheetName, cell, fmt.Sprintf("%c2", 'A'+len(headers)-1))
		} else {
			// 写入数据
			for i, asset := range assets {
				row := i + 2
				data := s.formatAssetData(asset, searchType)
				for j, value := range data {
					cell := fmt.Sprintf("%c%d", 'A'+j, row)
					f.SetCellValue(sheetName, cell, value)
				}
			}
		}

		// 设置自动筛选
		lastCol := string('A' + len(headers) - 1)
		lastRow := max(len(assets)+1, 2)
		f.AutoFilter(sheetName, fmt.Sprintf("A1:%s%d", lastCol, lastRow), nil)

		// 冻结首行
		f.SetPanes(sheetName, &excelize.Panes{
			Freeze:      true,
			Split:       false,
			XSplit:      0,
			YSplit:      1,
			TopLeftCell: "A2",
			ActivePane:  "bottomLeft",
		})

		// 调整列宽
		for i := range headers {
			col := string('A' + i)
			f.SetColWidth(sheetName, col, col, 20)
		}
	}

	// 保存文件
	return f.SaveAs(filename)
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatAssetData 根据不同类型格式化资产数据
func (s *Scanner) formatAssetData(asset model.Asset, searchType string) []string {
	switch searchType {
	case "site":
		return []string{
			asset.IP,
			asset.Domain,
			asset.Port,
			asset.Service,
			asset.Title,
			asset.StatusCode,
			asset.Location,
			asset.ICPOrg,
			asset.UpdatedAt,
		}
	case "domain":
		return []string{
			asset.Domain,
			asset.Registrar,
			asset.RegisterTime,
			asset.ExpireTime,
			asset.Status,
			asset.ICPOrg,
			asset.UpdatedAt,
		}
	// ... 其他类型的格式化逻辑
	default:
		return []string{asset.Title, asset.Service}
	}
}

// Search 执行搜索
func (s *Scanner) Search(query string, queryType string, page, size int) ([]model.Asset, error) {
	var allAssets []model.Asset
	currentPage := page

	// 添加API限制说明
	const maxResults = 100 // 0.zone API限制单次最多返回100条记录

	for {
		// 构造请求数据
		data := map[string]interface{}{
			"query":       query,
			"query_type":  queryType,
			"page":        currentPage,
			"pagesize":    size,
			"zone_key_id": s.key,
		}

		// 发送POST请求
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("JSON编码失败: %v", err)
		}

		// 创建请求
		req, err := http.NewRequest("POST", "https://0.zone/api/data/", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %v", err)
		}

		// 设置请求头
		req.Header.Set("Host", "0.zone")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(jsonData)))

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("请求失败: %v", err)
		}
		defer resp.Body.Close()

		// 解析响应
		var response struct {
			Code     int    `json:"code"`
			Message  string `json:"message"`
			Total    string `json:"total"` // 注意这里改为 string
			Page     int    `json:"page"`
			Pagesize int    `json:"pagesize"`
			Data     []struct {
				IP        string      `json:"ip"`
				Port      string      `json:"port"`
				Service   string      `json:"service"`
				Title     string      `json:"title"`
				URL       string      `json:"url"`
				Domain    interface{} `json:"domain"`
				Company   interface{} `json:"company"`
				Country   string      `json:"country"`
				Province  string      `json:"province"`
				City      string      `json:"city"`
				Platform  string      `json:"platform"`
				Component string      `json:"component"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("解析响应失败: %v", err)
		}

		if response.Code != 0 {
			return nil, fmt.Errorf("API错误: %s", response.Message)
		}

		// 处理总页数和总记录数
		total, _ := strconv.Atoi(response.Total)
		if total == 0 {
			break
		}
		totalPages := (total + size - 1) / size

		fmt.Printf("正在获取第 %d/%d 页数据，总记录数: %d (API限制最多返回%d条)\n",
			currentPage, totalPages, total, maxResults)

		// 转换结果
		for _, item := range response.Data {
			asset := model.Asset{
				IP:      item.IP,
				Port:    item.Port,
				Service: item.Service,
				Title:   item.Title,
				Source:  "Zone",
			}

			// 处理 URL 和域名
			if item.URL != "" {
				if u, err := url.Parse(item.URL); err == nil {
					asset.Domain = u.Hostname()
				}
			}

			// 处理组件信息
			if item.Component != "" {
				if asset.Service != "" {
					asset.Service += "/"
				}
				asset.Service += item.Component
			}

			// 处理位置信息
			asset.Location = formatLocation(item.Country, item.Province, item.City)

			allAssets = append(allAssets, asset)
		}

		// 检查是否需要继续获取下一页
		if currentPage >= totalPages {
			break
		}
		currentPage++

		// 添加延时，避免请求过快
		time.Sleep(time.Second)

		// 检查是否达到API限制
		if len(allAssets) >= maxResults {
			fmt.Printf("已达到API返回上限(%d条记录)\n", maxResults)
			break
		}
	}

	fmt.Printf("共获取到 %d 条记录\n", len(allAssets))
	return allAssets, nil
}

// buildQuery 构建查询语句
func buildQuery(company string) string {
	// 构建完整的查询条件，确保每个值都用引号包围
	conditions := []string{
		fmt.Sprintf(`company=="%s"`, company),
		fmt.Sprintf(`title=="%s"`, company),
		fmt.Sprintf(`banner=="%s"`, company),
		fmt.Sprintf(`html_banner=="%s"`, company),
		fmt.Sprintf(`component=="%s"`, company),
		fmt.Sprintf(`ssl_info.detail=="%s"`, company),
	}
	return strings.Join(conditions, "||")
}

// SearchCompany 搜索公司信息
func (s *Scanner) SearchCompany(company string, maxPage, pageSize int) ([]model.Asset, error) {
	fmt.Printf("正在搜索公司: %s\n", company)

	var allAssets []model.Asset

	// 构建查询语句
	query := buildQuery(company)
	assets, err := s.Search(query, "site", 1, pageSize)
	if err != nil {
		fmt.Printf("- site搜索失败: %v\n", err)
	} else {
		fmt.Printf("- 找到%d个site资产\n", len(assets))
		allAssets = append(allAssets, assets...)
	}

	return allAssets, nil
}
