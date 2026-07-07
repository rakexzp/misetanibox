<template>
  <section class="traffic-card">
    <div class="traffic-head">
      <div class="title-wrap">
        <h3>Сетевой трафик</h3>
      </div>

      <div
        class="outbound-ip"
        :title="outboundIPTitle"
        @click="refreshOutboundIPRouteAware('manual')"
      >
        <span class="ip-label">Текущий исходящий IP</span>
        <span class="ip-value" :class="{ detecting: !outboundIPText || outboundIPText === 'Ошибка проверки' }">
          {{ outboundIPText }}
        </span>
        <span v-if="globalState.ipDetecting && outboundIPHasValue" class="ip-refreshing">Обновление…</span>
      </div>

      <button class="reset-btn" @click="handleReset">
        Сброс
      </button>
    </div>

    <div class="wave-row">
      <!-- 上传 -->
      <div class="wave-col">
        <div class="wave-label-row">
          <span class="speed-label">Скорость отдачи</span>
          <strong class="speed-val">{{ traffic.up }}</strong>
        </div>
        <div class="wave-box">
          <svg class="wave-svg" viewBox="0 0 320 100" preserveAspectRatio="none">
            <path class="wave-area" :d="uploadAreaPath" />
          </svg>
        </div>
        <span class="total-label">Всего {{ traffic.uploadTotal || '0 B' }}</span>
      </div>

      <!-- 下载 -->
      <div class="wave-col">
        <div class="wave-label-row">
          <span class="speed-label">Скорость загрузки</span>
          <strong class="speed-val">{{ traffic.down }}</strong>
        </div>
        <div class="wave-box">
          <svg class="wave-svg" viewBox="0 0 320 100" preserveAspectRatio="none">
            <path class="wave-area" :d="downloadAreaPath" />
          </svg>
        </div>
        <span class="total-label">Всего {{ traffic.downloadTotal || '0 B' }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { globalState, refreshOutboundIPRouteAware } from '../store';
import {
  waveState,
  resetWaveState,
  buildMonotoneAreaPath,
} from '../trafficWaveState';

type TrafficSnapshot = {
  up: string;
  down: string;
  upRaw?: number;
  downRaw?: number;
  uploadTotal?: string;
  downloadTotal?: string;
  uploadTotalRaw?: number;
  downloadTotalRaw?: number;
};

const props = defineProps<{
  traffic: TrafficSnapshot;
}>();

const outboundIPHasValue = computed(() => {
  return !!(globalState.outboundIP?.preferred);
});

const outboundIPText = computed(() => {
  const r = globalState.outboundIP;

  if (!r) {
    return globalState.ipDetecting ? 'Проверка…' : 'Не проверено';
  }

  if (!r.preferred) {
    return globalState.ipDetecting ? 'Проверка…' : 'Ошибка проверки';
  }

  return r.preferred;
});

const outboundIPTitle = computed(() => {
  if (!globalState.outboundIP) return '';

  const r = globalState.outboundIP;

  const modeText =
    r.mode === 'proxy'
      ? 'проверка через прокси'
      : r.mode === 'tun-route'
        ? 'проверка через TUN-маршрут'
        : 'прямая проверка';

  const lines = [
    `Режим: ${modeText}`,
    `IPv6: ${r.ipv6 || 'недоступно'}`,
    `IPv4: ${r.ipv4 || 'недоступно'}`,
  ];

  if (r.source) {
    lines.push(`Источник: ${r.source}`);
  }

  if (r.message && !r.preferred) {
    lines.push(`Статус: ${r.message}`);
  }

  return lines.join('\n');
});

const uploadAreaPath = computed(() =>
  buildMonotoneAreaPath(waveState.smoothedUploadRatios)
);

const downloadAreaPath = computed(() =>
  buildMonotoneAreaPath(waveState.smoothedDownloadRatios)
);

const handleReset = async () => {
  await (API as any).ResetTrafficTotals();
  resetWaveState();
};
</script>

<style scoped>
.traffic-card {
  padding: 24px 28px;
  border-radius: 20px;
  background: var(--surface);
  border: none;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.03);
}

.traffic-head {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  margin-bottom: 20px;
}

.title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  justify-self: start;
}

.title-wrap h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-main);
}

.outbound-ip {
  justify-self: center;
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 8px;
  transition: 0.2s;
}

.outbound-ip:hover {
  background: var(--surface-hover);
}

.ip-label {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
  opacity: 1;
  white-space: nowrap;
}

.ip-value {
  font-family: var(--font-mono);
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: inline-block;
  vertical-align: bottom;
  font-synthesis-weight: none;
  text-rendering: geometricPrecision;
}

.ip-value.detecting {
  font-family: var(--font-sans);
  color: var(--text-muted);
  font-weight: 600;
}

.ip-refreshing {
  font-family: var(--font-sans);
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
  margin-left: 4px;
  font-synthesis-weight: none;
}

.reset-btn {
  border: none;
  background: var(--text-main);
  color: var(--accent-fg);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  transition: 0.2s ease;
  justify-self: end;
}

.reset-btn:hover {
  opacity: 0.85;
}

.wave-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.wave-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.wave-label-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.speed-label {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
  letter-spacing: 0.05em;
}

.speed-val {
  font-family: var(--font-mono);
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-main);
  font-variant-numeric: tabular-nums;
}

.wave-box {
  height: 128px;
  overflow: hidden;
  border-radius: 10px;
  background: var(--surface-panel);
}

.wave-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.wave-area {
  opacity: 1;
  fill: var(--text-main);
}

.total-label {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
</style>
