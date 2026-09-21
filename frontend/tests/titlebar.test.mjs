import { readFileSync } from 'node:fs';
import test from 'node:test';
import assert from 'node:assert/strict';
import { compile } from 'vue';

// Render the actual titlebar without starting the Wails backend or unrelated views.
const source = readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8');
const render = compile(source.slice(source.indexOf('<div class="drag-bar"'), source.indexOf('\n\n', source.indexOf('<div class="drag-bar"'))));
function buttons(mode, platform = 'windows') {
  const vnode = render({
    globalState: { uiMode: mode, platform }, ICONS: {}, isMaximized: false,
    toggleUiMode() {}, WindowMinimise() {}, handleToggleMaximise() {}, handleClose() {},
  }, []);
  const result = [];
  function visit(node) {
    if (!node || typeof node !== 'object') return;
    if (node.type === 'button') result.push(node.props.title);
    if (Array.isArray(node.children)) node.children.forEach(visit);
  }
  visit(vnode);
  return result;
}
test('Lite titlebar has only window controls, not a duplicate Pro entry', () => {
  assert.deepEqual(buttons('lite'), ['Свернуть', 'Развернуть/Восстановить', 'Закрыть']);
});
test('Pro titlebar retains the Lite switch and macOS omits custom window buttons', () => {
  assert.deepEqual(buttons('full'), ['Переключить в простой режим (Lite)', 'Свернуть', 'Развернуть/Восстановить', 'Закрыть']);
  assert.deepEqual(buttons('lite', 'darwin'), []);
});
