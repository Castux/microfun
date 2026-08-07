// Web Worker that hosts the thunky wasm module. One worker handles runs
// sequentially; each run instantiates a fresh module so the runtime starts
// clean (and so a program that calls os.Exit doesn't poison later runs).
// A run that never terminates is the main thread's problem: it terminates
// the whole worker and spawns a new one (see runner.js).

importScripts("wasm_exec.js");

let modulePromise = null;

function getModule() {
    if (!modulePromise) {
        modulePromise = fetch("thunky.wasm").then(resp => {
            if (!resp.ok) throw new Error("could not fetch thunky.wasm (" + resp.status + ")");
            return resp.arrayBuffer();
        }).then(bytes => WebAssembly.compile(bytes));
    }
    return modulePromise;
}

// Stdin for the current run, served byte-by-byte to Go through fs.read.
let stdinBytes = new Uint8Array(0);
let stdinPos = 0;

// Route the Go program's stdout/stderr to the page. Output can arrive one
// byte at a time (the `write` builtin), so each stream gets a stateful UTF-8
// decoder that holds partial sequences between chunks.
function installFS() {
    const decoders = { 1: new TextDecoder(), 2: new TextDecoder() };
    const fs = globalThis.fs; // the wasm_exec.js polyfill

    fs.writeSync = (fd, buf) => {
        const dec = decoders[fd] || decoders[1];
        const text = dec.decode(buf, { stream: true });
        if (text !== "") postMessage({ type: "output", fd, text });
        return buf.length;
    };
    fs.write = (fd, buf, offset, length, position, callback) => {
        if (offset !== 0 || length !== buf.length || position !== null) {
            callback(new Error("unsupported partial write"));
            return;
        }
        callback(null, fs.writeSync(fd, buf));
    };
    fs.read = (fd, buffer, offset, length, position, callback) => {
        if (fd !== 0 || position !== null) {
            const err = new Error("bad file descriptor");
            err.code = "EBADF";
            callback(err);
            return;
        }
        const n = Math.min(length, stdinBytes.length - stdinPos);
        buffer.set(stdinBytes.subarray(stdinPos, stdinPos + n), offset);
        stdinPos += n;
        callback(null, n);
    };
}

onmessage = async event => {
    const { id, source, path, stdin, dump } = event.data;
    try {
        const module = await getModule();

        stdinBytes = new TextEncoder().encode(stdin || "");
        stdinPos = 0;
        installFS();

        globalThis.__thunky_source = source;
        globalThis.__thunky_path = path || "playground.þ";
        globalThis.__thunky_dump = dump || "";

        const go = new Go();
        let exitCode = 0;
        go.exit = code => { exitCode = code; };

        const instance = await WebAssembly.instantiate(module, go.importObject);
        await go.run(instance);

        postMessage({ type: "done", id, exitCode });
    } catch (err) {
        postMessage({ type: "error", id, message: String((err && err.message) || err) });
    }
};

postMessage({ type: "ready" });
