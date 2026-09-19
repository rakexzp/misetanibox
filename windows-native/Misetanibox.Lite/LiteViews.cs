using Microsoft.UI;
using Microsoft.UI.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Windows.Foundation;

namespace Misetanibox.Lite;

internal static class UI
{
    internal static TextBlock Text(string text, double size = 14, double opacity = 1) => new()
    {
        Text = text, FontSize = size, Opacity = opacity, TextWrapping = TextWrapping.Wrap,
        FontFamily = new FontFamily("Segoe UI Variable")
    };
    internal static Button Button(string text, Action action)
    {
        var button = new Button { Content = text, MinHeight = 40 };
        button.Click += (_, _) => action();
        return button;
    }
    internal static Button Icon(Symbol symbol, string label, Action action)
    {
        var button = Button("", action);
        button.Content = new SymbolIcon(symbol); button.Width = 44; button.Height = 44;
        button.CornerRadius = new CornerRadius(22);
        AutomationProperties.SetName(button, label); ToolTipService.SetToolTip(button, label);
        return button;
    }
    internal static ScrollViewer Scroll(UIElement content) => new() { Content = content, HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
}

internal sealed class CoverView : Grid
{
    private readonly TextBlock subscription = UI.Text("Загружаем подписки…", 12, .7);
    private readonly TextBlock headline = UI.Text("ТВОЙ\nИНТЕРНЕТ", 48);
    private readonly TextBlock server = UI.Text("", 17, .85);
    private readonly TextBlock state = UI.Text("НЕ ПОДКЛЮЧЕНО", 12, .65);
    private readonly TextBlock note = UI.Text("VPN в этой сборке пока недоступен. Можно добавить подписку и выбрать сервер.", 12, .7);
    private readonly Button primary;
    private readonly Button servers;
    private bool onboarding;
    internal CoverView(Action add, Action subscriptions, Action pickServer, Action settings)
    {
        RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        var head = new Grid(); head.ColumnDefinitions.Add(new ColumnDefinition()); head.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var brand = new StackPanel { Spacing = 6 };
        var wordmark = UI.Text("MISETANIBOX", 18); wordmark.FontWeight = FontWeights.ExtraBold; wordmark.CharacterSpacing = 50;
        brand.Children.Add(wordmark); brand.Children.Add(subscription); head.Children.Add(brand);
        var gear = UI.Icon(Symbol.Setting, "Настройки", settings); Grid.SetColumn(gear, 1); head.Children.Add(gear); Children.Add(head);
        var content = new StackPanel { Spacing = 16, VerticalAlignment = VerticalAlignment.Bottom, Margin = new Thickness(0, 44, 0, 28) };
        headline.FontWeight = FontWeights.ExtraBold; headline.LineHeight = 53;
        content.Children.Add(state); content.Children.Add(headline); content.Children.Add(server);
        Grid.SetRow(content, 1); Children.Add(content);
        var footer = new StackPanel { Spacing = 16 };
        var actions = new Grid { ColumnSpacing = 12 }; actions.ColumnDefinitions.Add(new ColumnDefinition()); actions.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        primary = UI.Button("ПОДКЛЮЧИТЬ", () => { if (onboarding) add(); });
        primary.HorizontalAlignment = HorizontalAlignment.Stretch; primary.Height = 58; primary.CornerRadius = new CornerRadius(29);
        primary.FontWeight = FontWeights.SemiBold; actions.Children.Add(primary);
        servers = UI.Icon(Symbol.List, "Серверы", pickServer); servers.Width = 58; servers.Height = 58; servers.CornerRadius = new CornerRadius(29);
        Grid.SetColumn(servers, 1); actions.Children.Add(servers); footer.Children.Add(actions); footer.Children.Add(note);
        var manage = UI.Button("Подписки", subscriptions); manage.HorizontalAlignment = HorizontalAlignment.Left;
        footer.Children.Add(manage); Grid.SetRow(footer, 2); Children.Add(footer);
        MinHeight = 490;
    }
    internal void Update(LiteProfile? profile, bool hasProfiles, string selected, bool running, bool systemProxy, bool ready)
    {
        onboarding = ready && !hasProfiles;
        subscription.Text = profile is null ? (ready ? "Без выбранной подписки" : "Загружаем подписки…") : profile.Name + "\n" + profile.Detail;
        headline.Text = onboarding ? "Добавь свою подписку" : selected.Length > 0 ? selected : "ТВОЙ\nИНТЕРНЕТ";
        headline.FontSize = onboarding || selected.Length > 0 ? 38 : 48;
        headline.MaxLines = 3;
        headline.TextTrimming = TextTrimming.CharacterEllipsis;
        state.Text = onboarding ? "ДОБРО ПОЖАЛОВАТЬ В LITE" : running && systemProxy ? "СИСТЕМНЫЙ ПРОКСИ ВКЛЮЧЁН" : "НЕ ПОДКЛЮЧЕНО";
        server.Text = onboarding ? "По ссылке, через DNS или из файла.\nВсё остальное — здесь, без лишнего." : selected.Length > 0 ? "Выбранный сервер · сохранён в подписке" : profile is null ? "Выбери подписку" : "Выбери сервер";
        primary.Content = onboarding ? "ДОБАВИТЬ ПОДПИСКУ" : "ПОДКЛЮЧИТЬ";
        primary.IsEnabled = onboarding; // Runtime activation deliberately stays unavailable.
        servers.IsEnabled = ready && profile is not null;
    }
}

internal sealed class SubscriptionsView : StackPanel
{
    internal SubscriptionsView(IEnumerable<LiteProfile> profiles, string active, Action add, Action<string> select, Action<string> refresh, Action<LiteProfile> delete)
    {
        Spacing = 20;
        Children.Add(UI.Text("Твои подписки", 28));
        Children.Add(UI.Text("Выбери, откуда брать серверы.", 14, .65));
        Children.Add(UI.Button("+ Добавить подписку", add));
        foreach (var profile in profiles)
        {
            var item = new StackPanel { Spacing = 10, Margin = new Thickness(0, 10, 0, 12) };
            item.Children.Add(UI.Text(profile.Name, 20)); item.Children.Add(UI.Text(profile.Detail, 12, .65));
            var actions = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8 };
            var use = UI.Button(profile.Id == active ? "Выбрана" : "Выбрать", () => select(profile.Id)); use.IsEnabled = profile.Id != active;
            actions.Children.Add(use);
            var update = UI.Icon(Symbol.Refresh, "Обновить подписку", () => refresh(profile.Id));
            update.IsEnabled = profile.Type == "remote"; ToolTipService.SetToolTip(update, profile.Type == "remote" ? "Обновить подписку" : "Локальный файл не обновляется по сети"); actions.Children.Add(update);
            actions.Children.Add(UI.Icon(Symbol.Delete, "Удалить подписку", () => delete(profile)));
            item.Children.Add(actions); Children.Add(item);
            Children.Add(new Border { Height = 1, Background = new SolidColorBrush(Colors.Gray), Opacity = .2 });
        }
    }
}

internal sealed class ServersView : Grid
{
    private readonly ComboBox groups = new() { Header = "Группа", HorizontalAlignment = HorizontalAlignment.Stretch };
    private readonly ListView list = new() { SelectionMode = ListViewSelectionMode.Single };
    private readonly TextBlock hint = UI.Text("", 12, .65);
    private readonly Button choose;
    private readonly LiteTopology topology;
    internal ServersView(LiteTopology topology, Action<string, string> select)
    {
        this.topology = topology;
        RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto }); RowDefinitions.Add(new RowDefinition()); RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        var header = new StackPanel { Spacing = 12, Margin = new Thickness(0, 0, 0, 16) };
        header.Children.Add(UI.Text("Серверы", 28)); header.Children.Add(groups); header.Children.Add(hint); Children.Add(header);
        Grid.SetRow(list, 1); Children.Add(list);
        choose = UI.Button("Выбрать сервер", () => { if (groups.SelectedItem is LiteGroup g && list.SelectedItem is ListViewItem item && item.Tag is string name) select(g.Name, name); });
        choose.HorizontalAlignment = HorizontalAlignment.Stretch; choose.Margin = new Thickness(0, 16, 0, 0);
        Grid.SetRow(choose, 2); Children.Add(choose);
        list.SelectionChanged += (_, _) => choose.IsEnabled = groups.SelectedItem is LiteGroup g && g.CanSelect && list.SelectedItem != null;
        groups.SelectionChanged += (_, _) => Fill();
        foreach (var group in topology.Groups) groups.Items.Add(group);
        groups.SelectedItem = topology.Groups.FirstOrDefault(g => g.Name == topology.Selector) ?? topology.Groups.FirstOrDefault();
        if (groups.Items.Count == 0) { hint.Text = "В профиле нет групп выбора серверов."; choose.IsEnabled = false; }
        MinHeight = 380;
    }
    private void Fill()
    {
        list.Items.Clear(); choose.IsEnabled = false;
        if (groups.SelectedItem is not LiteGroup group) return;
        hint.Text = group.CanSelect ? "Выбор сохранится для этой подписки. Проверка задержки пока недоступна." : "Автоматическая группа: ручной выбор недоступен.";
        if (group.Providers.Length > 0) hint.Text += " Динамические узлы провайдеров появятся только после поддержки запуска ядра.";
        foreach (var member in group.Members)
        {
            var row = new StackPanel { Spacing = 3, Margin = new Thickness(0, 6, 0, 6) };
            row.Children.Add(UI.Text(member, 16));
            string type = topology.Nodes.GetValueOrDefault(member) ?? (topology.Groups.Any(g => g.Name == member) ? "Группа" : "Правило");
            row.Children.Add(UI.Text(type, 11, .6));
            var item = new ListViewItem { Content = row, Tag = member, HorizontalContentAlignment = HorizontalAlignment.Stretch };
            list.Items.Add(item); if (member == group.Selected) list.SelectedItem = item;
        }
        list.IsEnabled = group.CanSelect;
    }
}

internal sealed class SettingsView : StackPanel
{
    internal SettingsView(bool light, bool closeToTray, bool trayReady, Action<bool> theme, Action<bool> tray, Action exit)
    {
        Spacing = 24;
        Children.Add(UI.Text("Настройки", 28));
        Children.Add(UI.Text("Для этого запуска приложения", 13, .65));
        var appearance = new ToggleSwitch { Header = "Светлая тема", IsOn = light, OnContent = "Включена", OffContent = "Тёмная" };
        appearance.Toggled += (_, _) => theme(appearance.IsOn); Children.Add(appearance);
        var minimize = new ToggleSwitch { Header = "При закрытии — в трей", IsOn = closeToTray && trayReady, IsEnabled = trayReady, OnContent = "Оставаться в трее", OffContent = "Завершать приложение" };
        minimize.Toggled += (_, _) => tray(minimize.IsOn); Children.Add(minimize);
        Children.Add(UI.Text(trayReady ? "Двойной щелчок по значку в трее открывает Lite. Правая кнопка — выход." : "Трей недоступен. Закрытие окна завершит приложение.", 13, .65));
        Children.Add(UI.Text("О приложении", 18));
        Children.Add(UI.Text("Нативный Lite · предварительная версия\n\nПодписки и выбор серверов работают без подключения. VPN, TUN и проверка задержки пока недоступны. Приложение не устанавливает ядро или службы и не меняет системный прокси.", 13, .7));
        Children.Add(UI.Button("Выйти из приложения", exit));
    }
}
