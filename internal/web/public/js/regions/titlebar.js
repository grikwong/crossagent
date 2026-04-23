// TitleBar — logo, status dot, project filter, entry-point buttons,
// terminal toggle. Action buttons (New, Models, Manage projects…) delegate
// to the existing legacy handlers via element.click().

import { store, setState } from '../state.js';
import { esc, hashKey } from '../util.js';

let root = null;
let lastKey = '';

function clickLegacy(id) {
  const btn = document.getElementById(id);
  if (btn) btn.click();
}

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const key = hashKey(
    store.projects.length,
    store.selectedProjectFilter,
    store.terminalDrawerOpen,
    store.session.active,
  );
  if (key === lastKey) return;
  lastKey = key;

  const projects = store.projects || [];
  const filter = store.selectedProjectFilter || '';
  const projectOptions = projects
    .map(p => `<option value="${esc(p.name)}" ${p.name === filter ? 'selected' : ''}>${esc(p.name)} (${p.workflow_count})</option>`)
    .join('');

  root.innerHTML = `
    <div class="tb-left">
      <span class="tb-logo">Crossagent</span>
      <span class="tb-version" id="tb-version"></span>
    </div>
    <div class="tb-center">
      <label class="tb-filter" aria-label="Filter by project">
        <select id="tb-project-filter">
          <option value="">All projects</option>
          ${projectOptions}
          <option disabled>──────</option>
          <option value="__manage__">Manage projects…</option>
        </select>
      </label>
      <button class="tb-btn" id="tb-new" title="Create a new workflow">+ New</button>
      <button class="tb-btn" id="tb-models" title="Configure agent adapters for each phase">Models</button>
    </div>
    <div class="tb-right">
      <span id="tb-status-dot" class="tb-status-dot ${store.session.active ? 'is-active' : ''}"
        title="${store.session.active ? 'Session running' : 'Idle'}"></span>
      <button class="tb-btn" id="tb-terminal-toggle" aria-pressed="${store.terminalDrawerOpen ? 'true' : 'false'}">
        Terminal${store.terminalDrawerOpen ? ' ▾' : ' ▴'}
      </button>
    </div>
  `;

  const versionEl = document.getElementById('app-version');
  const tbVersion = root.querySelector('#tb-version');
  if (versionEl && tbVersion) tbVersion.textContent = versionEl.textContent;

  root.querySelector('#tb-project-filter').addEventListener('change', (e) => {
    const v = e.target.value;
    if (v === '__manage__') {
      e.target.value = store.selectedProjectFilter;
      clickLegacy('manage-projects-btn');
      return;
    }
    setState({ selectedProjectFilter: v });
  });

  root.querySelector('#tb-new').addEventListener('click', () => clickLegacy('new-btn'));
  root.querySelector('#tb-models').addEventListener('click', () => clickLegacy('manage-agents-btn'));

  root.querySelector('#tb-terminal-toggle').addEventListener('click', () => {
    setState({ terminalDrawerOpen: !store.terminalDrawerOpen });
  });
}
