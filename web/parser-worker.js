// A Go runtime of its own: hostile parsers cannot keep the UI thread busy.
// The first message starts it; for an open PDF, later ones are its questions.
importScripts('./wasm_exec.js');
let started = false;
self.onmessage = async ({ data }) => {
  try {
    if (started) {
      self.glossParserNext(data.input);
      return;
    }
    started = true;
    self.glossParserInput = data.input;
    const go = new Go();
    go.argv = ['gloss', '--gloss-parser-worker'];
    const instance = await WebAssembly.instantiate(data.module, go.importObject);
    await go.run(instance);
  } catch (error) {
    self.postMessage(JSON.stringify({ Error: String(error), Ended: true }));
  }
};
