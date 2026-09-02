<template>
  <div class="chains">
    <div class="ch-head">
      <div>
        <h3 class="ch-title">Цепочки прокси</h3>
        <p class="ch-sub">Трафик идёт через несколько серверов подряд: вход → промежуточные → выход. Реализовано через dialer-proxy mihomo.</p>
      </div>
      <div class="ch-head-actions">
        <button class="action-btn warp-btn" :class="{ on: warpEnabled }" :disabled="warpBusy" @click="toggleWarp"
          :title="'Cloudflare WARP как узел-выход — добавь его в цепочку последним'">
          {{ warpBusy ? 'WARP…' : (warpEnabled ? '✓ WARP-выход' : '+ WARP-выход') }}
        </button>
        <button v-if="!editing" class="primary-btn accent-btn" @click="newChain">+ Создать цепочку</button>
      </div>
    </div>

    <!-- редактор -->
    <div v-if="editing" class="ch-editor glass-card">
      <input v-model="draft.name" class="ch-name-input" maxlength="32" placeholder="Название цепочки (напр. NL → DE)" />
      <div class="ch-cols">
        <div class="ch-col">
          <div class="ch-col-head">
            <span>Доступные узлы</span>
            <input v-model="search" class="ch-search" placeholder="Поиск…" />
          </div>
          <div class="ch-list">
            <div v-for="n in filteredNodes" :key="n" class="ch-node" @click="addNode(n)">
              <span class="truncate">{{ n }}</span><span class="ch-add">+</span>
            </div>
            <div v-if="!filteredNodes.length" class="ch-empty">Нет узлов. Подключись/загрузи подписку.</div>
          </div>
        </div>
        <div class="ch-col">
          <div class="ch-col-head"><span>Цепочка ({{ draft.nodes.length }})</span></div>
          <div class="ch-list">
            <div v-for="(n, i) in draft.nodes" :key="i" class="ch-node chosen">
              <span class="ch-idx">{{ i === 0 ? 'ВХОД' : (i === draft.nodes.length - 1 ? 'ВЫХОД' : i + 1) }}</span>
              <span class="truncate">{{ n }}</span>
              <span class="ch-ctrls">
                <button @click="move(i, -1)" :disabled="i === 0" title="Выше">↑</button>
                <button @click="move(i, 1)" :disabled="i === draft.nodes.length - 1" title="Ниже">↓</button>
                <button @click="removeNode(i)" title="Убрать">✕</button>
              </span>
            </div>
            <div v-if="!draft.nodes.length" class="ch-empty">Добавь узлы слева по порядку (минимум 2).</div>
          </div>
        </div>
      </div>
      <div class="ch-editor-actions">
        <button class="action-btn" @click="cancelEdit">Отмена</button>
        <button class="primary-btn accent-btn" :disabled="!canSave" @click="saveChain">Сохранить цепочку</button>
      </div>
    </div>

    <!-- список цепочек -->
    <div v-else class="ch-cards">
      <div v-if="!chains.length" class="ch-empty-big">Пока нет цепочек. Создай первую — она появится в списке серверов как «🔗 …».</div>
      <div v-for="(c, i) in chains" :key="i" class="ch-card glass-card">
        <div class="ch-card-name">🔗 {{ c.name }}</div>
        <div class="ch-card-path">
          <template v-for="(n, j) in c.nodes" :key="j">
            <span class="ch-hop">{{ shortName(n) }}</span>
            <span v-if="j < c.nodes.length - 1" class="ch-arrow">→</span>
          </template>
        </div>
        <div class="ch-card-actions">
          <button class="action-btn" @click="editChain(i)">Изменить</button>
          <button class="action-btn danger" @click="deleteChain(i)">Удалить</button>
        </div>
      </div>
    </div>

    <p v-if="!editing" class="ch-note">
      Цепочки появляются в «Прокси-узлы» как <b>🔗 &lt;имя&gt;</b> — выбери их как обычный сервер. Узлы должны существовать в активной подписке. UDP через цепочку работает не на всех протоколах.
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { showAlert, showConfirm } from '../store';

const GROUP_TYPES = new Set(['selector', 'urltest', 'url-test', 'fallback', 'loadbalance', 'load-balance',
  'relay', 'smart', 'direct', 'reject', 'reject-drop', 'rejectdrop', 'pass', 'compatible', 'dns']);

const WARP_NODE = 'WARP';

const chains = ref<{ name: string; nodes: string[] }[]>([]);
const nodes = ref<string[]>([]);
const editing = ref(false);
const editIndex = ref(-1);
const draft = ref<{ name: string; nodes: string[] }>({ name: '', nodes: [] });
const search = ref('');
const warpEnabled = ref(false);
const warpBusy = ref(false);

// WARP-узел показываем в списке выбора первым, когда включён
const availableNodes = computed(() => {
  const base = nodes.value.filter((n) => n !== WARP_NODE);
  return warpEnabled.value ? [WARP_NODE, ...base] : base;
});
const filteredNodes = computed(() => {
  const q = search.value.trim().toLowerCase();
  return availableNodes.value.filter((n) => !q || n.toLowerCase().includes(q));
});
const canSave = computed(() => draft.value.name.trim().length > 0 && draft.value.nodes.length >= 2);

function shortName(n: string) { return n.length > 16 ? n.slice(0, 15) + '…' : n; }

async function loadNodes() {
  try {
    const data: any = await API.GetInitialData();
    const groups = data?.groups || {};
    const set = new Set<string>();
    for (const [name, info] of Object.entries<any>(groups)) {
      const type = String(info?.type || '').toLowerCase();
      if (!GROUP_TYPES.has(type)) set.add(name);
      // также вытащить одиночные узлы из .all, если сами по себе не группы
      for (const child of (info?.all || [])) {
        const ci = groups[child];
        const ct = String(ci?.type || '').toLowerCase();
        if (ci && !GROUP_TYPES.has(ct)) set.add(child);
        else if (!ci) set.add(child);
      }
    }
    // убрать имена самих цепочек-выходов (🔗) и внутренних хопов (⛓)
    nodes.value = [...set].filter((n) => !n.startsWith('🔗') && !n.startsWith('⛓')).sort();
  } catch { nodes.value = []; }
}
async function loadChains() {
  try { chains.value = (await API.GetProxyChains()) || []; } catch { chains.value = []; }
}

function newChain() { draft.value = { name: '', nodes: [] }; editIndex.value = -1; search.value = ''; editing.value = true; }
function editChain(i: number) { draft.value = { name: chains.value[i].name, nodes: [...chains.value[i].nodes] }; editIndex.value = i; search.value = ''; editing.value = true; }
function cancelEdit() { editing.value = false; }
function addNode(n: string) { draft.value.nodes.push(n); }
function removeNode(i: number) { draft.value.nodes.splice(i, 1); }
function move(i: number, dir: number) {
  const j = i + dir;
  if (j < 0 || j >= draft.value.nodes.length) return;
  const arr = draft.value.nodes;
  [arr[i], arr[j]] = [arr[j], arr[i]];
}

async function toggleWarp() {
  warpBusy.value = true;
  try {
    if (warpEnabled.value) {
      await API.DisableWarp();
      warpEnabled.value = false;
    } else {
      await API.EnableWarp();
      warpEnabled.value = true;
      await showAlert('WARP-выход включён. Добавь узел «WARP» в цепочку последним — трафик будет выходить через Cloudflare WARP.', 'Готово');
    }
  } catch (e) {
    await showAlert('Не удалось переключить WARP: ' + e, 'Ошибка', true);
  } finally {
    warpBusy.value = false;
  }
}

async function persist() {
  await API.SaveProxyChains(chains.value as any);
}
async function saveChain() {
  const c = { name: draft.value.name.trim(), nodes: draft.value.nodes.slice() };
  if (editIndex.value >= 0) chains.value[editIndex.value] = c;
  else chains.value.push(c);
  try {
    await persist();
    editing.value = false;
    await showAlert('Цепочка сохранена. Она появится в «Прокси-узлы» как «🔗 ' + c.name + '».', 'Готово');
  } catch (e) { await showAlert('Не удалось сохранить: ' + e, 'Ошибка', true); }
}
async function deleteChain(i: number) {
  if (!(await showConfirm('Удалить цепочку «' + chains.value[i].name + '»?', 'Удаление'))) return;
  chains.value.splice(i, 1);
  try { await persist(); } catch (e) { await showAlert('Ошибка: ' + e, 'Ошибка', true); }
}

async function loadWarp() {
  try { warpEnabled.value = await API.GetWarpEnabled(); } catch { warpEnabled.value = false; }
}

onMounted(() => { loadChains(); loadNodes(); loadWarp(); });
</script>

<style scoped>
.chains { height: 100%; display: flex; flex-direction: column; overflow-y: auto; }
.ch-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 18px; }
.ch-title { font-size: 1.15rem; }
.ch-sub { color: var(--text-muted); font-size: 0.82rem; margin-top: 4px; max-width: 560px; }
.glass-card { background: var(--surface-panel); border: 1px solid var(--surface); border-radius: 14px; padding: 18px; }
.ch-editor { margin-bottom: 16px; }
.ch-name-input {
  width: 100%; padding: 12px 14px; border-radius: 9px; border: 1px solid var(--surface-hover);
  background: var(--surface); color: var(--text-main); font-size: 1rem; margin-bottom: 14px;
}
.ch-name-input:focus { outline: none; border-color: var(--accent); }
.ch-cols { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.ch-col { display: flex; flex-direction: column; min-height: 240px; }
.ch-col-head { display: flex; justify-content: space-between; align-items: center; font-size: 0.78rem; color: var(--text-sub); font-weight: 700; margin-bottom: 8px; text-transform: uppercase; letter-spacing: 0.04em; }
.ch-search { padding: 5px 10px; border-radius: 7px; border: 1px solid var(--surface-hover); background: var(--surface); color: var(--text-main); font-size: 0.8rem; width: 130px; }
.ch-list { flex: 1; overflow-y: auto; border: 1px solid var(--surface); border-radius: 9px; background: var(--app-bg); max-height: 300px; }
.ch-node { display: flex; align-items: center; gap: 8px; padding: 9px 12px; border-bottom: 1px solid var(--surface); cursor: pointer; font-size: 0.85rem; }
.ch-node:hover { background: var(--surface); }
.ch-node.chosen { cursor: default; }
.ch-add { margin-left: auto; color: var(--accent); font-weight: 700; }
.ch-idx { font-size: 0.62rem; font-weight: 800; color: var(--accent); min-width: 40px; }
.ch-ctrls { margin-left: auto; display: flex; gap: 3px; }
.ch-ctrls button { width: 24px; height: 24px; border-radius: 6px; border: 1px solid var(--surface-hover); background: var(--surface); color: var(--text-sub); cursor: pointer; font-size: 0.75rem; }
.ch-ctrls button:disabled { opacity: 0.35; cursor: default; }
.ch-empty, .ch-empty-big { color: var(--text-muted); font-size: 0.82rem; padding: 16px; text-align: center; }
.ch-empty-big { grid-column: 1 / -1; padding: 44px 24px; font-size: 0.9rem; border: 1px dashed var(--surface-hover); border-radius: 14px; background: var(--surface); }
.ch-editor-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 14px; }
.ch-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 14px; }
.ch-card-name { font-weight: 700; margin-bottom: 8px; }
.ch-card-path { display: flex; flex-wrap: wrap; align-items: center; gap: 5px; font-size: 0.78rem; color: var(--text-sub); margin-bottom: 12px; }
.ch-hop { background: var(--surface); padding: 3px 8px; border-radius: 6px; }
.ch-arrow { color: var(--accent); }
.ch-card-actions { display: flex; gap: 8px; }
.action-btn.danger { color: #d9534f; }
.ch-note { color: var(--text-sub); font-size: 0.78rem; margin-top: 16px; line-height: 1.6; padding: 12px 14px; border-radius: 10px; background: var(--surface); }
.ch-head-actions { display: flex; gap: 10px; align-items: center; flex: none; }
.warp-btn { border: 1px solid var(--surface-hover); background: var(--surface); color: var(--text-sub); border-radius: 9px; padding: 8px 14px; cursor: pointer; font-size: 0.82rem; white-space: nowrap; }
.warp-btn:hover:not(:disabled) { border-color: var(--accent); }
.warp-btn.on { border-color: var(--accent); color: var(--accent); }
.warp-btn:disabled { opacity: 0.6; cursor: default; }
</style>
