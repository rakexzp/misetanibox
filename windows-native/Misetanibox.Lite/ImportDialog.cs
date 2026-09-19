using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Windows.ApplicationModel.DataTransfer;
using Windows.Storage.Pickers;

namespace Misetanibox.Lite;

internal sealed class ImportDialog : ContentDialog
{
    internal bool Imported { get; private set; }
    internal ImportDialog(Window owner, XamlRoot root, ElementTheme theme, Func<string, object, Task> import)
    {
        XamlRoot = root; RequestedTheme = theme; Title = "Добавить подписку";
        PrimaryButtonText = "Добавить"; CloseButtonText = "Отмена"; DefaultButton = ContentDialogButton.Primary;
        var form = new StackPanel { Spacing = 14, MinWidth = 260 };
        var mode = new ComboBox { Header = "Источник", HorizontalAlignment = HorizontalAlignment.Stretch };
        foreach (var text in new[] { "По ссылке", "Через DNS", "Локальный YAML" }) mode.Items.Add(text);
        var source = new TextBox { Header = "Ссылка на подписку", PlaceholderText = "https://…" };
        var name = new TextBox { Header = "Название (необязательно)", PlaceholderText = "Моя подписка" };
        var hint = UI.Text("Ссылка хранится только в локальной службе.", 12, .65);
        var error = new InfoBar { Severity = InfoBarSeverity.Error, IsClosable = false };
        string path = "";
        var fileName = UI.Text("Файл не выбран", 12, .7);
        var paste = UI.Button("Вставить из буфера", () => { });
        var file = UI.Button("Выбрать файл…", () => { });
        bool working = false;
        void Validate() => IsPrimaryButtonEnabled = !working && (mode.SelectedIndex == 2 ? path.Length > 0 : source.Text.Trim().Length > 0);
        void SetBusy(bool value)
        {
            working = value;
            foreach (var control in form.Children.OfType<Control>()) control.IsEnabled = !value;
            IsSecondaryButtonEnabled = !value;
            PrimaryButtonText = value ? "Добавляем…" : "Добавить"; Validate();
        }
        paste.Click += async (_, _) =>
        {
            if (working) return;
            try
            {
                var data = Clipboard.GetContent();
                if (data.Contains(StandardDataFormats.Text)) source.Text = await data.GetTextAsync();
            }
            catch { error.Message = "Не удалось прочитать буфер обмена. Вставь ссылку вручную."; error.IsOpen = true; }
        };
        file.Click += async (_, _) =>
        {
            if (working) return;
            SetBusy(true);
            try
            {
                var picker = new FileOpenPicker(); picker.FileTypeFilter.Add(".yaml"); picker.FileTypeFilter.Add(".yml");
                WinRT.Interop.InitializeWithWindow.Initialize(picker, WinRT.Interop.WindowNative.GetWindowHandle(owner));
                var selected = await picker.PickSingleFileAsync();
                if (selected != null) { path = selected.Path; fileName.Text = selected.Name; }
            }
            catch { error.Message = "Не удалось открыть файл. Проверь доступ и попробуй снова."; error.IsOpen = true; }
            finally { SetBusy(false); }
        };
        mode.SelectionChanged += (_, _) =>
        {
            bool local = mode.SelectedIndex == 2;
            source.Visibility = paste.Visibility = local ? Visibility.Collapsed : Visibility.Visible;
            file.Visibility = fileName.Visibility = local ? Visibility.Visible : Visibility.Collapsed;
            source.Header = mode.SelectedIndex == 1 ? "Домен с TXT-записью" : "Ссылка на подписку";
            source.PlaceholderText = mode.SelectedIndex == 1 ? "sub.example.com" : "https://…";
            hint.Text = local ? "Выбери конфигурацию в формате YAML." : mode.SelectedIndex == 1 ? "Конфиг или ссылка будут получены из DNS TXT-записи домена." : "Ссылка хранится только в локальной службе.";
            error.IsOpen = false; Validate();
        };
        source.TextChanged += (_, _) => Validate();
        form.Children.Add(mode); form.Children.Add(source); form.Children.Add(paste); form.Children.Add(file); form.Children.Add(fileName);
        form.Children.Add(hint); form.Children.Add(name); form.Children.Add(error); Content = UI.Scroll(form);
        mode.SelectedIndex = 0;
        Closing += (_, args) => { if (working) args.Cancel = true; };
        PrimaryButtonClick += async (_, args) =>
        {
            args.Cancel = true;
            if (working) return;
            var deferral = args.GetDeferral(); SetBusy(true); error.IsOpen = false;
            try
            {
                string label = name.Text.Trim(), input = source.Text.Trim();
                if (mode.SelectedIndex == 0)
                {
                    if (!Uri.TryCreate(input, UriKind.Absolute, out var uri) || (uri.Scheme != "https" && uri.Scheme != "http"))
                    { error.Message = "Введи полную ссылку: https://…"; error.IsOpen = true; return; }
                    await import("profiles.addURL", new { name = label, url = input });
                }
                else if (mode.SelectedIndex == 1) await import("profiles.addDNS", new { name = label, domain = input });
                else await import("profiles.addLocal", new { name = label, path });
                Imported = true; args.Cancel = false;
            }
            catch (Exception ex) { error.Message = RussianError.Describe(ex); error.IsOpen = true; }
            finally { SetBusy(false); deferral.Complete(); }
        };
    }
}
