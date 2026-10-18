import { h, button, confirmButton, textInput, field, badge, card, logView, emptyState } from "/ui/components.js";
import { renderVarsPanel, renderLastRun, updateStepLastRun } from "./session.js";

const docEl = document.getElementById("doc");
const cwdEl = document.getElementById("cwd");
const activeFileEl = document.getElementById("active-file");
const fileTreeEl = document.getElementById("file-tree");
const sidebarEl = document.getElementById("sidebar");

let activeFile = null;
let runButtons = [];
let lastDoc = null;

async function loadFiles() {
  const res = await fetch("/api/files");
  if (!res.ok) {
    fileTreeEl.replaceChildren();
    return;
  }
  const tree = await res.json();
  if (!tree.children || !tree.children.length) {
    fileTreeEl.replaceChildren(h("div", { className: "tree-empty" }, "No .md files found"));
    return;
  }
  fileTreeEl.replaceChildren(...tree.children.map(renderTreeNode));
}

function renderTreeNode(node) {
  if (!node.dir) {
    return h("div", { className: "tree-file", "data-path": node.path, on: { click: () => void openFile(node.path) } },
      h("span", { className: "file-icon" }, "▤"),
      h("span", {}, node.name));
  }

  const wrapper = h("div", { className: "tree-node tree-dir" });
  wrapper.append(
    h("div", { className: "tree-label", on: { click: () => wrapper.classList.toggle("collapsed") } },
      h("span", { className: "tree-caret" }, "▾"),
      h("span", {}, node.name)),
    h("div", { className: "tree-children" }, (node.children || []).map(renderTreeNode)));
  return wrapper;
}

function highlightActiveFile() {
  for (const el of fileTreeEl.querySelectorAll(".tree-file")) {
    el.classList.toggle("active", el.dataset.path === activeFile);
  }
}

async function openFile(path, force = false) {
  const res = await fetch("/api/open", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ file: path, force }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    if (res.status === 409 && body.requires_confirmation) {
      const varsCount = lastDoc && lastDoc.session ? Object.keys(lastDoc.session.vars || {}).length : 0;
      const historyCount = lastDoc && lastDoc.session ? (lastDoc.session.history || []).length : 0;
      const proceed = confirm(
        `Switching runbooks will discard ${varsCount} captured variable(s) and ${historyCount} run(s) of history for the current session. Continue?`
      );
      if (proceed) {
        await openFile(path, true);
      }
      return;
    }
    alert(`Failed to open ${path}: ${body.error || res.statusText}`);
    return;
  }
  sidebarEl.classList.remove("open");
  await loadDoc();
}

async function loadDoc() {
  const res = await fetch("/api/doc");
  const doc = await res.json();
  renderDoc(doc);
}

async function refreshSession() {
  const res = await fetch("/api/session");
  const session = await res.json();
  if (lastDoc) {
    lastDoc.session = session;
  }
  cwdEl.textContent = session.cwd;
  renderVarsPanel(session.vars, lastDoc);
  for (const el of docEl.querySelectorAll(".step")) {
    updateStepLastRun(el, session.history);
  }
}

function renderDoc(doc) {
  runButtons = [];
  lastDoc = doc;

  if (!doc.active) {
    activeFile = null;
    activeFileEl.textContent = "Select a runbook";
    cwdEl.textContent = "";
    renderVarsPanel({}, null);
    docEl.replaceChildren(emptyState({ icon: "▤", text: "Choose a runbook from the file list to get started." }));
    highlightActiveFile();
    return;
  }

  activeFile = doc.active_file || null;
  activeFileEl.textContent = activeFile || "Runbook";
  cwdEl.textContent = doc.session.cwd;
  renderVarsPanel(doc.session.vars, doc);
  highlightActiveFile();

  docEl.replaceChildren(h("div", { className: "doc-inner" },
    doc.blocks.map((block) => (block.kind === "step" ? renderStep(block.step, doc.session.history) : renderProse(block.prose)))));
}

function renderProse(html) {
  const div = h("div", { className: "prose" });
  div.innerHTML = html; // server-rendered Markdown
  return div;
}

function renderStep(step, history) {
  const inputEls = {};
  const inputFields = (step.input || []).map((name) => {
    inputEls[name] = textInput();
    return field({ label: name, control: inputEls[name], secret: !!(step.sensitive && step.sensitive.includes(name)) });
  });

  const output = logView({ hidden: true });
  const execState = { id: null };
  const stopBtn = button({
    label: "Stop",
    hidden: true,
    onClick: () => {
      if (execState.id) void fetch(`/api/executions/${execState.id}/cancel`, { method: "POST" });
    },
  });

  const run = { button: null, reset: () => {} };
  const start = () => void runStep(step, inputEls, run, stopBtn, output, execState);
  if (step.requires_confirm) {
    const gate = confirmButton({ label: "Run", armLabel: "Confirm & Run", variant: "danger", size: "md", onConfirm: start });
    run.button = gate.el;
    run.reset = gate.reset;
  } else {
    run.button = button({ label: "Run", variant: "primary", size: "md", onClick: start });
  }
  runButtons.push(run.button);

  return card({
    title: step.name,
    badges: [badge(step.lang)],
    className: "step",
    "data-step-name": step.name,
    body: [
      h("pre", { className: "source" }, step.source),
      renderLastRun(step.name, history),
      inputFields.length ? h("div", { className: "inputs" }, inputFields) : null,
      step.requires_confirm ? h("div", { className: "confirm-banner" }, `⚠ Requires confirmation: ${step.confirm_reason}`) : null,
      h("div", { className: "card-actions" }, run.button, stopBtn),
      output.el,
    ],
  });
}

// setOthersDisabled implements Requirement 6.3: while one step's execution
// is in flight, every other step's Run control and the reset control are
// disabled too, mirroring the server-enforced single-in-flight guard before
// a rejected request would otherwise have to round-trip to discover it.
function setOthersDisabled(disabled, runningBtn) {
  for (const btn of runButtons) {
    if (btn === runningBtn) continue;
    btn.disabled = disabled;
  }
  resetGate.el.disabled = disabled;
}

async function runStep(step, inputEls, run, stopBtn, output, execState) {
  const inputs = {};
  for (const [name, el] of Object.entries(inputEls)) {
    if (!el.value) {
      alert(`Missing required input: ${name}`);
      return;
    }
    inputs[name] = el.value;
  }

  run.button.disabled = true;
  setOthersDisabled(true, run.button);
  output.el.hidden = false;
  output.clear();

  const res = await fetch(`/api/steps/${encodeURIComponent(step.name)}/run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ inputs, confirmed: step.requires_confirm }),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    output.append(`error: ${body.error || res.statusText}`);
    finish(run, stopBtn, execState);
    return;
  }

  const { execution_id } = await res.json();
  execState.id = execution_id;
  stopBtn.hidden = false;
  streamExecution(execution_id, output, run, stopBtn, execState);
}

// finish re-enables the controls once an execution ends or fails to start.
function finish(run, stopBtn, execState) {
  run.button.disabled = false;
  stopBtn.hidden = true;
  execState.id = null;
  setOthersDisabled(false, run.button);
  run.reset();
}

function streamExecution(executionID, output, run, stopBtn, execState) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${proto}//${location.host}/ws/executions/${executionID}`);

  ws.onmessage = (msg) => {
    const ev = JSON.parse(msg.data);
    if (ev.type === "stdout") {
      output.append(ev.data);
    } else if (ev.type === "stderr") {
      output.append(ev.data, "stderr");
    } else if (ev.type === "done") {
      if (ev.canceled) {
        output.append("canceled", "status");
      } else if (ev.timed_out) {
        output.append("timed out", "status");
      } else {
        output.append(`exit code ${ev.exit_code} (${ev.duration_ms}ms)`, "status");
      }
      finish(run, stopBtn, execState);
      void refreshSession();
    }
  };

  ws.onerror = () => finish(run, stopBtn, execState);
}

const resetGate = confirmButton({
  label: "Reset session",
  armLabel: "Confirm reset",
  onConfirm: async () => {
    await fetch("/api/session/reset", { method: "POST" });
    await loadDoc();
    resetGate.reset();
  },
});
resetGate.el.id = "reset-btn";
document.getElementById("reset-slot").replaceWith(resetGate.el);

document.getElementById("sidebar-toggle").addEventListener("click", () => {
  sidebarEl.classList.toggle("open");
});

void loadFiles();
void loadDoc();
