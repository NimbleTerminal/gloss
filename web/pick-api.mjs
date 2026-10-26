import { maxBytes, maxLabel } from './drop.mjs';
import { msg } from './i18n.mjs';
import { maxPicked } from './pickers.mjs';

// The upload page talks only in upload IDs; it never sends local host paths.
// These relative URLs preserve the request's token and work without a viewer.
export function createPickAPI(send = fetch, makeXHR = () => new XMLHttpRequest()) {
  async function request(path, method = 'GET', body) {
    const response = await send(path, {
      method, cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer',
      ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
    });
    if (!response.ok) throw new Error((await response.text()).trim() || msg('web.pick.requestFailed', 'Request failed ({{.Status}}).', { Status: response.status }));
    return response.json();
  }
  return {
    list: () => request('files'),
    remove: id => request(`files/${encodeURIComponent(id)}`, 'DELETE'),
    async confirm(ids, message = '') {
      const problem = messageProblem(message);
      if (problem) throw new Error(problem);
      return request('confirm', 'POST', { ids, ...(message.trim() ? { message } : {}) });
    },
    decline: () => request('decline', 'POST'),
    upload(file, progress = () => {}) {
      // A File goes straight into FormData: no arrayBuffer copy of large files.
      return new Promise((resolve, reject) => {
        const xhr = makeXHR();
        xhr.open('POST', 'files');
        xhr.timeout = 120000;
        xhr.upload.onprogress = event => {
          if (event.lengthComputable) progress(Math.round(event.loaded / event.total * 100));
        };
        xhr.onload = () => {
          if (xhr.status < 200 || xhr.status >= 300) {
            reject(new Error(xhr.responseText.trim() || msg('web.pick.uploadFailed', 'Upload failed ({{.Status}}).', { Status: xhr.status })));
            return;
          }
          try { resolve(JSON.parse(xhr.responseText).files); }
          catch { reject(new Error(msg('web.pick.uploadUnreadable', 'The upload response could not be read.'))); }
        };
        xhr.onerror = () => reject(new Error(msg('web.pick.uploadLost', 'The upload connection was lost.')));
        xhr.ontimeout = () => reject(new Error(msg('web.pick.uploadSlow', 'The upload took too long.')));
        xhr.onabort = () => reject(new Error(msg('web.pick.uploadInterrupted', 'The upload was interrupted.')));
        const body = new FormData();
        body.append('file', file, file.name);
        xhr.send(body);
      });
    },
  };
}

export const maxPickFiles = maxPicked;
export const maxPickBytes = maxBytes;
export const maxMessageCharacters = 2000;

export function messageProblem(message) {
  return Array.from(message).length > maxMessageCharacters ? msg('web.pick.messageLong', 'Message must be at most 2,000 characters.') : '';
}

// Drafts stay in this browser tab, scoped to the session URL. Nothing sends
// the draft to the requester until confirmation. Blocked storage is harmless.
export function messageDraft(storage, key) {
  return {
    load() { try { return storage.getItem(key) ?? ''; } catch { return ''; } },
    save(value) { try { value ? storage.setItem(key, value) : storage.removeItem(key); } catch {} },
    clear() { try { storage.removeItem(key); } catch {} },
  };
}

export function fileProblem(file, count) {
  if (count >= maxPickFiles) return msg('web.pick.tooMany', 'At most {{.Max}} files can be added.', { Max: maxPickFiles });
  if (!file.size) return msg('web.pick.empty', '{{.Name}} is empty.', { Name: file.name });
  if (file.size > maxPickBytes) return msg('web.pick.tooBig', '{{.Name}} is over {{.Limit}}.', { Name: file.name, Limit: maxLabel });
  return '';
}
