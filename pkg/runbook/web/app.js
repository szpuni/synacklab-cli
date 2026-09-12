const docEl = document.getElementById("doc");
const cwdEl = document.getElementById("cwd");
const activeFileEl = document.getElementById("active-file");
const fileTreeEl = document.getElementById("file-tree");
const sidebarEl = document.getElementById("sidebar");

let activeFile = null;

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

async function openFile(path) {
  const res = await fetch("/api/open", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ file: path }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
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
  cwdEl.textContent = session.cwd;
}

function renderDoc(doc) {
  docEl.innerHTML = "";

  if (!doc.active) {
    activeFile = null;
    activeFileEl.textContent = "Select a runbook";
    cwdEl.textContent = "";
    docEl.appendChild(renderEmptyState());
    highlightActiveFile();
    return;
  }

  activeFile = doc.active_file || null;
  activeFileEl.textContent = activeFile || "Runbook";
  cwdEl.textContent = doc.session.cwd;
  highlightActiveFile();

  const inner = document.createElement("div");
  inner.className = "doc-inner";
  for (const block of doc.blocks) {
    inner.appendChild(block.kind === "step" ? renderStep(block.step) : renderProse(block.prose));
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

function renderStep(step) {
  const card = document.createElement("section");
  card.className = "step";

  const header = document.createElement("div");
  header.className = "step-header";
  header.innerHTML = `<span class="step-name">${escapeHTML(step.name)}</span><span class="badge">${escapeHTML(step.lang)}</span>`;
  card.appendChild(header);

  const source = document.createElement("pre");
  source.className = "source";
  source.textContent = step.source;
  card.appendChild(source);

  const inputEls = {};
  if (step.input && step.input.length) {
    const inputsDiv = document.createElement("div");
    inputsDiv.className = "inputs";
    for (const name of step.input) {
      const field = document.createElement("div");
      field.className = "field";
      const label = document.createElement("label");
      label.className = "field-label";
      label.textContent = name;
      const input = document.createElement("input");
      input.className = "field-input";
      input.type = "text";
      inputEls[name] = input;
      field.appendChild(label);
      field.appendChild(input);
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
  card.appendChild(actions);

  const output = document.createElement("div");
  output.className = "output";
  output.hidden = true;
  card.appendChild(output);

  let confirmed = !step.requires_confirm;
  runBtn.addEventListener("click", () => {
    if (step.requires_confirm && !confirmed) {
      confirmed = true;
      runBtn.textContent = "Confirm & Run";
      return;
    }
    void runStep(step, inputEls, runBtn, output);
  });

  return card;
}

async function runStep(step, inputEls, runBtn, output) {
  const inputs = {};
  for (const [name, el] of Object.entries(inputEls)) {
    if (!el.value) {
      alert(`Missing required input: ${name}`);
      return;
    }
    inputs[name] = el.value;
  }

  runBtn.disabled = true;
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
    return;
  }

  const { execution_id } = await res.json();
  streamExecution(execution_id, output, runBtn);
}

function streamExecution(executionID, output, runBtn) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${proto}//${location.host}/ws/executions/${executionID}`);

  ws.onmessage = (msg) => {
    const ev = JSON.parse(msg.data);
    const line = document.createElement("div");
    if (ev.type === "stdout") {
      line.textContent = ev.data;
    } else if (ev.type === "stderr") {
      line.textContent = ev.data;
      line.className = "stderr-line";
    } else if (ev.type === "done") {
      line.className = "status-line";
      line.textContent = ev.timed_out
        ? "timed out"
        : `exit code ${ev.exit_code} (${ev.duration_ms}ms)`;
      runBtn.disabled = false;
      void refreshSession();
    }
    output.appendChild(line);
  };

  ws.onerror = () => {
    runBtn.disabled = false;
  };
}

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

document.getElementById("reset-btn").addEventListener("click", async () => {
  await fetch("/api/session/reset", { method: "POST" });
  await loadDoc();
});

document.getElementById("sidebar-toggle").addEventListener("click", () => {
  sidebarEl.classList.toggle("open");
});

void loadFiles();
void loadDoc();
