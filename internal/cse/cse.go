package cse

import (
	"cscan/internal/common/model"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Target struct {
	Value string
	Type  string // "ip" 或 "domain"
}

// Scanner 定义了网络空间搜索引擎的通用接口
type Scanner interface {
	// Name 返回扫描器名称
	Name() string

	// Search 执行搜索并返回资产列表
	Search(query string, page, size int) ([]model.Asset, error)
}

// 定义各平台的 API 调用间隔
const (
	DefaultInterval = 2 * time.Second  // 默认间隔改为2秒
	ZoneInterval    = 2 * time.Second  // 0.zone 保持2秒
	MaxRetryWait    = 60 * time.Second // 最大重试等待时间
)

// APIRateLimit API 速率限制管理
type APIRateLimit struct {
	interval    time.Duration
	lastRequest time.Time
	retryCount  int
	mu          sync.Mutex
}

// 创建速率限制管理器
func newAPIRateLimit(interval time.Duration) *APIRateLimit {
	return &APIRateLimit{
		interval:    interval,
		lastRequest: time.Now(),
	}
}

// 等待并自适应调整间隔
func (r *APIRateLimit) wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 计算需要等待的时间
	elapsed := time.Since(r.lastRequest)
	if elapsed < r.interval {
		time.Sleep(r.interval - elapsed)
	}
	r.lastRequest = time.Now()
}

// 处理错误并调整间隔
func (r *APIRateLimit) handleError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查错误是否与速率限制或API错误相关
	if isRateLimitError(err) || isAPIError(err) {
		r.retryCount++
		// 指数退避：每次错误将间隔时间翻倍，并增加随机抖动
		backoff := time.Duration(1<<uint(r.retryCount)) * time.Second
		jitter := time.Duration(rand.Int63n(1000)) * time.Millisecond
		r.interval = backoff + jitter

		if r.interval > MaxRetryWait {
			r.interval = MaxRetryWait
		}

		// 立即等待一段时间
		time.Sleep(r.interval)
	}
}

// 检查是否为速率限制错误或API错误
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "请求太多") ||
		strings.Contains(errStr, "请求过于频繁") ||
		strings.Contains(errStr, "429")
}

// 检查是否为API错误
func isAPIError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "API错误") ||
		strings.Contains(errStr, "cannot unmarshal") ||
		strings.Contains(errStr, "稍后再试")
}

// SearchEngine 网络空间搜索引擎管理器
type SearchEngine struct {
	scanners    []Scanner
	rateLimits  map[string]*APIRateLimit
	rateLimitMu sync.RWMutex
}

// NewSearchEngine 创建新的搜索引擎管理器
func NewSearchEngine(scanners ...Scanner) *SearchEngine {
	rateLimits := make(map[string]*APIRateLimit)
	for _, scanner := range scanners {
		interval := DefaultInterval
		if scanner.Name() == "Zone" {
			interval = ZoneInterval
		}
		rateLimits[scanner.Name()] = newAPIRateLimit(interval)
	}

	return &SearchEngine{
		scanners:   scanners,
		rateLimits: rateLimits,
	}
}

// Search 使用所有可用的扫描器执行搜索
func (e *SearchEngine) Search(query string, page, size int) ([]model.Asset, error) {
	var results []model.Asset
	for _, scanner := range e.scanners {
		assets, err := scanner.Search(query, page, size)
		if err != nil {
			continue
		}
		results = append(results, assets...)
	}
	return results, nil
}

// SearchTargets 批量搜索目标
func (e *SearchEngine) SearchTargets(targets []Target, maxPage, pageSize int) ([]model.Asset, error) {
	var (
		results []model.Asset
		mu      sync.Mutex
		wg      sync.WaitGroup
		errs    []error
	)

	// 使用通道来控制并发数
	semaphore := make(chan struct{}, 1) // 限制为1个并发，保证顺序执行

	for _, target := range targets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() {
				<-semaphore // 释放信号量
			}()

			assets, err := e.searchSingle(t, maxPage, pageSize)
			mu.Lock()
			if err != nil {
				errs = append(errs, err)
			} else {
				results = append(results, assets...)
			}
			mu.Unlock()
		}(target)
	}

	wg.Wait()

	if len(errs) > 0 {
		return results, fmt.Errorf("部分搜索失败: %v", errs)
	}

	return results, nil
}

func buildQuery(scannerName string, target Target) string {
	switch scannerName {
	case "Hunter":
		if target.Type == "ip" {
			return fmt.Sprintf(`ip="%s"`, target.Value)
		}
		return fmt.Sprintf(`domain.suffix="%s"`, target.Value)
	case "FOFA":
		if target.Type == "ip" {
			return fmt.Sprintf(`ip="%s"`, target.Value)
		}
		return fmt.Sprintf(`domain="%s"`, target.Value)
	case "Quake":
		if target.Type == "ip" {
			return fmt.Sprintf(`ip:%s`, target.Value)
		}
		return fmt.Sprintf(`domain:%s`, target.Value)
	default:
		return ""
	}
}

// searchSingle 搜索单个目标
func (e *SearchEngine) searchSingle(target Target, maxPage, pageSize int) ([]model.Asset, error) {
	var results []model.Asset

	for _, scanner := range e.scanners {
		if scanner == nil {
			continue
		}

		query := buildQuery(scanner.Name(), target)
		fmt.Printf("使用 %s 搜索: %s\n", scanner.Name(), query)

		// 获取该扫描器的速率限制器
		e.rateLimitMu.RLock()
		rateLimit := e.rateLimits[scanner.Name()]
		e.rateLimitMu.RUnlock()

		for page := 1; page <= maxPage; page++ {
			fmt.Printf("搜索第 %d 页...\n", page)

			// 等待适当的时间间隔
			rateLimit.wait()

			assets, err := scanner.Search(query, page, pageSize)
			if err != nil {
				// 处理错误并调整速率
				rateLimit.handleError(err)
				fmt.Printf("查询出错: %v，已增加延迟至 %v\n", err, rateLimit.interval)
				if page == 1 {
					// 如果是第一页就失败，尝试继续其他扫描器
					break
				}
				// 如果不是第一页，认为已经获取了部分数据，结束当前扫描器的查询
				break
			}
			if len(assets) == 0 {
				break
			}
			results = append(results, assets...)
		}
	}

	return results, nil
}
