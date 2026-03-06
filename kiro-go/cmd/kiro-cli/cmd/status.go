package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// formatDuration 将秒数格式化为人类可读的时间格式
func formatDuration(seconds interface{}) string {
	var secs int64
	switch v := seconds.(type) {
	case int:
		secs = int64(v)
	case int64:
		secs = v
	case float64:
		secs = int64(v)
	default:
		return fmt.Sprintf("%v 秒", seconds)
	}

	if secs < 60 {
		return fmt.Sprintf("%d 秒", secs)
	}

	minutes := secs / 60
	if minutes < 60 {
		return fmt.Sprintf("%d 分钟", minutes)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours < 24 {
		if remainingMinutes > 0 {
			return fmt.Sprintf("%d 小时 %d 分钟", hours, remainingMinutes)
		}
		return fmt.Sprintf("%d 小时", hours)
	}

	days := hours / 24
	remainingHours := hours % 24
	if remainingHours > 0 {
		return fmt.Sprintf("%d 天 %d 小时", days, remainingHours)
	}
	return fmt.Sprintf("%d 天", days)
}

var (
	statusRefresh bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看系统状态",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			color.Red("❌ %v", err)
			color.Yellow("💡 提示：运行 'kiro-cli config set password <密码>' 来设置密码")
			os.Exit(1)
		}

		// 如果启用了刷新标志，先刷新所有账号
		if statusRefresh {
			accounts, err := client.GetAccounts()
			if err == nil {
				color.Yellow("  🔄 正在刷新账号...")
				for _, a := range accounts {
					if id, ok := a["id"].(string); ok {
						client.RefreshAccount(id)
					}
				}
			}
		}

		status, err := client.GetStatus()
		if err != nil {
			color.Red("❌ 获取状态失败: %v", err)
			color.Yellow("💡 提示：检查 API 地址是否正确，或服务是否正在运行")
			os.Exit(1)
		}

		if viper.GetString("output") == "json" {
			json.NewEncoder(os.Stdout).Encode(status)
			return
		}

		fmt.Println()
		color.Cyan("╭─────────────────────────────────────╮")
		color.Cyan("│   📊 Kiro-Stack 系统状态            │")
		color.Cyan("╰─────────────────────────────────────╯")
		fmt.Println()

		color.Green("  ✓ 服务运行中")
		fmt.Println()

		fmt.Printf("  总请求数: %v\n", status["totalRequests"])
		fmt.Printf("  可用账号: %v / %v\n", status["available"], status["accounts"])

		if uptime, ok := status["uptime"]; ok {
			fmt.Printf("  运行时间: %s\n", formatDuration(uptime))
		}
		fmt.Println()

		// 显示快捷命令提示
		showQuickCommands()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVarP(&statusRefresh, "refresh", "r", true, "自动刷新所有账号（默认启用）")
}
