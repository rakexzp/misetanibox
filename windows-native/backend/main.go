//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Microsoft/go-winio"
	"goclashz/core/appcore"
	"goclashz/core/clash"
	"goclashz/windows-native/ipc"
	"golang.org/x/sys/windows"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"
)

type sink struct{ sequence atomic.Uint64 }

func (s *sink) Emit(_ string, _ ...any) { s.sequence.Add(1) }

func main() {
	native := false
	for _, arg := range os.Args[1:] {
		if arg == "--native-lite" {
			native = true
		}
	}
	if !native {
		fmt.Fprintln(os.Stderr, "--native-lite required")
		return
	}
	// Keep this non-inheritable handle open until process termination; closing
	// it explicitly would terminate this process before normal cleanup runs.
	job, err := ownProcessTree()
	if err != nil {
		fmt.Fprintln(os.Stderr, "process ownership unavailable")
		return
	}
	_ = job
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return
	}
	sid := user.User.Sid.String()
	ln, err := winio.ListenPipe(`\\.\pipe\Misetanibox.Lite.`+sid, &winio.PipeConfig{SecurityDescriptor: "D:P(D;;GA;;;NU)(A;;GA;;;SY)(A;;GA;;;" + sid + ")", InputBufferSize: 65536, OutputBufferSize: 65536})
	if err != nil {
		fmt.Fprintln(os.Stderr, "native endpoint unavailable")
		return
	}
	defer ln.Close()
	if err := clash.LoadIndex(); err != nil {
		fmt.Fprintln(os.Stderr, "profile index invalid")
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	events := &sink{}
	controller := appcore.NewController(appcore.Options{Events: events, Version: "native-dev"})
	controller.Bootstrap(ctx, appcore.BootstrapOptions{NativeLite: true})
	service := appcore.NewLiteService(controller)
	defer func() {
		controller.Supervisor.Stop()
		cleanup, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		_ = service.Stop(cleanup)
	}()
	go func() { <-ctx.Done(); ln.Close() }()
	// Exactly one UI lease. A broken/idle connection stops the owned runtime and
	// exits the sidecar, rather than accepting a second writer or leaving a core.
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	if err := verifyPeer(conn, sid); err != nil {
		return
	}
	for {
		conn.SetDeadline(time.Now().Add(90 * time.Second))
		var req ipc.Request
		if err := ipc.ReadFrame(conn, &req); err != nil {
			return
		}
		if err := req.Validate(); err != nil {
			return
		}
		requestCtx, stop := context.WithTimeout(ctx, 60*time.Second)
		result, err := dispatch(requestCtx, service, req)
		stop()
		response := ipc.Response{Version: ipc.Version, ID: req.ID, Result: result}
		if err != nil {
			response.Result = nil
			response.Error = &ipc.Error{Code: "operation_failed", Message: "Operation failed; check the profile, core assets and connection."}
			if errors.Is(err, appcore.ErrNativeProxyOwnershipUnavailable) {
				response.Error = &ipc.Error{Code: "system_proxy_ownership_not_ready", Message: "Connect is disabled: safe preservation and restoration of Windows proxy settings is not implemented."}
			}
		}
		if err := ipc.WriteFrame(conn, response); err != nil {
			return
		}
		if err := ipc.WriteFrame(conn, ipc.Event{Version: ipc.Version, Sequence: events.sequence.Add(1), Event: "snapshot-invalidated"}); err != nil {
			return
		}
		if req.Method == "shutdown" {
			return
		}
	}
}

func dispatch(ctx context.Context, s *appcore.LiteService, req ipc.Request) (any, error) {
	var p struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		URL       string `json:"url"`
		Domain    string `json:"domain"`
		Path      string `json:"path"`
		ProfileID string `json:"profileId"`
		Group     string `json:"group"`
		Convert   bool   `json:"convert"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
	}
	switch req.Method {
	case "snapshot":
		return s.Snapshot(), nil
	case "profiles.list":
		return s.Profiles(), nil
	case "profiles.addURL":
		return nil, s.AddURL(ctx, p.Name, p.URL, p.Convert)
	case "profiles.addDNS":
		return nil, s.AddDNS(ctx, p.Domain, p.Name)
	case "profiles.addLocal":
		return s.AddLocal(p.Path, p.Name)
	case "profiles.select":
		return nil, s.SelectProfile(ctx, p.ID)
	case "profiles.refresh":
		return nil, s.Refresh(ctx, p.ID)
	case "profiles.delete":
		return nil, s.Delete(ctx, p.ID)
	case "servers.list":
		return s.Servers()
	case "servers.select":
		return nil, s.SelectServer(ctx, p.ProfileID, p.Group, p.Name)
	case "servers.ping":
		return s.Ping(ctx, p.ProfileID, p.Name)
	case "connect":
		return nil, s.Connect(ctx)
	case "stop", "shutdown":
		return nil, s.Stop(ctx)
	default:
		return nil, errors.New("unknown_method")
	}
}

var _ net.Conn
