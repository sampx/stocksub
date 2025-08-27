package tencent

import (
	"testing"

	"stocksub/pkg/provider/core"
)

// TestTencentProvider_ImplementsCoreProvider ensures that Provider implements the core provider interfaces.
func TestTencentProvider_ImplementsCoreProvider(t *testing.T) {
	var _ core.RealtimeStockProvider = (*Provider)(nil)
	var _ core.Provider = (*Provider)(nil)
	var _ core.Closable = (*Provider)(nil)
}
