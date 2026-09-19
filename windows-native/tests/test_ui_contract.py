"""Source-level safety/contract checks, NOT a WinUI build or rendering test."""
import pathlib
import re
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
UI = ROOT / 'Misetanibox.Lite'

class NativeUIContract(unittest.TestCase):
    def test_no_runtime_activation_from_ui(self):
        source = '\n'.join(p.read_text() for p in UI.glob('*.cs') if p.name != 'BackendClient.cs')
        self.assertNotRegex(source, r'CallAsync\("(?:connect|servers.ping)"')

    def test_separate_russian_surfaces(self):
        source = '\n'.join(p.read_text() for p in UI.glob('*.cs'))
        for text in ['Добавь свою подписку', 'Подписки', 'Серверы', 'Настройки', 'ПОДКЛЮЧИТЬ']:
            self.assertIn(text, source)
        self.assertTrue((UI / 'ImportDialog.cs').exists())
        self.assertTrue((UI / 'LiteViews.cs').exists())

    def test_method_names_exist_in_dispatcher(self):
        methods = set(re.findall(r'(?:CallAsync|import)\("([\w.]+)"', '\n'.join(p.read_text() for p in UI.glob('*.cs'))))
        dispatcher = (ROOT / 'backend/main.go').read_text()
        for method in methods:
            self.assertIn('"' + method + '"', dispatcher)

    def test_transport_keeps_partial_exchange_guard(self):
        source = (UI / 'BackendClient.cs').read_text()
        self.assertIn('catch (Exception) when (!exchangeComplete)', source)
        self.assertLess(source.index('snapshot-invalidated'), source.index('exchangeComplete = true'))

    def test_polling_does_not_recreate_pages(self):
        source = (UI / 'MainWindow.cs').read_text()
        self.assertIn('RefreshSnapshot', source)
        self.assertIn('viewGeneration', source)
        self.assertNotIn('profiles.Items.Clear()', source)

if __name__ == '__main__':
    unittest.main()
