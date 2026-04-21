// Single source of truth for the UI. Replaces scattered globals in app.js.
// setState does a shallow merge and notifies subscribers.

const LS_KEYS = {
  density: 'crossagent.density',
  drawer: 'crossagent.terminalDrawerOpen',
};

function loadDensity() {
  try {
    const v = localStorage.getItem(LS_KEYS.density);
    return v === 'compact' ? 'compact' : 'comfortable';
  } catch {
    return 'comfortable';
  }
}

function loadDrawer() {
  try {
    return localStorage.getItem(LS_KEYS.drawer) === '1';
  } catch {
    return false;
  }
}

export const store = {
  // Server-derived truth
  workflows: [],
  projects: [],
  status: null,

  // UI truth
  selectedWorkflowId: null,
  selectedProjectFilter: '',
  workflowSearch: '',
  selectedRound: null,
  selectedPhase: null,
  selectedAttempt: null,

  // Browser-only (no server field)
  session: { active: false, id: null, phase: null, isOwner: true, adapter: null },

  // Preferences
  terminalDrawerOpen: loadDrawer(),
  density: loadDensity(),
};

const listeners = new Set();

export function setState(patch) {
  Object.assign(store, patch);
  if ('density' in patch) {
    try { localStorage.setItem(LS_KEYS.density, store.density); } catch { /* ignore */ }
  }
  if ('terminalDrawerOpen' in patch) {
    try { localStorage.setItem(LS_KEYS.drawer, store.terminalDrawerOpen ? '1' : '0'); } catch { /* ignore */ }
  }
  for (const fn of listeners) fn(store, patch);
}

export function subscribe(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
