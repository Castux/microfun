// The playground page: a full-screen editor wired to the wasm runner, with
// example loading, stage dumps, stdin, and shareable URLs (#code=base64).

"use strict";

const DEFAULT_PROGRAM = `import list, math, core in

let

  -- Primality test: n is prime when no divisor exists in [2, sqrt n]
  divides = d -> n -> eq 0 (mod n d),
  isPrime = n -> range 2 (floor (sqrt n)) > none (divides n) > and (gte 2 n),
  primes  = upFrom 2 > filter isPrime,    -- lazy infinite stream of primes

  -- Fibonacci as a self-referential lazy stream
  fibs = concat [1;1] (zipWith add fibs (tail fibs))

in

show [
  take 10 primes,
  take 10 fibs
]
`;

const EXAMPLES = ["examples.mf", "countdown.mf", "core_tests.mf"];

const output = document.getElementById("output");
const runBtn = document.getElementById("run-btn");
const stopBtn = document.getElementById("stop-btn");
const shareBtn = document.getElementById("share-btn");
const dumpSelect = document.getElementById("dump-select");
const exampleSelect = document.getElementById("example-select");
const stdinBox = document.getElementById("stdin");

const editor = CodeMirror(document.getElementById("editor"), {
    value: initialProgram(),
    mode: "microfun",
    theme: "mf",
    lineNumbers: true,
    lineWrapping: true,
    extraKeys: { "Ctrl-Enter": run, "Cmd-Enter": run },
});

function initialProgram() {
    const match = location.hash.match(/#code=(.+)/);
    if (match) {
        try {
            return decodeURIComponent(escape(atob(match[1])));
        } catch (err) { /* fall through to default */ }
    }
    return DEFAULT_PROGRAM;
}

for (const name of EXAMPLES) {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    exampleSelect.appendChild(opt);
}

exampleSelect.addEventListener("change", async () => {
    const name = exampleSelect.value;
    if (!name) return;
    try {
        const resp = await fetch("examples/" + name);
        if (!resp.ok) throw new Error(resp.status);
        editor.setValue(await resp.text());
    } catch (err) {
        output.textContent = "Could not load example: " + err.message;
    }
    exampleSelect.value = "";
});

async function run() {
    runBtn.disabled = true;
    stopBtn.disabled = false;
    output.textContent = "";
    let got = "";
    const result = await MicrofunRunner.run(editor.getValue(), {
        stdin: stdinBox.value,
        dump: dumpSelect.value,
        timeoutMs: 60000,
        onOutput: text => { got += text; output.textContent = got; },
    });
    runBtn.disabled = false;
    stopBtn.disabled = true;
    if (result.cancelled) return;
    if (result.timedOut) {
        output.textContent = got + "\n[stopped: exceeded 60s time limit]";
    } else if (result.hostError) {
        output.textContent = got + "\n[host error: " + result.hostError + "]";
    } else if (got === "") {
        output.textContent = "(no output — use show, peek or write to print)";
    }
}

runBtn.addEventListener("click", run);
stopBtn.addEventListener("click", () => MicrofunRunner.stop());

shareBtn.addEventListener("click", async () => {
    const encoded = btoa(unescape(encodeURIComponent(editor.getValue())));
    const url = location.origin + location.pathname + "#code=" + encoded;
    history.replaceState(null, "", "#code=" + encoded);
    try {
        await navigator.clipboard.writeText(url);
        shareBtn.textContent = "Copied!";
    } catch (err) {
        shareBtn.textContent = "URL updated";
    }
    setTimeout(() => { shareBtn.textContent = "Share"; }, 1500);
});
