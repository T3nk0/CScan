package co

import (
	"cscan/internal/co/zone"
	"cscan/internal/common/model"
	"fmt"
	"time"
)

// Scanner 定义了公司资产扫描器的接口
type Scanner interface {
	// Name 返回扫描器名称
	Name() string

	// SearchByCompany 根据公司名称搜索相关资产
	SearchByCompany(company string, page, size int) (map[string][]model.Asset, error)
}

// CompanyScanner 公司情报扫描器管理器
type CompanyScanner struct {
	scanners []Scanner
}

// NewCompanyScanner 创建新的公司情报扫描器管理器
func NewCompanyScanner(scanners ...Scanner) *CompanyScanner {
	return &CompanyScanner{
		scanners: scanners,
	}
}

// Search 使用所有可用的扫描器执行搜索
func (c *CompanyScanner) Search(company string, page, size int) ([]model.Asset, error) {
	var results []model.Asset
	for _, scanner := range c.scanners {
		assetMap, err := scanner.SearchByCompany(company, page, size)
		if err != nil {
			continue
		}
		// 合并所有类型的资产
		for _, assets := range assetMap {
			results = append(results, assets...)
		}
	}
	return results, nil
}

// SearchCompanies 批量搜索公司
func (c *CompanyScanner) SearchCompanies(companies []string, maxPage, pageSize int) ([]model.Asset, error) {
	allResults := make(map[string][]model.Asset)

	for i, company := range companies {
		fmt.Printf("处理公司 (%d/%d): %s\n", i+1, len(companies), company)

		for _, scanner := range c.scanners {
			if scanner == nil {
				continue
			}

			fmt.Printf("使用 %s 搜索...\n", scanner.Name())
			assetMap, err := scanner.SearchByCompany(company, 1, pageSize)
			if err != nil {
				fmt.Printf("查询出错: %v\n", err)
				continue
			}

			// 合并每种类型的资产
			for searchType, assets := range assetMap {
				allResults[searchType] = append(allResults[searchType], assets...)
			}
		}

		if i < len(companies)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// 调用 Zone Scanner 的 SaveResults 方法保存结果
	if zoneScanner := c.getZoneScanner(); zoneScanner != nil {
		if err := zoneScanner.SaveResults(allResults, "results.xlsx"); err != nil {
			return nil, fmt.Errorf("保存结果失败: %v", err)
		}
	}

	// 转换为扁平结构返回
	var flatResults []model.Asset
	for _, assets := range allResults {
		flatResults = append(flatResults, assets...)
	}

	return flatResults, nil
}

// getZoneScanner 获取 Zone Scanner 实例
func (c *CompanyScanner) getZoneScanner() *zone.Scanner {
	for _, scanner := range c.scanners {
		if s, ok := scanner.(*zone.Scanner); ok {
			return s
		}
	}
	return nil
}
