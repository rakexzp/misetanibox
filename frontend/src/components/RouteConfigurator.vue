<template>
  <Transition name="pop">
    <div v-if="open" class="modal-overlay" @click.self="close">
      <div class="rc-card glass-card" @click.stop>
        <!-- ШАГ 0: вопрос -->
        <template v-if="step === 'ask'">
          <h3 class="rc-title">Составить свою конфигурацию маршрутов?</h3>
          <p class="rc-sub">Для выбранных сервисов создаётся своя вкладка выбора сервера (например, YouTube через Германию, а всё остальное — через основной). Можно также пустить сервис напрямую или заблокировать.</p>
          <div class="rc-ask-actions">
            <button class="rc-big primary" @click="startCustom">
              <span class="rc-big-t">Да, выбрать по сервисам</span>
              <span class="rc-big-s">YouTube, TikTok, Instagram, игры…</span>
            </button>
            <button class="rc-big" @click="applyDefault">
              <span class="rc-big-t">Нет — глобально + РФ напрямую</span>
              <span class="rc-big-s">Весь трафик через VPN, российские сайты мимо</span>
            </button>
          </div>
        </template>

        <!-- ШАГ 1: плашки сервисов -->
        <template v-else>
          <div class="rc-head">
            <button class="rc-back" @click="step = 'ask'">‹</button>
            <h3 class="rc-title" style="margin:0">Маршруты по сервисам</h3>
          </div>
          <p class="rc-sub">Нажимай на плашку, чтобы сменить режим. «Свой выбор» — у сервиса появится своя вкладка в «Прокси-узлах».</p>

          <div v-for="grp in grouped" :key="grp.cat" class="rc-group">
            <div class="rc-group-title">{{ grp.title }}</div>
            <div class="rc-tiles">
              <button
                v-for="s in grp.items"
                :key="s.key"
                class="rc-tile"
                :class="'t-' + (sel[s.key] || 'proxy')"
                @click="cycle(s.key)"
              >
                <span class="rc-ico">{{ s.icon }}</span>
                <span class="rc-name">{{ s.label }}</span>
                <span class="rc-target">{{ targetLabel(sel[s.key]) }}</span>
              </button>
            </div>
          </div>

          <div class="rc-legend">
            <span><b class="d-proxy">общий</b></span>
            <span><b class="d-selector">свой выбор</b></span>
            <span><b class="d-direct">напрямую</b></span>
            <span><b class="d-reject">блок</b></span>
          </div>

          <div class="rc-actions">
            <button class="action-btn" @click="close">Отмена</button>
            <button class="primary-btn accent-btn" :disabled="saving" @click="applyCustom">
              {{ saving ? 'Применяю…' : 'Применить' }}
            </button>
          </div>
        </template>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { showAlert } from '../store';

const open = ref(false);
const step = ref<'ask' | 'custom'>('ask');
const saving = ref(false);
const catalog = ref<any[]>([]);
const sel = ref<Record<string, string>>({});

const CAT_TITLES: Record<string, string> = {
  popular: 'Популярное',
  media: 'Видео и музыка',
  social: 'Соцсети и мессенджеры',
  ru: 'Россия',
  block: 'Блокировка',
};

const grouped = computed(() => {
  const order = ['popular', 'media', 'social', 'ru', 'block'];
  return order
    .map((cat) => ({ cat, title: CAT_TITLES[cat] || cat, items: catalog.value.filter((s) => s.category === cat) }))
    .filter((g) => g.items.length);
});

const TARGETS = ['proxy', 'selector', 'direct', 'reject'];
function cycle(key: string) {
  const cur = sel.value[key] || 'proxy';
  const next = TARGETS[(TARGETS.indexOf(cur) + 1) % TARGETS.length];
  sel.value = { ...sel.value, [key]: next };
}
function targetLabel(t?: string) {
  return t === 'selector' ? 'свой выбор' : t === 'direct' ? 'напрямую' : t === 'reject' ? 'блок' : 'общий';
}

async function show() {
  step.value = 'ask';
  try {
    catalog.value = (await (API as any).GetServiceCatalog()) || [];
    const cfg: any = await (API as any).GetRouteConfig();
    sel.value = { ...(cfg?.services || {}) };
  } catch (e) { catalog.value = []; sel.value = {}; }
  open.value = true;
}
function close() { open.value = false; if (onDone.value) { const cb = onDone.value; onDone.value = null; cb(); } }

async function startCustom() {
  // при первом заходе подставим разумный дефолт
  if (!Object.keys(sel.value).length) sel.value = { youtube: 'selector', ru: 'direct', ads: 'reject' };
  step.value = 'custom';
}

async function applyDefault() {
  saving.value = true;
  try {
    await (API as any).SaveRouteConfig({ enabled: true, services: { ru: 'direct', ads: 'reject' } });
    close();
    await showAlert('Готово: глобально, российские сайты — напрямую, реклама — в блок.', 'Настроено');
  } catch (e) { await showAlert('Не удалось применить: ' + e, 'Ошибка'); }
  finally { saving.value = false; }
}

async function applyCustom() {
  saving.value = true;
  try {
    // сохраняем только явно не-proxy (proxy = дефолт, не засоряем)
    const services: Record<string, string> = {};
    for (const [k, v] of Object.entries(sel.value)) if (v && v !== 'proxy') services[k] = v;
    await (API as any).SaveRouteConfig({ enabled: true, services });
    close();
    await showAlert('Маршруты применены. При активной конфигурации ядро перезапущено.', 'Готово');
  } catch (e) { await showAlert('Не удалось применить: ' + e, 'Ошибка'); }
  finally { saving.value = false; }
}

const onDone = ref<null | (() => void)>(null);
async function showAfterSub(cb?: () => void) { onDone.value = cb || null; await show(); }
defineExpose({ show, showAfterSub });
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,.5); display: flex; align-items: center; justify-content: center; z-index: 60; padding: 20px; }
.rc-card { width: 100%; max-width: 520px; max-height: 86vh; overflow-y: auto; padding: 22px; }
.rc-title { font-size: 1.15rem; }
.rc-sub { color: var(--text-muted); font-size: 0.85rem; margin: 6px 0 16px; line-height: 1.5; }
.rc-ask-actions { display: flex; flex-direction: column; gap: 12px; }
.rc-big { display: flex; flex-direction: column; gap: 3px; text-align: left; padding: 16px; border-radius: 12px; border: 1px solid var(--surface-hover); background: var(--surface); cursor: pointer; }
.rc-big.primary { border-color: var(--accent); }
.rc-big:hover { border-color: var(--accent); }
.rc-big-t { font-weight: 700; color: var(--text-main); }
.rc-big-s { font-size: 0.8rem; color: var(--text-muted); }
.rc-head { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
.rc-back { background: none; border: none; color: var(--text-sub); font-size: 1.6rem; cursor: pointer; line-height: 1; }
.rc-group { margin-bottom: 14px; }
.rc-group-title { font-size: 0.72rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-sub); margin-bottom: 8px; }
.rc-tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 8px; }
.rc-tile { display: flex; align-items: center; gap: 9px; padding: 10px 12px; border-radius: 10px; border: 1px solid var(--surface-hover); background: var(--surface); cursor: pointer; text-align: left; }
.rc-ico { font-size: 1.1rem; flex: none; }
.rc-name { flex: 1; min-width: 0; font-size: 0.85rem; font-weight: 600; color: var(--text-main); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rc-target { font-size: 0.66rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.03em; flex: none; }
.rc-tile.t-proxy { border-left: 3px solid var(--surface-hover); } .rc-tile.t-proxy .rc-target { color: var(--text-muted); }
.rc-tile.t-selector { border-left: 3px solid var(--accent); } .rc-tile.t-selector .rc-target { color: var(--accent); }
.rc-tile.t-direct { border-left: 3px solid #4a90d9; } .rc-tile.t-direct .rc-target { color: #4a90d9; }
.rc-tile.t-reject { border-left: 3px solid #d9534f; } .rc-tile.t-reject .rc-target { color: #d9534f; }
.rc-legend { display: flex; gap: 16px; margin: 4px 0 14px; font-size: 0.75rem; }
.d-proxy { color: var(--text-muted); } .d-selector { color: var(--accent); } .d-direct { color: #4a90d9; } .d-reject { color: #d9534f; }
.rc-actions { display: flex; justify-content: flex-end; gap: 10px; }
</style>
