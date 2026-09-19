"""Source guards for startup ordering/privacy; NOT Windows runtime verification."""
import pathlib
import unittest

UI = pathlib.Path(__file__).resolve().parents[1] / 'Misetanibox.Lite'


class StartupContract(unittest.TestCase):
    def test_code_only_controls_have_resources_before_window_construction(self):
        source = (UI / 'App.xaml.cs').read_text()
        launch = source[source.index('protected override void OnLaunched'):]
        self.assertIn('Resources.MergedDictionaries.Add(new XamlControlsResources())', launch)
        self.assertLess(launch.index('Resources.MergedDictionaries.Add'), launch.index('new MainWindow()'))
        self.assertIn('OfType<XamlControlsResources>().Any()', launch)

    def test_exception_hooks_precede_xaml_initialization_without_suppressing_failure(self):
        source = (UI / 'App.xaml.cs').read_text()
        self.assertIn('UnhandledException +=', source)
        self.assertLess(source.index('UnhandledException +='), source.index('InitializeComponent();'))
        self.assertIn('AppDomain.CurrentDomain.UnhandledException +=', source)
        self.assertNotIn('Handled = true', source)
        self.assertIn('throw;', source)

    def test_diagnostics_are_local_bounded_and_do_not_dump_user_data(self):
        path = UI / 'StartupDiagnostics.cs'
        self.assertTrue(path.exists(), 'startup failures currently leave no application log')
        source = path.read_text()
        for required in ['SpecialFolder.LocalApplicationData', 'Misetanibox.Lite', 'startup.log',
                         'HResult', 'GetFrames()', 'GetMethod()', 'lock (', 'catch', 'MaxLogBytes']:
            self.assertIn(required, source)
        for forbidden in ['exception.Message', 'exception.ToString()', 'exception.Data',
                          'GetFileName()', 'Environment.CommandLine']:
            self.assertNotIn(forbidden, source)


if __name__ == '__main__':
    unittest.main()
