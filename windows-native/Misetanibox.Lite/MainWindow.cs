using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using System.Text.Json;
using Windows.Storage.Pickers;

namespace Misetanibox.Lite;

// All connection indicators come from backend snapshots. Buttons serialize
// through Run; no optimistic "connected" state is maintained in the view.
public sealed class MainWindow : Window
{
    private readonly BackendClient backend = new();
    private readonly StackPanel panel = new() { Spacing = 12, Margin = new Thickness(24) };
    private readonly TextBlock status = new() { Text = "Starting backend…" };
    private readonly TextBlock error = new() { TextWrapping = TextWrapping.Wrap };
    private readonly TextBox name = new() { Header = "Profile name" };
    private readonly TextBox source = new() { Header = "Subscription URL or DNS domain" };
    private readonly ComboBox profiles = new() { Header = "Subscriptions", HorizontalAlignment = HorizontalAlignment.Stretch };
    private readonly ComboBox groups = new() { Header = "Group", HorizontalAlignment = HorizontalAlignment.Stretch };
    private readonly ComboBox servers = new() { Header = "Server", HorizontalAlignment = HorizontalAlignment.Stretch };
    private readonly DispatcherTimer timer = new() { Interval = TimeSpan.FromSeconds(5) };
    private string profileId = "";
    private JsonElement topology;
    private bool busy, closing;
    private TrayIcon? tray;
    public MainWindow()
    {
        Title = "Misetanibox Lite";
        panel.Children.Add(new TextBlock { Text = "Misetanibox Lite", FontSize = 28 });
        panel.Children.Add(status); panel.Children.Add(error);
        panel.Children.Add(name); panel.Children.Add(source);
        Add("Paste explicitly", async () => { var data = Windows.ApplicationModel.DataTransfer.Clipboard.GetContent(); if (data.Contains(Windows.ApplicationModel.DataTransfer.StandardDataFormats.Text)) source.Text = await data.GetTextAsync(); });
        Add("Import URL", async () => await backend.CallAsync("profiles.addURL", new { name = name.Text, url = source.Text }));
        Add("Import DNS", async () => await backend.CallAsync("profiles.addDNS", new { name = name.Text, domain = source.Text }));
        Add("Import local YAML", async () => {
            var picker = new FileOpenPicker(); picker.FileTypeFilter.Add(".yaml"); picker.FileTypeFilter.Add(".yml");
            WinRT.Interop.InitializeWithWindow.Initialize(picker, WinRT.Interop.WindowNative.GetWindowHandle(this));
            var file = await picker.PickSingleFileAsync(); if (file != null) await backend.CallAsync("profiles.addLocal", new { name = name.Text, path = file.Path });
        });
        panel.Children.Add(profiles);
        Add("Use selected profile", async () => await backend.CallAsync("profiles.select", new { id = SelectedProfile() }));
        Add("Refresh selected subscription", async () => await backend.CallAsync("profiles.refresh", new { id = SelectedProfile() }));
        Add("Delete selected profile", async () => {
            var dialog = new ContentDialog { XamlRoot = panel.XamlRoot, Title = "Delete this profile?", PrimaryButtonText = "Delete", CloseButtonText = "Cancel" };
            if (await dialog.ShowAsync() == ContentDialogResult.Primary) await backend.CallAsync("profiles.delete", new { id = SelectedProfile() });
        });
        panel.Children.Add(groups); panel.Children.Add(servers);
        groups.SelectionChanged += (_, _) => FillMembers();
        Add("Use server", async () => await backend.CallAsync("servers.select", new { profileId, group = groups.SelectedItem?.ToString(), name = servers.SelectedItem?.ToString() }));
        Add("Ping server", async () => { var result = await backend.CallAsync("servers.ping", new { profileId, name = servers.SelectedItem?.ToString() }); error.Text = $"Delay: {result} ms"; });
        Add("Connect · system proxy", async () => { if (profileId.Length == 0) throw new InvalidOperationException("Import and select a valid profile first."); await backend.CallAsync("connect"); });
        Add("Full stop", async () => await backend.CallAsync("stop"));
        panel.Children.Add(new TextBlock { Text = "TUN unavailable: privileged helper policy is not ready.\nRequired core: mihomo v1.19.31; mips optional after verification. Core is not installed automatically.", TextWrapping = TextWrapping.Wrap });
        Add("Exit", async () => { closing = true; timer.Stop(); await backend.DisposeAsync(); Close(); });
        Content = new ScrollViewer { Content = panel };
        Closed += async (_, _) => { tray?.Dispose(); timer.Stop(); if (!closing) { closing = true; await backend.DisposeAsync(); } };
        tray = new TrayIcon(WinRT.Interop.WindowNative.GetWindowHandle(this), () => { AppWindow.Show(); Activate(); }, async () => { if (busy) return; closing = true; timer.Stop(); await backend.DisposeAsync(); Close(); });
        AppWindow.Closing += (_, args) => { if (!closing && tray.Ready) { args.Cancel = true; AppWindow.Hide(); } };
        timer.Tick += async (_, _) => { if (!busy) await Run(Refresh); };
        Activated += Start;
    }
    private async void Start(object sender, WindowActivatedEventArgs args)
    {
        Activated -= Start;
        await Run(async () => { await backend.StartAsync(); await Refresh(); timer.Start(); });
    }
    private void Add(string title, Func<Task> action)
    {
        var button = new Button { Content = title }; button.Click += async (_, _) => await Run(async () => { await action(); if (!closing) await Refresh(); }); panel.Children.Add(button);
    }
    private async Task Run(Func<Task> action)
    {
        if (busy) return; busy = true; error.Text = "";
        try { await action(); } catch (Exception e) { error.Text = e.Message; } finally { busy = false; }
    }
    private string SelectedProfile() => (profiles.SelectedItem as ComboBoxItem)?.Tag?.ToString() ?? throw new InvalidOperationException("Select a profile");
    private async Task Refresh()
    {
        var snapshot = await backend.CallAsync("snapshot");
        profileId = snapshot.GetProperty("activeProfile").GetString() ?? "";
        status.Text = snapshot.GetProperty("running").GetBoolean() && snapshot.GetProperty("systemProxy").GetBoolean() ? "System proxy connected" : "Stopped / not connected";
        string? selected = (profiles.SelectedItem as ComboBoxItem)?.Tag?.ToString();
        profiles.Items.Clear();
        foreach (var p in snapshot.GetProperty("profiles").EnumerateArray()) {
            var item = new ComboBoxItem { Content = p.GetProperty("name").GetString(), Tag = p.GetProperty("id").GetString() }; profiles.Items.Add(item);
            if ((string?)item.Tag == (selected ?? profileId)) profiles.SelectedItem = item;
        }
        if (profileId.Length == 0) { status.Text = "Welcome · import and select your first profile"; groups.Items.Clear(); servers.Items.Clear(); return; }
        topology = await backend.CallAsync("servers.list");
        var old = groups.SelectedItem?.ToString(); groups.Items.Clear();
        foreach (var g in topology.GetProperty("groups").EnumerateArray()) groups.Items.Add(g.GetProperty("name").GetString());
        groups.SelectedItem = old ?? topology.GetProperty("selector").GetString(); FillMembers();
    }
    private void FillMembers()
    {
        servers.Items.Clear(); if (topology.ValueKind != JsonValueKind.Object) return;
        foreach (var g in topology.GetProperty("groups").EnumerateArray()) {
            if (g.GetProperty("name").GetString() != groups.SelectedItem?.ToString()) continue;
            foreach (var member in g.GetProperty("members").EnumerateArray()) servers.Items.Add(member.GetString());
            servers.SelectedItem = g.GetProperty("selected").GetString();
        }
    }
}
