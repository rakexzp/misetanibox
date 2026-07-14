<template>
  <div class="lite-root">
    <div class="lite-phone">
            <header class="lite-head">
        <div class="lite-brand">Misetanibox</div>
        <div class="lite-head-actions">
          <button class="lite-icon-btn" title="Про-режим" @click="switchToPro">
            <span v-html="ICONS.settings"></span>
          </button>
        </div>
      </header>

            <div class="lite-center">
        <button
          class="lite-orb"
          :class="statusClass"
          :disabled="busy || !hasConfig"
          @click="toggleConnect"
        >
          <span class="lite-orb-ring"></span>
          <span class="lite-orb-icon" v-html="ICONS.power"></span>
        </button>
        <div class="lite-status">{{ statusText }}</div>
        <div class="lite-traffic" v-if="connected">
          <span><i v-html="ICONS.arrowDown"></i> {{ traffic.down }}</span>
          <span><i v-html="ICONS.arrowUp"></i> {{ traffic.up }}</span>
        </div>
      </div>

            <div class="lite-server" @click="openServers = !openServers">
        <div class="lite-server-main">
          <span class="lite-server-label">Сервер</span>
          <span class="lite-server-name truncate">{{ currentServer ? displayName(currentServer) : 'Не выбран' }}</span>
        </div>
        <span class="lite-server-ping" v-if="currentDelay != null" :class="pingClass(currentDelay)">
          {{ currentDelay > 0 ? currentDelay + ' мс' : '—' }}
        </span>
        <span class="lite-chev" :class="{ open: openServers }" v-html="ICONS.chevronRight"></span>
      </div>

            <Transition name="lite-expand">
        <div v-if="openServers" class="lite-server-list">
          <div class="lite-list-toolbar">
            <span>{{ servers.length }} серверов</span>
            <button class="lite-test-btn" @click.stop="pingAll" :disabled="testing">
              {{ testing ? 'Проверка…' : 'Проверить' }}
            </button>
          </div>
                    <button
            v-if="SMART_ENABLED"
            class="lite-server-item lite-smart-item"
            :class="{ active: currentServer === SMART_NAME }"
            @click="pickSmart"
          >
            <span class="lite-smart-ico" v-html="ICONS.zap"></span>
            <span class="lite-item-name">Смарт (авто)</span>
            <span class="lite-item-tag">умный выбор</span>
          </button>

          <div v-if="!servers.length" class="lite-empty">Нет серверов. Добавьте подписку.</div>
          <button
            v-for="s in servers"
            :key="s.name"
            class="lite-server-item"
            :class="{ active: s.name === currentServer }"
            @click="pick(s.name)"
          >
            <span class="lite-dot" :class="{ on: s.name === currentServer }"></span>
            <img v-if="flagUrl(s.name)" :src="flagUrl(s.name)!" class="lite-flag" alt="" />
            <span class="lite-item-name truncate">{{ displayName(s.name) }}</span>
            <span v-if="testingSet.has(s.name)" class="lite-ping-spin"></span>
            <span v-else class="lite-item-ping" :class="pingClass(delayOf(s.name))">
              {{ delayLabel(s.name) }}
            </span>
          </button>
        </div>
      </Transition>

            <Transition name="lite-expand">
        <div v-if="showConfigs" class="lite-config-list">
          <button
            v-for="c in configs"
            :key="c.id"
            class="lite-config-item"
            :class="{ active: c.id === resolvedId }"
            @click="selectConfig(c.id)"
          >
            <span class="lite-dot" :class="{ on: c.id === resolvedId }"></span>
            <span class="truncate">{{ c.name }}</span>
          </button>
        </div>
      </Transition>

            <footer class="lite-foot">
        <button
          class="lite-config-current truncate"
          :class="{ clickable: configs.length > 1 }"
          @click="configs.length > 1 && (showConfigs = !showConfigs)"
        >
          {{ resolvedName || 'Подписка не добавлена' }}
          <span v-if="configs.length > 1" class="lite-chev-mini" :class="{ open: showConfigs }" v-html="ICONS.chevronRight"></span>
        </button>
        <button class="lite-add-link" @click="showAddSub = true">
          {{ hasConfig ? '+ подписка' : '+ Добавить подписку' }}
        </button>
      </footer>
    </div>

        <Transition name="lite-fade">
      <div v-if="showAddSub" class="lite-modal-overlay" @click.self="closeAddSub">
        <div class="lite-modal">
          <h3>Добавить подписку</h3>

          <div class="lite-seg">
            <button class="lite-seg-btn" :class="{ active: addMode === 'url' }" @click="addMode = 'url'">По ссылке</button>
            <button class="lite-seg-btn" :class="{ active: addMode === 'dns' }" @click="addMode = 'dns'">Через DNS</button>
          </div>

          <label class="lite-field-label">{{ addMode === 'dns' ? 'Домен (TXT-запись)' : 'Ссылка или путь к файлу' }}</label>
          <input
            v-model="subUrl"
            class="lite-input"
            :placeholder="addMode === 'dns' ? 'sub.example.com' : 'https://… или C:\\…\\sub.txt'"
            :disabled="adding"
            @keyup.enter="addSub"
          />
          <p v-if="addMode === 'dns'" class="lite-field-hint">
            Конфиг или ссылка берутся из DNS TXT-записи домена (через зашифрованный DNS) — обходит блокировку самого адреса подписки.
          </p>
          <p v-else class="lite-field-hint">
            Можно вставить https-ссылку или путь к локальному .txt (внутри — URL подписки) / YAML Clash.
          </p>
          <label class="lite-field-label">Название (необязательно)</label>
          <input
            v-model="subName"
            class="lite-input"
            placeholder="Моя подписка"
            :disabled="adding"
            @keyup.enter="addSub"
          />
          <div class="lite-modal-actions">
            <button class="lite-btn-ghost" @click="closeAddSub" :disabled="adding">Отмена</button>
            <button class="lite-btn-primary" @click="addSub" :disabled="adding || !subUrl.trim()">
              {{ adding ? 'Добавление…' : (addMode === 'dns' ? 'Получить' : 'Добавить') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import {
  globalState, setUiMode, setTunIntent, showAlert, showConfirm, updateStateFromBackend,
} from '../store';
import { ICONS } from '../utils/icons';
import { flagUrl, displayName } from '../utils/flags';

defineProps<{ traffic: { up: string; down: string } }>();

const servers = ref<{ name: string }[]>([]);
const configs = ref<any[]>([]);
const currentServer = ref('');
const openServers = ref(false);
const busy = ref(false);
const testing = ref(false);
const SMART_NAME = '⚡ Смарт'; // должно совпадать с SmartGroupName в бэкенде (config.go)
const SMART_ENABLED = ref(false); // true когда установлено смарт-ядро (IsSmartCore)
const wantSmart = ref(false);
const testingSet = reactive(new Set<string>()); // узлы, у которых сейчас идёт замер задержки
const showAddSub = ref(false);
const showConfigs = ref(false);
const addMode = ref<'url' | 'dns'>('url');
const subUrl = ref('');
const subName = ref('');
const adding = ref(false);
let pollTimer: ReturnType<typeof setInterval> | null = null;

const resolvedId = computed(() => globalState.activeConfigId || configs.value[0]?.id || '');
const resolvedName = computed(() =>
  globalState.activeConfigName
  || configs.value.find((c) => c.id === resolvedId.value)?.name
  || configs.value[0]?.name
  || ''
);
const hasConfig = computed(() => !!resolvedId.value);
const connected = computed(() => globalState.isRunning && globalState.actualTun);

async function loadConfigs() {
  try {
    const data: any = await API.GetInitialData();
    if (data && data.activeConfig !== undefined) {
      globalState.activeConfigId = data.activeConfig;
      globalState.activeConfigName = data.activeConfigName || '';
      globalState.activeConfigType = data.activeConfigType || '';
    }
    configs.value = (await API.GetLocalConfigs()) || [];
  } catch (e) {
  }
}

async function ensureLoaded() {
  if (resolvedId.value && globalState.activeConfigId !== resolvedId.value) {
    try {
      await API.SelectLocalConfig(resolvedId.value);
      globalState.activeConfigId = resolvedId.value;
    } catch (e) {
    }
  }
  await loadServers();
}

async function selectConfig(id: string) {
  if (!id || id === resolvedId.value) { showConfigs.value = false; return; }
  try {
    await API.SelectLocalConfig(id);
    globalState.activeConfigId = id;
    showConfigs.value = false;
    await loadConfigs();
    await ensureLoaded();
  } catch (e) {
    await showAlert('Не удалось переключить конфигурацию: ' + e, 'Ошибка', true);
  }
}

const statusClass = computed(() => {
  if (busy.value) return 'connecting';
  return connected.value ? 'on' : 'off';
});
const statusText = computed(() => {
  if (busy.value) return 'Подключение…';
  if (!hasConfig.value) return 'Нет конфигурации';
  return connected.value ? 'Подключено' : 'Отключено';
});

const currentDelay = computed(() => delayOf(currentServer.value));

function delayOf(name: string): number | null {
  const d = globalState.proxyDelays[name];
  if (!d || d.delay == null) return null;
  return d.delay;
}
function delayLabel(name: string): string {
  const d = delayOf(name);
  if (d == null) return '—';
  return d > 0 ? d + ' мс' : 'таймаут';
}
function pingClass(d: number | null): string {
  if (d == null || d <= 0) return 'ping-bad';
  if (d < 200) return 'ping-good';
  if (d < 500) return 'ping-mid';
  return 'ping-bad';
}

const NON_SERVER_TYPES = new Set([
  'Selector', 'URLTest', 'Fallback', 'LoadBalance', 'Relay',
  'Direct', 'Reject', 'RejectDrop', 'Compatible', 'Pass', 'Dns',
]);

async function loadServers() {
  try {
    const data: any = await API.GetInitialData();
    const map = data?.groups || {};
    const g = map.GLOBAL;
    if (g) {
      currentServer.value = g.now || '';
      const all: string[] = g.all || [];
      servers.value = all
        .filter((n) => n !== 'GLOBAL')
        .filter((n) => {
          const t = map[n]?.type;
          return t && !NON_SERVER_TYPES.has(t);
        })
        .map((n) => ({ name: n }));
    }
  } catch (e) {
  }
}

async function pick(name: string) {
  try {
    wantSmart.value = false;
    await API.SelectProxy('GLOBAL', name);
    currentServer.value = name;
    openServers.value = false;
  } catch (e) {
    await showAlert('Не удалось выбрать сервер: ' + e, 'Ошибка', true);
  }
}

async function pickSmart() {
  wantSmart.value = true;
  openServers.value = false;
  if (globalState.isRunning) {
    try {
      await API.SelectProxy('GLOBAL', SMART_NAME);
      currentServer.value = SMART_NAME;
    } catch (e) {
      await showAlert('Не удалось включить Смарт: ' + e, 'Ошибка', true);
    }
  } else {
    await toggleConnect();
  }
}

async function syncState() {
  const latest = await (API as any).GetAppState().catch(() => null);
  if (latest) updateStateFromBackend(latest);
}

async function toggleConnect() {
  if (!hasConfig.value) {
    await showAlert('Сначала добавьте подписку в Про-режиме.', 'Нет конфигурации');
    return;
  }
  busy.value = true;
  try {
    if (connected.value) {
      setTunIntent(false);
      await API.ToggleTunMode(false);
      await syncState();
    } else {
      if (!globalState.isRunning) {
        await API.StartClash(resolvedId.value);
      }
      await API.UpdateClashMode('global').catch(() => {});
      setTunIntent(true);
      await enableTun();
      if (wantSmart.value) {
        await API.SelectProxy('GLOBAL', SMART_NAME).catch(() => {});
        currentServer.value = SMART_NAME;
      }
    }
    await loadServers();
  } catch (e) {
    await showAlert('Не удалось переключить подключение: ' + e, 'Ошибка', true);
  } finally {
    busy.value = false;
  }
}

async function enableTun() {
  try {
    await API.ToggleTunMode(true);
    await syncState();
  } catch (err: any) {
    const msg = String(err?.message || err || '');
    if (msg.includes('helper_install_required') || msg.includes('helper_repair_required')) {
      const ok = await showConfirm(
        'Режиму TUN нужна фоновая служба (GoclashZHelper).\n\nПодтверждение администратора запрашивается один раз, дальше всё работает без запросов.',
        'Нужна фоновая служба'
      );
      if (!ok) { setTunIntent(false); return; }
      await (API as any).InstallHelperService();
      await API.ToggleTunMode(true);
      await syncState();
      return;
    }
    setTunIntent(false);
    if (msg.includes('wintun_missing') || msg.includes('Wintun')) {
      await showAlert('Отсутствует драйвер Wintun. Установите его в Про-режиме на странице «Обновление компонентов».', 'Нет зависимости', true);
    } else {
      await showAlert('Не удалось включить TUN: ' + msg, 'Ошибка', true);
    }
  }
}

async function pingAll() {
  if (testing.value || !servers.value.length) return;
  if (!globalState.isRunning) {
    await showAlert('Сначала подключитесь — задержки измеряются через запущенное ядро.', 'Нужно подключение');
    return;
  }
  testing.value = true;
  servers.value.forEach((s) => testingSet.add(s.name));
  try {
    await API.TestAllProxies(servers.value.map((s) => s.name));
  } catch (e) {
  } finally {
    testing.value = false;
    setTimeout(() => testingSet.clear(), 12000);
  }
}

function switchToPro() {
  setUiMode('full');
}

function closeAddSub() {
  if (adding.value) return;
  showAddSub.value = false;
  subUrl.value = '';
  subName.value = '';
}

async function addSub() {
  const input = subUrl.value.trim();
  if (!input || adding.value) return;
  adding.value = true;
  try {
    if (addMode.value === 'dns') {
      await API.AddSubViaDNS(input, subName.value.trim());
    } else {
      await API.UpdateSub(subName.value.trim(), input);
    }
    await loadConfigs();
    await ensureLoaded();
    showAddSub.value = false;
    subUrl.value = '';
    subName.value = '';
    await showAlert('Подписка добавлена. Можно подключаться.', 'Готово');
  } catch (e) {
    await showAlert('Не удалось добавить: ' + e, 'Ошибка', true);
  } finally {
    adding.value = false;
  }
}

let unsubStart: (() => void) | null = null;
let unsubUpdate: (() => void) | null = null;

onMounted(async () => {
  try { SMART_ENABLED.value = await API.IsSmartCore(); } catch { SMART_ENABLED.value = false; }
  unsubStart = EventsOn('proxy-test-start', (name: string) => { testingSet.add(name); });
  unsubUpdate = EventsOn('proxy-delay-update', (data: any) => { if (data?.name) testingSet.delete(data.name); });
  await loadConfigs();
  await ensureLoaded();
  pollTimer = setInterval(() => { if (globalState.isRunning) loadServers(); }, 5000);
});
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
  if (unsubStart) unsubStart();
  if (unsubUpdate) unsubUpdate();
});
</script>

<style scoped>
.lite-root {
  position: relative;
  flex: 1;
  display: flex;
  align-items: stretch;
  justify-content: center;
  padding: 16px;
  overflow: hidden;
}

.lite-phone {
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: var(--surface);
  border: 1px solid var(--surface-hover);
  border-radius: 24px;
  padding: 20px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.12);
}

.lite-head { display: flex; align-items: center; justify-content: space-between; }
.lite-brand { font-size: 1.15rem; font-weight: 800; color: var(--text-main); letter-spacing: -0.01em; }
.lite-icon-btn {
  width: 36px; height: 36px; border-radius: 10px; border: none; cursor: pointer;
  background: transparent; color: var(--text-muted);
  display: flex; align-items: center; justify-content: center;
}
.lite-icon-btn:hover { background: var(--surface-hover, rgba(127,127,127,0.1)); color: var(--text-main); }
.lite-icon-btn :deep(svg) { width: 20px; height: 20px; }

.lite-center { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 24px 0 8px; }

.lite-orb {
  position: relative;
  width: 148px; height: 148px; border-radius: 50%;
  border: none; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  background: var(--surface-hover, rgba(127,127,127,0.08));
  transition: background 0.25s, box-shadow 0.25s, transform 0.1s;
}
.lite-orb:active { transform: scale(0.97); }
.lite-orb:disabled { opacity: 0.6; cursor: not-allowed; }
.lite-orb-icon { display: flex; }
.lite-orb-icon :deep(svg) { width: 52px; height: 52px; }
.lite-orb-ring {
  position: absolute; inset: -6px; border-radius: 50%;
  border: 2px solid transparent; transition: border-color 0.25s;
}

.lite-orb.off { color: var(--text-muted); }
.lite-orb.on {
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  color: var(--accent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 40%, transparent), 0 8px 30px color-mix(in srgb, var(--accent) 30%, transparent);
}
.lite-orb.on .lite-orb-ring { border-color: color-mix(in srgb, var(--accent) 45%, transparent); }
.lite-orb.connecting { color: var(--accent); }
.lite-orb.connecting .lite-orb-ring { border-color: var(--accent); border-top-color: transparent; animation: lite-spin 0.8s linear infinite; }
@keyframes lite-spin { to { transform: rotate(360deg); } }

.lite-status { font-size: 1.05rem; font-weight: 700; color: var(--text-main); }
.lite-traffic { display: flex; gap: 18px; font-size: 0.82rem; color: var(--text-muted); }
.lite-traffic i { display: inline-flex; vertical-align: middle; }
.lite-traffic i :deep(svg) { width: 13px; height: 13px; }

.lite-server {
  display: flex; align-items: center; gap: 10px;
  padding: 14px 16px; border-radius: 14px; cursor: pointer;
  background: var(--surface-hover, rgba(127,127,127,0.06));
  border: 1px solid var(--surface-hover);
}
.lite-server:hover { border-color: var(--text-muted); }
.lite-server-main { display: flex; flex-direction: column; min-width: 0; flex: 1; gap: 2px; }
.lite-server-label { font-size: 0.72rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; }
.lite-server-name { font-size: 0.95rem; font-weight: 600; color: var(--text-main); }
.lite-server-ping { font-size: 0.8rem; font-weight: 600; }
.lite-chev { display: inline-flex; color: var(--text-muted); transition: transform 0.2s; }
.lite-chev.open { transform: rotate(90deg); }
.lite-chev :deep(svg) { width: 18px; height: 18px; }

.lite-server-list { display: flex; flex-direction: column; gap: 4px; max-height: 34vh; overflow-y: auto; }
.lite-list-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 2px 6px 6px; font-size: 0.76rem; color: var(--text-muted); }
.lite-test-btn { border: none; background: transparent; color: var(--accent); font-weight: 600; cursor: pointer; font-size: 0.8rem; }
.lite-test-btn:disabled { opacity: 0.6; cursor: default; }
.lite-empty { padding: 18px 6px; text-align: center; color: var(--text-muted); font-size: 0.85rem; }

.lite-server-item {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 12px; border-radius: 10px; cursor: pointer;
  background: transparent; border: 1px solid transparent; text-align: left; width: 100%;
}
.lite-server-item:hover { background: var(--surface-hover, rgba(127,127,127,0.08)); }
.lite-server-item.active { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 10%, transparent); }
.lite-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--text-muted); flex-shrink: 0; opacity: 0.5; }
.lite-dot.on { background: var(--accent); opacity: 1; }
.lite-item-name { flex: 1; min-width: 0; font-size: 0.9rem; color: var(--text-main); }
.lite-item-ping { font-size: 0.78rem; font-weight: 600; flex-shrink: 0; }
.lite-ping-spin {
  width: 14px; height: 14px; flex-shrink: 0; border-radius: 50%;
  border: 2px solid var(--surface-hover, rgba(127,127,127,0.3));
  border-top-color: var(--accent);
  animation: lite-spin 0.7s linear infinite;
}

.lite-smart-item { border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent); background: color-mix(in srgb, var(--accent) 7%, transparent); }
.lite-smart-item.active { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); }
.lite-smart-ico { display: inline-flex; color: var(--accent); flex-shrink: 0; }
.lite-smart-ico :deep(svg) { width: 18px; height: 18px; }
.lite-item-tag { font-size: 0.72rem; color: var(--accent); font-weight: 600; flex-shrink: 0; }

.ping-good { color: #16a34a; }
.ping-mid { color: #d97706; }
.ping-bad { color: #dc2626; }

.lite-foot { display: flex; flex-direction: column; align-items: center; gap: 6px; font-size: 0.78rem; color: var(--text-muted); padding-top: 4px; }
.lite-config-current {
  max-width: 100%; border: none; background: transparent; color: var(--text-sub);
  font-size: 0.8rem; font-weight: 600; display: inline-flex; align-items: center; gap: 4px; padding: 2px 6px;
}
.lite-config-current.clickable { cursor: pointer; }
.lite-config-current.clickable:hover { color: var(--text-main); }
.lite-chev-mini { display: inline-flex; transition: transform 0.2s; }
.lite-chev-mini.open { transform: rotate(90deg); }
.lite-chev-mini :deep(svg) { width: 14px; height: 14px; }

.lite-config-list { display: flex; flex-direction: column; gap: 4px; max-height: 26vh; overflow-y: auto; }
.lite-config-item {
  display: flex; align-items: center; gap: 10px; padding: 9px 12px; border-radius: 10px;
  background: transparent; border: 1px solid transparent; cursor: pointer; text-align: left; width: 100%;
  color: var(--text-main); font-size: 0.88rem;
}
.lite-config-item:hover { background: var(--surface-hover, rgba(127,127,127,0.08)); }
.lite-config-item.active { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 10%, transparent); }
.lite-add-link {
  border: none; background: transparent; cursor: pointer;
  color: var(--accent); font-weight: 700; font-size: 0.82rem; padding: 4px 8px;
}
.lite-add-link:hover { text-decoration: underline; }

.lite-modal-overlay {
  position: absolute; inset: 0; z-index: 50;
  display: flex; align-items: center; justify-content: center; padding: 20px;
  background: rgba(0, 0, 0, 0.45);
}
.lite-modal {
  width: 100%; max-width: 360px;
  background: var(--surface); border: 1px solid var(--surface-hover);
  border-radius: 18px; padding: 20px; display: flex; flex-direction: column; gap: 8px;
  box-shadow: 0 16px 50px rgba(0, 0, 0, 0.3);
}
.lite-modal h3 { margin: 0 0 6px; font-size: 1.05rem; color: var(--text-main); }
.lite-seg { display: flex; gap: 4px; padding: 3px; border-radius: 10px; background: var(--surface-hover, rgba(127,127,127,0.1)); margin-bottom: 4px; }
.lite-seg-btn {
  flex: 1; padding: 7px; border: none; border-radius: 8px; cursor: pointer;
  background: transparent; color: var(--text-muted); font-weight: 600; font-size: 0.82rem;
}
.lite-seg-btn.active { background: var(--surface); color: var(--text-main); box-shadow: 0 1px 3px rgba(0,0,0,0.12); }
.lite-field-hint { font-size: 0.74rem; color: var(--text-muted); line-height: 1.4; margin: 2px 0 0; }
.lite-field-label { font-size: 0.76rem; color: var(--text-muted); margin-top: 6px; }
.lite-input {
  width: 100%; padding: 10px 12px; border-radius: 10px;
  border: 1px solid var(--surface-hover); background: var(--surface-hover, transparent);
  color: var(--text-main); outline: none; font-size: 0.9rem;
}
.lite-input:focus { border-color: var(--accent); }
.lite-modal-actions { display: flex; gap: 10px; margin-top: 14px; }
.lite-btn-ghost, .lite-btn-primary {
  flex: 1; padding: 10px; border-radius: 10px; cursor: pointer; font-weight: 700; font-size: 0.9rem; border: none;
}
.lite-btn-ghost { background: var(--surface-hover, rgba(127,127,127,0.12)); color: var(--text-main); }
.lite-btn-primary { background: var(--accent); color: #fff; }
.lite-btn-primary:disabled, .lite-btn-ghost:disabled { opacity: 0.6; cursor: default; }

.lite-fade-enter-active, .lite-fade-leave-active { transition: opacity 0.18s; }
.lite-fade-enter-from, .lite-fade-leave-to { opacity: 0; }

.truncate { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.lite-expand-enter-active, .lite-expand-leave-active { transition: opacity 0.18s, transform 0.18s; }
.lite-expand-enter-from, .lite-expand-leave-to { opacity: 0; transform: translateY(-6px); }
.lite-flag { width: 20px; height: 15px; border-radius: 2px; flex-shrink: 0; object-fit: cover; }
</style>
