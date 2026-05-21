package proxy

import (
	"testing"

	"kiro-api-proxy/config"
)

// Stage 7b codex review Critical 修复：DirectChannel.SellPrice 必须能被 billing 解析。
// 之前 billing 只查 v4 cfg.Channels，"direct:<id>" 形 channelID 会 fail-closed。

func directSellPriceTestSetup(t *testing.T) {
	t.Helper()
	t.Setenv("PIVOTSTACK_ENCRYPTION_KEY", "test-secret-key")
	oldDirect := config.GetDirectChannels()
	t.Cleanup(func() { _ = config.UpdateDirectChannels(oldDirect) })
	if err := config.UpdateDirectChannels(nil); err != nil {
		t.Fatalf("reset DirectChannels: %v", err)
	}
}

func TestDirectChannelSellPriceResolvesDefault(t *testing.T) {
	directSellPriceTestSetup(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{{
		ID: "billing-default", Type: "openai", Alias: "Bill Default", APIKeyEnc: "stub", Enabled: true,
		SellPrice: config.DirectSellPrice{
			Default: config.DirectSellPriceRow{InputPerM: 5.5, OutputPerM: 15.0},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	price, ok := resolveSellPriceForChannel("direct:billing-default", "gpt-anything")
	if !ok {
		t.Fatal("expected default sell price for direct channel")
	}
	if price.InputPerM != 5.5 || price.OutputPerM != 15.0 {
		t.Fatalf("price = %+v", price)
	}
}

func TestDirectChannelSellPricePerModelOverride(t *testing.T) {
	directSellPriceTestSetup(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{{
		ID: "billing-permodel", Type: "openai", Alias: "Bill PerModel", APIKeyEnc: "stub", Enabled: true,
		SellPrice: config.DirectSellPrice{
			Default: config.DirectSellPriceRow{InputPerM: 1.0, OutputPerM: 2.0},
			Models: map[string]config.DirectSellPriceRow{
				"gpt-4": {InputPerM: 30.0, OutputPerM: 60.0},
			},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	price, ok := resolveSellPriceForChannel("direct:billing-permodel", "gpt-4")
	if !ok {
		t.Fatal("expected per-model override")
	}
	if price.InputPerM != 30.0 || price.OutputPerM != 60.0 {
		t.Fatalf("per-model price = %+v", price)
	}
}

func TestDirectChannelSellPriceMissingFailsClosed(t *testing.T) {
	directSellPriceTestSetup(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{{
		ID: "billing-zero", Type: "openai", Alias: "Bill Zero", APIKeyEnc: "stub", Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolveSellPriceForChannel("direct:billing-zero", "gpt-x"); ok {
		t.Fatal("zero sell price should fail closed, not silent zero")
	}
}

func TestDirectChannelSellPriceSkippedWhenDeletedOrDisabled(t *testing.T) {
	directSellPriceTestSetup(t)
	if err := config.UpdateDirectChannels([]config.DirectChannel{
		{ID: "billing-disabled", Type: "openai", Alias: "Disabled", APIKeyEnc: "stub", Enabled: false,
			SellPrice: config.DirectSellPrice{Default: config.DirectSellPriceRow{InputPerM: 1, OutputPerM: 2}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolveSellPriceForChannel("direct:billing-disabled", "gpt-x"); ok {
		t.Fatal("disabled direct channel should not provide price")
	}
}

func TestResolveSellPriceFallsThroughToGlobal(t *testing.T) {
	directSellPriceTestSetup(t)
	// 不在 direct: 命名空间下，应走 config.GetSellPriceForChannel。
	// 因为我们没设全局售价，应返 false（fail-closed）。
	if _, ok := resolveSellPriceForChannel("ch-v4-legacy", "no-such-model-xyz"); ok {
		t.Fatal("non-direct channel without global price should fail closed")
	}
	// "direct:" 前缀但渠道不存在 → 也应 fail closed，不应走 fallback 命中其它 price。
	if _, ok := resolveSellPriceForChannel("direct:nonexistent", "gpt-x"); ok {
		t.Fatal("missing direct channel must fail closed")
	}
}
