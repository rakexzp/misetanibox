<template>
  <div class="workshop">
    <div class="ws-toolbar">
      <div class="ws-tabs">
        <button :class="{ active: sort === 'popular' }" @click="setSort('popular')">Популярные</button>
        <button :class="{ active: sort === 'new' }" @click="setSort('new')">Новые</button>
      </div>
      <input v-model="query" class="ws-search" placeholder="Поиск темы или автора…" @keyup.enter="reload" />
      <button class="ws-publish" @click="openPublish">Опубликовать</button>
    </div>

    <div v-if="loading" class="ws-note">Загрузка каталога…</div>
    <div v-else-if="!items.length" class="ws-note">Пока пусто. Опубликуй первую тему!</div>

    <div class="ws-grid">
      <div v-for="t in items" :key="t.id" class="ws-card">
        <div class="ws-preview" :style="previewStyle(t)">
          <span v-if="!t.preview" class="ws-preview-fallback" :style="{ background: t.accent || '#888' }"></span>
        </div>
        <div class="ws-card-body">
          <div class="ws-card-title" :title="t.name">{{ t.name }}</div>
          <div class="ws-card-author">@{{ t.author }}</div>
          <div class="ws-card-stats">
            <span>↓ {{ t.downloads }}</span>
            <span>♥ {{ t.likes }}</span>
          </div>
          <div class="ws-card-actions">
            <button class="ws-btn primary" @click="install(t)" :disabled="busyId === t.id">
              {{ busyId === t.id ? '…' : 'Установить' }}
            </button>
            <button class="ws-icon-btn" title="Нравится" @click="like(t)">♥</button>
            <button class="ws-icon-btn" title="Пожаловаться" @click="report(t)">⚑</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="items.length >= 30" class="ws-more">
      <button class="ws-btn" @click="loadMore">Ещё</button>
    </div>

    <Transition name="ws-pop">
      <div v-if="showPublish" class="ws-modal-overlay" @click.self="showPublish = false">
        <div class="ws-modal">
          <h3>Опубликовать тему</h3>
          <p class="ws-modal-hint">Заворачиваем твоё текущее оформление — цвета, фоны карточек и их положение.</p>
          <label class="ws-field">Название
            <input v-model="pubName" maxlength="48" placeholder="Reaper Dark" />
          </label>
          <label class="ws-field">Автор (ник)
            <input v-model="pubAuthor" maxlength="48" placeholder="whxteangel" />
          </label>
          <div class="ws-modal-actions">
            <button class="ws-btn" @click="showPublish = false" :disabled="publishing">Отмена</button>
            <button class="ws-btn primary" @click="doPublish" :disabled="publishing || !pubName.trim()">
              {{ publishing ? 'Публикация…' : 'Опубликовать' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { globalState, setCardBg, showAlert, showConfirm } from '../store';
import { getSavedColors, saveColors, type CustomColors } from '../utils/colors';

const CARD_KEYS = ['hero', 'sysproxy', 'tun', 'mode'];

const items = ref<any[]>([]);
const loading = ref(false);
const sort = ref<'popular' | 'new'>('popular');
const query = ref('');
const page = ref(0);
const busyId = ref<number | null>(null);

const showPublish = ref(false);
const publishing = ref(false);
const pubName = ref('');
const pubAuthor = ref(localStorage.getItem('mise_authorName') || '');

function parse(s: string) { try { return JSON.parse(s); } catch { return null; } }
function lsObj(k: string) { return parse(localStorage.getItem(k) || '{}') || {}; }

const previewStyle = (t: any) => t.preview
  ? { backgroundImage: `url("${t.preview}")` }
  : {};

async function reload() {
  loading.value = true; page.value = 0;
  try {
    const res = parse(await API.WorkshopList(sort.value, query.value.trim(), 0));
    items.value = res?.items || [];
  } catch (e) { await showAlert('Не удалось загрузить каталог: ' + e, 'Ошибка', true); }
  loading.value = false;
}
async function loadMore() {
  page.value++;
  const res = parse(await API.WorkshopList(sort.value, query.value.trim(), page.value));
  if (res?.items?.length) items.value.push(...res.items);
}
function setSort(s: 'popular' | 'new') { sort.value = s; reload(); }

async function like(t: any) {
  try { const r = parse(await API.WorkshopLike(t.id)); if (r) t.likes = r.likes; } catch { /* ignore */ }
}
async function report(t: any) {
  if (!(await showConfirm('Пожаловаться на тему «' + t.name + '»?', 'Жалоба'))) return;
  try { await API.WorkshopReport(t.id, ''); await showAlert('Жалоба отправлена. Спасибо!', 'Готово'); } catch { /* ignore */ }
}

async function install(t: any) {
  busyId.value = t.id;
  try {
    const m = parse(await API.WorkshopDownload(t.id));
    if (!m?.manifest) throw new Error('пустой манифест');
    await applyTheme(m.manifest);
    await showAlert('Тема «' + t.name + '» установлена.', 'Готово');
    t.downloads = (t.downloads || 0) + 1;
  } catch (e) { await showAlert('Не удалось установить: ' + e, 'Ошибка', true); }
  busyId.value = null;
}

async function applyTheme(m: any) {
  // цвета
  if (m.colors?.accent && m.colors?.text && m.colors?.bg) {
    saveColors({ accent: m.colors.accent, text: m.colors.text, bg: m.colors.bg } as CustomColors);
  }
  // светлая/тёмная
  if (m.theme === 'light' || m.theme === 'dark') {
    globalState.theme = m.theme;
    localStorage.setItem('goclashz_theme', m.theme);
  }
  // карточки
  const opts = lsObj('mise_cardBgOpts');
  const presets = lsObj('mise_cardPresets');
  for (const key of CARD_KEYS) {
    const card = m.cards?.[key];
    if (!card) continue;
    opts[key] = { x: card.x ?? 50, y: card.y ?? 50, zoom: card.zoom ?? 100, scrim: card.scrim ?? 45 };
    if (card.gradient) {
      presets[key] = card.gradient;
      try { await API.ClearCardBg(key); } catch { /* ignore */ }
      setCardBg(key, '');
    } else if (typeof card.img === 'string' && card.img.startsWith('asset:')) {
      const idx = parseInt(card.img.split(':')[1]);
      const url = m.assets?.[idx];
      if (url) {
        const dataUri = await API.WorkshopFetchImage(url);
        const applied = await API.SetCardBgData(key, dataUri);
        delete presets[key];
        setCardBg(key, applied);
      }
    }
  }
  localStorage.setItem('mise_cardBgOpts', JSON.stringify(opts));
  localStorage.setItem('mise_cardPresets', JSON.stringify(presets));
}

function openPublish() {
  pubAuthor.value = localStorage.getItem('mise_authorName') || pubAuthor.value;
  showPublish.value = true;
}

async function doPublish() {
  publishing.value = true;
  try {
    const colors = getSavedColors();
    const bgs = await API.GetCardBgs();       // { key: dataURI }
    const opts = lsObj('mise_cardBgOpts');
    const presets = lsObj('mise_cardPresets');
    const cards: Record<string, any> = {};
    const images: string[] = [];
    for (const key of CARD_KEYS) {
      const o = opts[key] || {};
      const card: any = { x: o.x ?? 50, y: o.y ?? 50, zoom: o.zoom ?? 100, scrim: o.scrim ?? 45 };
      if (bgs[key]) { card.img = 'asset:' + images.length; images.push(bgs[key]); }
      else if (presets[key]) { card.gradient = presets[key]; }
      else continue;
      cards[key] = card;
    }
    const manifest: any = {
      format: 1,
      name: pubName.value.trim(),
      author: pubAuthor.value.trim() || 'аноним',
      theme: globalState.theme,
      cards,
    };
    if (colors) manifest.colors = colors;
    const res = parse(await API.WorkshopPublish(JSON.stringify(manifest), images));
    if (res?.id) {
      localStorage.setItem('mise_authorName', manifest.author);
      showPublish.value = false;
      await showAlert('Тема опубликована в Мастерской!', 'Готово');
      reload();
    } else {
      throw new Error('сервер не принял тему');
    }
  } catch (e) { await showAlert('Не удалось опубликовать: ' + e, 'Ошибка', true); }
  publishing.value = false;
}

onMounted(reload);
</script>

<style scoped>
.workshop { height: 100%; display: flex; flex-direction: column; }
.ws-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 18px; flex: none; }
.ws-tabs { display: flex; gap: 6px; }
.ws-tabs button {
  padding: 8px 16px; border-radius: 8px; border: none; background: var(--surface);
  color: var(--text-sub); font-weight: 600; cursor: pointer; font-size: 0.85rem;
}
.ws-tabs button.active { background: var(--accent); color: var(--accent-fg); }
.ws-search {
  flex: 1; padding: 9px 14px; border-radius: 8px; border: 1px solid var(--surface-hover);
  background: var(--surface-panel); color: var(--text-main); font-size: 0.85rem;
}
.ws-search:focus { outline: none; border-color: var(--accent); }
.ws-publish {
  padding: 9px 18px; border-radius: 8px; border: none; background: var(--accent);
  color: var(--accent-fg); font-weight: 700; cursor: pointer; font-size: 0.85rem;
}
.ws-note { color: var(--text-muted); text-align: center; padding: 40px; }
.ws-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px; overflow-y: auto; align-content: start; padding-bottom: 20px;
}
.ws-card {
  background: var(--surface-panel); border-radius: 12px; overflow: hidden;
  display: flex; flex-direction: column; border: 1px solid var(--surface);
}
.ws-preview { height: 110px; background-size: cover; background-position: center; background-color: var(--surface); }
.ws-preview-fallback { display: block; width: 100%; height: 100%; }
.ws-card-body { padding: 12px 14px; display: flex; flex-direction: column; gap: 3px; }
.ws-card-title { font-weight: 700; font-size: 0.95rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ws-card-author { font-size: 0.75rem; color: var(--text-muted); }
.ws-card-stats { display: flex; gap: 14px; font-size: 0.75rem; color: var(--text-sub); margin: 4px 0 8px; }
.ws-card-actions { display: flex; gap: 6px; }
.ws-btn {
  flex: 1; padding: 8px; border-radius: 7px; border: 1px solid var(--surface-hover);
  background: var(--surface); color: var(--text-main); font-weight: 600; cursor: pointer; font-size: 0.82rem;
}
.ws-btn.primary { background: var(--accent); color: var(--accent-fg); border-color: var(--accent); }
.ws-btn:disabled { opacity: 0.5; cursor: default; }
.ws-icon-btn {
  width: 34px; border-radius: 7px; border: 1px solid var(--surface-hover);
  background: var(--surface); color: var(--text-sub); cursor: pointer; font-size: 0.9rem; flex: none;
}
.ws-more { text-align: center; padding: 12px; flex: none; }
.ws-modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 200; }
.ws-modal { background: var(--surface-panel); border-radius: 14px; padding: 24px; width: 360px; max-width: 90vw; }
.ws-modal h3 { margin-bottom: 6px; }
.ws-modal-hint { font-size: 0.8rem; color: var(--text-muted); margin-bottom: 14px; }
.ws-field { display: flex; flex-direction: column; gap: 5px; font-size: 0.8rem; color: var(--text-sub); margin-bottom: 12px; }
.ws-field input {
  padding: 10px 12px; border-radius: 8px; border: 1px solid var(--surface-hover);
  background: var(--surface); color: var(--text-main); font-size: 0.9rem;
}
.ws-field input:focus { outline: none; border-color: var(--accent); }
.ws-modal-actions { display: flex; gap: 10px; margin-top: 6px; }
.ws-pop-enter-active, .ws-pop-leave-active { transition: opacity 0.18s; }
.ws-pop-enter-from, .ws-pop-leave-to { opacity: 0; }
</style>
