package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	Run: func(cmd *cobra.Command, args []string) {
		color.Cyan("=== PivotStack CLI 配置 ===\n")
		fmt.Printf("API 地址: %s\n", viper.GetString("api_url"))
		fmt.Printf("输出格式: %s\n", viper.GetString("output"))

		if viper.GetString("password") != "" {
			fmt.Printf("密码: %s\n", "********")
		} else {
			color.Yellow("密码: 未设置")
		}

		if viper.ConfigFileUsed() != "" {
			fmt.Printf("\n配置文件: %s\n", viper.ConfigFileUsed())
		} else {
			color.Yellow("\n配置文件: 未找到 (使用默认值)")
		}
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置项",
	Long:  `设置配置项。可用的 key: api_url, password, output`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		validKeys := map[string]bool{
			"api_url":  true,
			"password": true,
			"output":   true,
		}

		if !validKeys[key] {
			color.Red("错误: 无效的配置项 '%s'", key)
			fmt.Println("可用的配置项: api_url, password, output")
			os.Exit(1)
		}

		viper.Set(key, value)

		home, err := os.UserHomeDir()
		if err != nil {
			color.Red("错误: %v", err)
			os.Exit(1)
		}

		configPath := home + "/.kiro-cli.yaml"
		if err := viper.WriteConfigAs(configPath); err != nil {
			color.Red("保存配置失败: %v", err)
			os.Exit(1)
		}

		// 设置文件权限为 600（仅所有者可读写）
		os.Chmod(configPath, 0600)

		color.Green("✓ 配置已保存: %s = %s", key, value)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}
