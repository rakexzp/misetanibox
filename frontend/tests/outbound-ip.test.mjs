import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import { computed, reactive } from 'vue';

// Run TrafficCard's actual computed display values without Wails or chart setup.
function display(outboundIP, extra = {}) {
  const source = readFileSync(new URL('../src/components/TrafficCard.vue', import.meta.url), 'utf8');
  const start = source.indexOf('const outboundIPHasValue =');
  const end = source.indexOf('const uploadAreaPath =', start);
  assert.ok(start >= 0 && end > start);
  const globalState = reactive({ outboundIP, ipDetecting: false, outboundIPStale: false, ...extra });
  const context = vm.createContext({ computed, globalState });
  vm.runInContext(ts.transpileModule(source.slice(start, end), {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText, context);
  return { globalState, read: expression => vm.runInContext(expression, context) };
}

const dualStack = { preferred: '2001:db8::1', ipv6: '2001:db8::1', ipv4: '203.0.113.7' };

test('current outgoing address selects explicit IPv4 while retaining IPv6 diagnostics', () => {
  const { read } = display(dualStack);
  assert.equal(read('outboundIPText.value'), '203.0.113.7');
  assert.equal(read('outboundIPHasValue.value'), true);
  assert.match(read('outboundIPTitle.value'), /IPv6: 2001:db8::1/);
});

test('IPv6-only result is unavailable, not an IPv4 value', () => {
  const { read } = display({ preferred: '2001:db8::1', ipv6: '2001:db8::1', ipv4: '' });
  assert.equal(read('outboundIPText.value'), 'IPv4 недоступен');
  assert.equal(read('outboundIPHasValue.value'), false);
});

test('old cached preferred IPv6 never appears during or after refresh', () => {
  const cached = JSON.parse('{"preferred":"2001:db8::2","ipv6":"2001:db8::2"}');
  const { read, globalState } = display(cached, { ipDetecting: true });
  assert.equal(read('outboundIPText.value'), 'Проверка…');
  assert.equal(read('outboundIPHasValue.value'), false);
  globalState.ipDetecting = false;
  assert.equal(read('outboundIPText.value'), 'IPv4 недоступен');
  globalState.outboundIP = dualStack;
  assert.equal(read('outboundIPText.value'), '203.0.113.7');
});

test('stale address is not presented as current after a route change or failed probe', () => {
  const { read, globalState } = display(dualStack, { outboundIPStale: true });
  assert.equal(read('outboundIPText.value'), 'IPv4 недоступен');
  assert.equal(read('outboundIPHasValue.value'), false);
  globalState.ipDetecting = true;
  assert.equal(read('outboundIPText.value'), 'Проверка…');
});

test('unprobed and initial checking states remain distinct', () => {
  const { read, globalState } = display(null);
  assert.equal(read('outboundIPText.value'), 'Не проверено');
  globalState.ipDetecting = true;
  assert.equal(read('outboundIPText.value'), 'Проверка…');
});
