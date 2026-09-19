using System.Diagnostics;
using System.Text;

namespace Misetanibox.Lite;

internal static class StartupDiagnostics
{
    private const long MaxLogBytes = 128 * 1024;
    private static readonly object sync = new();

    // Callers pass fixed stage names only, never subscription or IPC content.
    internal static void Stage(string stage) => Write(stage, null);
    internal static void Failure(string stage, Exception? exception) => Write(stage, exception);

    private static void Write(string stage, Exception? exception)
    {
        try
        {
            lock (sync)
            {
                string local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
                if (string.IsNullOrEmpty(local)) return; // Never fall back to the working directory.
                string directory = Path.Combine(local, "Misetanibox.Lite", "logs");
                Directory.CreateDirectory(directory);
                string path = Path.Combine(directory, "startup.log");
                if (File.Exists(path) && new FileInfo(path).Length >= MaxLogBytes)
                    File.Move(path, path + ".previous", overwrite: true);
                var text = new StringBuilder();
                text.AppendLine($"{DateTimeOffset.UtcNow:O} pid={Environment.ProcessId} build={typeof(App).Assembly.ManifestModule.ModuleVersionId} {stage}");
                // Messages, Data, source paths and ToString can contain URLs/tokens.
                // Keep HRESULTs and method-only stacks, including inner exceptions.
                for (int depth = 0; exception is not null && depth < 8; depth++, exception = exception.InnerException)
                {
                    text.AppendLine($"exception[{depth}]={exception.GetType().FullName} HRESULT=0x{exception.HResult:X8}");
                    foreach (var frame in (new StackTrace(exception, false).GetFrames() ?? []).Take(40))
                    {
                        var method = frame.GetMethod();
                        if (method is not null) text.AppendLine($"  at {method.DeclaringType?.FullName}.{method.Name} IL={frame.GetILOffset()}");
                    }
                }
                File.AppendAllText(path, text.ToString());
            }
        }
        catch { /* Diagnostics must never replace the original failure. */ }
    }
}
