using System.Buffers.Binary;
using System.Diagnostics;
using System.IO.Pipes;
using System.Security.Principal;
using System.Text.Json;

namespace Misetanibox.Lite;

public sealed class BackendClient : IAsyncDisposable
{
    private readonly SemaphoreSlim gate = new(1);
    private NamedPipeClientStream? pipe;
    private Process? process;
    private long nextId;
    public async Task StartAsync()
    {
        string sid = WindowsIdentity.GetCurrent().User!.Value;
        process = Process.Start(new ProcessStartInfo(Path.Combine(AppContext.BaseDirectory, "Misetanibox.Backend.exe"), "--native-lite") { UseShellExecute = false, CreateNoWindow = true });
        pipe = new NamedPipeClientStream(".", "Misetanibox.Lite." + sid, PipeDirection.InOut, PipeOptions.Asynchronous, TokenImpersonationLevel.Identification);
        await pipe.ConnectAsync(15000);
        if (!GetNamedPipeServerProcessId(pipe.SafePipeHandle, out uint serverPid) || process is null || serverPid != process.Id)
        {
            pipe.Dispose();
            throw new IOException("Unexpected backend process");
        }
    }
    [System.Runtime.InteropServices.DllImport("kernel32.dll", SetLastError = true)]
    [return: System.Runtime.InteropServices.MarshalAs(System.Runtime.InteropServices.UnmanagedType.Bool)]
    private static extern bool GetNamedPipeServerProcessId(Microsoft.Win32.SafeHandles.SafePipeHandle pipe, out uint processId);
    public async Task<JsonElement> CallAsync(string method, object? parameters = null)
    {
        await gate.WaitAsync();
        bool exchangeComplete = false;
        try
        {
            using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(70));
            var ct = timeout.Token;
            if (pipe is null || !pipe.IsConnected) throw new IOException("Backend disconnected. Restart Lite.");
            string id = Interlocked.Increment(ref nextId).ToString();
            byte[] payload = JsonSerializer.SerializeToUtf8Bytes(new { version = 1, id, method, @params = parameters });
            if (payload.Length > 1048576) throw new IOException("Request too large");
            byte[] header = new byte[4]; BinaryPrimitives.WriteUInt32LittleEndian(header, (uint)payload.Length);
            await pipe.WriteAsync(header, ct); await pipe.WriteAsync(payload, ct); await pipe.FlushAsync(ct);
            JsonElement response = await ReadAsync(ct);
            if (response.GetProperty("version").GetInt32() != 1 || response.GetProperty("id").GetString() != id) throw new IOException("Protocol mismatch");
            // Each response is followed by one invalidation; snapshot polling is
            // authoritative even when the UI missed prior invalidations.
            var invalidation = await ReadAsync(ct);
            if (invalidation.GetProperty("version").GetInt32() != 1 || invalidation.GetProperty("event").GetString() != "snapshot-invalidated" || !invalidation.GetProperty("sequence").TryGetUInt64(out _)) throw new IOException("Protocol mismatch");
            exchangeComplete = true;
            if (response.TryGetProperty("error", out var error)) throw new IOException(error.GetProperty("message").GetString());
            return response.TryGetProperty("result", out var result) ? result.Clone() : default;
        }
        catch (Exception) when (!exchangeComplete)
        {
            // A partial frame/timeout loses stream alignment. Never issue another
            // mutation on this lease; EOF asks the backend to stop its runtime.
            pipe?.Dispose();
            pipe = null;
            throw new IOException("Backend communication failed. Restart Lite.");
        }
        finally { gate.Release(); }
    }
    private async Task<JsonElement> ReadAsync(CancellationToken ct)
    {
        byte[] header = new byte[4]; await pipe!.ReadExactlyAsync(header, ct);
        uint length = BinaryPrimitives.ReadUInt32LittleEndian(header);
        if (length == 0 || length > 1048576) throw new IOException("Invalid frame");
        byte[] bytes = new byte[(int)length]; await pipe.ReadExactlyAsync(bytes, ct);
        using var doc = JsonDocument.Parse(bytes); return doc.RootElement.Clone();
    }
    public async ValueTask DisposeAsync()
    {
        if (pipe?.IsConnected == true) { try { await CallAsync("shutdown"); } catch (IOException) { } }
        pipe?.Dispose(); process?.Dispose(); gate.Dispose();
    }
}
