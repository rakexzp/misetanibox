<template>
  <div class="lite-root" :class="statusClass">
    <div class="cover-bg" :style="bgStyle"></div>
    <div class="cover-scrim"></div>

    <header class="cover-head">
      <div class="cover-brand-block">
        <span class="cover-brand">MISETANIBOX</span>
        <span class="cover-meta">{{ resolvedName || 'Без подписки' }}<template v-if="subMeta"> · {{ subMeta }}</template></span>
      </div>
      <div class="cover-head-actions">
        <button class="cover-icon-btn" title="Свои обои" @click="pickLiteBg"><span v-html="ICONS.image"></span></button>
        <button class="cover-icon-btn" title="Про-режим" @click="switchToPro"><span v-html="ICONS.settings"></span></button>
      </div>
    </header>

    <div class="cover-spacer"></div>

    <div class="cover-hero" :class="statusClass" :key="statusClass">
      <span class="hero-line hero-code" style="--i: 0">
        <img v-if="exitFlag" :src="exitFlag" class="hero-flag" alt="" />{{ exitCode }}
      </span>
      <span class="hero-line hero-timer" style="--i: 1">{{ busy ? '··:··' : sessionText }}</span>
    </div>
    <div class="cover-sub">
      <span class="cover-sub-name truncate">{{ currentServer ? serverLabel(currentServer) : (hasConfig ? 'Сервер не выбран' : 'Добавьте подписку') }}</span>
      <span v-if="pingingCurrent" class="cover-sub-ping is-pinging"><span class="lite-ping-spin"></span>пингую</span>
      <span v-else-if="currentDelay != null" class="cover-sub-ping">{{ currentDelay > 0 ? currentDelay + ' мс' : '—' }}</span>
    </div>
    <div v-if="connected" class="cover-sub-traffic"><i v-html="ICONS.arrowDown"></i><span class="tv">{{ traffic.down }}</span><i v-html="ICONS.arrowUp"></i><span class="tv">{{ traffic.up }}</span></div>

    <div class="cover-bar">
      <button class="cover-primary" :disabled="busy || !hasConfig" @click="toggleConnect">
        {{ busy ? 'ПОДКЛЮЧАЮ…' : (connected ? 'ОТКЛЮЧИТЬ' : 'ПОДКЛЮЧИТЬ') }}
      </button>
      <button class="cover-pill cover-round" title="Серверы" @click="openServers = true"><span v-html="ICONS.list"></span></button>
    </div>

    <Transition name="lite-fade">
      <div v-if="openServers" class="cover-sheet-overlay" @click.self="openServers = false">
        <div class="cover-sheet">
          <span class="cover-sheet-handle"></span>
          <div class="lite-list-toolbar">
            <span>{{ servers.length }} серверов</span>
            <button class="lite-test-btn" @click.stop="pingAll" :disabled="testing">
              {{ testing ? 'Проверка…' : 'Проверить' }}
            </button>
          </div>
          <div class="lite-server-list">
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
            <button
              v-if="autoGroup"
              class="lite-server-item lite-smart-item"
              :class="{ active: currentServer === autoGroup }"
              @click="pick(autoGroup)"
            >
              <span class="lite-smart-ico" v-html="ICONS.zap"></span>
              <span class="lite-item-name">Авто</span>
              <span class="lite-item-tag">быстрейший</span>
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
      </div>
    </Transition>

    <Transition name="lite-fade">
      <div v-if="showAddSub" class="lite-modal-overlay" @click.self="closeAddSub">
        <div class="lite-modal">
          <h3>Добавить подписку</h3>
          <div class="lite-seg">
            <button class="lite-seg-btn" :class="{ active: addMode === 'url' }" @click="addMode = 'url'">По ссылке</button>
            <button class="lite-seg-btn" :class="{ active: addMode === 'dns' }" @click="addMode = 'dns'">Через DNS</button>
          </div>
          <label class="lite-field-label">{{ addMode === 'dns' ? 'Домен (TXT-запись)' : 'Ссылка на подписку' }}</label>
          <input
            v-model="subUrl"
            class="lite-input"
            :placeholder="addMode === 'dns' ? 'sub.example.com' : 'https://...'"
            :disabled="adding"
            @keyup.enter="addSub"
          />
          <p v-if="addMode === 'dns'" class="lite-field-hint">
            Конфиг или ссылка берутся из DNS TXT-записи домена (через зашифрованный DNS) — обходит блокировку самого адреса подписки.
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
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import {
  globalState, setUiMode, setTunIntent, showAlert, showConfirm, updateStateFromBackend, setCardBg,
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

// обложка: свои обои (ключ lite) → обои героя Консоли → дефолтный градиент
const bgStyle = computed(() => {
  const url = globalState.cardBgs.lite || globalState.cardBgs.hero;
  return url ? { backgroundImage: `url(${url})` } : {};
});
async function pickLiteBg() {
  try {
    const url = await (API as any).SetCardBg('lite');
    if (url) setCardBg('lite', url);
  } catch (e) { await showAlert('Не удалось выбрать обои: ' + e, 'Ошибка', true); }
}
const titleLines = computed(() => {
  if (busy.value) return ['ПОДКЛЮ', 'ЧАЮ…'];
  if (!hasConfig.value) return ['НЕТ', 'ПОДПИСКИ'];
  return connected.value ? ['ЗАЩИТА', 'ВКЛЮЧЕНА'] : ['ЗАЩИТА', 'ВЫКЛЮЧЕНА'];
});
// срок подписки (если панель отдаёт expire), иначе число серверов
const subMeta = computed(() => {
  const cfg: any = configs.value.find((c: any) => c.id === resolvedId.value);
  const exp = Number(cfg?.expire || 0);
  if (exp > 0) {
    const days = Math.ceil((exp * 1000 - Date.now()) / 86400000);
    if (days < 0) return 'истекла';
    if (days <= 30) return `осталось ${days} дн.`;
    const d = new Date(exp * 1000); const p = (n: number) => String(n).padStart(2, '0');
    return `до ${p(d.getDate())}.${p(d.getMonth() + 1)}.${d.getFullYear()}`;
  }
  return servers.value.length ? `${servers.value.length} серверов` : '';
});

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
const pingingCurrent = computed(() => !!currentServer.value && testingSet.has(currentServer.value));

// таймер сессии
const sessionStart = ref(0);
const nowTick = ref(Date.now());
let tickTimer: ReturnType<typeof setInterval> | null = null;
watch(connected, (on) => {
  if (on) { sessionStart.value = Date.now(); if (!tickTimer) tickTimer = setInterval(() => { nowTick.value = Date.now(); }, 1000); }
  else { sessionStart.value = 0; if (tickTimer) { clearInterval(tickTimer); tickTimer = null; } }
}, { immediate: true });
const sessionText = computed(() => {
  if (!sessionStart.value) return '00:00';
  const sec = Math.max(0, Math.floor((nowTick.value - sessionStart.value) / 1000));
  const p = (n: number) => String(n).padStart(2, '0');
  const h = Math.floor(sec / 3600);
  return h > 0 ? `${h}:${p(Math.floor((sec % 3600) / 60))}` : `${p(Math.floor(sec / 60))}:${p(sec % 60)}`;
});

// страна фактического выхода: идём по цепочке now от выбранного пункта до узла
const groupMap = ref<Record<string, any>>({});
const exitLeaf = computed(() => {
  const map = groupMap.value;
  let n = currentServer.value;
  const seen = new Set<string>();
  while (n && map[n]?.now && !seen.has(n)) { seen.add(n); n = map[n].now; }
  return n;
});
const exitFlag = computed(() => (exitLeaf.value ? flagUrl(exitLeaf.value) : '') || '');
const exitCode = computed(() => {
  const leaf = exitLeaf.value;
  const code = (flagUrl(leaf) || '').match(/\/([a-z]{2})\.svg/i)?.[1];
  if (code) return code.toUpperCase();
  if (!hasConfig.value) return '—';
  return leaf && leaf === autoGroup.value ? 'АВТО' : (leaf ? displayName(leaf).slice(0, 4).toUpperCase() : '—');
});
function pingOne(name: string) {
  if (!name || name === SMART_NAME || testingSet.has(name)) return;
  testingSet.add(name);
  API.TestAllProxies([name]).catch(() => testingSet.delete(name));
  setTimeout(() => testingSet.delete(name), 12000);
}
function serverLabel(name: string): string {
  if (!name) return '';
  if (name === autoGroup.value) return 'Авто · быстрейший';
  return displayName(name);
}

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

const NO_EXIT_TYPES = new Set(['Direct', 'Reject', 'RejectDrop', 'Pass', 'Compatible', 'Dns']);
const mainSelector = ref('GLOBAL'); // главный селектор подписки («Зарубежные сайты»); GLOBAL — запасной
const autoGroup = ref('');          // url-test/fallback группа внутри селектора → пункт «Авто»
function isGroupType(t: string | undefined) { return !!t && (t === 'Selector' || t === 'URLTest' || t === 'Fallback' || t === 'LoadBalance' || t === 'Smart'); }
async function loadServers() {
  try {
    const data: any = await API.GetInitialData();
    const cacheKey = 'mise_lite_servers_' + resolvedId.value;
    if (data?.isOffline) {
      // оффлайн-снимок не видит provider-узлы — берём последний онлайн-список
      try {
        const c = JSON.parse(localStorage.getItem(cacheKey) || 'null');
        if (c && Array.isArray(c.servers) && c.servers.length) {
          mainSelector.value = c.mainSelector || 'GLOBAL';
          autoGroup.value = c.autoGroup || '';
          servers.value = c.servers;
          if (!currentServer.value) currentServer.value = c.current || '';
          return;
        }
      } catch {}
    }
    const map = data?.groups || {};
    if (!data?.isOffline) groupMap.value = map;
    // главный селектор: первый Selector в порядке конфига (GLOBAL.all хранит порядок), запасной — самый длинный
    let best = '';
    if (!best) {
      // запасной вариант: самый большой Selector, чей текущий выбор — не DIRECT
      let bestLen = 0;
      for (const [n, g] of Object.entries<any>(map)) {
        if (n === 'GLOBAL' || g?.type !== 'Selector') continue;
        const len = (g.all || []).length;
        const exits = String(g.now || '').toUpperCase() !== 'DIRECT';
        if (len > bestLen || (len === bestLen && exits && best && String(map[best]?.now || '').toUpperCase() === 'DIRECT')) { best = n; bestLen = len; }
      }
    }
    // точный ответ от бэкенда: политика правила MATCH (что подставляется в «Зарубежные сайты»)
    try {
      const fin: string = await (API as any).GetMainSelector();
      if (fin && map[fin]?.type === 'Selector') best = fin;
    } catch {}
    mainSelector.value = best || 'GLOBAL';
    const g = map[mainSelector.value];
    if (!g) return;
    const all: string[] = g.all || [];
    autoGroup.value = all.find((n) => map[n]?.type === 'URLTest' || map[n]?.type === 'Fallback') || '';
    currentServer.value = g.now || '';
    servers.value = all
      .filter((n) => n !== 'GLOBAL')
      .filter((n) => {
        const t = map[n]?.type;
        // страны в подписке — Fallback/URLTest-группы: показываем как серверы; прячем не-выходы, вложенные Selector и «Авто»
        return t && !NO_EXIT_TYPES.has(t) && t !== 'Selector' && n !== autoGroup.value;
      })
      .map((n) => ({ name: n }));
    if (!data?.isOffline) {
      try { localStorage.setItem(cacheKey, JSON.stringify({ mainSelector: mainSelector.value, autoGroup: autoGroup.value, servers: servers.value, current: currentServer.value })); } catch {}
    }
  } catch (e) {
    console.error('lite:loadServers', String(e));
  }
}
async function pick(name: string) {
  try {
    wantSmart.value = false;
    await API.SelectProxy(mainSelector.value, name);
    currentServer.value = name;
    openServers.value = false;
    if (delayOf(name) == null) pingOne(name);
  } catch (e) {
    await showAlert('Не удалось выбрать сервер: ' + e, 'Ошибка', true);
  }
}

async function pickSmart() {
  wantSmart.value = true;
  openServers.value = false;
  if (globalState.isRunning) {
    try {
      await API.SelectProxy(mainSelector.value, SMART_NAME);
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
      // режим правил: РФ напрямую, мост/dialer-proxy подписки работают как в Про
      await API.UpdateClashMode('rule').catch(() => {});
      setTunIntent(true);
      await enableTun();
      if (wantSmart.value) {
        await API.SelectProxy(mainSelector.value, SMART_NAME).catch(() => {});
        currentServer.value = SMART_NAME;
      }
    }
    await loadServers();
    // GLOBAL по умолчанию смотрит в DIRECT — берём первый реальный узел
    const noExit = new Set(['', 'DIRECT', 'REJECT', 'REJECT-DROP', 'PASS', 'COMPATIBLE']);
    if (globalState.isRunning && noExit.has(currentServer.value.toUpperCase()) && (autoGroup.value || servers.value.length)) {
      await pick(autoGroup.value || servers.value[0].name);
    }
    if (globalState.isRunning && currentServer.value && delayOf(currentServer.value) == null) {
      pingOne(currentServer.value);
    }
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
  position: relative; flex: 1; min-height: 0; overflow: hidden;
  display: flex; flex-direction: column; gap: 14px;
  padding: 24px 22px 26px; color: #fff; background: #0f1013;
  --display: 'Unbounded', 'Arial Black', Impact, sans-serif;
}
.cover-bg {
  position: absolute; inset: 0; background-size: cover; background-position: center;
  background-image: radial-gradient(120% 70% at 20% 0%, #4a5262 0%, transparent 60%), radial-gradient(90% 60% at 90% 40%, #2b303a 0%, transparent 60%), linear-gradient(160deg, #23262e, #0f1013);
}
.cover-scrim { position: absolute; inset: 0; background: linear-gradient(180deg, rgba(0,0,0,0.55) 0%, rgba(0,0,0,0.15) 28%, rgba(0,0,0,0.3) 55%, rgba(0,0,0,0.85) 100%); }
.cover-head, .cover-title, .cover-sub, .cover-sub-traffic, .cover-bar, .cover-spacer { position: relative; }

.cover-head { display: flex; align-items: flex-start; justify-content: space-between; }
.cover-brand-block { display: flex; flex-direction: column; gap: 4px; }
.cover-brand { font-family: var(--display) !important; font-size: 15px; font-weight: 900; letter-spacing: -0.02em; }
.cover-meta { font-family: var(--font-mono) !important; font-size: 11px; letter-spacing: 0.12em; color: rgba(255,255,255,0.7); }
.cover-head-actions { display: flex; gap: 6px; }
.cover-icon-btn {
  width: 40px; height: 40px; border-radius: 12px; border: none; cursor: pointer;
  background: rgba(255,255,255,0.12); color: #fff; display: flex; align-items: center; justify-content: center;
  -webkit-backdrop-filter: blur(14px); backdrop-filter: blur(14px);
  transition: background 0.2s, transform 0.1s;
}
.cover-icon-btn:hover { background: rgba(255,255,255,0.2); }
.cover-icon-btn:active { transform: scale(0.95); }
.cover-icon-btn :deep(svg) { width: 18px; height: 18px; }

.cover-spacer { flex: 1; }

.cover-hero { position: relative; display: flex; flex-direction: column; gap: 2px; align-items: flex-start; }
.hero-line { display: block; animation: hero-in 0.55s cubic-bezier(0.2, 0.8, 0.2, 1) both; animation-delay: calc(var(--i) * 90ms); }
@keyframes hero-in { from { opacity: 0; transform: translateY(14px); } to { opacity: 1; transform: none; } }
.hero-code, .hero-ping, .hero-timer { font-family: var(--display) !important; font-weight: 900; font-size: clamp(48px, 15vw, 64px); line-height: 0.92; letter-spacing: -0.04em; text-transform: uppercase; color: #fff; white-space: nowrap; }
.hero-code { display: flex; align-items: center; gap: 12px; }
.hero-flag { width: 0.62em; height: 0.46em; border-radius: 0.08em; object-fit: cover; box-shadow: 0 2px 10px rgba(0,0,0,0.35); }
.hero-ping { display: flex; align-items: baseline; gap: 6px; font-variant-numeric: tabular-nums; }
.hero-ping small { font-family: var(--font-mono) !important; font-size: 0.3em; letter-spacing: 0.12em; color: rgba(255,255,255,0.7); text-transform: lowercase; }
.hero-timer { font-family: var(--font-mono) !important; font-weight: 800; letter-spacing: -0.02em; font-variant-numeric: tabular-nums; }
.cover-hero.off .hero-code, .cover-hero.off .hero-ping, .cover-hero.off .hero-timer { color: rgba(255,255,255,0.45); }
.cover-hero.connecting .hero-code { animation: hero-in 0.55s both, cover-pulse 1.2s 0.6s ease-in-out infinite; }
@keyframes cover-pulse { 50% { opacity: 0.55; } }
.hero-dots { display: inline-flex; gap: 8px; align-items: center; height: 0.7em; }
.hero-dots b { width: 0.22em; height: 0.22em; border-radius: 50%; background: #fff; animation: hero-dot 1s ease-in-out infinite; }
.hero-dots b:nth-child(2) { animation-delay: 0.15s; } .hero-dots b:nth-child(3) { animation-delay: 0.3s; }
@keyframes hero-dot { 0%, 100% { opacity: 0.25; transform: translateY(0); } 50% { opacity: 1; transform: translateY(-0.12em); } }

.cover-sub { display: flex; align-items: center; gap: 10px; min-width: 0; }
.cover-sub-traffic { display: flex; align-items: center; gap: 4px; margin-top: -6px; font-size: 12px; color: rgba(255,255,255,0.7); white-space: nowrap; }
.cover-sub-traffic .tv { display: inline-block; min-width: 11ch; text-align: left; margin-right: 10px; font-family: var(--font-mono) !important; font-variant-numeric: tabular-nums; }
.cover-sub-traffic i { display: inline-flex; }
.cover-sub-traffic i :deep(svg) { width: 11px; height: 11px; }
.cover-sub-traffic i { display: inline-flex; }
.cover-sub-traffic i :deep(svg) { width: 11px; height: 11px; }
.cover-sub-name { font-size: 15px; font-weight: 600; color: rgba(255,255,255,0.92); }
.cover-sub-ping { font-family: var(--font-mono) !important; font-size: 13px; color: rgba(255,255,255,0.7); flex-shrink: 0; min-width: 6ch; text-align: right; font-variant-numeric: tabular-nums; display: inline-flex; align-items: center; justify-content: flex-end; gap: 6px; }
.cover-sub-ping.is-pinging { color: rgba(255,255,255,0.55); }
.cover-sub-ping .lite-ping-spin { width: 11px; height: 11px; }

.cover-bar { display: flex; align-items: center; gap: 10px; }
.cover-primary {
  flex: 1 1 auto; min-width: 150px; min-height: 52px; border: none; border-radius: 999px; cursor: pointer;
  background: #fff; color: #111; font-family: var(--display) !important; font-size: 12px; font-weight: 700; letter-spacing: 0.14em;
  transition: transform 0.1s, opacity 0.2s;
}
.cover-primary:hover { opacity: 0.92; }
.cover-primary:active { transform: scale(0.97); }
.cover-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.cover-pill {
  min-height: 52px; padding: 0 12px; flex-shrink: 1; min-width: 0; overflow: hidden; border: none; border-radius: 999px; cursor: default;
  background: rgba(255,255,255,0.14); color: #fff; display: inline-flex; align-items: center; gap: 6px;
  -webkit-backdrop-filter: blur(16px); backdrop-filter: blur(16px);
  font-family: var(--font-mono) !important; font-size: 11px; font-weight: 600; white-space: nowrap;
}
.cover-round { width: 52px; padding: 0; justify-content: center; cursor: pointer; transition: background 0.2s, transform 0.1s; }
.cover-round:hover { background: rgba(255,255,255,0.22); }
.cover-round:active { transform: scale(0.95); }
.cover-round :deep(svg) { width: 26px; height: 26px; }

/* лист серверов */
.cover-sheet-overlay { position: absolute; inset: 0; z-index: 20; background: rgba(0,0,0,0.35); display: flex; align-items: flex-end; }
.cover-sheet {
  width: 100%; max-height: 72%; display: flex; flex-direction: column; gap: 8px;
  padding: 10px 14px 18px; border-radius: 24px 24px 0 0;
  background: rgba(20,20,22,0.94); color: #fff;
  -webkit-backdrop-filter: blur(30px) saturate(1.6); backdrop-filter: blur(30px) saturate(1.6);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.18), 0 -16px 50px rgba(0,0,0,0.4);
}
.cover-sheet-handle { width: 36px; height: 5px; border-radius: 3px; background: rgba(255,255,255,0.35); align-self: center; }
.lite-list-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 2px 6px 4px; font-size: 12px; color: rgba(255,255,255,0.6); }
.lite-test-btn { border: none; background: transparent; color: #fff; font-weight: 600; cursor: pointer; font-size: 13px; min-height: 36px; padding: 0 8px; }
.lite-test-btn:disabled { opacity: 0.6; cursor: default; }
.lite-server-list { display: flex; flex-direction: column; gap: 2px; overflow-y: auto; min-height: 0; }
.lite-empty { padding: 18px 6px; text-align: center; color: rgba(255,255,255,0.6); font-size: 14px; }
.lite-server-item {
  display: flex; align-items: center; gap: 10px; min-height: 44px;
  padding: 10px 12px; border-radius: 12px; cursor: pointer;
  background: transparent; border: none; text-align: left; width: 100%; color: #fff;
}
.lite-server-item:hover { background: rgba(255,255,255,0.08); }
.lite-server-item.active { background: rgba(255,255,255,0.14); box-shadow: inset 0 0 0 1px rgba(255,255,255,0.35); }
.lite-dot { width: 8px; height: 8px; border-radius: 50%; background: rgba(255,255,255,0.35); flex-shrink: 0; }
.lite-dot.on { background: #fff; }
.lite-flag { width: 20px; height: 15px; border-radius: 3px; object-fit: cover; flex-shrink: 0; }
.lite-item-name { flex: 1; min-width: 0; font-size: 15px; }
.lite-item-ping { font-family: var(--font-mono) !important; font-size: 12px; font-weight: 600; flex-shrink: 0; color: rgba(255,255,255,0.75); min-width: 6ch; text-align: right; font-variant-numeric: tabular-nums; }
.lite-item-ping.good { color: #fff; }
.lite-item-ping.bad { color: rgba(255,255,255,0.45); }
.lite-ping-spin { width: 14px; height: 14px; flex-shrink: 0; border-radius: 50%; border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff; animation: lite-spin 0.8s linear infinite; }
@keyframes lite-spin { to { transform: rotate(360deg); } }
.lite-smart-ico { display: inline-flex; color: #fff; }
.lite-smart-ico :deep(svg) { width: 16px; height: 16px; }
.lite-item-tag { font-size: 11px; color: rgba(255,255,255,0.55); text-transform: uppercase; letter-spacing: 0.04em; }
.lite-config-list { display: flex; flex-direction: column; gap: 2px; padding-top: 4px; border-top: 1px solid rgba(255,255,255,0.1); }
.lite-config-item { display: flex; align-items: center; gap: 10px; min-height: 40px; padding: 8px 12px; border-radius: 10px; border: none; background: transparent; color: #fff; cursor: pointer; text-align: left; width: 100%; font-size: 14px; }
.lite-config-item.active { background: rgba(255,255,255,0.12); }
.lite-foot { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding-top: 6px; border-top: 1px solid rgba(255,255,255,0.1); }
.lite-config-current { border: none; background: transparent; color: rgba(255,255,255,0.75); font-size: 13px; min-height: 40px; padding: 0 4px; display: inline-flex; align-items: center; gap: 4px; min-width: 0; }
.lite-config-current.clickable { cursor: pointer; }
.lite-chev-mini { display: inline-flex; transition: transform 0.2s; }
.lite-chev-mini.open { transform: rotate(90deg); }
.lite-chev-mini :deep(svg) { width: 14px; height: 14px; }
.lite-add-link { border: none; background: transparent; color: #fff; font-weight: 600; font-size: 13px; cursor: pointer; min-height: 40px; padding: 0 6px; white-space: nowrap; }

/* модалка подписки */
.lite-modal-overlay { position: absolute; inset: 0; z-index: 30; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; padding: 20px; -webkit-backdrop-filter: blur(8px); backdrop-filter: blur(8px); }
.lite-modal { width: 100%; max-width: 360px; display: flex; flex-direction: column; gap: 10px; padding: 20px; border-radius: 20px; background: rgba(24,24,26,0.92); color: #fff; box-shadow: inset 0 1px 0 rgba(255,255,255,0.15), 0 24px 60px rgba(0,0,0,0.45); }
.lite-modal h3 { margin: 0 0 4px; font-size: 17px; font-weight: 700; }
.lite-seg { display: flex; gap: 4px; padding: 3px; border-radius: 12px; background: rgba(255,255,255,0.1); margin-bottom: 4px; }
.lite-seg-btn { flex: 1; min-height: 40px; border: none; border-radius: 9px; background: transparent; color: rgba(255,255,255,0.7); font-size: 13px; font-weight: 600; cursor: pointer; }
.lite-seg-btn.active { background: #fff; color: #111; }
.lite-field-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: rgba(255,255,255,0.55); margin-top: 4px; }
.lite-field-hint { margin: 0; font-size: 12px; line-height: 1.5; color: rgba(255,255,255,0.55); }
.lite-input { min-height: 44px; padding: 0 12px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.15); background: rgba(255,255,255,0.08); color: #fff; font-size: 14px; outline: none; }
.lite-input:focus { border-color: rgba(255,255,255,0.5); }
.lite-modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 6px; }
.lite-btn-ghost { min-height: 42px; padding: 0 16px; border: none; border-radius: 12px; background: rgba(255,255,255,0.1); color: #fff; font-weight: 600; cursor: pointer; }
.lite-btn-primary { min-height: 42px; padding: 0 18px; border: none; border-radius: 12px; background: #fff; color: #111; font-weight: 700; cursor: pointer; }
.lite-btn-primary:disabled, .lite-btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }

.lite-fade-enter-active, .lite-fade-leave-active { transition: opacity 0.22s ease; }
.lite-fade-enter-from, .lite-fade-leave-to { opacity: 0; }
.lite-fade-enter-active .cover-sheet, .lite-fade-leave-active .cover-sheet { transition: transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1); }
.lite-fade-enter-from .cover-sheet, .lite-fade-leave-to .cover-sheet { transform: translateY(40px); }
.truncate { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
