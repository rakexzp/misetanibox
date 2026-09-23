import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
import { ref, reactive, computed, watch, nextTick, effectScope } from 'vue';

export function harness(api = {}) {
  const listeners = new Map(), alerts = [], mounted = [], unmounted = [], timers = new Map();
  let timerID = 0;
  const storage = { getItem: () => null, setItem: () => {} };
  const events = new Proxy({}, { get: (_, name) => (...args) => {
    for (const cb of [...(listeners.get(name) || [])]) cb(...args);
  } });
  const scope = effectScope();
  const shared = {
    ref, reactive, computed, watch, nextTick, console,
    localStorage: storage, sessionStorage: storage,
    document: { documentElement: { classList: { toggle() {} } } },
    setTimeout: (cb, ms) => { timers.set(++timerID, {cb, ms}); return timerID; },
    clearTimeout: id => timers.delete(id), requestAnimationFrame: () => {},
    API: { GetAppState: async () => ({}), GetAppBehavior: async () => ({}), GetInitialData: async () => ({}), DetectOutboundIP: async () => null, ...api },
    EventsOn: (name, cb) => {
      if (!listeners.has(name)) listeners.set(name, new Set());
      listeners.get(name).add(cb);
      return () => listeners.get(name).delete(cb);
    },
  };
  shared.window = shared;
  const execute = (source, context) => scope.run(() => vm.runInContext(ts.transpileModule(source.replace(/^import .*;$/gm, ''), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  }).outputText, context));
  const storeContext = vm.createContext({...shared, exports: {}});
  execute(readFileSync(new URL('../src/store.ts', import.meta.url), 'utf8'), storeContext);
  const store = storeContext.exports;
  const context = vm.createContext({
    ...shared, ...store, ICONS: {},
    onMounted: f => mounted.push(f), onUnmounted: f => unmounted.push(f), onActivated: () => {}, onDeactivated: () => {},
    showAlert: async (...args) => alerts.push(args),
  });
  const source = readFileSync(new URL('../src/components/Proxies.vue', import.meta.url), 'utf8');
  execute(source.split('<script setup lang="ts">')[1].split('</script>')[0], context);
  const run = code => scope.run(() => vm.runInContext(code, context));
  run("localGroups.value = [{ name: 'group', proxies: [{name: 'node', testing: false}] }]; currentGroup.value = 'group'");
  const ready = Promise.all(mounted.map(f => f()));
  return { run, events, alerts, state: store.globalState, store, ready, timers, listeners,
    dispose: () => { unmounted.forEach(f => f()); scope.stop(); timers.clear(); } };
}

export function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
