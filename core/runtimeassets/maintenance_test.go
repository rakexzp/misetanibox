package runtimeassets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPendingTransactionBlocksHealth(t *testing.T) {
	previous := bundledCoreMaintenance.Load()
	defer bundledCoreMaintenance.Store(previous)
	EnableBundledCoreMaintenance()
	target := filepath.Join(t.TempDir(), "clash.exe")
	if e := os.WriteFile(target+".seed-transaction.json", []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	h := checkCoreByPath(context.Background(), target)
	if h.Ready || h.Error != "Замена ядра не завершена; автозапуск отложен до восстановления" {
		t.Fatalf("%+v", h)
	}
}
