// The compiled module is shared, but each parse gets a fresh, terminable worker.
export function parserWorkers(module, makeWorker = () => new Worker(new URL('./parser-worker.js', import.meta.url))) {
  let active = 0;
  return (input, timeout) => new Promise((resolve, reject) => {
    if (active >= 2) return reject(new Error('Parser busy; try again'));
    if (input.length > 256 * 1024 * 1024) return reject(new Error('Parser input exceeds limit'));
    const worker = makeWorker();
    active++;
    let settled = false;
    const finish = (error, value) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      worker.terminate();
      active--;
      if (error) reject(error); else resolve(value);
    };
    const timer = setTimeout(() => finish(new Error('Parser timed out; worker stopped')), timeout);
    worker.onmessage = ({ data }) => {
      if (typeof data !== 'string' || data.length > 256 * 1024 * 1024) finish(new Error('Invalid parser reply'));
      else finish(null, data);
    };
    worker.onerror = error => finish(new Error(error.message || 'Parser worker failed'));
    try { worker.postMessage({ input, module }); } catch (error) { finish(error); }
  });
}
