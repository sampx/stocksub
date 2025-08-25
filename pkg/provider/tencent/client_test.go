package tencent

import (
	"testing"

	"stocksub/pkg/subscriber"
)

// TestTencentProvider_ImplementsSubscriberProvider ensures that Provider implements the subscriber.Provider interface.
func TestTencentProvider_ImplementsSubscriberProvider(t *testing.T) {
	var _ subscriber.Provider = (*Provider)(nil)
}
