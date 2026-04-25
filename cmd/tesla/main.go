// 命令 tesla 是 Tesla Fleet API 的命令行入口。
//
// 实际逻辑全部在 internal/cli 中,本文件保持极简,
// 以便 ldflags 注入版本元数据时无需了解内部结构。
package main

import "github.com/wmango/tesla-cli/internal/cli"

func main() {
	cli.Execute()
}
