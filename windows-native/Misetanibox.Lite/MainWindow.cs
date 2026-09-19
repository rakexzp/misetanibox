using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Windows.Foundation;
using Windows.Graphics;

namespace Misetanibox.Lite;

public sealed class MainWindow : Window
{
    private readonly BackendClient backend = new();
    private readonly Grid root = new() { RequestedTheme = ElementTheme.Dark };
    private readonly Grid layout = new() { Padding = new Thickness(26), MaxWidth = 540, HorizontalAlignment = HorizontalAlignment.Stretch };
    private readonly ContentControl page = new() { HorizontalContentAlignment = HorizontalAlignment.Stretch, VerticalContentAlignment = VerticalAlignment.Stretch };
    private readonly InfoBar error = new() { Severity = InfoBarSeverity.Error, IsClosable = true };
    private readonly ProgressBar progress = new() { IsIndeterminate = true, Height = 3, Visibility = Visibility.Collapsed };
    private readonly DispatcherTimer timer = new() { Interval = TimeSpan.FromSeconds(5) };
    private readonly CoverView cover;
    private readonly ScrollViewer coverScroll;
    private readonly Button back;
    private LiteProfile[] profiles = [];
    private string profileId = "", selectedServer = "", surface = "home";
    private LiteTopology? topology;
    private bool busy, closing, started, dialogOpen, exitRequested, closeToTray = true;
    private bool running, systemProxy;
    private long viewGeneration;
    private TrayIcon? tray;

    public MainWindow()
    {
        Title = "Misetanibox Lite";
        // Standard system title bar keeps resizing, accessibility and DPI behavior native.
        AppWindow.Resize(new SizeInt32(500, 740));
        if (AppWindow.Presenter is Microsoft.UI.Windowing.OverlappedPresenter presenter)
        {
            presenter.PreferredMinimumWidth = 360;
            presenter.PreferredMinimumHeight = 560;
        }
        layout.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        layout.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        layout.RowDefinitions.Add(new RowDefinition());
        back = UI.Button("Назад", () => Navigate("home"));
        back.Visibility = Visibility.Collapsed; back.Margin = new Thickness(0, 0, 0, 16); layout.Children.Add(back);
        error.Margin = new Thickness(0, 0, 0, 12); Grid.SetRow(error, 1); layout.Children.Add(error);
        Grid.SetRow(page, 2); layout.Children.Add(page); root.Children.Add(layout);
        progress.VerticalAlignment = VerticalAlignment.Top; root.Children.Add(progress);
        cover = new CoverView(() => _ = AddSubscription(), () => Navigate("subscriptions"), () => _ = OpenServers(), () => Navigate("settings"));
        coverScroll = UI.Scroll(cover);
        coverScroll.SizeChanged += (_, _) => cover.MinHeight = Math.Max(490, coverScroll.ActualHeight);
        Content = root; ApplyTheme(false); Navigate("home");
        root.SizeChanged += (_, _) => layout.Padding = new Thickness(root.ActualWidth < 380 ? 16 : 26);
        Closed += async (_, _) =>
        {
            tray?.Dispose(); timer.Stop();
            if (!closing) { closing = true; await backend.DisposeAsync(); }
        };
        tray = new TrayIcon(WinRT.Interop.WindowNative.GetWindowHandle(this), () => { AppWindow.Show(); Activate(); }, () => _ = Exit());
        AppWindow.Closing += (sender, args) =>
        {
            if (closing) return;
            args.Cancel = true;
            if (closeToTray && tray.Ready) AppWindow.Hide(); else _ = Exit();
        };
        timer.Tick += async (_, _) =>
        {
            if (busy || closing || !started) return;
            // Do not replace controls or selections on the five-second heartbeat.
            await Run(() => RefreshSnapshot(), quiet: true);
        };
        Activated += Start;
    }
    private async void Start(object sender, WindowActivatedEventArgs args)
    {
        Activated -= Start;
        await Run(async () =>
        {
            await backend.StartAsync(); started = true;
            await RefreshSnapshot();
            timer.Start(); // Keep the lease alive even if topology cannot be read.
            await LoadTopology();
        });
    }
    private void ApplyTheme(bool light)
    {
        root.RequestedTheme = light ? ElementTheme.Light : ElementTheme.Dark;
        var brush = new LinearGradientBrush { StartPoint = new Point(0, 0), EndPoint = new Point(.8, 1) };
        brush.GradientStops.Add(new GradientStop { Offset = 0, Color = light ? ColorHelper.FromArgb(255, 225, 231, 240) : ColorHelper.FromArgb(255, 64, 73, 90) });
        brush.GradientStops.Add(new GradientStop { Offset = .48, Color = light ? ColorHelper.FromArgb(255, 240, 242, 246) : ColorHelper.FromArgb(255, 35, 40, 49) });
        brush.GradientStops.Add(new GradientStop { Offset = 1, Color = light ? ColorHelper.FromArgb(255, 250, 250, 252) : ColorHelper.FromArgb(255, 12, 14, 19) });
        root.Background = brush;
    }
    private void UpdateCover() => cover.Update(profiles.FirstOrDefault(p => p.Id == profileId), profiles.Length > 0, selectedServer, running, systemProxy, started);
    private void Navigate(string target)
    {
        if (closing) return;
        viewGeneration++; surface = target; back.Visibility = target == "home" ? Visibility.Collapsed : Visibility.Visible;
        if (target == "home") { UpdateCover(); page.Content = coverScroll; }
        else if (target == "subscriptions") page.Content = UI.Scroll(new SubscriptionsView(profiles, profileId,
            () => _ = AddSubscription(), id => _ = SelectProfile(id), id => _ = RefreshProfile(id), p => _ = DeleteProfile(p)));
        else if (target == "settings") page.Content = UI.Scroll(new SettingsView(root.RequestedTheme == ElementTheme.Light, closeToTray, tray?.Ready == true,
            ApplyTheme, value => closeToTray = value, () => _ = Exit()));
        else if (target == "servers" && topology is not null) page.Content = new ServersView(topology, (group, name) => _ = SelectServer(group, name));
    }
    private async Task Run(Func<Task> action, bool quiet = false)
    {
        if (busy || closing) return;
        busy = true;
        if (!quiet) { progress.Visibility = Visibility.Visible; page.IsEnabled = false; back.IsEnabled = false; error.IsOpen = false; }
        try { await action(); }
        catch (Exception ex) { if (!closing) { error.Message = RussianError.Describe(ex); error.IsOpen = true; } }
        finally
        {
            busy = false;
            if (!quiet) { progress.Visibility = Visibility.Collapsed; page.IsEnabled = true; back.IsEnabled = true; }
            if (exitRequested) await Exit();
        }
    }
    private async Task RefreshSnapshot()
    {
        var snapshot = await backend.CallAsync("snapshot");
        if (closing) return;
        string oldProfile = profileId;
        profiles = Data.Profiles(snapshot); profileId = Data.Text(snapshot, "activeProfile");
        running = Data.Flag(snapshot, "running"); systemProxy = Data.Flag(snapshot, "systemProxy");
        if (oldProfile != profileId) { topology = null; selectedServer = ""; }
        UpdateCover();
    }
    private async Task LoadTopology()
    {
        if (profileId.Length == 0) { topology = null; selectedServer = ""; UpdateCover(); return; }
        string requestedProfile = profileId;
        var response = LiteTopology.Parse(await backend.CallAsync("servers.list"));
        if (closing || profileId != requestedProfile || response.ProfileId != requestedProfile) return;
        topology = response;
        selectedServer = response.Groups.FirstOrDefault(g => g.Name == response.Selector)?.Selected ?? "";
        UpdateCover();
    }
    private async Task OpenServers()
    {
        long generation = viewGeneration;
        await Run(async () =>
        {
            await LoadTopology();
            if (generation == viewGeneration && topology is not null) Navigate("servers");
        });
    }
    private async Task SelectProfile(string id)
    {
        await Run(async () =>
        {
            await backend.CallAsync("profiles.select", new { id });
            await RefreshSnapshot(); Navigate("home");
            await LoadTopology();
        });
    }
    private async Task RefreshProfile(string id)
    {
        long generation = viewGeneration;
        await Run(async () =>
        {
            await backend.CallAsync("profiles.refresh", new { id }); await RefreshSnapshot();
            if (generation == viewGeneration) Navigate("subscriptions");
            await LoadTopology();
        });
    }
    private async Task DeleteProfile(LiteProfile profile)
    {
        if (busy || dialogOpen || closing) return;
        dialogOpen = true;
        var dialog = new ContentDialog
        {
            XamlRoot = root.XamlRoot, RequestedTheme = root.RequestedTheme, Title = "Удалить подписку?",
            Content = UI.Text("«" + profile.Name + "» будет удалена с этого устройства."),
            PrimaryButtonText = "Удалить", CloseButtonText = "Отмена", DefaultButton = ContentDialogButton.Close
        };
        try
        {
            if (await dialog.ShowAsync() != ContentDialogResult.Primary) return;
            await Run(async () =>
            {
                await backend.CallAsync("profiles.delete", new { id = profile.Id }); await RefreshSnapshot();
                Navigate(profiles.Length == 0 ? "home" : "subscriptions"); await LoadTopology();
            });
        }
        finally { dialogOpen = false; if (exitRequested) await Exit(); }
    }
    private async Task AddSubscription()
    {
        if (busy || dialogOpen || closing || !started) return;
        dialogOpen = true;
        var before = profiles.Select(p => p.Id).ToHashSet();
        var dialog = new ImportDialog(this, root.XamlRoot, root.RequestedTheme, async (method, parameters) =>
        {
            // Heartbeats remain active while the user edits; only mutations pause them.
            if (busy) throw new InvalidOperationException("Операция ещё выполняется");
            busy = true;
            try { await backend.CallAsync(method, parameters); }
            finally { busy = false; }
        });
        try
        {
            await dialog.ShowAsync();
            if (!dialog.Imported || closing) return;
            await Run(async () =>
            {
                await RefreshSnapshot();
                // Imports need not auto-select in appcore. Select only a new validated profile.
                var added = profiles.FirstOrDefault(p => !before.Contains(p.Id));
                Navigate("home");
                if (added is not null && added.Id != profileId)
                {
                    await backend.CallAsync("profiles.select", new { id = added.Id }); await RefreshSnapshot();
                }
                await LoadTopology();
            });
        }
        finally { dialogOpen = false; if (exitRequested) await Exit(); }
    }
    private async Task SelectServer(string group, string name)
    {
        string requestedProfile = profileId;
        long generation = viewGeneration;
        await Run(async () =>
        {
            await backend.CallAsync("servers.select", new { profileId = requestedProfile, group, name });
            await LoadTopology();
            if (requestedProfile == profileId && generation == viewGeneration) Navigate("home");
        });
    }
    private async Task Exit()
    {
        if (closing) return;
        if (busy || dialogOpen) { exitRequested = true; return; }
        closing = true; timer.Stop();
        try { await backend.DisposeAsync(); }
        finally { Close(); }
    }
}
