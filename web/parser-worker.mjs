const limit = 256 * 1024 * 1024;

// The compiled module is shared. Each session is a worker of its own, kept
// for the questions asked of one document and terminated when the session
// closes (gloss closes one whose worker has ended) or a question times out
// or fails.
// At most two questions are answered at once; an idle session holds no slot.
export function parserWorkers(module, makeWorker = () => new Worker(new URL('./parser-worker.js', import.meta.url))) {
  let active = 0;
  return {
    session() {
      let worker = null, pending = null;
      const stop = () => {
        if (worker) worker.terminate();
        worker = null;
      };
      return {
        ask(input, timeout) {
          return new Promise((resolve, reject) => {
            if (pending) return reject(new Error('Parser session busy'));
            if (active >= 2) return reject(new Error('Parser busy; try again'));
            if (input.length > limit) return reject(new Error('Parser input exceeds limit'));
            const first = !worker;
            if (first) worker = makeWorker();
            const current = worker;
            active++;
            let settled = false;
            const finish = (error, value) => {
              if (settled) return;
              settled = true;
              clearTimeout(timer);
              active--;
              pending = null;
              current.onmessage = current.onerror = null;
              if (error) {
                if (worker === current) stop();
                reject(error);
              } else resolve(value);
            };
            pending = finish;
            const timer = setTimeout(() => finish(new Error('Parser timed out; worker stopped')), timeout);
            current.onmessage = ({ data }) => {
              if (typeof data !== 'string' || data.length > limit) finish(new Error('Invalid parser reply'));
              else finish(null, data);
            };
            current.onerror = error => finish(new Error(error.message || 'Parser worker failed'));
            try { current.postMessage(first ? { input, module } : { input }); } catch (error) { finish(error); }
          });
        },
        close() {
          if (pending) pending(new Error('Parser stopped'));
          stop();
        },
      };
    },
  };
}
