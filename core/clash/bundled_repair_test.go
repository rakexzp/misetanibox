package clash

import (
	"context"
	"strings"
	"testing"

	"goclashz/core/runtimeassets"
)

func TestExplicitRepairPreservesSmart(t *testing.T) {
	called := false
	_, err := repairBundledCoreLocked(context.Background(), true, func(context.Context, string, func(string) error) (runtimeassets.StockSyncResult, error) {
		called = true
		return runtimeassets.StockSyncResult{Updated: true}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "Smart") || called {
		t.Fatalf("Smart repair: called=%v err=%v", called, err)
	}
}
func TestExplicitRepairDoesNotReportSkippedSuccess(t *testing.T) {
	_, err := repairBundledCoreLocked(context.Background(), false, func(context.Context, string, func(string) error) (runtimeassets.StockSyncResult, error) {
		return runtimeassets.StockSyncResult{Message: "skipped"}, nil
	})
	if err == nil {
		t.Fatal("skipped repair reported success")
	}
}
