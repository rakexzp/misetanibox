<template>
  <div class="overview-layout" :class="{ 'has-custom-bg': !!globalState.consoleBg }">
    <div v-if="globalState.consoleBg" class="console-custom-bg" :style="{ backgroundImage: `url(${globalState.consoleBg})` }"></div>
    <section class="hero-panel card-panel">
      <div class="status-core">
        <div class="restart-trigger" :class="{ 'is-loading': isRestarting }" @click="handleRestartCore" title="Перезапуск ядра">
          <div class="orb-visual" v-show="!isRestarting">
            <div class="orb" :class="{ 'active': consoleServiceOn }"></div>
            <div class="orb-glow" v-if="consoleServiceOn"></div>
          </div>
          <svg class="refresh-icon scanner-svg" :class="{ 'spin': isRestarting }" viewBox="0 0 24 24">
            <circle class="scanner-track" cx="12" cy="12" r="10"></circle>
            <circle class="scanner-bar" cx="12" cy="12" r="10"></circle>
          </svg>
        </div>
        <div class="status-meta">
          <span class="micro-title">Статус службы</span>
          <h2 class="status-heading">{{ consoleServiceTitle }}</h2>
          <span class="version-tag">Mihomo {{ globalState.version || 'Core' }}</span>
        </div>
      </div>
      <div class="active-config-display">
        <span class="micro-title">Активная конфигурация</span>
        <div class="config-name truncate" :title="globalState.activeConfigName">
          {{ globalState.activeConfigName || 'Не выбрана' }}
        </div>
      </div>
    </section>

    <section class="switch-row">
      <div class="action-card" :class="{ 'on': sysProxyCardOn }" @click="toggleSysProxy">
        <div class="card-content">
          <div class="icon-ring" v-html="ICONS.sysProxy"></div>
          <div class="text-group">
            <span class="card-title">Системный прокси</span>
            <span class="card-hint">{{ sysProxyLabel }}</span>
          </div>
        </div>
        <div class="status-node"></div>
      </div>

      <div class="action-card" :class="{ 'on': tunCardOn }" @click="toggleTun">
        <div class="card-content">
          <div class="icon-ring" v-html="ICONS.tun"></div>
          <div class="text-group">
            <span class="card-title">Режим TUN</span>
            <span class="card-hint">{{ tunLabel }}</span>
          </div>
        </div>
        <div class="status-node"></div>
      </div>
    </section>

    <section class="mode-section rules-card">
      <div class="rules-head">
        <h3 class="section-heading">Режим маршрутизации</h3>
      </div>
      <div class="segmented-control">
        <div v-for="m in modes" :key="m.val" class="seg-item" :class="{ active: globalState.mode === m.val }" @click="handleModeChange(m.val)">
          {{ m.label }}
        </div>
        <div class="seg-slider" :style="sliderStyle"></div>
      </div>
    </section>

    <TrafficCard :traffic="traffic" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { globalState, showAlert, showConfirm, updateStateFromBackend, scheduleOutboundIPRefresh, setSystemProxyIntent, setTunIntent } from '../store';
import { ICONS } from '../utils/icons';
import TrafficCard from './TrafficCard.vue';

defineProps<{
  traffic: {
    up: string;
    down: string;
    upRaw?: number;
    downRaw?: number;
    uploadTotal?: string;
    downloadTotal?: string;
    uploadTotalRaw?: number;
    downloadTotalRaw?: number;
  }
}>();

const modes = [
  { label: 'Правила', val: 'rule' },
  { label: 'Глобальный', val: 'global' },
  { label: 'Прямое соединение', val: 'direct' }
];

const sliderStyle = computed(() => ({
  left: `${(modes.findIndex(m => m.val === globalState.mode) * 100) / 3}%`
}));

const isRestarting = ref(false);

const desiredActive = computed(() => globalState.systemProxy || globalState.tun);
const actualActive = computed(() => globalState.actualSystemProxy || globalState.actualTun || globalState.isRunning);
const consoleServiceOn = computed(() => desiredActive.value || actualActive.value);

const consoleServiceTitle = computed(() => {
  if (isRestarting.value) return 'Перезапуск ядра...';
  // TUN 开启时：用 actualTun 判断，与 IP 检测的 state.Tun 一致
  if (globalState.tun && globalState.actualTun) return 'Перехват активен';
  // 系统代理：用 isRunning 判断
  if (globalState.systemProxy && globalState.isRunning) return 'Перехват активен';
  if (actualActive.value) return 'Работает';
  return 'Служба остановлена';
});

const handleRestartCore = async () => {
  if (isRestarting.value) return;
  const ok = await showConfirm("Перезапустить службу ядра? Возможен кратковременный обрыв сети.", "Перезапуск ядра");
  if (!ok) return;
  isRestarting.value = true;
  try {
    await (API as any).RestartCore();
    isRestarting.value = false;
    await showAlert("Служба ядра успешно перезапущена", 'Успешно');
  } catch (e) {
    isRestarting.value = false;
    await showAlert("Перезапуск не удался: " + e, 'Ошибка');
  }
};

// ==========================================
// 切换状态管理（三层模型：intent / actual / pending）
// ==========================================
const sysProxyCardOn = computed(() => globalState.systemProxy);
const tunCardOn = computed(() => globalState.tun);

const sysProxyLabel = computed(() => {
  return sysProxyCardOn.value
    ? 'Системные настройки сети изменены'
    : 'HTTP-трафик не перехватывается';
});

const tunLabel = computed(() => {
  return tunCardOn.value
    ? 'Виртуальный адаптер подключён'
    : 'Драйвер прозрачного прокси не загружен';
});

const toggleSysProxy = async () => {
  if (globalState.systemProxyPending) return;

  const previous = globalState.systemProxy;
  const target = !previous;

  setSystemProxyIntent(target);
  globalState.systemProxyPending = true;

  try {
    await API.ToggleSystemProxy(target);
    const latest = await (API as any).GetAppState().catch(() => null);
    if (latest) updateStateFromBackend(latest);
  } catch (err: any) {
    // 只有明确失败才回滚
    setSystemProxyIntent(previous);

    const msg = String(err?.message || err || '');
    if (msg.includes('no active config selected') || msg.includes('ErrNoActiveConfig')) {
      showAlert('Конфигурация не добавлена\n\nПеред включением системного прокси добавьте и примените конфигурацию.', 'Внимание');
    } else {
      showAlert('Не удалось включить системный прокси: ' + msg, 'Ошибка', true);
    }
  } finally {
    globalState.systemProxyPending = false;
  }
};

const toggleTun = async () => {
  if (globalState.tunPending) return;

  const previous = globalState.tun;
  const target = !previous;

  setTunIntent(target);
  globalState.tunPending = true;

  try {
    await API.ToggleTunMode(target);
    const latest = await (API as any).GetAppState().catch(() => null);
    if (latest) updateStateFromBackend(latest);
  } catch (err: any) {
    const msg = String(err?.message || err || '');

    if (msg.includes('helper_install_required') || msg.includes('helper_repair_required')) {
      const confirmed = await showConfirm(
        'Режиму TUN нужна инициализация фоновой службы (GoclashZHelper)\n\nПодтверждение администратора требуется один раз, дальше TUN и автовосстановление работают без запросов.',
        'Нужна фоновая служба'
      );

      if (confirmed) {
        try {
          setTunIntent(target);
          await (API as any).InstallHelperService();
          await API.ToggleTunMode(target);
          const latest = await (API as any).GetAppState().catch(() => null);
          if (latest) updateStateFromBackend(latest);
          return;
        } catch (e: any) {
          setTunIntent(previous);
          showAlert('Не удалось инициализировать фоновую службу: ' + String(e?.message || e), 'Ошибка', true);
          return;
        }
      }

      setTunIntent(previous);
      return;
    }

    // 其他明确失败：回滚
    setTunIntent(previous);

    if (msg.includes('no active config selected') || msg.includes('ErrNoActiveConfig')) {
      showAlert('Конфигурация не добавлена\n\nПеред включением режима TUN добавьте и примените конфигурацию.', 'Внимание');
    } else if (msg.includes('wintun_missing') || msg.includes('Wintun')) {
      showAlert('Отсутствует драйвер Wintun. Установите его на странице «Обновление компонентов».', 'Нет зависимости', true);
    } else {
      showAlert('Не удалось включить режим TUN: ' + msg, 'Ошибка', true);
    }
  } finally {
    globalState.tunPending = false;
  }
};

// ==========================================
// 模式切换
// ==========================================
let modeWorkerActive = false;
let pendingModeTarget: string | null = null;
let previousMode = globalState.mode;

// Keep previousMode in sync with external mode changes (e.g., from backend events)
watch(() => globalState.mode, (newVal, oldVal) => {
  if (!modeWorkerActive) {
    previousMode = oldVal;
  }
});

const handleModeChange = (val: string) => {
  if (globalState.mode === val) return;
  if (modeWorkerActive) { pendingModeTarget = val; return; }
  previousMode = globalState.mode;
  runModeWorker(val);
};

const runModeWorker = async (targetMode: string) => {
  modeWorkerActive = true;
  try {
    await API.UpdateClashMode(targetMode);
    await API.FlushFakeIP().catch(() => {});
    await API.CloseAllConnections().catch(() => {});
    if (targetMode === 'global') {
      const data: any = await API.GetInitialData();
      if (data?.groups) {
        const keys = data.groupOrder?.length > 0 ? data.groupOrder : Object.keys(data.groups);
        for (const name of keys) {
          const item = data.groups[name];
          if (!item) continue;
          if (item.type === 'Selector' && !['GLOBAL', 'DIRECT', 'REJECT'].includes(name)) {
            if (item.now) await API.SelectProxy('GLOBAL', item.now).catch(() => {});
            break;
          }
        }
      }
    }
    const latest = await (API as any).GetAppState();
    updateStateFromBackend(latest);
  } catch (err) {
    globalState.mode = previousMode;
    await showAlert("Не удалось переключить режим: " + err, 'Ошибка');
  } finally {
    if (pendingModeTarget !== null && pendingModeTarget !== targetMode) {
      const next = pendingModeTarget;
      pendingModeTarget = null;
      await runModeWorker(next);
    } else {
      pendingModeTarget = null;
      modeWorkerActive = false;
    }
  }
};
</script>

<style scoped>
.overview-layout { display: flex; flex-direction: column; gap: 24px; min-height: 100%; overflow: visible; position: relative; }

/* пользовательский фон Консоли */
.console-custom-bg {
  position: absolute;
  inset: -8px;
  z-index: 0;
  background-size: cover;
  background-position: center;
  border-radius: 18px;
  pointer-events: none;
}
.console-custom-bg::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 18px;
  /* затемняющий скрим, чтобы текст карточек читался поверх любой картинки */
  background: linear-gradient(180deg, rgba(0,0,0,0.35), rgba(0,0,0,0.55));
}
.overview-layout.has-custom-bg > section,
.overview-layout.has-custom-bg > div:not(.console-custom-bg) { position: relative; z-index: 1; }
/* карточки становятся полупрозрачными, чтобы картинка проступала */
.overview-layout.has-custom-bg :deep(.card-panel),
.overview-layout.has-custom-bg :deep(.action-card) {
  background-color: color-mix(in srgb, var(--surface) 72%, transparent);
}
.hero-panel { padding: 28px 24px; background: var(--surface); border-radius: 20px; display: grid; grid-template-columns: 1fr 1fr; gap: 16px; align-items: center; box-shadow: 0 4px 20px rgba(0,0,0,0.03); }
.status-core { position: relative; display: flex; align-items: center; justify-content: center; width: 100%; }
.restart-trigger { position: absolute; left: 10px; width: 56px; height: 56px; display: flex; align-items: center; justify-content: center; cursor: pointer; border-radius: 50%; transition: 0.3s; }
.restart-trigger:hover { background: var(--surface-hover); }
.restart-trigger.is-loading { cursor: not-allowed; pointer-events: none; }
.orb-visual { position: relative; width: 10px; height: 10px; z-index: 2; transition: all 0.3s cubic-bezier(0.4,0,0.2,1); }
.orb { width: 100%; height: 100%; border-radius: 50%; background: var(--text-muted); transition: 0.4s; }
.orb.active { background: var(--text-main); box-shadow: 0 0 10px var(--text-main); }
.orb-glow { position: absolute; top: 0; left: 0; width: 100%; height: 100%; border-radius: 50%; background: var(--text-main); animation: pulse 2s infinite; }
.refresh-icon { position: absolute; top: 50%; left: 50%; transform: translate(-50%,-50%) rotate(-90deg); width: 44px; height: 44px; opacity: 0.6; transition: all 0.4s cubic-bezier(0.4,0,0.2,1); }
.scanner-track { fill: none; stroke: var(--surface-hover); stroke-width: 2.5; }
.scanner-bar { fill: none; stroke: var(--text-muted); stroke-width: 2.5; transition: all 0.4s cubic-bezier(0.4,0,0.2,1); stroke-dasharray: 62.8; stroke-dashoffset: 62.8; }
.restart-trigger:hover:not(.is-loading) .refresh-icon { opacity: 1; transform: translate(-50%,-50%) rotate(90deg); }
.restart-trigger:hover:not(.is-loading) .scanner-bar { stroke: var(--text-main); stroke-dashoffset: 20; }
.refresh-icon.spin { opacity: 1; animation: spin-centered 1.2s linear infinite; }
.refresh-icon.spin .scanner-bar { stroke: var(--accent); stroke-dasharray: 62.8; animation: scan-dash 1.2s infinite ease-in-out; }
@keyframes spin-centered { 0%{transform:translate(-50%,-50%) rotate(-90deg)} 100%{transform:translate(-50%,-50%) rotate(270deg)} }
@keyframes scan-dash { 0%{stroke-dashoffset:60} 50%{stroke-dashoffset:15} 100%{stroke-dashoffset:60} }
.status-meta { display: flex; flex-direction: column; align-items: center; text-align: center; }
.micro-title { font-size: 0.85rem; font-weight: 700; color: var(--text-sub); letter-spacing: 0.02em; margin-bottom: 8px; }
.status-heading { font-size: 1.6rem; font-weight: 600; margin: 0; line-height: 1.2; color: var(--text-main); }
.version-tag { font-family: var(--font-mono); font-size: 0.75rem; color: var(--text-sub); opacity: 0.8; margin-top: 8px; }
.truncate { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.active-config-display { text-align: center; min-width: 0; display: flex; flex-direction: column; align-items: center; }
.active-config-display .config-name { font-size: 1.1rem; font-weight: 700; color: var(--text-main); }
.switch-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.action-card { padding: 20px 24px; background: var(--surface); border: none; border-radius: 16px; display: flex; justify-content: space-between; align-items: center; cursor: pointer; transition: all 0.2s cubic-bezier(0.4,0,0.2,1); }
.action-card.on { background: var(--accent); }
.icon-ring { width: 40px; height: 40px; border-radius: 12px; background: var(--surface-hover); display: flex; align-items: center; justify-content: center; color: var(--text-sub); transition: 0.3s; }
.icon-ring :deep(svg) { width: 22px; height: 22px; }
.on .icon-ring { background: rgba(128,128,128,0.25) !important; color: var(--accent-fg); }
.card-title { display: block; font-size: 1rem; font-weight: 600; margin-bottom: 2px; color: var(--text-main); }
.card-hint { font-size: 0.75rem; color: var(--text-sub); }
.on .card-title { color: var(--accent-fg); }
.on .card-hint { color: var(--accent-fg); opacity: 0.7; }
.status-node { width: 6px; height: 6px; border-radius: 50%; background: var(--text-muted); }
.on .status-node { background: var(--accent-fg); box-shadow: 0 0 8px var(--accent-fg); }
.rules-card { padding: 24px 28px; background: var(--surface); border-radius: 20px; box-shadow: 0 4px 20px rgba(0,0,0,0.03); display: flex; flex-direction: column; gap: 20px; }
.rules-head { display: flex; align-items: center; justify-content: space-between; }
.section-heading { margin: 0; font-size: 1rem; font-weight: 600; color: var(--text-main); }
.segmented-control { background: var(--surface-panel); padding: 4px; border-radius: 12px; display: flex; position: relative; border: none; overflow: hidden; }
.seg-item { flex: 1; text-align: center; padding: 14px 0; font-size: 0.9rem; font-weight: 600; color: var(--text-main); cursor: pointer; z-index: 1; transition: 0.3s; }
.seg-item.active { color: var(--accent-fg); }
.seg-slider { position: absolute; top: 10px; bottom: 10px; width: calc(33.33% - 20px); margin-left: 10px; background: var(--text-main); border-radius: 8px; z-index: 0; box-shadow: 0 2px 10px rgba(0,0,0,0.1); transition: left 0.3s cubic-bezier(0.4,0,0.2,1); }
</style>
