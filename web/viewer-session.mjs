import { msg } from './i18n.mjs';

// Pure presentation policy: a viewing interruption must not cover the document.
export function sessionNotice(pick, reason) {
  if (reason === 'closed') return { cover: true, text: msg('web.serve.finished', 'gloss has finished. You can close this tab.') };
  const text = reason === 'timeout' ? msg('web.serve.expired', 'Session expired.') : msg('web.serve.lost', 'Connection lost.');
  return { cover: pick, text: `${text} ${pick ? msg('web.serve.pickGone', 'This file request is no longer available.') : msg('web.serve.viewGone', 'This view is no longer interactive. Ask the requester to reopen it.')}` };
}

export function expiryNotice({ reason, seconds_left: left, idle }) {
  if (reason || left == null || left > 60) return { text: '', extend: false };
  return {
    text: idle
      ? msg('web.serve.idleExpiry', 'Viewing session expires after inactivity in {{.Seconds}}s. Keep viewing to continue.', { Seconds: left })
      : msg('web.serve.deadline', 'This session ends in {{.Seconds}}s (requester’s time limit).', { Seconds: left }),
    extend: idle,
  };
}

// Only trusted input reaches activity; polling and terminal output never do.
export function installActivity(target, send, now = Date.now) {
  let last = -Infinity;
  const activity = event => {
    if (!event.isTrusted || now() - last < 3000) return;
    last = now();
    send();
  };
  for (const name of ['keydown', 'pointerdown', 'pointermove', 'wheel', 'touchstart']) {
    target.addEventListener(name, activity, { passive: true });
  }
  return () => {
    for (const name of ['keydown', 'pointerdown', 'pointermove', 'wheel', 'touchstart']) target.removeEventListener(name, activity);
  };
}
