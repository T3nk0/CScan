package excel

import (
	"bufio"
	"cscan/internal/common/model"
	"cscan/internal/cse"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// 添加错误常量
const (
	ErrInvalidFormat = "无效的文件格式"
	ErrEmptyFile     = "文件为空"
)

// ReadTargets 从文本文件读取目标
func ReadTargets(filename string) ([]cse.Target, error) {
	if !strings.HasSuffix(filename, ".txt") {
		return nil, fmt.Errorf(ErrInvalidFormat)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return nil, fmt.Errorf(ErrEmptyFile)
	}

	// 用于去重的map
	seen := make(map[string]bool)
	var targets []cse.Target

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' { // 跳过空行和注释
			continue
		}

		// 处理每一行内容
		cellTargets := parseTargets(line)
		for _, target := range cellTargets {
			if !seen[target.Value] {
				seen[target.Value] = true
				targets = append(targets, target)
				fmt.Printf("添加唯一目标: %s (%s)\n", target.Value, target.Type)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	if len(targets) == 0 {
		fmt.Println("警告: 未找到任何有效目标，请确保target.txt文件存在且包含有效内容")
	} else {
		fmt.Printf("共读取到 %d 个唯一目标\n", len(targets))
	}
	return targets, nil
}

// ReadCompanies 从文本文件读取公司名称
func ReadCompanies(filename string) ([]string, error) {
	// 读取文件内容
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	var companies []string
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		company := strings.TrimSpace(scanner.Text())
		// 跳过空行、注释和明显不是公司名的内容
		if company == "" || company[0] == '#' || strings.Contains(company, "PK") || strings.Contains(company, ".xml") {
			continue
		}

		// 基本的公司名称验证
		if isValidCompanyName(company) {
			if !seen[company] {
				seen[company] = true
				companies = append(companies, company)
				fmt.Printf("添加公司: %s\n", company)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	if len(companies) == 0 {
		fmt.Println("警告: 未找到任何有效公司名称，请确保target.txt文件存在且包含有效内容")
	} else {
		fmt.Printf("共读取到 %d 个公司\n", len(companies))
	}
	return companies, nil
}

// isValidCompanyName 验证公司名称的有效性
func isValidCompanyName(name string) bool {
	// 跳过明显的非公司名内容
	invalidPatterns := []string{
		"PK",
		".xml",
		"<?xml",
		"[]",
		"{}",
		"<",
		">",
		"@",
		"http",
		".com",
		".cn",
		".jp",
		".org",
		".net",
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(name, pattern) {
			return false
		}
	}

	// 公司名称的基本验证规则
	// 1. 长度至少2个字符
	if len(name) < 2 {
		return false
	}

	// 2. 不能只包含数字和符号
	hasLetter := false
	for _, r := range name {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}

	// 3. 常见的公司名称后缀
	companySuffixes := []string{
		"公司",
		"集团",
		"有限",
		"股份",
		"企业",
		"工厂",
		"厂",
		"Corporation",
		"Corp",
		"Inc",
		"Ltd",
		"Limited",
		"LLC",
		"Co",
	}

	// 检查是否包含常见的公司名称后缀
	for _, suffix := range companySuffixes {
		if strings.Contains(name, suffix) {
			return true
		}
	}

	// 如果没有明显的公司后缀，但看起来像是中文名称（包含至少2个汉字）
	chineseCount := 0
	for _, r := range name {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		}
	}
	return chineseCount >= 2
}

// normalizeDomain 规范化域名，只保留合适的层级
func normalizeDomain(domain string) string {
	// 分割域名
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain // 如果只有两级或更少，直接返回
	}

	// 处理特殊的二级域名后缀
	specialTLDs := map[string]bool{
		"com.cn": true,
		"org.cn": true,
		"net.cn": true,
		"gov.cn": true,
		"edu.cn": true,
		"co.jp":  true,
		"co.uk":  true,
		// 可以根据需要添加更多
	}

	// 检查是否有特殊的二级域名后缀
	lastTwo := strings.Join(parts[len(parts)-2:], ".")
	if specialTLDs[lastTwo] {
		if len(parts) > 3 {
			// 返回倒数第三级 + 特殊后缀
			return strings.Join(parts[len(parts)-3:], ".")
		}
		return domain
	}

	// 普通域名，返回倒数第二级 + 顶级域名
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return domain
}

// parseTargets 解析内容，返回所有有效的目标
func parseTargets(content string) []cse.Target {
	var targets []cse.Target

	// 处理所有可能的分隔符，包括换行符
	parts := strings.FieldsFunc(content, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == ' ' || r == '\t' || r == '\n'
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 处理可能的URL格式
		part = cleanURL(part)

		// 尝试提取IP地址
		if isValidIP(part) {
			targets = append(targets, cse.Target{
				Value: part,
				Type:  "ip",
			})
			continue
		}

		// 尝试提取域名
		if isDomain(part) {
			// 规范化域名
			normalizedDomain := normalizeDomain(part)
			// 检查是否已经添加过这个域名
			exists := false
			for _, t := range targets {
				if t.Type == "domain" && t.Value == normalizedDomain {
					exists = true
					break
				}
			}
			if !exists {
				targets = append(targets, cse.Target{
					Value: normalizedDomain,
					Type:  "domain",
				})
			}
		}
	}

	return targets
}

// cleanURL 清理URL，提取域名部分
func cleanURL(url string) string {
	// 移除协议前缀
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "www.")

	// 如果包含路径，只保留域名部分
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[0:idx]
	}

	return url
}

// isValidIP 验证IP地址的有效性
func isValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}
	return true
}

// isDomain 验证域名的有效性
func isDomain(domain string) bool {
	// 域名的基本验证规则
	domainPattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(domainPattern, domain)

	// 额外检查常见的无效域名情况
	if !matched {
		return false
	}

	// 确保不是IP地址格式
	if isValidIP(domain) {
		return false
	}

	// 检查是否包含有效的顶级域名
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	// 检查顶级域名是否合法
	tld := parts[len(parts)-1]
	validTLDs := map[string]bool{
		"com": true, "net": true, "org": true, "edu": true, "gov": true,
		"cn": true, "jp": true, "uk": true, "ru": true, "de": true,
		"fr": true, "br": true, "in": true, "au": true, "info": true,
		"biz": true, "io": true, "co": true, "me": true, "tv": true,
		// 可以根据需要添加更多有效的顶级域名
	}

	return validTLDs[tld]
}

// SaveResults 将结果保存到Excel文件，并进行去重和添加筛选功能
func SaveResults(results []model.Asset, filename string) error {
	f := excelize.NewFile()
	defer f.Close()

	// 设置表头
	headers := []string{"IP", "域名", "端口", "服务", "标题", "状态码", "ICP主体", "地理位置", "来源"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue("Sheet1", cell, header)
	}

	// 去重处理
	uniqueResults := deduplicateAssets(results)
	fmt.Printf("去重前: %d 条记录, 去重后: %d 条记录\n", len(results), len(uniqueResults))

	// 写入数据
	for i, asset := range uniqueResults {
		row := i + 2
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", row), asset.IP)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", row), asset.Domain)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", row), asset.Port)
		f.SetCellValue("Sheet1", fmt.Sprintf("D%d", row), asset.Service)
		f.SetCellValue("Sheet1", fmt.Sprintf("E%d", row), asset.Title)
		f.SetCellValue("Sheet1", fmt.Sprintf("F%d", row), asset.StatusCode)
		f.SetCellValue("Sheet1", fmt.Sprintf("G%d", row), asset.ICPOrg)
		f.SetCellValue("Sheet1", fmt.Sprintf("H%d", row), asset.Location)
		f.SetCellValue("Sheet1", fmt.Sprintf("I%d", row), asset.Source)
	}

	// 添加筛选功能
	lastRow := len(uniqueResults) + 1
	if lastRow < 2 {
		lastRow = 2 // 确保至少有一行数据
	}

	// 设置自动筛选
	if err := f.AutoFilter("Sheet1", fmt.Sprintf("A1:I%d", lastRow), []excelize.AutoFilterOptions{}); err != nil {
		return fmt.Errorf("设置筛选失败: %v", err)
	}

	// 调整列宽以适应内容
	for i := 0; i < len(headers); i++ {
		col := string('A' + i)
		if err := f.SetColWidth("Sheet1", col, col, 20); err != nil {
			fmt.Printf("设置列宽失败 %s: %v\n", col, err)
		}
	}

	// 冻结首行
	if err := f.SetPanes("Sheet1", &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		fmt.Printf("冻结首行失败: %v\n", err)
	}

	// 保存文件
	return f.SaveAs(filename)
}

// deduplicateAssets 对资产进行去重
func deduplicateAssets(assets []model.Asset) []model.Asset {
	seen := make(map[string]bool)
	var result []model.Asset

	for _, asset := range assets {
		// 生成唯一标识
		key := generateAssetKey(asset)
		if !seen[key] {
			seen[key] = true
			result = append(result, asset)
		}
	}

	return result
}

// generateAssetKey 生成资产的唯一标识
func generateAssetKey(asset model.Asset) string {
	// 如果有IP和端口，使用"IP:端口"作为key
	if asset.IP != "" && asset.Port != "" {
		return fmt.Sprintf("%s:%s", asset.IP, asset.Port)
	}
	// 如果有域名，使用域名作为key
	if asset.Domain != "" {
		return asset.Domain
	}
	// 如果都没有，使用所有非空字段组合
	parts := []string{
		asset.IP,
		asset.Domain,
		asset.Port,
		asset.Service,
		asset.Title,
		asset.StatusCode,
		asset.ICPOrg,
		asset.Location,
	}
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, "|")
}
