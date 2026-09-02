package clash

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"sync"

	"goclashz/core/logger"
	"goclashz/core/traffic"
)

// фактические порты рантайма: при конфликте с чужим ядром (FlClash/ClashX и т.п.) сдвигаемся на свободные
var effectivePorts struct {
	mu         sync.RWMutex
	mixedPort  int
	controller string
	secret     string
}

func init() {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		effectivePorts.secret = hex.EncodeToString(b)
	} else {
		effectivePorts.secret = fmt.Sprintf("mise-%d", len(b))
	}
	traffic.AuthSecret = APISecret
}

// APISecret — секрет внешнего контроллера текущего процесса
func APISecret() string {
	effectivePorts.mu.RLock()
	defer effectivePorts.mu.RUnlock()
	return effectivePorts.secret
}

// EffectiveProxyPort — реальный mixed-port ядра (0 = ещё не строили конфиг)
func EffectiveProxyPort() int {
	effectivePorts.mu.RLock()
	defer effectivePorts.mu.RUnlock()
	return effectivePorts.mixedPort
}

func portFree(host string, port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func pickFreePort(host string, want int) int {
	for p := want; p < want+30 && p < 65535; p++ {
		if portFree(host, p) {
			return p
		}
	}
	l, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return want
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// resolveRuntimePorts — порты для runtime-конфига. Пока наше ядро живо (live-reload),
// порты не трогаем; иначе занятый порт = чужой процесс → берём свободный.
func resolveRuntimePorts(wantMixed int, wantController string) (int, string) {
	effectivePorts.mu.Lock()
	defer effectivePorts.mu.Unlock()

	if IsRunning() && effectivePorts.mixedPort != 0 && effectivePorts.controller != "" {
		return effectivePorts.mixedPort, effectivePorts.controller
	}

	mixed := wantMixed
	if mixed != 0 && (!portFree("127.0.0.1", mixed) || !portFree("0.0.0.0", mixed)) {
		mixed = pickFreePort("127.0.0.1", wantMixed+1)
		logger.Warnf("порт прокси %d занят другим процессом, используем %d", wantMixed, mixed)
	}

	controller := NormalizeControllerHostPort(wantController)
	host, portStr, err := net.SplitHostPort(controller)
	if err == nil {
		port, _ := strconv.Atoi(portStr)
		probeHost := host
		if probeHost == "" || probeHost == "0.0.0.0" || probeHost == "::" {
			probeHost = "127.0.0.1"
		}
		if port != 0 && !portFree(probeHost, port) {
			np := pickFreePort(probeHost, port+1)
			logger.Warnf("порт API %d занят другим процессом, используем %d", port, np)
			controller = net.JoinHostPort(host, strconv.Itoa(np))
		}
	}

	effectivePorts.mixedPort = mixed
	effectivePorts.controller = controller
	return mixed, controller
}
