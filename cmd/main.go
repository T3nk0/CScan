package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cscan/internal/co"
	"cscan/internal/co/zone"
	"cscan/internal/common/banner"
	"cscan/internal/common/config"
	"cscan/internal/common/excel"
	"cscan/internal/common/model"
	"cscan/internal/cse"
	"cscan/internal/cse/fofa"
	"cscan/internal/cse/hunter"
	"cscan/internal/cse/quake"
)

// 版本信息
const (
	Version = "v1.0.0"
)

func main() {
	// 打印 banner
	banner.PrintBanner()

	// 检查配置文件是否存在
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		// 配置文件不存在，创建默认配置
		defaultConfig := &config.Config{
			HunterAPIKey: "your-hunter-key",
			FofaEmail:    "your-fofa-email",
			FofaAPIKey:   "your-fofa-key",
			QuakeAPIKey:  "your-quake-key",
			ZoneAPIKey:   "your-zone-key",
			MaxPage:      10,
			PageSize:     100,
		}

		if err := config.Save("config.json", defaultConfig); err != nil {
			fmt.Printf("创建配置文件失败: %v\n", err)
			fmt.Println("请手动创建 config.json 文件并填写以下内容:")
			fmt.Println(`{
    "hunter_api_key": "your-hunter-key",
    "fofa_email": "your-fofa-email",
    "fofa_api_key": "your-fofa-key",
    "quake_api_key": "your-quake-key",
    "zone_api_key": "your-zone-key",
    "max_page": 10,
    "page_size": 100
}`)
			os.Exit(1)
		}
		fmt.Println("已创建默认配置文件 config.json，请修改配置后重新运行程序")
		os.Exit(0)
	}

	// 定义命令行参数
	var (
		module     = flag.String("m", "", "模块选择 (cse/co)")
		filename   = flag.String("f", "target.txt", "输入文件路径 (txt格式)")
		outputFile = flag.String("o", "results.xlsx", "输出文件路径 (xlsx格式)")
		version    = flag.Bool("v", false, "显示版本信息")
	)

	// 自定义 Usage 信息
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: cscan -m <module> [options] [submodule]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -h\t\t显示帮助信息\n")
		fmt.Fprintf(os.Stderr, "  -m string\t模块选择 (必需参数)\n")
		fmt.Fprintf(os.Stderr, "    \t\tcse: 网络空间测绘引擎\n")
		fmt.Fprintf(os.Stderr, "    \t\tco:  公司情报搜索\n")
		fmt.Fprintf(os.Stderr, "  -f string\t输入文件路径 (默认: target.txt)\n")
		fmt.Fprintf(os.Stderr, "  -o string\t输出文件路径 (默认: results.xlsx)\n")
		fmt.Fprintf(os.Stderr, "  -v\t\t显示版本信息\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  cscan -m cse -f targets.txt -o results.xlsx\t运行所有网络空间测绘引擎\n")
		fmt.Fprintf(os.Stderr, "  cscan -m cse fofa -f targets.txt -o fofa.xlsx\t仅运行 Fofa 引擎\n")
		fmt.Fprintf(os.Stderr, "  cscan -m cse hunter -o hunter_results\t\t仅运行 Hunter 引擎\n")
		fmt.Fprintf(os.Stderr, "  cscan -m co -f companies.txt -o company_assets\t运行公司情报搜索\n")
	}

	// 创建一个新的 FlagSet 来处理子模块
	subflags := flag.NewFlagSet("submodule", flag.ExitOnError)
	subflags.StringVar(filename, "f", "target.txt", "输入文件路径 (txt格式)")
	subflags.StringVar(outputFile, "o", "results.xlsx", "输出文件路径 (xlsx格式)")

	// 首先解析主要参数
	flag.Parse()

	// 获取子模块名称（如果有）
	var submodule string
	args := flag.Args()
	if len(args) > 0 {
		// 检查第一个非标志参数是否是子模块
		validSubmodule := false
		switch *module {
		case "cse":
			if args[0] == "hunter" || args[0] == "fofa" || args[0] == "quake" {
				validSubmodule = true
			}
		case "co":
			if args[0] == "zone" {
				validSubmodule = true
			}
		}

		if validSubmodule {
			submodule = args[0]
			// 如果有子模块，解析剩余参数
			if len(args) > 1 {
				if err := subflags.Parse(args[1:]); err != nil {
					fmt.Printf("解析子模块参数错误: %v\n", err)
					os.Exit(1)
				}
			}
		}
	}

	// 检查版本参数
	if *version {
		fmt.Printf("CScan %s\n", Version)
		fmt.Println("网络空间资产搜索工具")
		os.Exit(0)
	}

	// 检查必需参数
	if *module == "" {
		fmt.Println("错误: 必须指定模块类型 (-m)")
		fmt.Println("可用模块: cse (网络空间测绘引擎) 或 co (公司情报)")
		flag.Usage()
		os.Exit(1)
	}

	// 检查并加载配置
	cfg, err := config.LoadOrCreate("config.json")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 检查配置是否完整
	if err := validateConfig(cfg, *module); err != nil {
		fmt.Printf("配置验证失败: %v\n", err)
		os.Exit(1)
	}

	// 检查输入文件
	if _, err := os.Stat(*filename); os.IsNotExist(err) {
		fmt.Printf("错误: 输入文件 %s 不存在\n", *filename)
		os.Exit(1)
	}

	// 检查输入文件后缀
	if !strings.HasSuffix(*filename, ".txt") {
		fmt.Println("错误: 输入文件必须是 .txt 格式")
		os.Exit(1)
	}

	// 处理输出文件名
	*outputFile = ensureXLSXExtension(*outputFile)

	// 根据模块类型初始化不同的扫描器
	switch *module {
	case "cse":
		var scanners []cse.Scanner
		switch submodule {
		case "":
			// 使用所有扫描器
			scanners = []cse.Scanner{
				hunter.NewScanner(cfg.HunterAPIKey),
				fofa.NewScanner(cfg.FofaEmail, cfg.FofaAPIKey),
				quake.NewScanner(cfg.QuakeAPIKey),
			}
		case "hunter":
			scanners = []cse.Scanner{hunter.NewScanner(cfg.HunterAPIKey)}
		case "fofa":
			scanners = []cse.Scanner{fofa.NewScanner(cfg.FofaEmail, cfg.FofaAPIKey)}
		case "quake":
			scanners = []cse.Scanner{quake.NewScanner(cfg.QuakeAPIKey)}
		default:
			fmt.Printf("未知的子模块: %s\n", submodule)
			fmt.Println("可用子模块: hunter, fofa, quake")
			os.Exit(1)
		}

		engine := cse.NewSearchEngine(scanners...)

		// 从文件读取目标
		targets, err := readTargets(*filename)
		if err != nil {
			fmt.Printf("读取目标失败: %v\n", err)
			return
		}
		fmt.Printf("开始处理 %d 个目标\n", len(targets))

		// 执行搜索
		results, err := engine.SearchTargets(targets, cfg.MaxPage, cfg.PageSize)
		if err != nil {
			fmt.Printf("搜索失败: %v\n", err)
			return
		}
		fmt.Printf("搜索完成，共获取到 %d 条结果\n", len(results))

		// 保存结果
		if err := saveResults(results, *outputFile); err != nil {
			fmt.Printf("保存结果失败: %v\n", err)
			return
		}
		fmt.Printf("结果已保存到 %s\n", *outputFile)

	case "co":
		switch submodule {
		case "", "zone":
			// 使用 zone scanner
			scanner := zone.NewScanner(cfg.ZoneAPIKey)
			companyScanner := co.NewCompanyScanner(scanner)

			// 从文件读取公司名称
			companies, err := readCompanies(*filename)
			if err != nil {
				fmt.Printf("读取公司名称失败: %v\n", err)
				return
			}

			// 执行搜索
			results, err := companyScanner.SearchCompanies(companies, cfg.MaxPage, cfg.PageSize)
			if err != nil {
				fmt.Printf("搜索失败: %v\n", err)
				return
			}

			// 保存结果
			if err := saveResults(results, *outputFile); err != nil {
				fmt.Printf("保存结果失败: %v\n", err)
				return
			}
			fmt.Printf("结果已保存到 %s\n", *outputFile)

		default:
			fmt.Printf("未知的子模块: %s\n", submodule)
			fmt.Println("可用子模块: zone")
			os.Exit(1)
		}

	default:
		fmt.Printf("未知的模块类型: %s\n", *module)
		fmt.Println("可用模块: cse (网络空间测绘引擎) 或 co (公司情报)")
		os.Exit(1)
	}
}

func readTargets(filename string) ([]cse.Target, error) {
	return excel.ReadTargets(filename)
}

func readCompanies(filename string) ([]string, error) {
	return excel.ReadCompanies(filename)
}

func saveResults(results []model.Asset, filename string) error {
	return excel.SaveResults(results, filename)
}

// ensureXLSXExtension 确保文件名以 .xlsx 结尾
func ensureXLSXExtension(filename string) string {
	// 如果没有扩展名，添加 .xlsx
	if !strings.Contains(filename, ".") {
		return filename + ".xlsx"
	}

	// 如果扩展名不是 .xlsx，替换为 .xlsx
	ext := filepath.Ext(filename)
	if ext != ".xlsx" {
		return strings.TrimSuffix(filename, ext) + ".xlsx"
	}

	return filename
}

// validateConfig 验证配置是否完整
func validateConfig(cfg *config.Config, moduleType string) error {
	if cfg.MaxPage <= 0 {
		return fmt.Errorf("max_page 必须大于 0")
	}
	if cfg.PageSize <= 0 {
		return fmt.Errorf("page_size 必须大于 0")
	}

	// 根据使用的模块检查对应的 API Key
	if moduleType == "cse" {
		if cfg.HunterAPIKey == "your-hunter-key" &&
			cfg.FofaAPIKey == "your-fofa-key" &&
			cfg.QuakeAPIKey == "your-quake-key" {
			return fmt.Errorf("请至少配置一个搜索引擎的 API Key")
		}
	} else if moduleType == "co" {
		if cfg.ZoneAPIKey == "your-zone-key" {
			return fmt.Errorf("请配置 Zone API Key")
		}
	}

	return nil
}
