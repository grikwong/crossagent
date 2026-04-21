// ArtifactReader — sticky header + marked-rendered markdown body.
// Reads: store.status, store.selectedRound, store.selectedPhase, store.selectedAttempt.
// Fetches: /api/workflow/{name}/artifact/{type}[?attempt=N]  OR
//          /api/workflow/{name}/rounds/{n}/artifact/{type}[?attempt=N]

import { store } from '../state.js';
import { wfApi } from '../api.js';
import { esc, hashKey } from '../util.js';

let root = null;
let lastKey = '';
let lastLoaded = '';
let inFlight = 0;

function buildPath() {
  const s = store.status;
  if (!s || !store.selectedPhase) return null;
  const phase = store.selectedPhase;
  const round = store.selectedRound;
  const attempt = store.selectedAttempt;
  const qs = attempt != null ? `?attempt=${encodeURIComponent(attempt)}` : '';
  if (round != null) {
    return `/rounds/${encodeURIComponent(round)}/artifact/${encodeURIComponent(phase)}${qs}`;
  }
  return `/artifact/${encodeURIComponent(phase)}${qs}`;
}

async function fetchContent() {
  const s = store.status;
  if (!s || !s.name) return { content: '' };
  const path = buildPath();
  if (!path) return { content: '' };
  const token = ++inFlight;
  const data = await wfApi(s.name, path);
  if (token !== inFlight) return null; // stale
  return data || {};
}

function renderHeader() {
  const s = store.status;
  const phase = store.selectedPhase;
  const round = store.selectedRound != null ? store.selectedRound : ((s && s.followup_round || 0) + 1);
  if (!s || !phase) return '';
  return `
    <div class="ar-head">
      <span class="ar-dot ar-dot--${phase}"></span>
      <span class="ar-file">R${round} / ${esc(phase)}.md</span>
      <div class="ar-actions"></div>
    </div>
  `;
}

export function mount(el) {
  root = el;
  render();
}

export async function render() {
  if (!root) return;
  const s = store.status;
  const key = hashKey(s && s.name, store.selectedRound, store.selectedPhase, store.selectedAttempt);
  if (key === lastKey) return;
  lastKey = key;

  if (!s || !store.selectedPhase) {
    root.innerHTML = `
      <div class="ar-empty">Click a phase cell above to view its artifact.</div>
    `;
    return;
  }

  root.innerHTML = `
    ${renderHeader()}
    <div class="ar-body"><div class="ar-loading">Loading…</div></div>
  `;

  const data = await fetchContent();
  if (!data) return; // stale
  const body = root.querySelector('.ar-body');
  if (!body) return;

  if (data.error) {
    body.innerHTML = `<div class="ar-error">${esc(data.error)}</div>`;
    return;
  }
  const content = data.content || '';
  if (!content) {
    body.innerHTML = `<div class="ar-empty">Artifact not yet written.</div>`;
    return;
  }
  const html = (typeof window.marked !== 'undefined' && window.marked.parse)
    ? window.marked.parse(content)
    : `<pre>${esc(content)}</pre>`;
  body.innerHTML = `<div class="ar-md">${html}</div>`;
  lastLoaded = key;
}
