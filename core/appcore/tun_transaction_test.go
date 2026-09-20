package appcore

import (
	"errors"
	"goclashz/core/clash"
	"strings"
	"testing"
)

func TestTunTransactionRollback(t *testing.T) {
	old := clash.GetDefaultTunConfig()
	next := old
	next.Stack = "mips"
	for _, recoveryFails := range []bool{false, true} {
		saved := old
		calls := 0
		err := applyTunTransaction(&old, &next, func(c *clash.TunConfig) error { saved = *c; return nil }, func() error {
			calls++
			if calls == 1 || recoveryFails {
				return errors.New("runtime failed")
			}
			return nil
		})
		if err == nil || saved.Stack != "gvisor" || calls != 2 {
			t.Fatalf("rollback: %v %+v %d", err, saved, calls)
		}
		if recoveryFails && !strings.Contains(err.Error(), "восстанов") {
			t.Fatalf("recovery error hidden: %v", err)
		}
	}
}

func TestTunTransactionRestoreWriteFailure(t *testing.T) {
	cfg := clash.GetDefaultTunConfig()
	writes, restarts := 0, 0
	err := applyTunTransaction(&cfg, &cfg, func(*clash.TunConfig) error {
		writes++
		if writes == 2 {
			return errors.New("disk failure")
		}
		return nil
	}, func() error { restarts++; return errors.New("runtime failure") })
	if err == nil || !strings.Contains(err.Error(), "disk failure") || restarts != 1 {
		t.Fatalf("restore failure not surfaced: %v, restarts %d", err, restarts)
	}
}

func TestTunTransactionSaveFailureDoesNotRestart(t *testing.T) {
	cfg := clash.GetDefaultTunConfig()
	err := applyTunTransaction(&cfg, &cfg, func(*clash.TunConfig) error { return errors.New("invalid") }, func() error { t.Fatal("stopped before validation"); return nil })
	if err == nil {
		t.Fatal("save failure swallowed")
	}
}
