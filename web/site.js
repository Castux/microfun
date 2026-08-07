// Documentation shell: fetches the repo's markdown, renders it, and upgrades
// thunky code blocks into editable, runnable snippets backed by the wasm
// runner. index.html?page=<id> selects a page.

"use strict";

// Pages, grouped for the sidebar. `file` is relative to the site root, which
// mirrors the repository layout (see web/build.sh).
const PAGES = [
    { id: "home", title: "Overview", file: "README.md", group: "Thunky" },
    { id: "language", title: "Language reference", file: "docs/LANGUAGE.md", group: "Thunky" },

    { id: "tut", title: "Introduction", file: "docs/tutorial/README.md", group: "Tutorial" },
    { id: "tut1", title: "1. Getting started", file: "docs/tutorial/01-getting-started.md", group: "Tutorial" },
    { id: "tut2", title: "2. Functions", file: "docs/tutorial/02-functions-and-application.md", group: "Tutorial" },
    { id: "tut3", title: "3. Pipes & composition", file: "docs/tutorial/03-pipes-and-composition.md", group: "Tutorial" },
    { id: "tut4", title: "4. Pattern matching", file: "docs/tutorial/04-pattern-matching.md", group: "Tutorial" },
    { id: "tut5", title: "5. Let & recursion", file: "docs/tutorial/05-let-and-recursion.md", group: "Tutorial" },
    { id: "tut6", title: "6. Lists", file: "docs/tutorial/06-lists.md", group: "Tutorial" },
    { id: "tut7", title: "7. Lazy evaluation", file: "docs/tutorial/07-lazy-evaluation.md", group: "Tutorial" },
    { id: "tut8", title: "8. Standard library", file: "docs/tutorial/08-standard-library.md", group: "Tutorial" },
    { id: "tut9", title: "9. Modules", file: "docs/tutorial/09-modules.md", group: "Tutorial" },

    { id: "impl0", title: "0. Overview", file: "docs/implementation/0.Overview.md", group: "Implementation" },
    { id: "impl1", title: "1. Lexer", file: "docs/implementation/1.Lexer.md", group: "Implementation" },
    { id: "impl2", title: "2. Parser", file: "docs/implementation/2.Parser.md", group: "Implementation" },
    { id: "impl3", title: "3. Resolver", file: "docs/implementation/3.Resolver.md", group: "Implementation" },
    { id: "impl4", title: "4. Core IR & lowering", file: "docs/implementation/4.Core IR and Lowering.md", group: "Implementation" },
    { id: "impl5", title: "5. Bytecode & compiler", file: "docs/implementation/5.Bytecode and Compiler.md", group: "Implementation" },
    { id: "impl6", title: "6. The G-machine", file: "docs/implementation/6.The G-machine.md", group: "Implementation" },
    { id: "improvements", title: "Future improvements", file: "docs/implementation/IMPROVEMENTS.md", group: "Implementation" },
];

// Fenced-block info strings that stay as plain, non-interactive code. Anything
// else (untagged, `thunky`, `th`) becomes an editable, runnable snippet;
// `thunky-static` is Thunky source that is a fragment rather than a whole
// program — highlighted and editable, but with no Run button.
const STATIC_LANGS = new Set(["text", "txt", "output", "ebnf", "bnf", "sh", "bash", "console", "go", "yaml", "json", "none"]);

function currentPage() {
    const id = new URLSearchParams(location.search).get("page") || "home";
    return PAGES.find(p => p.id === id) || PAGES[0];
}

function buildSidebar(active) {
    const nav = document.getElementById("sidebar");
    let currentGroup = null;
    let list = null;
    for (const page of PAGES) {
        if (page.group !== currentGroup) {
            currentGroup = page.group;
            const heading = document.createElement("div");
            heading.className = "nav-group";
            heading.textContent = currentGroup;
            nav.appendChild(heading);
            list = document.createElement("ul");
            nav.appendChild(list);
        }
        const li = document.createElement("li");
        const a = document.createElement("a");
        a.href = page.id === "home" ? "index.html" : "index.html?page=" + page.id;
        a.textContent = page.title;
        if (page.id === active.id) a.classList.add("active");
        li.appendChild(a);
        list.appendChild(li);
    }
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

// Rewrite relative links between the markdown files to ?page= URLs. Links are
// resolved against the linking page's own directory, so `../LANGUAGE.md` from a
// tutorial chapter and `docs/LANGUAGE.md` from the README both find the same
// page. A link with no matching page is left alone.
function rewriteLinks(root, fromFile) {
    const baseDir = fromFile.includes("/") ? fromFile.replace(/\/[^/]*$/, "") : "";
    for (const a of root.querySelectorAll("a[href]")) {
        const href = a.getAttribute("href");
        if (/^[a-z]+:/i.test(href) || href.startsWith("#")) continue;
        const [pathPart, fragment] = href.split("#");
        if (!pathPart) continue;

        // Normalise "<baseDir>/<href>" by resolving . and .. segments.
        const segments = [];
        for (const seg of (baseDir ? baseDir + "/" : "").concat(decodeURIComponent(pathPart)).split("/")) {
            if (seg === "" || seg === ".") continue;
            if (seg === "..") segments.pop();
            else segments.push(seg);
        }
        const resolved = segments.join("/");

        const target = PAGES.find(p => p.file === resolved);
        if (target) {
            a.setAttribute("href", "index.html?page=" + target.id + (fragment ? "#" + fragment : ""));
        }
    }
}

// --- runnable snippets ---

// Shared editor settings. Thunky sources are indented with tabs, displayed
// four columns wide; Tab inserts a real tab rather than spaces.
const EDITOR_INDENT = {
    indentUnit: 4,
    tabSize: 4,
    indentWithTabs: true,
    extraKeys: {
        Tab: cm => cm.execCommand(cm.somethingSelected() ? "indentMore" : "insertTab"),
        "Shift-Tab": cm => cm.execCommand("indentLess"),
    },
};

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
        theme: "thunky",
        readOnly: !runnable,
        viewportMargin: Infinity,
        lineWrapping: true,
        ...EDITOR_INDENT,
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
        output.textContent = describeRun(got, result) || "(no output)";
    });

    editor.setOption("extraKeys", {
        ...EDITOR_INDENT.extraKeys,
        "Ctrl-Enter": () => runBtn.click(),
        "Cmd-Enter": () => runBtn.click(),
    });
}

// describeRun renders a finished run: whatever the program printed, plus a
// note only when something went wrong that the output does not already
// explain. The compiler and runtime write their own diagnostics to
// stdout/stderr, which the worker forwards, so a failing program's error text
// is already in `got` — appending "[host error]" on top of it would be noise.
function describeRun(got, result) {
    if (result.timedOut) {
        return got + (got ? "\n" : "") + "[stopped: exceeded the time limit]";
    }
    if (result.hostError) {
        return got + (got ? "\n" : "") + "[could not run: " + result.hostError + "]";
    }
    if (result.exitCode) {
        // A located diagnostic was already printed; only say so if it was not.
        return got || "[exited with status " + result.exitCode + "]";
    }
    return got;
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
    document.title = page.id === "home" ? "Thunky (Þunky)" : page.title + " — Thunky";
    buildSidebar(page);

    const content = document.getElementById("content");
    let md;
    try {
        const resp = await fetch(encodeURI(page.file));
        if (!resp.ok) throw new Error(resp.status);
        md = await resp.text();
    } catch (err) {
        content.textContent = "Could not load " + page.file + " (" + err.message + ")";
        return;
    }

    content.innerHTML = marked.parse(md);
    addHeadingIds(content);
    rewriteLinks(content, page.file);
    upgradeCodeBlocks(content);

    if (location.hash) {
        const target = document.getElementById(location.hash.slice(1));
        if (target) target.scrollIntoView();
    }
}

main();
