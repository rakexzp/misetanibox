import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import { ref, computed } from 'vue';

function harness(api = {}) {
  const source = readFileSync(new URL('../src/components/Proxies.vue', import.meta.url), 'utf8');
  const script = source.split('<script setup lang="ts">')[1].split('</script>')[0].replace(/^import .*;$/gm, '');
  const events = {}, alerts = [], mounted = [];
  const state = { proxyDelays: {}, mode: 'rule' };
  const context = vm.createContext({
    ref, computed, watch: () => () => {}, nextTick: () => {},
    onMounted: f => mounted.push(f), onUnmounted: () => {}, onActivated: () => {}, onDeactivated: () => {},
    localStorage: { getItem: () => null }, globalState: state, ICONS: {},
    API: { GetAppBehavior: async () => ({}), GetInitialData: async () => ({}), ...api },
    EventsOn: (name, cb) => { events[name] = cb; return () => {}; },
    showAlert: async (...args) => alerts.push(args), console,
  });
  vm.runInContext(ts.transpileModule(script, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
  const run = code => vm.runInContext(code, context);
  run("localGroups.value = [{ name: 'group', proxies: [{name: 'node', testing: false}] }]; currentGroup.value = 'group'");
  mounted.forEach(f => f());
  return { run, events, alerts, state };
}

test('group finish failure displays explanation, resets all state and permits retry', async () => {
  const h = harness({ TestAllProxies: async () => {} });
  await h.run('testAllDelays()');
  h.events['proxy-test-finished']('Не удалось запустить тест задержки: Сначала выберите и примените конфигурацию в управлении подписками', { status: 'error' });
  assert.equal(h.alerts.length, 1);
  assert.match(h.alerts[0][0], /Сначала выберите/);
  assert.equal(h.run('isTesting.value'), false);
  assert.equal(h.run('localGroups.value[0].proxies[0].testing'), false);
  await h.run('testAllDelays()');
  assert.equal(h.run('isTesting.value'), true);
});

test('successful and cancelled completion never raises error alert', () => {
  const h = harness();
  h.events['proxy-test-finished']('Тест задержки завершён', {status: 'success'});
  h.events['proxy-test-finished']('Тест задержки отменён', {status: 'cancelled'});
  h.events['proxy-test-finished']('Тест задержки завершён');
  h.events['proxy-test-finished']('Тест задержки отменён');
  assert.equal(h.alerts.length, 0);
});

test('single rejection replaces lightning with error and safe explanation', async () => {
  const h = harness({ TestProxy: async () => { throw 'Не удалось запустить тест задержки: Сначала выберите и примените конфигурацию в управлении подписками'; } });
  await h.run('testSingleDelay(localGroups.value[0].proxies[0])');
  assert.equal(h.alerts.length, 1);
  assert.equal(h.state.proxyDelays.node.delay, 0);
  assert.equal(h.state.proxyDelays.node.status, 'test-error');
  assert.match(h.state.proxyDelays.node.message, /Сначала выберите/);
  assert.equal(h.run('localGroups.value[0].proxies[0].testing'), false);
});

test('bridge rejection is visible without exposing arbitrary secrets', async () => {
  const h = harness({ TestAllProxies: async () => { throw 'https://private.example/sub/SECRET?token=secret'; } });
  await h.run('testAllDelays()');
  assert.equal(h.alerts.length, 1);
  assert.doesNotMatch(h.alerts[0][0], /SECRET|private|token/);
  assert.equal(h.run('isTesting.value'), false);
});

test('single cancellation is not a failure alert', async () => {
  const h = harness({ TestProxy: async () => { throw 'Тест задержки отменён'; } });
  await h.run('testSingleDelay(localGroups.value[0].proxies[0])');
  assert.equal(h.alerts.length, 0);
  assert.equal(h.run('localGroups.value[0].proxies[0].testing'), false);
});
