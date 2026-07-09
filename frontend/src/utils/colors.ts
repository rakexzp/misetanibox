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

// осветлить/затемнить цвет (к белому если тёмный, к чёрному если светлый) — для оттенков фона
function shade(hex: string, amt: number) {
  return mix(hex, luminance(hex) < 0.5 ? '#FFFFFF' : '#000000', amt);
}

// вывести все переменные из трёх ОПОРНЫХ цветов, но независимо:
// фон/поверхности — только из bg; буквы — только из text; акцент — сам по себе.
export function derivePalette(c: CustomColors): Record<string, string> {
  const { accent, text, bg } = c;
  return {
    '--app-bg': bg,
    '--surface-panel': shade(bg, 0.06),
    '--surface': shade(bg, 0.11),
    '--surface-hover': shade(bg, 0.16),
    '--text-main': text,
    '--text-sub': mix(text, bg, 0.40),
    '--text-muted': mix(text, bg, 0.62),
    '--accent': accent,
    '--accent-fg': contrastFg(accent),
    '--status-online': accent,
  };
}

const STYLE_ID = 'mise-custom-colors';

// Тёмная тема живёт классом .dark на .app-shell, а он перебивает переменные с :root.
// Поэтому инжектим <style> с !important на селекторы, которые реально совпадают
// и со светлой (:root), и с тёмной (.app-shell.dark) темой.
export function applyColors(c: CustomColors | null) {
  let el = document.getElementById(STYLE_ID) as HTMLStyleElement | null;
  if (!c) {
    if (el) el.remove();
    return;
  }
  if (!el) {
    el = document.createElement('style');
    el.id = STYLE_ID;
    document.head.appendChild(el);
  }
  const p = derivePalette(c);
  const decls = Object.entries(p).map(([k, v]) => `${k}:${v} !important;`).join('');
  el.textContent = `:root, .app-shell, .app-shell.dark, body, #app { ${decls} }`;
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
