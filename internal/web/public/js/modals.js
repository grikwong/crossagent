// Modal orchestration helpers.
//
// The full new/followup/adddir/projects/agents/elicit/tour modal logic
// still lives in app.js — those handlers are tightly coupled to DOM ids
// and form state. What we extract here are the two pieces that can stand
// alone cleanly: backdrop-click-to-close and Escape-to-close-topmost.

// Modals that implement their own dismiss logic (promise-backed). They
// are skipped by the generic closers.
const PROMISE_MODALS = new Set(['suggest-modal', 'elicit-modal']);

export function installModalClosers({ onEscape } = {}) {
  document.querySelectorAll('.modal-backdrop').forEach(backdrop => {
    backdrop.addEventListener('click', () => {
      const modal = backdrop.closest('.modal');
      if (modal && !PROMISE_MODALS.has(modal.id)) {
        modal.classList.add('hidden');
      }
    });
  });

  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    const openModals = [...document.querySelectorAll('.modal:not(.hidden)')]
      .filter(m => !PROMISE_MODALS.has(m.id));
    if (openModals.length > 0) {
      openModals[openModals.length - 1].classList.add('hidden');
    }
    if (typeof onEscape === 'function') onEscape(e);
  });
}
