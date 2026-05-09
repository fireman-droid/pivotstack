package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// formatQuotaWithBar 格式化配额并添加进度条
func formatQuotaWithBar(current, limit float64) string {
	if limit == 0 {
		return "N/A"
	}

	percent := current / limit
	barLength := 8
	filled := int(percent * float64(barLength))
	if filled > barLength {
		filled = barLength
	}

	bar := strings.Repeat("■", filled) + strings.Repeat("□", barLength-filled)
	return fmt.Sprintf("%.0f/%.0f [%s] %.0f%%", current, limit, bar, percent*100)
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "账号管理",
	Long:  `管理 PivotStack 账号池：列表、刷新、启用、禁用、删除`,
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有账号",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			color.Red("❌ %v", err)
			color.Yellow("💡 提示：运行 'kiro-cli config set password <密码>' 来设置密码")
			os.Exit(1)
		}

		accounts, err := client.GetAccounts()
		if err != nil {
			color.Red("❌ 获取账号列表失败: %v", err)
			color.Yellow("💡 提示：检查 API 地址是否正确，或服务是否正在运行")
			os.Exit(1)
		}

		if viper.GetString("output") == "json" {
			json.NewEncoder(os.Stdout).Encode(accounts)
			return
		}

		fmt.Println()
		// 表格输出
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("Email", "状态", "试用配额", "订阅")

		for _, a := range accounts {
			email := getString(a, "email")
			enabled := getBool(a, "enabled")
			banStatus := getString(a, "banStatus")
			trialCurrent := getFloat(a, "trialUsageCurrent")
			trialLimit := getFloat(a, "trialUsageLimit")
			subType := getString(a, "subscriptionType")

			status := "✓ 运行中"
			if !enabled {
				status = "✗ 已禁用"
			} else if banStatus != "" && banStatus != "ACTIVE" {
				status = "⚠ 已封禁"
			}

			trialQuota := formatQuotaWithBar(trialCurrent, trialLimit)

			table.Append([]string{email, status, trialQuota, subType})
		}

		table.Render()
		fmt.Println()
		color.Cyan("  共 %d 个账号", len(accounts))
		fmt.Println()

		// 显示快捷命令提示
		showQuickCommands()
	},
}

var accountRefreshCmd = &cobra.Command{
	Use:   "refresh <id>",
	Short: "刷新账号配额",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			color.Red("错误: %v", err)
			os.Exit(1)
		}

		id := args[0]
		if err := client.RefreshAccount(id); err != nil {
			color.Red("刷新失败: %v", err)
			os.Exit(1)
		}

		color.Green("✓ 账号 %s 刷新成功", id)
	},
}

var accountEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "启用账号",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			color.Red("错误: %v", err)
			os.Exit(1)
		}

		id := args[0]
		if err := client.UpdateAccount(id, map[string]interface{}{"enabled": true}); err != nil {
			color.Red("启用失败: %v", err)
			os.Exit(1)
		}

		color.Green("✓ 账号 %s 已启用", id)
	},
}

var accountDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "禁用账号",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			color.Red("错误: %v", err)
			os.Exit(1)
		}

		id := args[0]
		if err := client.UpdateAccount(id, map[string]interface{}{"enabled": false}); err != nil {
			color.Red("禁用失败: %v", err)
			os.Exit(1)
		}

		color.Yellow("✓ 账号 %s 已禁用", id)
	},
}

var accountDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "删除账号",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			color.Red("错误: %v", err)
			os.Exit(1)
		}

		id := args[0]
		fmt.Printf("确定删除账号 %s 吗？此操作不可恢复。[y/N]: ", id)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("已取消")
			return
		}

		if err := client.DeleteAccount(id); err != nil {
			color.Red("删除失败: %v", err)
			os.Exit(1)
		}

		color.Green("✓ 账号 %s 已删除", id)
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountRefreshCmd)
	accountCmd.AddCommand(accountEnableCmd)
	accountCmd.AddCommand(accountDisableCmd)
	accountCmd.AddCommand(accountDeleteCmd)
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return 0
}
