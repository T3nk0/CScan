package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	HunterAPIKey string `json:"hunter_api_key"`
	FofaEmail    string `json:"fofa_email"`
	FofaAPIKey   string `json:"fofa_api_key"`
	ZoneAPIKey   string `json:"zone_api_key"`
	QuakeAPIKey  string `json:"quake_api_key"`
	MaxPage      int    `json:"max_page"`
	PageSize     int    `json:"page_size"`
}

// 默认配置
var defaultConfig = Config{
	HunterAPIKey: "your-hunter-key",
	FofaEmail:    "your-fofa-email",
	FofaAPIKey:   "your-fofa-key",
	ZoneAPIKey:   "your-zone-key",
	QuakeAPIKey:  "your-quake-key",
	MaxPage:      5,
	PageSize:     100,
}

// Load 加载配置文件，如果文件不存在则创建默认配置
func Load(path string) (*Config, error) {
	// 确保配置目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 创建默认配置文件
		if err := createDefaultConfig(path); err != nil {
			return nil, fmt.Errorf("创建默认配置文件失败: %v", err)
		}
		fmt.Printf("已创建默认配置文件: %s\n", path)
		fmt.Println("请修改配置文件中的API密钥后再运行程序")
		os.Exit(0)
	}

	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// createDefaultConfig 创建默认配置文件
func createDefaultConfig(path string) error {
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// validateConfig 验证配置是否有效
func validateConfig(cfg *Config) error {
	if cfg.MaxPage <= 0 {
		return fmt.Errorf("max_page 必须大于 0")
	}
	if cfg.PageSize <= 0 {
		return fmt.Errorf("page_size 必须大于 0")
	}

	// 检查是否使用了默认值
	if cfg.HunterAPIKey == defaultConfig.HunterAPIKey ||
		cfg.FofaEmail == defaultConfig.FofaEmail ||
		cfg.FofaAPIKey == defaultConfig.FofaAPIKey ||
		cfg.ZoneAPIKey == defaultConfig.ZoneAPIKey ||
		cfg.QuakeAPIKey == defaultConfig.QuakeAPIKey {
		return fmt.Errorf("请修改配置文件中的默认API密钥")
	}

	return nil
}

// LoadOrCreate 合并 Load 和配置文件创建逻辑
func LoadOrCreate(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := createDefaultConfig(path); err != nil {
			return nil, fmt.Errorf("创建配置文件失败: %v", err)
		}
		fmt.Printf("已创建默认配置文件: %s\n", path)
		fmt.Println("请修改配置文件中的API密钥后再运行程序")
		os.Exit(0)
	}

	return Load(path)
}

// Save 保存配置到文件
func Save(filename string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	// 使用安全的文件权限
	err = os.WriteFile(filename, data, 0600)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}
