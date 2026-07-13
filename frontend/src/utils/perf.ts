// Экономия GPU: пауза непрерывных анимаций, когда окно свёрнуто/в трее,
// и «экономный режим» (полное отключение волны/глоу) для слабых/горячих видеокарт.

export function economyEnabled(): boolean {
  try { return localStorage.getItem('mise_economy') === '1'; } catch { return false; }
}

function applyEconomy(on: boolean) {
  document.body.classList.toggle('economy', on);
}

export function setEconomy(on: boolean) {
  try { localStorage.setItem('mise_economy', on ? '1' : '0'); } catch { /* ignore */ }
  applyEconomy(on);
  // wave-state слушает это событие, чтобы включить/выключить пересчёт волны
  window.dispatchEvent(new Event('mise-economy-change'));
}

export function initPerf() {
  const applyHidden = () => document.body.classList.toggle('is-hidden', document.hidden);
  document.addEventListener('visibilitychange', applyHidden);
  applyHidden();
  applyEconomy(economyEnabled());
}
