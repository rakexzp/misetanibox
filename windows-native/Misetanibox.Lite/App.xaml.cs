using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
namespace Misetanibox.Lite;
public partial class App : Application
{
    private MainWindow? window;
    public App()
    {
        UnhandledException += (_, args) => StartupDiagnostics.Failure("xaml.unhandled", args.Exception);
        AppDomain.CurrentDomain.UnhandledException += (_, args) =>
            StartupDiagnostics.Failure("clr.unhandled", args.ExceptionObject as Exception);
        StartupDiagnostics.Stage("app.initialize");
        try { InitializeComponent(); }
        catch (Exception ex) { StartupDiagnostics.Failure("app.initialize", ex); throw; }
        StartupDiagnostics.Stage("app.initialized");
    }
    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        try
        {
            StartupDiagnostics.Stage("resources.initialize");
            // Unpackaged publish can omit App.xaml's XBF/PRI. WinUI skips missing
            // App.xaml, so InitializeComponent alone does not guarantee resources.
            // InfoBar (unlike the old form controls) requires these theme resources.
            if (!Resources.MergedDictionaries.OfType<XamlControlsResources>().Any())
                Resources.MergedDictionaries.Add(new XamlControlsResources());
            StartupDiagnostics.Stage("window.construct");
            window = new MainWindow();
            StartupDiagnostics.Stage("window.activate");
            window.Activate();
            StartupDiagnostics.Stage("window.activated");
        }
        catch (Exception ex) { StartupDiagnostics.Failure("app.launch", ex); throw; }
    }
}
