package banner

import "fmt"

const banner = `
    ___    ___                    
  / ___/ / ___/ _____ ____ _ ____ 
 / /     \__ \ / ___// __ '// __ \
/ /___  ___/ // /__ / /_/ // / / /
\ __ / / __ / \___/ \__,_//_/ /_/ v1.0.1
                                    
     CScan - 网络空间资产搜索工具 By T3nk0 [Tools.com专版]              
===========================================
`

// PrintBanner 打印程序 banner
func PrintBanner() {
	fmt.Println(banner)
}
