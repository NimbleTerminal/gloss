import test from 'node:test';
import assert from 'node:assert/strict';
import { msg, useMessages } from './i18n.mjs';

const page = text => ({ getElementById: id => id === 'gloss-messages' && text !== null ? { textContent: text } : null });

test('with no messages element, the fallback', () => {
  useMessages(page(null));
  assert.equal(msg('web.pick.send', 'Send files'), 'Send files');
  useMessages(undefined);
  assert.equal(msg('web.pick.send', 'Send files'), 'Send files');
});
test('the translation from the page', () => {
  useMessages(page('{"web.pick.send":"Enviar archivos"}'));
  assert.equal(msg('web.pick.send', 'Send files'), 'Enviar archivos');
  assert.equal(msg('web.pick.cancel', 'Cancel'), 'Cancel');
});
test('variables are substituted, in translations and fallbacks', () => {
  useMessages(page('{"web.pick.sendMany":"Enviar {{.Count}} archivos"}'));
  assert.equal(msg('web.pick.sendMany', 'Send {{.Count}} files', { Count: 3 }), 'Enviar 3 archivos');
  assert.equal(msg('web.pick.removeName', 'Remove {{.Name}}', { Name: '{{.Count}}.png' }), 'Remove {{.Count}}.png');
  assert.equal(msg('web.pick.x', 'Left {{.Missing}}'), 'Left {{.Missing}}');
});
test('invalid messages mean the fallbacks', () => {
  for (const text of ['{not json', '["a"]', 'null', '{"web.pick.send":7}']) {
    useMessages(page(text));
    assert.equal(msg('web.pick.send', 'Send files'), 'Send files');
  }
});
