// TerminalDrawer — wraps the existing xterm element in a collapsible drawer.
//
// The xterm instance is attached to #terminal by app.js's initTerminal().
// We DOM-move that element into the drawer on first mount so the WebSocket,
// PTY session, and scrollback all persist untouched.

import { store, setState, subscribe } from '../state.js';

let root = null;
let mounted = false;

function doFit() {
  if (typeof window.__crossagentScheduleFit === 'function') {
    window.__crossagentScheduleFit({ notifyPty: true });
  }
}

export function mount(el) {
  root = el;
  if (mounted) return;
  mounted = true;

  const host = root.querySelector('.td-body');
  const termEl = document.getElementById('terminal');
  if (termEl && host && termEl.parentElement !== host) {
    host.appendChild(termEl);
  }

  root.querySelector('#td-close').addEventListener('click', () => {
    setState({ terminalDrawerOpen: false });
  });

  subscribe((s, patch) => {
    if ('terminalDrawerOpen' in patch) applyOpen();
    if ('session' in patch) updateStatus();
  });

  applyOpen();
  updateStatus();
}

function applyOpen() {
  if (!root) return;
  root.dataset.open = store.terminalDrawerOpen ? 'true' : 'false';
  if (store.terminalDrawerOpen) {
    // After the CSS transition, xterm dimensions become real.
    setTimeout(doFit, 220);
  }
}

function updateStatus() {
  if (!root) return;
  const statusEl = root.querySelector('#td-status');
  if (!statusEl) return;
  if (store.session.active) {
    statusEl.textContent = `running · ${store.session.adapter || 'session'}`;
    statusEl.className = 'td-status td-status--run';
  } else {
    statusEl.textContent = 'no active run';
    statusEl.className = 'td-status td-status--idle';
  }
}
