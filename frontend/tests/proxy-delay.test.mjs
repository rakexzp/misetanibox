import test from 'node:test';
import assert from 'node:assert/strict';
import { nextTick } from 'vue';
import { harness, deferred } from './proxy-delay-harness.mjs';

const fallbackData = { groupOrder: ['group'], groups: { group: {type: 'Selector', now: 'FALLBACK', all: ['FALLBACK']}, FALLBACK: {type: 'Fallback', now: 'leaf', all: ['leaf']} } };

test('early delay event survives delayed initial state, finish, reload and core stop', async () => {
  const snapshot = deferred();
  const h = harness({ GetAppState: () => snapshot.promise, GetInitialData: async () => fallbackData });
  const init = h.store.initStore();
  await h.ready;
  h.events['app-state-sync']({activeConfig: 'new', isRunning: true});
  h.events['proxy-delay-update']({name: 'FALLBACK', delay: 42, status: 'success'});
  assert.equal(h.state.proxyDelays.FALLBACK?.delay, 42);
  assert.equal(h.listeners.get('proxy-delay-update').size, 2);
  snapshot.resolve({activeConfig: 'old', isRunning: false});
  await init;
  assert.equal(h.state.activeConfigId, 'new');
  h.events['proxy-test-finished']('Тест задержки завершён', {status: 'success', targets: 1, completed: 1, success: 1, failed: 0, skipped: 0, requested: 2});
  await h.run('loadData()');
  h.events['app-state-sync']({isRunning: false});
  await nextTick();
  assert.equal(h.run("formatDelay(globalState.proxyDelays.FALLBACK)"), '42ms');
  assert.match(h.run('delaySummary.value'), /1\/1/);
  h.dispose();
});

test('single RPC result without event displays 42 and uses retention', async () => {
  const h = harness({TestProxy: async () => 42});
  h.state.delayRetentionTime = '30';
  await h.run('testSingleDelay(localGroups.value[0].proxies[0])');
  assert.equal(h.run('formatDelay(globalState.proxyDelays.node)'), '42ms');
  assert.equal([...h.timers.values()].filter(t => t.ms === 30000).length, 1);
  h.dispose();
});

test('single RPC cannot replace a newer event or changed profile', async () => {
  for (const change of ['event', 'profile', 'clear', 'unmount']) {
    const result = deferred();
    const h = harness({TestProxy: () => result.promise});
    await h.store.initStore();
    const pending = h.run('testSingleDelay(localGroups.value[0].proxies[0])');
    if (change === 'profile') h.state.activeConfigId = 'other';
    if (change === 'clear') h.events['delay-cache-clear']();
    if (change === 'unmount') h.dispose();
    if (change === 'event') h.events['proxy-delay-update']({name: 'node', delay: 55, status: 'success'});
    result.resolve(42);
    await pending;
    assert.equal(h.state.proxyDelays.node?.delay, change === 'event' ? 55 : undefined, change);
    h.dispose();
  }
});

test('event and RPC duplicate keeps original retention timer', async () => {
  const result = deferred();
  const h = harness({TestProxy: () => result.promise});
  await h.store.initStore();
  h.state.delayRetentionTime = '30';
  const pending = h.run('testSingleDelay(localGroups.value[0].proxies[0])');
  h.events['proxy-delay-update']({name: 'node', delay: 42, status: 'success'});
  const timer = [...h.timers.entries()].find(([, t]) => t.ms === 30000)?.[0];
  result.resolve(42);
  await pending;
  assert.equal(h.timers.has(timer), true);
  assert.equal([...h.timers.values()].filter(t => t.ms === 30000).length, 1);
  h.dispose();
});

test('failed initial snapshot keeps live subscriptions without duplicate initialization', async () => {
  const initial = deferred();
  const h = harness({GetAppState: () => initial.promise});
  const pending = h.store.initStore();
  await h.store.initStore();
  initial.reject(new Error('snapshot unavailable'));
  await pending;
  await h.store.initStore();
  h.events['app-state-sync']({activeConfig: 'recovered'});
  h.events['proxy-delay-update']({name: 'node', delay: 42, status: 'success'});
  assert.equal(h.state.activeConfigId, 'recovered');
  assert.equal(h.state.proxyDelays.node.delay, 42);
  assert.equal(h.listeners.get('app-state-sync').size, 1);
  assert.equal(h.listeners.get('proxy-delay-update').size, 2);
  h.dispose();
});

test('all-failed batch and partial or cancelled batch have bounded visible summaries', () => {
  const h = harness();
  h.events['proxy-test-finished']('Не удалось измерить задержку: нет успешных измерений', {status: 'error', targets: 2, completed: 2, success: 0, failed: 2, skipped: 0});
  assert.match(h.run('delaySummary.value'), /Узлы: 2\/2, успешно: 0, ошибок: 2/);
  assert.match(h.alerts[0][0], /нет успешных измерений/);
  h.events['proxy-test-finished']('private SECRET', {status: 'partial', targets: 2, completed: 2, success: 1, failed: 1, skipped: 0});
  assert.match(h.run('delaySummary.value'), /частично/);
  assert.doesNotMatch(h.run('delaySummary.value'), /SECRET/);
  h.events['proxy-test-finished']('Тест задержки отменён', {status: 'cancelled', targets: 2, completed: 0, success: 0, failed: 0, skipped: 2});
  assert.match(h.run('delaySummary.value'), /отменён/);
  assert.equal(h.alerts.length, 1);
  h.dispose();
});

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
