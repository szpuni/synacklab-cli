const docEl = document.getElementById("doc");
const cwdEl = document.getElementById("cwd");
const activeFileEl = document.getElementById("active-file");
const fileTreeEl = document.getElementById("file-tree");
const sidebarEl = document.getElementById("sidebar");
const resetBtn = document.getElementById("reset-btn");

let activeFile = null;
let runButtons = [];

// createConfirmGate is the single arm-then-confirm state machine shared by
// every destructive control (per-step Run buttons that require confirmation,
// and the reset-session button): the first click arms it (relabeling the
// button) and the second click fires onConfirmed. reset() forces it back to
// unarmed, used whenever the pending action completes or fails to start.
function createConfirmGate(button, { armLabel, baseLabel, onConfirmed }) {
  let armed = false;
  return {
    handleClick() {
      if (!armed) {
        armed = true;
        button.textContent = armLabel;
        return;
      }
      onConfirmed();
    },
    reset() {
      armed = false;
      button.textContent = baseLabel;
    },
  };
}

async function loadFiles() {
  const res = await fetch("/api/files");
  if (!res.ok) {
    fileTreeEl.innerHTML = "";
    return;
  }
  const tree = await res.json();
  fileTreeEl.innerHTML = "";
  if (!tree.children || !tree.children.length) {
    const empty = document.createElement("div");
    empty.className = "tree-empty";
    empty.textContent = "No .md files found";
    fileTreeEl.appendChild(empty);
    return;
  }
  for (const child of tree.children) {
    fileTreeEl.appendChild(renderTreeNode(child));
  }
}

function renderTreeNode(node) {
  if (!node.dir) {
    const item = document.createElement("div");
    item.className = "tree-file";
    item.dataset.path = node.path;
    item.innerHTML = `<span class="file-icon">▤</span><span>${escapeHTML(node.name)}</span>`;
    item.addEventListener("click", () => void openFile(node.path));
    return item;
  }

  const wrapper = document.createElement("div");
  wrapper.className = "tree-node tree-dir";

  const label = document.createElement("div");
  label.className = "tree-label";
  label.innerHTML = `<span class="tree-caret">▾</span><span>${escapeHTML(node.name)}</span>`;
  label.addEventListener("click", () => wrapper.classList.toggle("collapsed"));
  wrapper.appendChild(label);

  const children = document.createElement("div");
  children.className = "tree-children";
  for (const child of node.children || []) {
    children.appendChild(renderTreeNode(child));
  }
  wrapper.appendChild(children);

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
  renderVarsPanel(session.vars);
  for (const el of docEl.querySelectorAll(".step")) {
    updateStepLastRun(el, session.history);
  }
}

function renderDoc(doc) {
  docEl.innerHTML = "";
  runButtons = [];
  lastDoc = doc;

  if (!doc.active) {
    activeFile = null;
    activeFileEl.textContent = "Select a runbook";
    cwdEl.textContent = "";
    varsPanelEl.hidden = true;
    docEl.appendChild(renderEmptyState());
    highlightActiveFile();
    return;
  }

  activeFile = doc.active_file || null;
  activeFileEl.textContent = activeFile || "Runbook";
  cwdEl.textContent = doc.session.cwd;
  renderVarsPanel(doc.session.vars);
  highlightActiveFile();

  const inner = document.createElement("div");
  inner.className = "doc-inner";
  for (const block of doc.blocks) {
    inner.appendChild(block.kind === "step" ? renderStep(block.step, doc.session.history) : renderProse(block.prose));
  }
  docEl.appendChild(inner);
}

function renderEmptyState() {
  const div = document.createElement("div");
  div.className = "empty-state";
  div.innerHTML = `<div class="empty-state-icon">▤</div><div>Choose a runbook from the file list to get started.</div>`;
  return div;
}

function renderProse(html) {
  const div = document.createElement("div");
  div.className = "prose";
  div.innerHTML = html;
  return div;
}

function renderStep(step, history) {
  const card = document.createElement("section");
  card.className = "step";
  card.dataset.stepName = step.name;

  const header = document.createElement("div");
  header.className = "step-header";
  header.innerHTML = `<span class="step-name">${escapeHTML(step.name)}</span><span class="badge">${escapeHTML(step.lang)}</span>`;
  card.appendChild(header);

  const source = document.createElement("pre");
  source.className = "source";
  source.textContent = step.source;
  card.appendChild(source);

  card.appendChild(renderLastRun(step.name, history));

  const inputEls = {};
  if (step.input && step.input.length) {
    const inputsDiv = document.createElement("div");
    inputsDiv.className = "inputs";
    for (const name of step.input) {
      const isSensitive = !!(step.sensitive && step.sensitive.includes(name));
      const field = document.createElement("div");
      field.className = "field";
      const label = document.createElement("label");
      label.className = "field-label";
      label.textContent = name;
      const input = document.createElement("input");
      input.className = "field-input";
      input.type = isSensitive ? "password" : "text";
      if (isSensitive) input.autocomplete = "off";
      inputEls[name] = input;
      field.appendChild(label);

      const inputRow = document.createElement("div");
      inputRow.className = "field-input-row";
      inputRow.appendChild(input);
      if (isSensitive) {
        const toggle = document.createElement("button");
        toggle.type = "button";
        toggle.className = "reveal-toggle";
        toggle.title = "Reveal value";
        toggle.textContent = "👁";
        toggle.addEventListener("click", () => {
          input.type = input.type === "password" ? "text" : "password";
        });
        inputRow.appendChild(toggle);
      }
      field.appendChild(inputRow);
      inputsDiv.appendChild(field);
    }
    card.appendChild(inputsDiv);
  }

  if (step.requires_confirm) {
    const confirmBanner = document.createElement("div");
    confirmBanner.className = "confirm-banner";
    confirmBanner.textContent = `⚠ Requires confirmation: ${step.confirm_reason}`;
    card.appendChild(confirmBanner);
  }

  const actions = document.createElement("div");
  actions.className = "step-actions";
  const runBtn = document.createElement("button");
  runBtn.type = "button";
  runBtn.className = `btn btn-md ${step.requires_confirm ? "btn-danger" : "btn-primary"}`;
  runBtn.textContent = "Run";
  actions.appendChild(runBtn);
  runButtons.push(runBtn);

  const stopBtn = document.createElement("button");
  stopBtn.type = "button";
  stopBtn.className = "btn btn-ghost btn-sm";
  stopBtn.textContent = "Stop";
  stopBtn.hidden = true;
  actions.appendChild(stopBtn);

  card.appendChild(actions);

  const output = document.createElement("div");
  output.className = "output";
  output.hidden = true;
  card.appendChild(output);

  const execState = { id: null };
  stopBtn.addEventListener("click", () => {
    if (execState.id) void fetch(`/api/executions/${execState.id}/cancel`, { method: "POST" });
  });

  const gate = step.requires_confirm
    ? createConfirmGate(runBtn, {
        armLabel: "Confirm & Run",
        baseLabel: "Run",
        onConfirmed: () => void runStep(step, inputEls, runBtn, stopBtn, output, execState, gate),
      })
    : null;

  runBtn.addEventListener("click", () => {
    if (gate) {
      gate.handleClick();
    } else {
      void runStep(step, inputEls, runBtn, stopBtn, output, execState, gate);
    }
  });

  return card;
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
  resetBtn.disabled = disabled;
}

async function runStep(step, inputEls, runBtn, stopBtn, output, execState, gate) {
  const inputs = {};
  for (const [name, el] of Object.entries(inputEls)) {
    if (!el.value) {
      alert(`Missing required input: ${name}`);
      return;
    }
    inputs[name] = el.value;
  }

  runBtn.disabled = true;
  setOthersDisabled(true, runBtn);
  output.hidden = false;
  output.textContent = "";

  const res = await fetch(`/api/steps/${encodeURIComponent(step.name)}/run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ inputs, confirmed: step.requires_confirm }),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    output.textContent = `error: ${body.error || res.statusText}`;
    runBtn.disabled = false;
    setOthersDisabled(false, runBtn);
    gate?.reset();
    return;
  }

  const { execution_id } = await res.json();
  execState.id = execution_id;
  stopBtn.hidden = false;
  streamExecution(execution_id, output, runBtn, stopBtn, execState, gate);
}

function streamExecution(executionID, output, runBtn, stopBtn, execState, gate) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${proto}//${location.host}/ws/executions/${executionID}`);

  ws.onmessage = (msg) => {
    const ev = JSON.parse(msg.data);
    const pinned = output.scrollTop + output.clientHeight >= output.scrollHeight - 4;

    const line = document.createElement("div");
    if (ev.type === "stdout") {
      line.textContent = ev.data;
    } else if (ev.type === "stderr") {
      line.textContent = ev.data;
      line.className = "stderr-line";
    } else if (ev.type === "done") {
      line.className = "status-line";
      if (ev.canceled) {
        line.textContent = "canceled";
      } else if (ev.timed_out) {
        line.textContent = "timed out";
      } else {
        line.textContent = `exit code ${ev.exit_code} (${ev.duration_ms}ms)`;
      }
      runBtn.disabled = false;
      stopBtn.hidden = true;
      execState.id = null;
      setOthersDisabled(false, runBtn);
      gate?.reset();
      void refreshSession();
    }
    output.appendChild(line);
    if (pinned) output.scrollTop = output.scrollHeight;
  };

  ws.onerror = () => {
    runBtn.disabled = false;
    stopBtn.hidden = true;
    execState.id = null;
    setOthersDisabled(false, runBtn);
    gate?.reset();
  };
}

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

const resetGate = createConfirmGate(resetBtn, {
  armLabel: "Confirm reset",
  baseLabel: "Reset session",
  onConfirmed: async () => {
    await fetch("/api/session/reset", { method: "POST" });
    await loadDoc();
    resetGate.reset();
  },
});
resetBtn.addEventListener("click", () => resetGate.handleClick());

document.getElementById("sidebar-toggle").addEventListener("click", () => {
  sidebarEl.classList.toggle("open");
});

void loadFiles();
void loadDoc();
