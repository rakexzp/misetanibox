using System.Text.Json;

namespace Misetanibox.Lite;

internal sealed record LiteProfile(string Id, string Name, string Type, long Expire, long Upload, long Download, long Total)
{
    public string Detail
    {
        get
        {
            var parts = new List<string> { Type == "remote" ? "Подписка" : "Локальный профиль" };
            if (Expire > 0)
            {
                try { parts.Add("до " + DateTimeOffset.FromUnixTimeSeconds(Expire).ToLocalTime().ToString("dd.MM.yyyy")); }
                catch (ArgumentOutOfRangeException) { }
            }
            if (Total > 0) parts.Add($"{Bytes(Upload + Download)} / {Bytes(Total)}");
            return string.Join(" · ", parts);
        }
    }
    private static string Bytes(long value) => value >= 1073741824 ? $"{value / 1073741824d:0.#} ГБ" : $"{value / 1048576d:0.#} МБ";
}
internal sealed record LiteGroup(string Name, string Type, string[] Members, string[] Providers, string Selected)
{
    public bool CanSelect => Type.Equals("select", StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Name;
}
internal sealed record LiteTopology(string ProfileId, string Selector, LiteGroup[] Groups, Dictionary<string, string> Nodes)
{
    public static LiteTopology Parse(JsonElement json) => new(
        Data.Text(json, "profileId"), Data.Text(json, "selector"),
        Data.Array(json, "groups").Select(g => new LiteGroup(Data.Text(g, "name"), Data.Text(g, "type"),
            Data.Strings(g, "members"), Data.Strings(g, "providers"), Data.Text(g, "selected"))).ToArray(),
        Data.Array(json, "nodes").GroupBy(n => Data.Text(n, "name")).ToDictionary(g => g.Key, g => Data.Text(g.First(), "type")));
}
internal static class Data
{
    internal static string Text(JsonElement json, string key) => json.TryGetProperty(key, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() ?? "" : "";
    internal static long Number(JsonElement json, string key) => json.TryGetProperty(key, out var v) && v.TryGetInt64(out var n) ? n : 0;
    internal static bool Flag(JsonElement json, string key) => json.TryGetProperty(key, out var v) && v.ValueKind == JsonValueKind.True;
    internal static IEnumerable<JsonElement> Array(JsonElement json, string key) => json.TryGetProperty(key, out var v) && v.ValueKind == JsonValueKind.Array ? v.EnumerateArray().ToArray() : [];
    internal static string[] Strings(JsonElement json, string key) => Array(json, key).Select(v => v.GetString() ?? "").ToArray();
    internal static LiteProfile[] Profiles(JsonElement json) => Array(json, "profiles").Select(p => new LiteProfile(
        Text(p, "id"), Text(p, "name"), Text(p, "type"), Number(p, "expire"), Number(p, "upload"), Number(p, "download"), Number(p, "total"))).ToArray();
}

// Do not display raw backend errors: they can contain URLs, paths or headers.
internal static class RussianError
{
    internal static string Describe(Exception error)
    {
        string message = error.Message;
        if (message.Contains("profile_changed")) return "Подписка изменилась. Выбери сервер ещё раз.";
        if (message.Contains("selection_not_in_profile")) return "Этот сервер больше не входит в выбранную группу. Обнови список.";
        if (message.Contains("system_proxy_ownership_not_ready")) return "Подключение в этой сборке пока недоступно. Подписками можно пользоваться без запуска VPN.";
        if (message.Contains("Backend") || message.Contains("backend") || message.Contains("pipe", StringComparison.OrdinalIgnoreCase))
            return "Нет связи с локальной службой. Закрой приложение через «Выйти» и открой снова.";
        if (error is OperationCanceledException || message.Contains("timeout", StringComparison.OrdinalIgnoreCase)) return "Не удалось дождаться ответа. Проверь сеть и попробуй снова.";
        if (message.Contains("core", StringComparison.OrdinalIgnoreCase) || message.Contains("mihomo", StringComparison.OrdinalIgnoreCase)) return "Ядро недоступно. Подписка сохранена, но VPN в этой сборке не запускается.";
        return "Не удалось выполнить действие. Проверь источник подписки, доступ к сети и формат YAML. Можно попробовать ещё раз.";
    }
}
