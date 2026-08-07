// Documentation shell: fetches the repo's markdown, renders it, and upgrades
// thunky code blocks into editable, runnable snippets backed by the wasm
// runner. index.html?page=<id> selects a page.

"use strict";

const PAGES = [
    { id: "home", title: "thunky", file: "README.md" },
    { id: "language", title: "Language reference", file: "docs/LANGUAGE.md" },
    { id: "overview", title: "0. Overview", file: "docs/0.Overview.md" },
    { id: "lexer", title: "1. Lexer", file: "docs/1.Lexer.md" },
    { id: "parser", title: "2. Parser", file: "docs/2.Parser.md" },
    { id: "resolver", title: "3. Resolver", file: "docs/3.Resolver.md" },
    { id: "core-ir", title: "4. Core IR and Lowering", file: "docs/4.Core IR and Lowering.md" },
    { id: "bytecode", title: "5. Bytecode and Compiler", file: "docs/5.Bytecode and Compiler.md" },
    { id: "g-machine", title: "6. The G-machine", file: "docs/6.The G-machine.md" },
    { id: "improvements", title: "Future improvements", file: "docs/IMPROVEMENTS.md" },
];

// Fenced-block languages that stay as plain code. Everything else (untagged,
// `mf`, `thunky`) becomes an editable, runnable snippet; `mf-static` is
// thunky that is a fragment rather than a program, shown highlighted but
// without a Run button.
const STATIC_LANGS = new Set(["text", "txt", "ebnf", "bnf", "sh", "bash", "console", "go", "yaml", "none"]);

function currentPage() {
    const id = new URLSearchParams(location.search).get("page") || "home";
    return PAGES.find(p => p.id === id) || PAGES[0];
}

function buildSidebar(active) {
    const nav = document.getElementById("sidebar");
    const list = document.createElement("ul");
    for (const page of PAGES) {
        const li = document.createElement("li");
        const a = document.createElement("a");
        a.href = page.id === "home" ? "index.html" : "index.html?page=" + page.id;
        a.textContent = page.title;
        if (page.id === active.id) a.classList.add("active");
        li.appendChild(a);
        list.appendChild(li);
    }
    nav.appendChild(list);
}

// GitHub-style slugs so intra-doc #anchors keep working.
function slugify(text) {
    return text.toLowerCase().replace(/[^\w\- ]+/g, "").trim().replace(/ +/g, "-");
}

function addHeadingIds(root) {
    const used = new Set();
    for (const h of root.querySelectorAll("h1, h2, h3, h4, h5, h6")) {
        let slug = slugify(h.textContent);
        let n = 1;
        while (used.has(slug)) slug = slugify(h.textContent) + "-" + n++;
        used.add(slug);
        h.id = slug;
    }
}

// Rewrite relative links between the markdown files to ?page= URLs.
function rewriteLinks(root) {
    for (const a of root.querySelectorAll("a[href]")) {
        const href = a.getAttribute("href");
        if (/^[a-z]+:/.test(href) || href.startsWith("#")) continue;
        const [pathPart, fragment] = href.split("#");
        const base = decodeURIComponent(pathPart).replace(/^\.\//, "").replace(/^docs\//, "");
        const target = PAGES.find(p => p.file.replace(/^docs\//, "") === base);
        if (target) {
            a.setAttribute("href", "index.html?page=" + target.id + (fragment ? "#" + fragment : ""));
        }
    }
}

// --- runnable snippets ---

const OUTPUT_BUILTINS = /\b(show|peek|write|bwrite)\b/;

// A doc example is often a bare expression annotated with `-- result`
// comments. Running it verbatim would print nothing, so when a program never
// mentions an output builtin we run `show (<program>)` instead (keeping any
// import clause in front).
function autoShow(src) {
    const stripped = src.replace(/('[^']*'|"[^"]*")/g, "''").replace(/--[^\n]*/g, "");
    if (OUTPUT_BUILTINS.test(stripped)) return { src, wrapped: false };
    if (/^\s*module\b|\bmodule\b/.test(stripped)) return { src, wrapped: false };

    const importClause = src.match(/^\s*import[\w\s,]*?\bin\b/);
    if (importClause) {
        return { src: importClause[0] + "\nshow (\n" + src.slice(importClause[0].length) + "\n)", wrapped: true };
    }
    return { src: "show (\n" + src + "\n)", wrapped: true };
}

function makeSnippet(pre, code, runnable) {
    const original = code.textContent.replace(/\n$/, "");
    const box = document.createElement("div");
    box.className = "snippet";
    pre.replaceWith(box);

    const editorHost = document.createElement("div");
    editorHost.className = "snippet-editor";
    box.appendChild(editorHost);

    const editor = CodeMirror(editorHost, {
        value: original,
        mode: "thunky",
        theme: "mf",
        readOnly: !runnable,
        viewportMargin: Infinity,
        lineWrapping: true,
    });

    if (!runnable) return;

    const bar = document.createElement("div");
    bar.className = "snippet-bar";
    const runBtn = document.createElement("button");
    runBtn.textContent = "Run";
    const resetBtn = document.createElement("button");
    resetBtn.textContent = "Reset";
    const note = document.createElement("span");
    note.className = "snippet-note";
    bar.append(runBtn, resetBtn, note);
    box.appendChild(bar);

    const output = document.createElement("pre");
    output.className = "snippet-output";
    output.hidden = true;
    box.appendChild(output);

    resetBtn.addEventListener("click", () => {
        editor.setValue(original);
        output.hidden = true;
        note.textContent = "";
    });

    runBtn.addEventListener("click", async () => {
        const { src, wrapped } = autoShow(editor.getValue());
        note.textContent = wrapped ? "showing final value" : "";
        output.hidden = false;
        output.textContent = "";
        output.classList.add("running");
        let got = "";
        const result = await ThunkyRunner.run(src, {
            path: "snippet.þ",
            onOutput: text => { got += text; output.textContent = got; },
        });
        output.classList.remove("running");
        if (result.cancelled) return;
        if (result.timedOut) {
            output.textContent = got + "\n[stopped: exceeded time limit]";
        } else if (result.hostError) {
            output.textContent = got + "\n[host error: " + result.hostError + "]";
        } else if (got === "") {
            output.textContent = "(no output)";
        }
    });

    editor.setOption("extraKeys", { "Ctrl-Enter": () => runBtn.click(), "Cmd-Enter": () => runBtn.click() });
}

function upgradeCodeBlocks(root) {
    for (const pre of [...root.querySelectorAll("pre")]) {
        const code = pre.querySelector("code");
        if (!code) continue;
        const langMatch = (code.className || "").match(/language-([\w-]+)/);
        const lang = langMatch ? langMatch[1] : "";
        if (STATIC_LANGS.has(lang)) continue;
        const runnable = lang !== "mf-static";
        makeSnippet(pre, code, runnable);
    }
}

async function main() {
    const page = currentPage();
    document.title = page.id === "home" ? "thunky" : page.title + " — thunky";
    buildSidebar(page);

    const content = document.getElementById("content");
    let md;
    try {
        const resp = await fetch(page.file);
        if (!resp.ok) throw new Error(resp.status);
        md = await resp.text();
    } catch (err) {
        content.textContent = "Could not load " + page.file + " (" + err.message + ")";
        return;
    }

    content.innerHTML = marked.parse(md);
    addHeadingIds(content);
    rewriteLinks(content);
    upgradeCodeBlocks(content);

    if (location.hash) {
        const target = document.getElementById(location.hash.slice(1));
        if (target) target.scrollIntoView();
    }
}

main();
