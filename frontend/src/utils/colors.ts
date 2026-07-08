// Пользовательская покраска: 3 цвета (акцент/текст/фон) → полная палитра CSS-переменных.
// Применяется поверх light/dark инлайн на :root (перебивает тему). Сброс = убрать переопределения.

export type CustomColors = { accent: string; text: string; bg: string };

const STORAGE_KEY = 'mise_colors';

function clamp(n: number) { return Math.max(0, Math.min(255, Math.round(n))); }

function hexToRgb(hex: string): [number, number, number] {
  let h = hex.replace('#', '');
  if (h.length === 3) h = h.split('').map((c) => c + c).join('');
  const n = parseInt(h, 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
function rgbToHex(r: number, g: number, b: number) {
  return '#' + [r, g, b].map((v) => clamp(v).toString(16).padStart(2, '0')).join('');
}
// смешать два цвета: a на (1-t), b на t
function mix(a: string, b: string, t: number) {
  const [r1, g1, b1] = hexToRgb(a);
  const [r2, g2, b2] = hexToRgb(b);
  return rgbToHex(r1 + (r2 - r1) * t, g1 + (g2 - g1) * t, b1 + (b2 - b1) * t);
}
function luminance(hex: string) {
  const [r, g, b] = hexToRgb(hex).map((v) => v / 255);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
function contrastFg(hex: string) { return luminance(hex) > 0.55 ? '#111111' : '#FFFFFF'; }

// вывести все переменные интерфейса из трёх опорных цветов
export function derivePalette(c: CustomColors): Record<string, string> {
  const { accent, text, bg } = c;
  return {
    '--app-bg': bg,
    '--surface-panel': mix(bg, text, 0.05),
    '--surface': mix(bg, text, 0.09),
    '--surface-hover': mix(bg, text, 0.14),
    '--text-main': text,
    '--text-sub': mix(text, bg, 0.42),
    '--text-muted': mix(text, bg, 0.66),
    '--accent': accent,
    '--accent-fg': contrastFg(accent),
    '--status-online': accent,
  };
}

const VARS = Object.keys(derivePalette({ accent: '#000', text: '#000', bg: '#fff' }));

export function applyColors(c: CustomColors | null) {
  const root = document.documentElement;
  if (!c) {
    VARS.forEach((v) => root.style.removeProperty(v));
    return;
  }
  const p = derivePalette(c);
  Object.entries(p).forEach(([k, v]) => root.style.setProperty(k, v));
}

export function getSavedColors(): CustomColors | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch { return null; }
}
export function saveColors(c: CustomColors | null) {
  if (c) localStorage.setItem(STORAGE_KEY, JSON.stringify(c));
  else localStorage.removeItem(STORAGE_KEY);
  applyColors(c);
}

// применить сохранённое при старте
export function initColors() { applyColors(getSavedColors()); }
