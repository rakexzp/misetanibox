# Native Windows Lite — experimental, not release-ready

`backend` is a real appcore sidecar. `Misetanibox.Lite` is an unpackaged x64
WinUI 3/.NET 8 project (no WebView). Windows App SDK 1.8.260804001 is pinned from
the official NuGet flat-container metadata:
https://api.nuget.org/v3-flatcontainer/microsoft.windowsappsdk/index.json .
The manual `Windows Native Prototype` workflow restores/builds C# on Windows and
runs isolated Go tests. There is no C# test project or interactive UI acceptance yet.

Build on Windows with Go 1.25 and .NET 8 SDK:
`powershell -File windows-native/build-windows.ps1`.
Download the workflow artifact `Misetanibox-Lite-Windows-x64-prototype`, extract
both the artifact and its inner ZIP completely, then launch `Misetanibox.Lite.exe`
without elevation on Windows 10 1809+ x64. Keep all adjacent files: the folder
contains the backend, self-contained .NET and Windows App SDK dependencies.
No installation/elevation occurs. Core assets are NOT bundled or installed.
The existing helper is compiled into `helper-not-enabled` only; do not install or
run it. It is not trusted for native TUN until the security gates below are met.
This archive is an app-only prototype: connect, ping and TUN do not work.

## Ownership and lifecycle

UI starts backend with `--native-lite`; this flag selects a fixed independent
user cache directory `Misetanibox.Lite`, ignoring legacy data-dir overrides.
Native-only Windows data directory locking happens during utils initialization,
before directory writes/config reads. Legacy migration/core fallback paths are
not exposed in native mode. Wails does not acquire this new lock, preserving its
existing second-launch activation path (still needs Windows regression testing).
Sharing a data directory with Wails is not supported. Isolation of data does not
isolate machine/user proxy settings or fixed core API/listener ports; running both
clients at once is not supported.

Native bootstrap skips legacy cleanup, updater, startup-task checks and automatic
restore/delay tests. One UI pipe connection owns the backend. EOF, shutdown or
90 seconds idle ends the backend and stops its runtime; UI polls every 5 seconds.
A kill-on-close Windows Job owns backend and directly spawned cores; job setup
failure blocks startup. Before enabling native connect, persistent WinINet proxy
restoration after hard backend termination needs a guardian/recovery path. The
current fail-closed native runtime never writes proxy settings.
Close-to-tray is implemented through Shell_NotifyIcon; double click restores,
right click exits. If tray creation fails, closing performs shutdown instead.
Explorer restart recovery and tray accessibility/menu remain follow-up work.

## JSON contract v1

Pipe: `\\.\pipe\Misetanibox.Lite.<current SID>`. Protected DACL allows the current
user/SYSTEM and denies network logons. Server checks client process token SID.
Client checks the pipe server PID against its launched backend process before
sending requests. No universal command execution or privileged endpoint.

Frames: uint32 little-endian UTF-8 JSON byte length, then payload (1..1048576).
Request: `{version:1,id:string,method:string,params?:object}`. IDs 1..128 bytes,
methods 1..64 bytes. Each request gets a response followed by exactly one
invalidation `{version:1,sequence:uint64,event:"snapshot-invalidated"}`. Events
coalesce appcore changes; they are not pushed while a request is idle. Client
polling/resnapshot is authoritative. Response is `{version:1,id,result?:any}` or
`{version:1,id,error:{code:"operation_failed",message:sanitizedString}}`.
Request context deadline is 60 seconds; cancellation is cooperative, not a hard
execution bound. Dispatch remains synchronous so timed-out operations cannot
continue alongside subsequent mutations. Client timeout is 70 seconds; any
incomplete exchange closes the lease instead of reusing a misaligned stream.
A context-ignoring operation can still delay EOF handling and runtime cleanup.

Methods/params:
- `snapshot`, `profiles.list`, `servers.list`: no params.
- `profiles.addURL`: `{name,url,convert?:bool}`; `profiles.addDNS`: `{name,domain}`.
- `profiles.addLocal`: `{name,path}` returns imported ID (native picker supplies path).
- `profiles.select`, `profiles.refresh`, `profiles.delete`: `{id}`.
- `servers.select`: `{profileId,group,name}`; `servers.ping`: `{profileId,name}` returns milliseconds.
- `connect` and `servers.ping`: fail closed with `system_proxy_ownership_not_ready`.
  The existing Windows guard only records an endpoint and disables it (using a
  substring match); it cannot restore prior WinINet/RAS/PAC/bypass settings.
  Native mode therefore never acquires or clears a system proxy, including idle
  supervisor cleanup and core-exit callbacks. Foreign proxy settings are left
  alone. No native core is started through connect/ping/supervisor activation,
  avoiding the shared listener/API ports until independent ports are implemented.
  This is a safe incomplete prototype, not a functioning VPN client.
- `stop`, `shutdown`: stop the process-owned runtime without clearing foreign proxy settings.

Snapshot: `{profiles:Profile[],activeProfile:string,desired:bool,running:bool,
systemProxy:bool,tunAvailable:false,tunReason:"helper_security_not_ready",
requiredCoreVersion:"v1.19.31"}`. Profile: `{id,name,type,updated,upload,download,
total,expire}`. Server topology: `{profileId,selector,groups,nodes}`; group:
`{name,type,members:string[],providers:string[],selected:string}`; node:
`{name,type}`. No URL/header/proxy secret in normal DTOs. Static provider groups
are preserved, but dynamic provider members are not selectable yet.

## Release gates still open

- Harden existing helper server: arbitrary caller executable/args/replacement
  remain. Native TUN is explicitly unavailable; no install endpoint is exposed.
- Protect/verify registered core and DLL assets and implement lease/job cleanup.
- Require verified official mihomo v1.19.31 assets before connection/mips gating.
  `requiredCoreVersion` currently declares a target, not runtime enforcement.
  No user core was changed. mips is optional mihomo IP stack, not architecture or
  Wintun replacement: https://github.com/MetaCubeX/mihomo/releases/tag/v1.19.31 .
- Settings/theme/startup, explicit probe, pending-state DTO, cancellation/reconnect,
  typed C# DTO/MVVM separation, C# tests and Windows build/runtime acceptance.
- UI is currently a functional single scrolling surface, not the final four
  polished Lite surfaces. Do not distribute it as a finished client.

Available checks: `go test -race ./core/appcore ./core/clash ./core/instance
./windows-native/ipc`, Windows backend/desktop cross-build. Windows host required
for WinUI build, named-pipe ACL/peer/lock/tray behavior and system proxy/TUN tests.
