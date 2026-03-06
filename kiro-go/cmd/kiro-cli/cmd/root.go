package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"kiro-api-proxy/cmd/kiro-cli/client"
)

var (
	cfgFile  string
	apiURL   string
	password string
	output   string
)

var rootCmd = &cobra.Command{
	Use:   "kiro-cli",
	Short: "Kiro Stack 命令行管理工具",
	Long:  `Kiro Stack CLI - 通过命令行管理账号池、查看状态、执行常用操作`,
	Run: func(cmd *cobra.Command, args []string) {
		showMenu()
	},
}

func showMenu() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          🚀 Kiro Stack CLI - 功能面板                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("📊 系统管理")
	fmt.Println("  kiro-cli status                    查看系统状态")
	fmt.Println()
	fmt.Println("👥 账号管理")
	fmt.Println("  kiro-cli account list              列出所有账号")
	fmt.Println("  kiro-cli account refresh <id>      刷新账号配额")
	fmt.Println("  kiro-cli account enable <id>       启用账号")
	fmt.Println("  kiro-cli account disable <id>      禁用账号")
	fmt.Println("  kiro-cli account delete <id>       删除账号")
	fmt.Println()
	fmt.Println("⚙️  配置管理")
	fmt.Println("  kiro-cli config show               显示当前配置")
	fmt.Println("  kiro-cli config set <key> <value>  设置配置项")
	fmt.Println()
	fmt.Println("🔧 全局选项")
	fmt.Println("  --output json                      JSON 格式输出")
	fmt.Println("  --api-url <url>                    指定 API 地址")
	fmt.Println("  --password <pwd>                   临时指定密码")
	fmt.Println()
	fmt.Println("💡 提示")
	fmt.Println("  使用 'kiro-cli <命令> --help' 查看详细帮助")
	fmt.Println("  使用 'kiro-cli completion bash|zsh|powershell' 生成自动补全")
	fmt.Println()
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认: $HOME/.kiro-cli.yaml)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "API 地址 (默认: http://localhost:8088)")
	rootCmd.PersistentFlags().StringVar(&password, "password", "", "管理员密码")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "输出格式: table|json")

	viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".kiro-cli")
	}

	viper.SetDefault("api_url", "http://localhost:8088")
	viper.SetDefault("output", "table")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// 配置文件加载成功，静默处理
	}
}

func getClient() (*client.Client, error) {
	apiURL := viper.GetString("api_url")
	password := viper.GetString("password")

	if password == "" {
		return nil, fmt.Errorf("密码未设置，请使用 --password 或运行 'kiro-cli config set password <密码>'")
	}

	return client.New(apiURL, password), nil
}

// showQuickCommands 显示常用命令提示
func showQuickCommands() {
	fmt.Println("  ────────────────────────────────────")
	fmt.Println("  💡 常用命令：")
	fmt.Println("     kiro-cli status              查看系统状态")
	fmt.Println("     kiro-cli account list        列出所有账号")
	fmt.Println("     kiro-cli config show         显示配置")
	fmt.Println("     kiro-cli --help              查看所有命令")
	fmt.Println()
}
