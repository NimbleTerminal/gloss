import { msg } from './i18n.mjs';
import { BoobaTerminal, resolveBoobaURLs } from './static/booba/booba.js';
import { parseRendererFromURL } from './static/ghostty-web/ghostty-web.js';
import { installDrop } from './drop.mjs';
import { fetchDocument } from './remote.mjs';
import { upload } from './upload.mjs';
import { pickFiles, inputPicker } from './pickers.mjs';
import { sessionNotice, expiryNotice, installActivity } from './viewer-session.mjs';

const status = document.querySelector('#status');
const hint = document.querySelector('#drop');
const invitation = hint.textContent;
const choose = document.querySelector('#choose-files');
const message = document.querySelector('#message');
const lifetime = document.querySelector('#lifetime');
const keepViewing = document.querySelector('#keep-viewing');
const doneViewing = document.querySelector('#done-viewing');
const pick = document.body.dataset.pick === 'true';
let ended = false, removeActivity = () => {};
function sessionEnded(reason) {
  // A definitive server reason can follow the websocket disconnect.
  const notice = sessionNotice(pick, reason);
  if (reason !== 'disconnected') ended = true;
  removeActivity();
  choose.disabled = true;
  if (doneViewing) doneViewing.disabled = true;
  keepViewing.hidden = true;
  lifetime.textContent = notice.cover ? '' : notice.text;
  status.textContent = notice.text;
  status.hidden = !notice.cover;
}
async function activity() {
  if (ended) return;
  try { await fetch('activity', { method: 'POST' }); } catch { /* Session poll reports failure. */ }
}
keepViewing.addEventListener('click', activity);
doneViewing?.addEventListener('click', async () => {
  try {
    const response = await fetch('finish', { method: 'POST' });
    if (response.ok) sessionEnded('closed');
    else sessionEnded('disconnected');
  } catch { sessionEnded('disconnected'); }
});
async function watchSession() {
  while (!ended) {
    try {
      const response = await fetch('session', { cache: 'no-store' });
      if (!response.ok) throw new Error('Session unavailable');
      const state = await response.json();
      if (state.reason) { sessionEnded(state.reason); return; }
      const notice = expiryNotice(state);
      lifetime.textContent = notice.text;
      keepViewing.hidden = !notice.extend;
    } catch {
      if (!ended) sessionEnded('disconnected');
      return;
    }
  }
}
let terminal;
function notice(text) {
	message.textContent = text;
  hint.textContent = text;
  hint.hidden = false;
  setTimeout(() => { hint.hidden = true; hint.textContent = invitation; }, 4000);
}
async function chosen(names, contents, skipped = []) {
  if (!names.length) {
    if (skipped.length) notice(skipped.join('; '));
    return;
  }
  choose.disabled = true;
  try {
    await upload(names, contents);
    message.textContent = `${names.length === 1 ? msg('web.serve.addedOne', '1 file added.') : msg('web.serve.addedMany', '{{.Count}} files added.', { Count: names.length })} ${skipped.join('; ')}`;
  } catch (error) {
    notice(error.message || String(error));
  } finally {
    choose.disabled = ended;
  }
  terminal?.focus();
}
let fetching = false;
installDrop(window, hint, chosen, notice, document.body.dataset.fetch === 'true' ? async address => {
  if (fetching) throw new Error(msg('web.serve.fetching', 'A document is already being fetched.'));
  fetching = true;
  try {
    const [name, data] = await fetchDocument(address);
    await chosen([name], [data]);
  } finally { fetching = false; }
} : null);
choose.addEventListener('click', async () => {
  try {
    await chosen(...await pickFiles(window, inputPicker(document)));
  } catch (error) {
    notice(error.message || String(error));
  }
});
// An export made in the viewer is kept by the server for this page, which
// saves it as a download: where the browser saves, not where gloss runs.
async function takeExports(open) {
  let after = 0;
  while (open()) {
    let list;
    try {
      const response = await fetch(`exports?after=${after}`, { cache: 'no-store' });
      if (!response.ok) return;
      list = await response.json();
    } catch {
      return; // The server has gone with the viewer.
    }
    for (const { id, name } of list) {
      const link = document.createElement('a');
      link.href = `export/${id}`;
      link.download = name;
      link.click();
      after = id + 1;
    }
  }
}
try {
  terminal = new BoobaTerminal('terminal', { renderer: parseRendererFromURL() });
  let connected = false;
  terminal.onStatusChange = state => {
    if (state === 'connected') {
      connected = true;
      choose.disabled = false;
      if (doneViewing) doneViewing.disabled = false;
      status.hidden = true;
      removeActivity();
      removeActivity = installActivity(window, activity);
      activity();
      watchSession();
    } else if (connected && state === 'disconnected') {
      // gloss ends with its viewer, and its server with it.
      if (!ended) sessionEnded('disconnected');
    }
  };
  terminal.onTitleChange = title => { document.title = title || 'gloss'; };
  takeExports(() => !ended);
  await terminal.init();
  const urls = resolveBoobaURLs(document.baseURI);
  terminal.connectAuto(urls.wsUrl, null, null);
  terminal.focus();
} catch (error) {
  console.error(error);
  status.textContent = msg('web.serve.startFailed', 'Unable to start gloss: {{.Error}}', { Error: error.message || error });
  status.hidden = false;
  choose.disabled = true;
}
