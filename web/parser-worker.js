// A disposable Go runtime: hostile parsers cannot keep the UI thread busy.
importScripts('./wasm_exec.js');
self.onmessage = async ({ data }) => {
  try {
    self.glossParserInput = data.input;
    const go = new Go();
    go.argv = ['gloss', '--gloss-parser-worker'];
    const instance = await WebAssembly.instantiate(data.module, go.importObject);
    await go.run(instance);
  } catch (error) {
    self.postMessage(JSON.stringify({ Error: String(error) }));
  }
};
