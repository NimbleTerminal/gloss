import test from 'node:test';
import assert from 'node:assert/strict';
import { parserWorkers } from './parser-worker.mjs';

function fixture() {
  const workers = [];
  const run = parserWorkers({}, () => {
    const worker = { stopped: false, postMessage() {}, terminate() { this.stopped = true; } };
    workers.push(worker);
    return worker;
  });
  return { workers, run };
}
test('timeouts terminate parsers and release their slots', async () => {
  const { workers, run } = fixture();
  await assert.rejects(run('{}', 5), /timed out/);
  assert.equal(workers[0].stopped, true);
  const next = run('{}', 1000);
  workers[1].onmessage({ data: '{}' });
  assert.equal(await next, '{}');
  assert.equal(workers[1].stopped, true);
});
test('bounded concurrency, error and success all clean up workers', async () => {
  const { workers, run } = fixture();
  const first = run('{}', 1000), second = run('{}', 1000);
  await assert.rejects(run('{}', 1000), /busy/);
  workers[0].onmessage({ data: '{}' });
  workers[1].onerror({ message: 'broken' });
  await first;
  await assert.rejects(second, /broken/);
  assert.ok(workers.every(w => w.stopped));
});
