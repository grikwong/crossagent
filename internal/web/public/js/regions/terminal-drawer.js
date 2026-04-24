// TerminalDrawer — wraps the existing xterm element in a collapsible drawer.
//
// The xterm instance is attached to #terminal by app.js's initTerminal().
// We DOM-move that element into the drawer on first mount so the WebSocket,
// PTY session, and scrollback all persist untouched.

import { store, setState } from '../state.js';

let root = null;
let mounted = false;

function doFit() {
  if (typeof window.__crossagentScheduleFit === 'function') {
    window.__crossagentScheduleFit({ notifyPty: true });
  }
}

function doRefresh() {
  if (typeof window.__crossagentRefreshTerminalView === 'function') {
    window.__crossagentRefreshTerminalView();
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

  applyOpen();
  updateStatus();
}

// Called by v2.js on every store change. Handles both open/close and status.
export function render() {
  applyOpen();
  updateStatus();
}

function applyOpen() {
  if (!root) return;
  const wasOpen = root.dataset.open === 'true';
  root.dataset.open = store.terminalDrawerOpen ? 'true' : 'false';
  if (store.terminalDrawerOpen && !wasOpen) {
    // Only schedule a fit on the false→true open transition, not on every render
    // while already open. This eliminates the race where unrelated store updates
    // queue repeated delayed fits during an active session.
    let fired = false;
    const onEnd = () => {
      if (fired) return;
      fired = true;
      root.removeEventListener('transitionend', onEnd);
      doFit();
      doRefresh();
    };
    root.addEventListener('transitionend', onEnd);
    // Fallback: CSS transition is ~180ms; 280ms gives margin for reduced-motion or hidden tabs.
    setTimeout(onEnd, 280);
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
