import test from 'node:test';
import assert from 'node:assert/strict';
import { parserWorkers } from './parser-worker.mjs';

function fixture() {
  const workers = [];
  const pool = parserWorkers({}, () => {
    const worker = { stopped: false, sent: [], postMessage(m) { this.sent.push(m); }, terminate() { this.stopped = true; } };
    workers.push(worker);
    return worker;
  });
  return { workers, pool };
}
test('timeouts terminate parsers and release their slots', async () => {
  const { workers, pool } = fixture();
  const session = pool.session();
  await assert.rejects(session.ask('{}', 5), /timed out/);
  assert.equal(workers[0].stopped, true);
  // The next question starts a new worker, with the module.
  const next = session.ask('{}', 1000);
  assert.ok(workers[1].sent[0].module);
  workers[1].onmessage({ data: '{}' });
  assert.equal(await next, '{}');
  session.close();
  assert.equal(workers[1].stopped, true);
});
test('a session keeps its worker between questions', async () => {
  const { workers, pool } = fixture();
  const session = pool.session();
  const first = session.ask('open', 1000);
  workers[0].onmessage({ data: 'opened' });
  assert.equal(await first, 'opened');
  const second = session.ask('page', 1000);
  assert.equal(workers.length, 1);
  assert.deepEqual(workers[0].sent[1], { input: 'page' });
  workers[0].onmessage({ data: 'page 2' });
  assert.equal(await second, 'page 2');
  assert.equal(workers[0].stopped, false);
  session.close();
  assert.equal(workers[0].stopped, true);
});
test('closing rejects the open question', async () => {
  const { workers, pool } = fixture();
  const session = pool.session();
  const asked = session.ask('{}', 1000);
  session.close();
  await assert.rejects(asked, /stopped/);
  assert.equal(workers[0].stopped, true);
  // Its slot is free again.
  const other = pool.session(), again = pool.session();
  const both = [other.ask('{}', 1000), again.ask('{}', 1000)];
  workers[1].onmessage({ data: '{}' });
  workers[2].onmessage({ data: '{}' });
  assert.deepEqual(await Promise.all(both), ['{}', '{}']);
  other.close(); again.close();
});
test('bounded concurrency, error and success all clean up workers', async () => {
  const { workers, pool } = fixture();
  const a = pool.session(), b = pool.session(), c = pool.session();
  const first = a.ask('{}', 1000), second = b.ask('{}', 1000);
  await assert.rejects(c.ask('{}', 1000), /busy/);
  await assert.rejects(a.ask('{}', 1000), /session busy/);
  workers[0].onmessage({ data: '{}' });
  workers[1].onerror({ message: 'broken' });
  await first;
  await assert.rejects(second, /broken/);
  assert.equal(workers[1].stopped, true);
  // An idle session holds no slot.
  const third = c.ask('{}', 1000);
  workers[2].onmessage({ data: '{}' });
  await third;
  a.close(); c.close();
  assert.ok(workers.every(w => w.stopped));
});
