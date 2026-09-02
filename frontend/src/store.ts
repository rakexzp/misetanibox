import { reactive } from 'vue';
import { EventsOn } from '../wailsjs/runtime/runtime';
import * as API from '../wailsjs/go/main/App';
import { runtimeassets } from '../wailsjs/go/models';

export type UpdateTaskState = {
  key: string;
  title: string;
  status: string;
  stage: string;
  progress: number;
  bytesDone: number;
  bytesTotal: number;
  speedBps: number;
  etaSeconds: number;
  error: string;
  startedAt: number;
  finishedAt: number;
};

export type OutboundIPResult = {
  preferred: string;
  ipv4: string;
  ipv6: string;
  mode: string;
  source: string;
  source4: string;
  source6: string;
  message: string;
  message4: string;
  message6: string;
  complete: boolean;
};

export function normalizeOutboundIP(raw: any): OutboundIPResult {
  return {
    preferred: raw?.preferred || '',
    ipv4: raw?.ipv4 || '',
    ipv6: raw?.ipv6 || '',
    mode: raw?.mode || '',
    source: raw?.source || '',
    source4: raw?.source4 || '',
    source6: raw?.source6 || '',
    message: raw?.message || '',
    message4: raw?.message4 || '',
    message6: raw?.message6 || '',
    complete: raw?.complete ?? false,
  };
}

const cachedHideLogs = localStorage.getItem('goclashz_hideLogs') === 'true';
const cachedTheme = localStorage.getItem('goclashz_theme') || 'light';
const cachedUiMode = (localStorage.getItem('goclashz_uiMode') as 'full' | 'lite') || 'lite';
const cachedActiveConfigId = localStorage.getItem('goclashz_activeConfigId') || ''; // 👈 新增缓存预热
const cachedOutboundIP = localStorage.getItem('goclashz_outboundIP');

let initialOutboundIP: OutboundIPResult | null = null;
try {
  initialOutboundIP = cachedOutboundIP
    ? normalizeOutboundIP(JSON.parse(cachedOutboundIP))
    : null;
} catch {
  initialOutboundIP = null;
}

const delayTimers: Record<string, number> = {};

function readControlIntent(): { systemProxy?: boolean; tun?: boolean } | null {
  try {
    const raw = sessionStorage.getItem('goclashz_control_intent');
    return raw ? JSON.parse(raw) : null;
  } catch { return null; }
}

function persistControlIntent() {
  sessionStorage.setItem('goclashz_control_intent', JSON.stringify({
    systemProxy: globalState.systemProxy,
    tun: globalState.tun,
  }));
}

export function setSystemProxyIntent(value: boolean) {
  globalState.systemProxy = value;
  persistControlIntent();
}

export function setTunIntent(value: boolean) {
  globalState.tun = value;
  persistControlIntent();
}

export function setUiMode(mode: 'full' | 'lite') {
  globalState.uiMode = mode;
  localStorage.setItem('goclashz_uiMode', mode);
}

export function setCardBg(key: string, dataUrl: string) {
  if (dataUrl) globalState.cardBgs[key] = dataUrl;
  else delete globalState.cardBgs[key];
}

const cachedIntent = readControlIntent();

export const globalState = reactive({
  isRunning: false,
  platform: '' as string, // 'windows' | 'linux' — прячем неприменимые разделы
  mode: 'rule',
  theme: cachedTheme,
  uiMode: cachedUiMode as 'full' | 'lite', // full = Про, lite = Happ-подобный простой режим
  cardBgs: {} as Record<string, string>, // пер-карточные фоны (key → data-URI), грузятся с бэкенда
  hideLogs: cachedHideLogs,
  systemProxy: cachedIntent?.systemProxy ?? false,
  tun: cachedIntent?.tun ?? false,
  actualSystemProxy: false,
  actualTun: false,
  systemProxyPending: false,
  tunPending: false,
  controlStateHydrated: false,
  version: '',
  appVersion: '',
  logLevel: 'error',
  appLogLevel: 'info',
  isAdmin: false,
  tunStatus: { hasWintun: false, isAdmin: false },
  assetStatus: null as runtimeassets.RuntimeAssetStatus | null,
  delayRetention: true,
  delayRetentionTime: 'long',

  updateReady: false,
  newAppVersion: '',
  updateDownloaded: false,
  downloadedPath: '',
  appUpdateChecking: false, // 👈 新增：跟踪软件更新检查状态

  activeConfigId: cachedActiveConfigId,
  activeConfigName: '',
  activeConfigType: '',

  proxyDelays: {} as Record<string, { delay: number | null, status: string, message: string }>,

  outboundIP: initialOutboundIP as OutboundIPResult | null,
  outboundIPStale: false,
  ipDetecting: false,
  appUpdateProgress: null as {
    totalBytes: number;
    bytesDone: number;
    speedBps: number;
    etaSec: number;
    isDownloaded?: boolean;
    version?: string;
    path?: string;
  } | null,

  componentUpdate: {
    tasks: {} as Record<string, UpdateTaskState>,
    checkingCoreUpdate: false,
    pendingCoreUpdate: false,
    updatingCore: false,
    coreUpdateInfo: { local: '', remote: '', releaseUrl: '' },
    networkBusy: false,
  },

  modal: {
    show: false,
    type: 'alert' as 'alert' | 'confirm',
    title: '',
    message: '',
    detail: '',
    isDanger: false,
    onConfirm: null as Function | null,
    onCancel: null as Function | null,
  }
});

const normalizeLogLevel = (level?: string) => {
  const v = String(level || '').toLowerCase().trim();
  if (v === 'warning') return 'warn';
  if (v === 'trace') return 'debug';
  if (v === 'fatal' || v === 'panic') return 'error';
  if (['debug', 'info', 'warn', 'error'].includes(v)) return v;
  return 'info';
};

export function updateStateFromBackend(rawData: any) {
  if (!rawData) return;

  const oldRunning = globalState.isRunning;

  if (rawData.isRunning !== undefined) globalState.isRunning = rawData.isRunning;
  else if (rawData.IsRunning !== undefined) globalState.isRunning = rawData.IsRunning;

  if (rawData.platform !== undefined) {
    globalState.platform = rawData.platform;
    document.documentElement.classList.toggle('mac', rawData.platform === 'darwin');
  }
  if (rawData.mode !== undefined) globalState.mode = rawData.mode;
  else if (rawData.Mode !== undefined) globalState.mode = rawData.Mode;

  const newTheme = rawData.theme ?? rawData.Theme;
  if (newTheme !== undefined) {
    globalState.theme = newTheme;
    localStorage.setItem('goclashz_theme', newTheme);
  }

  const newHideLogs = rawData.hideLogs ?? rawData.HideLogs;
  if (newHideLogs !== undefined) {
    globalState.hideLogs = newHideLogs;
    localStorage.setItem('goclashz_hideLogs', String(newHideLogs));
  }

  const newLogLevel = rawData.logLevel ?? rawData.LogLevel;
  if (newLogLevel !== undefined) {
    globalState.logLevel = normalizeLogLevel(newLogLevel || 'error');
  }

  const newAppLogLevel = rawData.appLogLevel ?? rawData.AppLogLevel;
  if (newAppLogLevel !== undefined) {
    globalState.appLogLevel = normalizeLogLevel(newAppLogLevel || 'info');
  }

  const oldActualProxy = globalState.actualSystemProxy;
  const oldActualTun = globalState.actualTun;

  const newProxy = rawData.actualSystemProxy ?? rawData.systemProxy ?? rawData.SystemProxy;
  const newTun = rawData.actualTun ?? rawData.tun ?? rawData.Tun;

  if (newProxy !== undefined) globalState.actualSystemProxy = !!newProxy;
  if (newTun !== undefined) globalState.actualTun = !!newTun;

  if (!globalState.controlStateHydrated) {
    const cached = readControlIntent();
    const desiredProxy = rawData.desiredSystemProxy ?? rawData.DesiredSystemProxy;
    const desiredTun = rawData.desiredTun ?? rawData.DesiredTun;

    if (desiredProxy !== undefined) {
      globalState.systemProxy = !!desiredProxy;
    } else if (!cached && newProxy !== undefined) {
      globalState.systemProxy = !!newProxy;
    }

    if (desiredTun !== undefined) {
      globalState.tun = !!desiredTun;
    } else if (!cached && newTun !== undefined) {
      globalState.tun = !!newTun;
    }

    globalState.controlStateHydrated = true;
    persistControlIntent();
  }

  const proxyChanged = newProxy !== undefined && oldActualProxy !== !!newProxy;
  const tunChanged = newTun !== undefined && oldActualTun !== !!newTun;
  const runningChanged = oldRunning !== globalState.isRunning;

  if (proxyChanged || tunChanged || runningChanged) {
    scheduleRouteAwareIPRefresh('state-change');
  }

  if (rawData.version !== undefined) globalState.version = rawData.version;
  else if (rawData.Version !== undefined) globalState.version = rawData.Version;

  if (rawData.appVersion !== undefined) globalState.appVersion = rawData.appVersion;
  else if (rawData.AppVersion !== undefined) globalState.appVersion = rawData.AppVersion;

  if (rawData.isAdmin !== undefined) globalState.isAdmin = rawData.isAdmin;
  else if (rawData.IsAdmin !== undefined) globalState.isAdmin = rawData.IsAdmin;

  if (rawData.activeConfig !== undefined) {
    globalState.activeConfigId = rawData.activeConfig;
    localStorage.setItem('goclashz_activeConfigId', rawData.activeConfig);
  } else if (rawData.ActiveConfig !== undefined) {
    globalState.activeConfigId = rawData.ActiveConfig;
    localStorage.setItem('goclashz_activeConfigId', rawData.ActiveConfig);
  }

  if (rawData.activeConfigName !== undefined) globalState.activeConfigName = rawData.activeConfigName;
  else if (rawData.ActiveConfigName !== undefined) globalState.activeConfigName = rawData.ActiveConfigName;

  if (rawData.activeConfigType !== undefined) globalState.activeConfigType = rawData.activeConfigType;
  else if (rawData.ActiveConfigType !== undefined) globalState.activeConfigType = rawData.ActiveConfigType;

  if (rawData.delayRetention !== undefined) globalState.delayRetention = rawData.delayRetention;
  else if (rawData.DelayRetention !== undefined) globalState.delayRetention = rawData.DelayRetention;

  if (rawData.delayRetentionTime !== undefined) globalState.delayRetentionTime = rawData.delayRetentionTime;
  else if (rawData.DelayRetentionTime !== undefined) globalState.delayRetentionTime = rawData.DelayRetentionTime;

  if (rawData.updateReady !== undefined) globalState.updateReady = rawData.updateReady;
  else if (rawData.UpdateReady !== undefined) globalState.updateReady = rawData.UpdateReady;

  if (rawData.newAppVersion !== undefined) globalState.newAppVersion = rawData.newAppVersion;
  else if (rawData.NewAppVersion !== undefined) globalState.newAppVersion = rawData.NewAppVersion;

  if (rawData.updateDownloaded !== undefined) globalState.updateDownloaded = rawData.updateDownloaded;
  else if (rawData.UpdateDownloaded !== undefined) globalState.updateDownloaded = rawData.UpdateDownloaded;

  if (rawData.downloadedPath !== undefined) globalState.downloadedPath = rawData.downloadedPath;
  else if (rawData.DownloadedPath !== undefined) globalState.downloadedPath = rawData.DownloadedPath;
}

type DelayRetentionTime = 'long' | '30' | '60' | '300' | string;

export function updateProxyDelay(
  name: string,
  delay: number | null,
  status: string = 'success',
  message: string = '',
  retentionTime: DelayRetentionTime = 'long'
) {
  globalState.proxyDelays[name] = { delay, status, message };

  if (delayTimers[name]) {
    clearTimeout(delayTimers[name]);
    delete delayTimers[name];
  }

  if (delay !== null && retentionTime !== 'long') {
    const seconds = parseInt(retentionTime);
    if (isNaN(seconds) || seconds <= 0) return;

    delayTimers[name] = window.setTimeout(() => {
      if (globalState.proxyDelays[name]) {
        globalState.proxyDelays[name].delay = null;
        globalState.proxyDelays[name].status = 'unknown';
        globalState.proxyDelays[name].message = '';
      }
      delete delayTimers[name];
    }, seconds * 1000);
  }
}

export function showAlert(message: string, title: string = 'Уведомление', isDanger: boolean = false): Promise<void> {
  return new Promise((resolve) => {
    globalState.modal.type = 'alert';
    globalState.modal.title = title;
    globalState.modal.message = message;
    globalState.modal.detail = ''; // Clear detail to prevent state leakage
    globalState.modal.isDanger = isDanger;
    globalState.modal.onConfirm = () => resolve();
    globalState.modal.onCancel = () => resolve(); // Alert 模式下点击遮罩层取消也视为 resolve
    globalState.modal.show = true;
  });
}

export function showConfirm(message: string, title: string = 'Подтверждение', isDanger: boolean = false): Promise<boolean> {
  return new Promise((resolve) => {
    globalState.modal.type = 'confirm';
    globalState.modal.title = title;
    globalState.modal.message = message;
    globalState.modal.detail = ''; // Clear detail to prevent state leakage
    globalState.modal.isDanger = isDanger;
    globalState.modal.onConfirm = () => resolve(true);
    globalState.modal.onCancel = () => resolve(false);
    globalState.modal.show = true;
  });
}

let ipDetectSeq = 0;
let pendingIPRefresh: { force?: boolean; clearBeforeStart?: boolean; reason?: string; silent?: boolean; expectedRoute?: string } | null = null;
const ipRefreshTimers: Record<string, number> = {};

export function scheduleOutboundIPRefresh(
  key: string,
  opts: {
    force?: boolean;
    delay?: number;
    clearBeforeStart?: boolean;
    reason?: string;
    silent?: boolean;
  } = {}
) {
  if (ipRefreshTimers[key]) {
    clearTimeout(ipRefreshTimers[key]);
  }

  if (opts.clearBeforeStart) {
    globalState.outboundIP = null;
  }

  ipRefreshTimers[key] = window.setTimeout(() => {
    delete ipRefreshTimers[key];
    refreshOutboundIP({
      force: opts.force,
      clearBeforeStart: opts.clearBeforeStart,
      reason: opts.reason,
      silent: opts.silent,
    });
  }, opts.delay ?? 800);
}

type OutboundRoute = 'direct' | 'proxy' | 'tun-route' | 'switching';

function getDesiredOutboundRoute(): OutboundRoute {
  if (globalState.tun) return 'tun-route';
  if (globalState.systemProxy) return 'proxy';
  return 'direct';
}

function getActualOutboundRoute(): OutboundRoute {
  if (globalState.actualTun) return 'tun-route';
  if (globalState.actualSystemProxy && globalState.isRunning) return 'proxy';
  return 'direct';
}

function getEffectiveOutboundRoute(): OutboundRoute {
  const desired = getDesiredOutboundRoute();
  const actual = getActualOutboundRoute();
  if (desired !== 'direct' && actual !== desired) return 'switching';
  return actual;
}

let lastEffectiveRoute: OutboundRoute | '' = '';
let routeSettleTimer: number | null = null;

function scheduleRouteAwareIPRefresh(reason: string) {
  const route = getEffectiveOutboundRoute();

  if (route === 'switching') {
    return;
  }

  if (route === lastEffectiveRoute) {
    return;
  }

  lastEffectiveRoute = route;

  if (routeSettleTimer) {
    clearTimeout(routeSettleTimer);
  }

  const delay = route === 'tun-route' ? 1500 : route === 'proxy' ? 800 : 600;

  routeSettleTimer = window.setTimeout(() => {
    routeSettleTimer = null;
    globalState.outboundIPStale = false;
    refreshOutboundIP({
      force: true,
      clearBeforeStart: false,
      reason: `route-${route}:${reason}`,
      silent: false,
      expectedRoute: route,
    });
  }, delay);
}

export function refreshOutboundIPRouteAware(reason = 'manual') {
  const route = getEffectiveOutboundRoute();

  if (route === 'switching') {
    globalState.outboundIPStale = true;
    return;
  }

  return refreshOutboundIP({
    force: true,
    clearBeforeStart: false,
    reason: `route-${route}:${reason}`,
    silent: false,
    expectedRoute: route,
  });
}

export async function refreshOutboundIP(options?: {
  force?: boolean;
  clearBeforeStart?: boolean;
  reason?: string;
  silent?: boolean;
  expectedRoute?: string;
}) {
  if (globalState.ipDetecting) {
    pendingIPRefresh = {
      force: true,
      clearBeforeStart: options?.clearBeforeStart,
      reason: options?.reason,
      silent: options?.silent,
      expectedRoute: options?.expectedRoute,
    };
    return;
  }

  const seq = ++ipDetectSeq;

  if (options?.clearBeforeStart) {
    globalState.outboundIP = null;
  }

  if (!options?.silent) {
    globalState.ipDetecting = true;
  }

  try {
    const result = await (API as any).GetOutboundIPForRoute(
      !!options?.force,
      options?.expectedRoute || ''
    );

    if (seq !== ipDetectSeq) return;

    if (result?.message?.includes('Переключение маршрута')) {
      if (!options?.silent && seq === ipDetectSeq) {
        globalState.ipDetecting = false;
      }
      return;
    }

    if (result && result.preferred) {
      const cleaned = normalizeOutboundIP(result);
      globalState.outboundIP = cleaned;
      globalState.outboundIPStale = false;
      localStorage.setItem('goclashz_outboundIP', JSON.stringify(cleaned));
    } else if (!options?.silent) {
      if (globalState.outboundIP?.preferred) {
        globalState.outboundIPStale = true;
      } else {
        globalState.outboundIP = {
          preferred: '',
          ipv4: '',
          ipv6: '',
          mode: '',
          source: '',
          source4: '',
          source6: '',
          message: result?.message || 'Ошибка проверки',
          message4: '',
          message6: '',
          complete: false,
        };
      }
    }
  } catch {
    if (seq !== ipDetectSeq) return;

    if (!options?.silent) {
      if (globalState.outboundIP?.preferred) {
        globalState.outboundIPStale = true;
      } else {
        globalState.outboundIP = {
          preferred: '',
          ipv4: '',
          ipv6: '',
          mode: '',
          source: '',
          source4: '',
          source6: '',
          message: 'Ошибка сетевого запроса',
          message4: '',
          message6: '',
          complete: false,
        };
      }
    }
  } finally {
    if (!options?.silent && seq === ipDetectSeq) {
      globalState.ipDetecting = false;
    }

    if (pendingIPRefresh) {
      const next = pendingIPRefresh;
      pendingIPRefresh = null;
      setTimeout(() => refreshOutboundIP(next), 0);
    }
  }
}

let storeInited = false;

export async function initStore() {
  if (storeInited) return;
  storeInited = true;
  try {
    const initialState = await API.GetAppState();
    updateStateFromBackend(initialState);
  } catch (err) {
    console.error("初始化应用状态失败:", err);
  }

  EventsOn("app-state-sync", (newState: any) => {
    updateStateFromBackend(newState);
  });

  EventsOn("component-update-tasks-sync", (tasksList: UpdateTaskState[]) => {
    const newTasks: Record<string, UpdateTaskState> = {};
    if (tasksList) {
      tasksList.forEach(t => { newTasks[t.key] = t; });
    }
    globalState.componentUpdate.tasks = newTasks;
  });

  EventsOn("core-update-check-start", () => { globalState.componentUpdate.checkingCoreUpdate = true; });
  EventsOn("core-update-check-error", (err: string) => {
    globalState.componentUpdate.checkingCoreUpdate = false;
  });
  EventsOn("core-update-none", (data: any) => {
    globalState.componentUpdate.checkingCoreUpdate = false;
    globalState.componentUpdate.pendingCoreUpdate = false;
    globalState.componentUpdate.coreUpdateInfo = {
      local: data?.local || '',
      remote: data?.remote || '',
      releaseUrl: ''
    };
  });
  EventsOn("core-update-available", (data: any) => {
    globalState.componentUpdate.checkingCoreUpdate = false;
    globalState.componentUpdate.pendingCoreUpdate = true;
    globalState.componentUpdate.coreUpdateInfo = {
      local: data.local || '',
      remote: data.remote || '',
      releaseUrl: data.releaseUrl || ''
    };
  });

  EventsOn("core-update-start", () => { globalState.componentUpdate.updatingCore = true; });
  EventsOn("core-update-success", () => {
    globalState.componentUpdate.updatingCore = false;
    globalState.componentUpdate.pendingCoreUpdate = false;
  });
  EventsOn("core-update-error", () => {
    globalState.componentUpdate.updatingCore = false;
    globalState.componentUpdate.pendingCoreUpdate = false;
  });
  EventsOn("core-update-cancelled", () => {
    globalState.componentUpdate.updatingCore = false;
  });

  EventsOn("app-update-progress", (progress: any) => {
    globalState.appUpdateProgress = progress;
  });
  EventsOn("app-update-downloaded", (payload: any) => {
    if (globalState.appUpdateProgress) {
      globalState.appUpdateProgress.isDownloaded = true;
      globalState.appUpdateProgress.bytesDone = globalState.appUpdateProgress.totalBytes;
      globalState.appUpdateProgress.version = payload?.version;
      globalState.appUpdateProgress.path = payload?.path;
    } else {
      globalState.appUpdateProgress = {
        totalBytes: 100,
        bytesDone: 100,
        speedBps: 0,
        etaSec: 0,
        isDownloaded: true,
        version: payload?.version,
        path: payload?.path
      };
    }
  });
  EventsOn("app-update-error", () => {
    globalState.appUpdateProgress = null;
  });

  refreshOutboundIP();
  
  EventsOn("core-restarted", () => scheduleOutboundIPRefresh('core-restarted', { force: true, delay: 1500, reason: 'core-restarted' }));
  ['geoip', 'geosite', 'mmdb', 'asn'].forEach(key => {
    EventsOn(`geo-update-${key}-success`, () => scheduleOutboundIPRefresh(`geo-${key}`, { force: true, delay: 1500, reason: `geo-${key}` }));
  });
  EventsOn("core-update-success", () => scheduleOutboundIPRefresh('core-update', { force: true, delay: 1500, reason: 'core-update' }));
  EventsOn("driver-install-success", () => scheduleOutboundIPRefresh('driver-install', { force: true, delay: 1500, reason: 'driver-install' }));

  const clearAllDelays = () => {
    globalState.proxyDelays = {};
    Object.values(delayTimers).forEach(clearTimeout);
    for (const key in delayTimers) delete delayTimers[key];
  };

  EventsOn("delay-cache-clear", clearAllDelays);

  EventsOn("proxy-delay-update", (data: any) => {
    if (!data || !data.name) return;

    if (data.status === 'busy') return;

    const status = data.status || 'success';
    const message = data.message || '';
    const delay = data.delay ?? null;

    const existing = globalState.proxyDelays[data.name];
    if (status !== 'success' && existing && existing.delay && existing.delay > 0) {
      existing.status = status;
      existing.message = message;
      return;
    }

    updateProxyDelay(
      data.name,
      delay,
      status,
      message,
      globalState.delayRetention ? globalState.delayRetentionTime : 'long'
    );
  });
}
