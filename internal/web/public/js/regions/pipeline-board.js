// PipelineBoard — the timeline grid. One row per round, 4 phase cells each.
// Reads: store.status, store.session, store.selectedRound, store.selectedPhase.
// Writes: store.selectedRound, store.selectedPhase.

import { store, setState } from '../state.js';
import { esc, hashKey } from '../util.js';
import { deriveRoundSummaries } from '../derive.js';

const PHASES = ['plan', 'review', 'implement', 'verify'];
const PHASE_LABELS = { plan: 'Plan', review: 'Review', implement: 'Implement', verify: 'Verify' };

let root = null;
let lastKey = '';

function phaseEngine(agents, phase) {
  if (!agents) return '';
  const a = agents[phase];
  if (!a) return '';
  if (typeof a === 'string') return a;
  return a.display_name || a.name || '';
}

function stateGlyph(state) {
  switch (state) {
    case 'done':    return '<span class="ph-glyph ph-glyph--ok">✓</span>';
    case 'retried': return '<span class="ph-glyph ph-glyph--warn">↻</span>';
    case 'running': return '<span class="ph-glyph ph-glyph--run">●</span>';
    case 'failed':  return '<span class="ph-glyph ph-glyph--err">✕</span>';
    default:        return '<span class="ph-glyph ph-glyph--muted">—</span>';
  }
}

function renderDots(n) {
  if (!n) return '';
  return Array.from({ length: n }).map(() => '<span class="ph-dot"></span>').join('');
}

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const rounds = deriveRoundSummaries(store.status, store.session);
  const selRound = store.selectedRound;
  const selPhase = store.selectedPhase;

  const key = hashKey(
    JSON.stringify(rounds.map(r => ({
      n: r.number,
      c: r.current,
      p: r.phases.map(p => `${p.phase}:${p.state}:${p.retries}`),
    }))),
    selRound, selPhase,
  );
  if (key === lastKey) return;
  lastKey = key;

  if (!store.status) {
    root.innerHTML = '';
    return;
  }

  const agents = store.status.agents || {};

  const header = `
    <div class="pb-header">
      <div></div>
      ${PHASES.map((p, i) => `
        <div class="pb-col-head pb-col-head--${p}">
          <span class="pb-col-dot"></span>
          <span class="pb-col-label">${i + 1}. ${PHASE_LABELS[p]}</span>
          <span class="pb-col-engine">${esc(phaseEngine(agents, p))}</span>
        </div>
      `).join('')}
    </div>
  `;

  const rowsHtml = rounds.map(r => {
    const cells = r.phases.map(p => {
      const isSel =
        (selRound === r.number || (selRound === null && r.current && selRound === null)) &&
        selPhase === p.phase &&
        ((r.current && selRound === null) || (!r.current && selRound === r.number));
      const pending = p.state === 'pending' || p.state === 'missing';
      const attemptsHtml = p.retries > 0
        ? Array.from({ length: p.retries }).map((_, i) => {
            const attempt = i + 1;
            const isAttemptSel = isSel && store.selectedAttempt === attempt;
            return `<button class="ph-attempt-chip ${isAttemptSel ? 'ph-attempt-chip--selected' : ''}"
                    data-round="${r.number}" data-phase="${p.phase}" data-attempt="${attempt}"
                    data-current="${r.current ? '1' : '0'}">attempt-${attempt}</button>`;
          }).join('')
        : '';
      return `
        <div class="phase-cell phase-cell--${p.phase} phase-cell--${p.state} ${isSel ? 'phase-cell--selected' : ''} ${pending ? 'phase-cell--dim' : ''}"
             data-round="${r.number}" data-phase="${p.phase}" data-current="${r.current ? '1' : '0'}">
          <div class="ph-top">
            <span class="ph-file">${esc(p.file)}</span>
            ${stateGlyph(p.state)}
          </div>
          <div class="ph-bot">
            <span class="ph-state">${esc(p.state)}</span>
            <span class="ph-dots">${renderDots(p.retries)}</span>
            ${p.retries > 0 ? `<span class="ph-attempts">${p.retries} attempt${p.retries > 1 ? 's' : ''}</span><button class="ph-attempts-btn" title="Show attempts">▾</button>` : ''}
          </div>
          ${attemptsHtml ? `<div class="ph-attempts-list">${attemptsHtml}</div>` : ''}
        </div>
      `;
    }).join('');
    return `
      <div class="pb-row ${r.current ? 'pb-row--current' : ''}">
        <div class="pb-round-badge">R${r.number}</div>
        ${cells}
      </div>
    `;
  }).join('');

  root.innerHTML = `${header}${rowsHtml}`;

  root.querySelectorAll('.phase-cell').forEach(el => {
    el.addEventListener('click', (e) => {
      // Attempt chip clicks select the attempt directly.
      if (e.target.classList.contains('ph-attempt-chip')) {
        e.stopPropagation();
        const round = parseInt(e.target.dataset.round, 10);
        const phase = e.target.dataset.phase;
        const attempt = parseInt(e.target.dataset.attempt, 10);
        const current = e.target.dataset.current === '1';
        setState({
          selectedRound: current ? null : round,
          selectedPhase: phase,
          selectedAttempt: attempt,
        });
        return;
      }
      // Attempts toggle chevron just opens/closes the attempt list locally.
      if (e.target.classList.contains('ph-attempts-btn')) {
        e.stopPropagation();
        const open = el.dataset.attemptsOpen === 'true';
        el.dataset.attemptsOpen = open ? 'false' : 'true';
        return;
      }
      const round = parseInt(el.dataset.round, 10);
      const phase = el.dataset.phase;
      const current = el.dataset.current === '1';
      setState({
        selectedRound: current ? null : round,
        selectedPhase: phase,
        selectedAttempt: null,
      });
    });
  });
}
