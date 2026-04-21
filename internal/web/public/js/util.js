// Shared helpers used across modules.

export const PHASE_NAMES = ['', 'plan', 'review', 'implement', 'verify'];

export const PHASE_IDS = { plan: 1, review: 2, implement: 3, verify: 4 };

export function esc(str) {
  const d = document.createElement('div');
  d.textContent = str || '';
  return d.innerHTML;
}

export function capitalize(s) {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : '';
}

// Stable string key for render-memoization. Undefined/null are coerced to ''.
export function hashKey(...parts) {
  return parts.map(p => (p == null ? '' : String(p))).join('|');
}
