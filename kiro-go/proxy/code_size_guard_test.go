package proxy

// 文件/函数体大小自检测试。v6 plan §0 硬约束：
//   - production .go 文件 ≤ 500 行
//   - 函数体 ≤ 80 行
//
// 由于 v5 历史代码里仍有 33 个函数 >80 行（handleClaudeStream / OpenAIToKiro 等），
// 函数体测试用「已知违规白名单」模式：当前违规列表是 baseline，新增违规 → fail。
// 历史违规逐步重构后从白名单移除即可。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 历史已知 >80 行的函数（不强制立即重构；新违规会 fail，重构后从白名单移除）。
var preexistingLongFuncs = map[string]bool{
	"handler_claude_stream.go::handleClaudeStream":                  true,
	"handler_openai_stream.go::handleOpenAIStream":                  true,
	"db_import.go::apiImportFromDB":                                 true,
	"handler_admin_auth_import.go::apiImportCredentialsBatch":       true,
	"handler_claude_messages.go::handleClaudeMessagesInternal":      true,
	"translator_openai_to_kiro.go::OpenAIToKiro":                    true,
	"kiro_call.go::CallKiroAPI":                                     true,
	"handler_openai_chat.go::handleOpenAIChat":                      true,
	"handler_channel_newapi.go::handleNewAPIChannelRequest":         true,
	"translator_claude_to_kiro.go::ClaudeToKiro":                    true,
	"handler_insights_freeloaders.go::apiInsightsFreeloaders":       true,
	"handler_admin_accounts_shared.go::apiBatchAccounts":            true,
	"handler_admin_providers.go::apiUpdateProvider":                 true,
	"handler_admin_migrate.go::apiMigrateProviderManualChannels":    true,
	"handler_admin_auth_import.go::apiImportCredentials":            true,
	"handler_stats_types.go::Predict":                               true,
	"handler_channel.go::handleChannelRequest":                      true,
	"kiro_stream.go::parseEventStream":                              true,
	"handler_claude_nonstream.go::handleClaudeNonStream":            true,
	"handler_user_redeem.go::handleUserRedeem":                      true,
	"handler_admin_apikeys_shared.go::apiUpdateApiKey":              true,
	"handler_export.go::apiExportAccounts":                          true,
	"kiro_api.go::RefreshAccountInfo":                               true,
	"handler_insights_whales.go::apiInsightsWhales":                 true,
	"handler_openai_nonstream.go::handleOpenAINonStream":            true,
	"kiro_sanitize_history.go::validateAndCleanToolPairingHistory":  true,
	"handler_insights_daily.go::apiInsightsDaily":                   true,
	"handler_stats_calllog.go::addCallLogWithKey":                   true,
	"handler_stats_logfile.go::loadLogsFromDisk":                    true,
	"handler_admin_providers.go::apiCreateProvider":                 true,
	"handler_serve.go::ServeHTTP":                                   true,
	"kiro_sanitize_payload.go::sanitizeJSONSchema":                  true,
	"newapi_reconcile_match.go::applyUpstreamReconcile":             true,
	"handler_admin_newapi_channels.go::apiPatchNewAPIChannel":       true,
}

func TestNoGoFileOver500Lines(t *testing.T) {
	root := proxyDir(t)
	maxLines := 500
	var violations []string
	walk := filepath.WalkDir
	_ = walk(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if strings.HasSuffix(name, "_test.go") {
			return nil // 测试文件不强制 ≤500
		}
		if strings.Contains(name, ".bak") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lc := strings.Count(string(data), "\n") + 1
		if lc > maxLines {
			violations = append(violations, name+" ("+itoa(lc)+" lines)")
		}
		return nil
	})
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Errorf("production .go files over %d lines:\n  %s",
			maxLines, strings.Join(violations, "\n  "))
	}
}

func TestFunctionBodiesUnder80Lines(t *testing.T) {
	root := proxyDir(t)
	maxBody := 80
	var newViolations []string
	walk := filepath.WalkDir
	_ = walk(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		if strings.Contains(name, ".bak") {
			return nil
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Logf("parse %s: %v (skipped)", path, perr)
			return nil
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			startLine := fset.Position(fn.Body.Lbrace).Line
			endLine := fset.Position(fn.Body.Rbrace).Line
			body := endLine - startLine - 1
			if body <= maxBody {
				continue
			}
			key := name + "::" + fn.Name.Name
			if preexistingLongFuncs[key] {
				continue // 在白名单里，允许
			}
			newViolations = append(newViolations, key+" ("+itoa(body)+" lines)")
		}
		return nil
	})
	sort.Strings(newViolations)
	if len(newViolations) > 0 {
		t.Errorf("new function bodies over %d lines (refactor or add to preexistingLongFuncs):\n  %s",
			maxBody, strings.Join(newViolations, "\n  "))
	}
}

func proxyDir(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
