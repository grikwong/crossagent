// PipelineHeader — eyebrow (project · round), workflow id, state pill,
// Follow Up + Advance buttons.

import { store } from '../state.js';
import { esc, hashKey } from '../util.js';
import { wfApi } from '../api.js';

let root = null;
let lastKey = '';

function stateLabel() {
  if (!store.status) return { label: 'No workflow', cls: 'idle' };
  if (store.status.phase === 'done') return { label: 'Complete', cls: 'ok' };
  if (store.session.active) return { label: 'Running', cls: 'run' };
  return { label: store.status.phase_label || store.status.phase || 'Idle', cls: 'idle' };
}

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const s = store.status;
  const key = hashKey(
    s && s.name, s && s.phase, s && s.followup_round,
    store.session.active ? 1 : 0,
  );
  if (key === lastKey) return;
  lastKey = key;

  if (!s || !s.name) {
    root.innerHTML = `<div class="pl-empty">Select or create a workflow.</div>`;
    return;
  }

  const round = (s.followup_round || 0) + 1;
  const st = stateLabel();

  root.innerHTML = `
    <div class="pl-left">
      <div class="pl-eyebrow">${esc((s.project || 'default').toUpperCase())} · ROUND ${round}</div>
      <div class="pl-title">${esc(s.name)}</div>
    </div>
    <div class="pl-right">
      <span class="pl-state-pill pl-state-pill--${st.cls}">
        <span class="pl-state-dot"></span>${esc(st.label)}
      </span>
      <button class="pl-btn" id="pl-followup">Follow Up</button>
      <button class="pl-btn pl-btn--primary" id="pl-advance">Advance →</button>
    </div>
  `;

  root.querySelector('#pl-advance').addEventListener('click', async () => {
    if (!s.name) return;
    await wfApi(s.name, '/advance', { method: 'POST' });
    if (typeof window.__crossagentRefreshStatus === 'function') {
      window.__crossagentRefreshStatus();
    }
  });

  root.querySelector('#pl-followup').addEventListener('click', () => {
    // Delegate to the legacy follow-up modal flow; no need to duplicate it.
    const btn = document.getElementById('followup-btn');
    if (btn) btn.click();
  });
}
