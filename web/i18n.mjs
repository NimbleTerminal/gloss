// The page's words in the user's language. The server writes the catalog's
// own strings into #gloss-messages; an English page, or a page without the
// element, has none, and every msg is its English fallback.
let messages = null;

// useMessages reads the messages of doc, the page; a missing element or one
// that does not hold an object of strings means none. msg calls it once.
export function useMessages(doc = globalThis.document) {
  messages = {};
  try {
    const parsed = JSON.parse(doc?.getElementById('gloss-messages')?.textContent ?? '{}');
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) messages = parsed;
  } catch { /* English, then. */ }
}

// msg returns the translation of id, or fallback, with each {{.Name}} replaced
// by vars.Name. A name vars lacks is left as written.
export function msg(id, fallback, vars = {}) {
  if (messages === null) useMessages();
  const text = typeof messages[id] === 'string' ? messages[id] : fallback;
  return text.replace(/\{\{\.(\w+)\}\}/g, (all, name) => Object.hasOwn(vars, name) ? String(vars[name]) : all);
}
