<template>
  <div class="overview-layout">
    <section class="hero-panel card-panel" :class="{ 'card-has-bg': hasBg('hero') }" :style="cardStyle('hero')">
      <span class="card-edit" @click.stop>
        <button class="card-edit-btn" @click.stop="openCardEditor('hero')" title="Настроить картинку">✎</button>
      </span>
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
      <div class="action-card" :class="{ 'on': sysProxyCardOn, 'card-has-bg': hasBg('sysproxy') }" :style="cardStyle('sysproxy')" @click="toggleSysProxy">
        <span class="card-edit" @click.stop>
          <button class="card-edit-btn" @click.stop="openCardEditor('sysproxy')" title="Настроить картинку">✎</button>
        </span>
        <div class="card-content">
          <div class="icon-ring" v-html="ICONS.sysProxy"></div>
          <div class="text-group">
            <span class="card-title">Системный прокси</span>
            <span class="card-hint">{{ sysProxyLabel }}</span>
          </div>
        </div>
        <div class="status-node"></div>
      </div>

      <div class="action-card" :class="{ 'on': tunCardOn, 'card-has-bg': hasBg('tun') }" :style="cardStyle('tun')" @click="toggleTun">
        <span class="card-edit" @click.stop>
          <button class="card-edit-btn" @click.stop="openCardEditor('tun')" title="Настроить картинку">✎</button>
        </span>
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

    <section class="mode-section rules-card" :class="{ 'card-has-bg': hasBg('mode') }" :style="cardStyle('mode')">
      <span class="card-edit" @click.stop>
        <button class="card-edit-btn" @click.stop="openCardEditor('mode')" title="Настроить картинку">✎</button>
      </span>
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

    <!-- Редактор фона карточки -->
    <Transition name="pop">
      <div v-if="editingCard" class="modal-overlay" @click.self="editingCard = null">
        <div class="bg-editor" @click.stop :style="{ transform: `translate(${editorPos.x}px, ${editorPos.y}px)` }">
          <h3 class="bg-editor-drag" @mousedown="startEditorDrag">Картинка карточки <span class="drag-hint">⠿</span></h3>

          <div class="bg-editor-actions">
            <button class="action-btn accent-btn" @click="pickCardImage(editingCard)">
              {{ globalState.cardBgs[editingCard] ? 'Сменить картинку' : 'Своя картинка' }}
            </button>
            <button v-if="hasBg(editingCard)" class="action-btn" @click="clearCard(editingCard)">Убрать</button>
          </div>

          <div class="bg-presets-label">Пресеты</div>
          <div class="bg-presets">
            <button
              v-for="p in BG_PRESETS"
              :key="p.id"
              class="bg-preset"
              :class="{ active: cardPresets[editingCard] === p.css }"
              :style="{ background: p.css }"
              @click="applyPreset(editingCard, p.css)"
              :title="p.id"
            ></button>
          </div>

          <template v-if="globalState.cardBgs[editingCard]">
            <label class="bg-row"><span>По горизонтали</span>
              <input type="range" min="0" max="100" v-model.number="optsFor(editingCard).x" @input="saveCardBgOpts" />
            </label>
            <label class="bg-row"><span>По вертикали</span>
              <input type="range" min="0" max="100" v-model.number="optsFor(editingCard).y" @input="saveCardBgOpts" />
            </label>
            <label class="bg-row"><span>Масштаб</span>
              <input type="range" min="100" max="300" v-model.number="optsFor(editingCard).zoom" @input="saveCardBgOpts" />
            </label>
          </template>
          <label v-if="hasBg(editingCard)" class="bg-row"><span>Затемнение</span>
            <input type="range" min="0" max="90" v-model.number="optsFor(editingCard).scrim" @input="saveCardBgOpts" />
          </label>

          <div class="bg-editor-footer">
            <button class="primary-btn accent-btn" @click="editingCard = null">Готово</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { globalState, showAlert, showConfirm, updateStateFromBackend, scheduleOutboundIPRefresh, setSystemProxyIntent, setTunIntent, setCardBg } from '../store';

// готовые пресеты-градиенты (без веса ассетов)
const BG_PRESETS: { id: string; css: string }[] = [
  { id: 'aurora', css: 'linear-gradient(135deg, #667eea, #764ba2)' },
  { id: 'sunset', css: 'linear-gradient(135deg, #ff6a00, #ee0979)' },
  { id: 'ocean',  css: 'linear-gradient(135deg, #2193b0, #6dd5ed)' },
  { id: 'forest', css: 'linear-gradient(135deg, #11998e, #38ef7d)' },
  { id: 'fire',   css: 'linear-gradient(135deg, #f12711, #f5af19)' },
  { id: 'grape',  css: 'linear-gradient(135deg, #8e2de2, #4a00e0)' },
  { id: 'night',  css: 'linear-gradient(135deg, #232526, #414345)' },
  { id: 'steel',  css: 'linear-gradient(135deg, #485563, #29323c)' },
];
// выбранный пресет-градиент на карточку (когда нет загруженной картинки)
const cardPresets = reactive<Record<string, string>>(
  JSON.parse(localStorage.getItem('mise_cardPresets') || '{}')
);
const saveCardPresets = () => localStorage.setItem('mise_cardPresets', JSON.stringify(cardPresets));

// пер-карточные настройки положения/масштаба/затемнения фона
interface CardBgOpts { x: number; y: number; zoom: number; scrim: number }
const defaultBgOpts = (): CardBgOpts => ({ x: 50, y: 50, zoom: 100, scrim: 45 });
const cardBgOpts = reactive<Record<string, CardBgOpts>>(
  JSON.parse(localStorage.getItem('mise_cardBgOpts') || '{}')
);
const saveCardBgOpts = () => localStorage.setItem('mise_cardBgOpts', JSON.stringify(cardBgOpts));
const optsFor = (key: string): CardBgOpts => {
  if (!cardBgOpts[key]) cardBgOpts[key] = defaultBgOpts();
  return cardBgOpts[key];
};

const editingCard = ref<string | null>(null);
const editorPos = ref({ x: 0, y: 0 });
const openCardEditor = (key: string) => { optsFor(key); editorPos.value = { x: 0, y: 0 }; editingCard.value = key; };

// перетаскивание окна редактора за заголовок
const startEditorDrag = (e: MouseEvent) => {
  const start = { mx: e.clientX, my: e.clientY, x: editorPos.value.x, y: editorPos.value.y };
  const move = (ev: MouseEvent) => {
    editorPos.value = { x: start.x + (ev.clientX - start.mx), y: start.y + (ev.clientY - start.my) };
  };
  const up = () => {
    window.removeEventListener('mousemove', move);
    window.removeEventListener('mouseup', up);
  };
  window.addEventListener('mousemove', move);
  window.addEventListener('mouseup', up);
};

const hasBg = (key: string) => !!(globalState.cardBgs[key] || cardPresets[key]);

const cardStyle = (key: string) => {
  const upload = globalState.cardBgs[key];
  const preset = cardPresets[key];
  if (!upload && !preset) return {};
  const o = optsFor(key);
  return {
    // загруженная картинка → url(); пресет → сам градиент
    backgroundImage: upload ? `url(${upload})` : preset,
    backgroundPosition: `${o.x}% ${o.y}%`,
    // 100 = cover (полностью покрывает карточку), выше = приближение (для градиентов не важно)
    backgroundSize: o.zoom <= 100 ? 'cover' : `${o.zoom}%`,
    '--card-scrim': String(o.scrim / 100),
  } as any;
};

const pickCardImage = async (key: string) => {
  try {
    const url = await (API as any).SetCardBg(key);
    if (url) {
      // загруженная картинка заменяет пресет
      delete cardPresets[key]; saveCardPresets();
      setCardBg(key, url); optsFor(key);
    }
  } catch (e) {
    await showAlert('Не удалось загрузить картинку: ' + e, 'Ошибка', true);
  }
};
const applyPreset = async (key: string, css: string) => {
  // пресет заменяет загруженную картинку
  try { await (API as any).ClearCardBg(key); } catch { /* ignore */ }
  setCardBg(key, '');
  cardPresets[key] = css; saveCardPresets();
  optsFor(key);
};
const clearCard = async (key: string) => {
  try { await (API as any).ClearCardBg(key); } catch { /* ignore */ }
  setCardBg(key, '');
  delete cardPresets[key]; saveCardPresets();
  editingCard.value = null;
};
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

/* карточки — позиционирующий контекст для кнопок редактирования и скрима */
.hero-panel, .action-card, .mode-section { position: relative; }

/* пер-карточная картинка: фон карточки + затемняющий скрим для читаемости текста.
   Размер/позицию задаёт инлайн-стиль из редактора — здесь БЕЗ !important, иначе перебьёт. */
.card-has-bg { background-repeat: no-repeat; position: relative; }
.card-has-bg > *:not(.card-edit) { position: relative; z-index: 1; }
.card-has-bg::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: rgba(0, 0, 0, var(--card-scrim, 0.45));
  z-index: 0;
}
/* поверх картинки текст белый */
.card-has-bg .card-title, .card-has-bg .card-hint, .card-has-bg .status-heading,
.card-has-bg .micro-title, .card-has-bg .version-tag, .card-has-bg .section-heading,
.card-has-bg .config-name { color: #fff !important; }

/* кнопки редактирования картинки на карточке (появляются при наведении) */
.card-edit {
  position: absolute; top: 8px; right: 8px; z-index: 2;
  display: flex; gap: 4px; opacity: 0; transition: opacity 0.15s;
}
.hero-panel:hover .card-edit, .action-card:hover .card-edit, .mode-section:hover .card-edit { opacity: 1; }
.card-edit-btn {
  width: 24px; height: 24px; border-radius: 7px; border: none; cursor: pointer;
  background: rgba(0,0,0,0.45); color: #fff; font-size: 13px; line-height: 1;
  display: flex; align-items: center; justify-content: center;
}
.card-edit-btn:hover { background: rgba(0,0,0,0.7); }

/* редактор фона карточки */
.bg-editor {
  width: 100%; max-width: 360px;
  background: var(--surface); border: 1px solid var(--surface-hover);
  border-radius: 16px; padding: 20px; display: flex; flex-direction: column; gap: 12px;
}
.bg-editor h3 { margin: 0; font-size: 1.05rem; color: var(--text-main); }
.bg-editor-drag { cursor: move; user-select: none; display: flex; align-items: center; justify-content: space-between; }
.bg-editor-drag .drag-hint { color: var(--text-muted); font-size: 1rem; opacity: 0.6; }
.bg-editor-actions { display: flex; gap: 8px; }
.bg-editor-actions .action-btn { flex: 1; }
.bg-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 0.85rem; color: var(--text-sub); }
.bg-row span { flex-shrink: 0; width: 110px; }
.bg-row input[type="range"] { flex: 1; accent-color: var(--accent); }
.bg-editor-footer { display: flex; justify-content: flex-end; margin-top: 4px; }
.bg-presets-label { font-size: 0.78rem; color: var(--text-muted); }
.bg-presets { display: grid; grid-template-columns: repeat(8, 1fr); gap: 6px; }
.bg-preset {
  aspect-ratio: 1; border-radius: 8px; border: 2px solid transparent; cursor: pointer; padding: 0;
}
.bg-preset:hover { transform: scale(1.08); }
.bg-preset.active { border-color: var(--text-main); box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--accent); }
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
