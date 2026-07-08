<template>
  <div class="app-routing">
        <div class="ar-modes">
      <button
        v-for="m in modeOptions"
        :key="m.value"
        class="ar-mode-btn"
        :class="{ active: mode === m.value }"
        @click="mode = m.value"
      >
        <span class="ar-mode-title">{{ m.title }}</span>
        <span class="ar-mode-desc">{{ m.desc }}</span>
      </button>
    </div>

        <template v-if="mode !== 'off'">
      <div class="ar-toolbar">
        <div class="search-bar compact">
          <span v-html="ICONS.search"></span>
          <input v-model="search" placeholder="Поиск приложений..." />
        </div>
        <button class="action-btn ar-browse" @click="browseExe" :disabled="loading">
          <span class="btn-icon" v-html="ICONS.plus"></span> Выбрать .exe
        </button>
      </div>

      <div class="ar-hint">
        Отмечено: {{ selected.size }}.
        {{ mode === 'blacklist'
          ? 'Отмеченные приложения пойдут напрямую, мимо VPN.'
          : 'Через VPN пойдут только отмеченные приложения, остальное — напрямую.' }}
        <br />
        <span class="ar-hint-dim">При включении автоматически используется режим маршрутизации по правилам (глобальный/прямой игнорируют правила приложений).</span>
      </div>

      <div class="ar-list">
        <div v-if="loading" class="ar-empty">Сканирование приложений…</div>
        <div v-else-if="!filteredApps.length" class="ar-empty">
          {{ search ? 'Ничего не найдено' : 'Приложения не найдены' }}
        </div>

        <label
          v-for="app in filteredApps"
          :key="app.path || app.exe"
          class="ar-item"
          :class="{ checked: selected.has(app.exe) }"
        >
          <img v-if="app.iconPng" class="ar-icon" :src="'data:image/png;base64,' + app.iconPng" alt="" />
          <span v-else class="ar-icon ar-icon-fallback" v-html="ICONS.rules"></span>
          <span class="ar-meta">
            <span class="ar-name truncate" :title="app.name">{{ app.name }}</span>
            <span class="ar-exe truncate" :title="app.path">{{ app.exe }}</span>
          </span>
          <input type="checkbox" :checked="selected.has(app.exe)" @change="toggle(app.exe)" />
        </label>
      </div>

      <div class="ar-footer">
        <span class="ar-note">Изменения применяются при следующем запуске ядра (или сразу, если оно уже работает).</span>
        <button class="primary-btn accent-btn" @click="apply" :disabled="loading || saving">
          {{ saving ? 'Применение…' : 'Применить' }}
        </button>
      </div>
    </template>

    <div v-else class="ar-off">
      Маршрутизация по приложениям выключена. Весь трафик идёт по правилам подписки.
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { showAlert } from '../store';
import { ICONS } from '../utils/icons';

type Mode = 'off' | 'blacklist' | 'whitelist';
interface AppRow { name: string; exe: string; path: string; iconPng: string }

const props = defineProps<{ configId: string }>();

const mode = ref<Mode>('off');
const apps = ref<AppRow[]>([]);
const selected = reactive(new Set<string>());
const search = ref('');
const loading = ref(false);
const saving = ref(false);

const modeOptions: { value: Mode; title: string; desc: string }[] = [
  { value: 'off', title: 'Выключено', desc: 'Как в подписке' },
  { value: 'blacklist', title: 'Мимо VPN', desc: 'Выбранные — напрямую' },
  { value: 'whitelist', title: 'Только через VPN', desc: 'Только выбранные' },
];

const filteredApps = computed(() => {
  const q = search.value.trim().toLowerCase();
  const list = q
    ? apps.value.filter(a => a.name.toLowerCase().includes(q) || a.exe.includes(q))
    : apps.value;
  return [...list].sort((a, b) => {
    const sa = selected.has(a.exe) ? 0 : 1;
    const sb = selected.has(b.exe) ? 0 : 1;
    if (sa !== sb) return sa - sb;
    return a.name.localeCompare(b.name, 'ru');
  });
});

function toggle(exe: string) {
  if (selected.has(exe)) selected.delete(exe);
  else selected.add(exe);
}

function ensureApp(a: AppRow) {
  if (!apps.value.some(x => x.exe === a.exe)) apps.value.push(a);
}

async function loadApps() {
  loading.value = true;
  try {
    const installed = await API.ListInstalledApps();
    apps.value = (installed || []).map((a: any) => ({
      name: a.name, exe: a.exe, path: a.path, iconPng: a.iconPng,
    }));
    selected.forEach(exe => {
      if (!apps.value.some(a => a.exe === exe)) {
        apps.value.push({ name: exe, exe, path: '', iconPng: '' });
      }
    });
  } catch (e) {
    await showAlert('Не удалось получить список приложений: ' + e, 'Ошибка', true);
  } finally {
    loading.value = false;
  }
}

async function loadRouting() {
  try {
    const ar: any = await API.GetAppRouting(props.configId);
    mode.value = (ar?.mode as Mode) || 'off';
    selected.clear();
    (ar?.apps || []).forEach((e: string) => selected.add(e.toLowerCase()));
  } catch {
    mode.value = 'off';
  }
}

async function browseExe() {
  try {
    const a: any = await API.SelectAppExe();
    if (a && a.exe) {
      ensureApp({ name: a.name, exe: a.exe, path: a.path, iconPng: a.iconPng });
      selected.add(a.exe);
    }
  } catch (e) {
    await showAlert('Не удалось добавить приложение: ' + e, 'Ошибка', true);
  }
}

async function apply() {
  saving.value = true;
  try {
    await API.SetAppRouting(props.configId, mode.value, Array.from(selected));
    await showAlert('Настройки маршрутизации по приложениям сохранены.', 'Готово');
  } catch (e) {
    await showAlert('Не удалось сохранить: ' + e, 'Ошибка', true);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  await loadRouting();
  await loadApps();
});

watch(mode, (m) => {
  if (m !== 'off' && !apps.value.length && !loading.value) loadApps();
});
</script>

<style scoped>
.app-routing { display: flex; flex-direction: column; gap: 14px; padding: 4px 2px 20px; }

.ar-modes { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.ar-mode-btn {
  display: flex; flex-direction: column; align-items: flex-start; gap: 4px;
  padding: 12px 14px; border-radius: 12px; cursor: pointer; text-align: left;
  background: var(--surface); border: 1.5px solid var(--surface-hover);
  transition: border-color 0.15s, background 0.15s;
}
.ar-mode-btn:hover { border-color: var(--text-muted); }
.ar-mode-btn.active { border-color: var(--accent); background: var(--surface-hover, var(--surface)); }
.ar-mode-title { font-weight: 700; font-size: 0.95rem; color: var(--text-main); }
.ar-mode-desc { font-size: 0.78rem; color: var(--text-muted); }

.ar-toolbar { display: flex; gap: 10px; align-items: center; }
.ar-browse { white-space: nowrap; }

.search-bar { display: flex; align-items: center; background: var(--surface); border: 1px solid var(--surface-hover); border-radius: 8px; padding: 8px 12px; flex: 1; }
.search-bar :deep(svg) { width: 16px; height: 16px; flex-shrink: 0; color: var(--text-muted); }
.search-bar input { border: none; background: transparent; color: var(--text-main); outline: none; margin-left: 8px; width: 100%; font-size: 0.9rem; }

.ar-hint { font-size: 0.82rem; color: var(--text-sub); line-height: 1.5; }
.ar-hint-dim { color: var(--text-muted); font-size: 0.78rem; }

.ar-list {
  display: flex; flex-direction: column; gap: 6px;
  max-height: 46vh; overflow-y: auto; padding-right: 4px;
}
.ar-empty { padding: 28px 0; text-align: center; color: var(--text-muted); font-size: 0.9rem; }

.ar-item {
  display: flex; align-items: center; gap: 12px; padding: 8px 12px;
  border-radius: 10px; background: var(--surface); border: 1.5px solid transparent;
  cursor: pointer; transition: border-color 0.12s, background 0.12s;
}
.ar-item:hover { background: var(--surface-hover, var(--surface)); }
.ar-item.checked { border-color: var(--accent); }

.ar-icon { width: 28px; height: 28px; flex-shrink: 0; object-fit: contain; }
.ar-icon-fallback { display: flex; align-items: center; justify-content: center; color: var(--text-muted); }
.ar-icon-fallback :deep(svg) { width: 20px; height: 20px; }

.ar-meta { display: flex; flex-direction: column; min-width: 0; flex: 1; }
.ar-name { font-size: 0.9rem; color: var(--text-main); font-weight: 600; }
.ar-exe { font-size: 0.76rem; color: var(--text-muted); font-family: var(--font-mono); }

.ar-item input[type="checkbox"] { width: 18px; height: 18px; flex-shrink: 0; accent-color: var(--accent); cursor: pointer; }

.ar-footer {
  display: flex; align-items: center; justify-content: space-between; gap: 14px;
  padding-top: 6px; border-top: 1px solid var(--surface-hover);
}
.ar-note { font-size: 0.76rem; color: var(--text-muted); line-height: 1.4; }

.ar-off { padding: 40px 0; text-align: center; color: var(--text-muted); font-size: 0.92rem; }

.truncate { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
