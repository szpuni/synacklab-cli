// session.js renders session state (Requirement 2): the captured-vars chip
// list, each step's most-recent-run line, and the on-demand log viewer.
// Loaded before app.js, which shares its top-level bindings (lastDoc,
// varsPanelEl) via the page's global scope — both script tags are classic
// (non-module) scripts in the same document.
const varsPanelEl = document.getElementById("vars-panel");

let lastDoc = null;

// sensitiveVarNames returns every session.vars key that should render
// masked: a captured variable's name matches a name its capturing step ever
// declared sensitive= (Requirement 5.5), keyed both by the step-qualified
// ("step.NAME") and short-alias ("NAME") forms capture.go already writes.
function sensitiveVarNames(doc) {
  const names = new Set();
  if (!doc || !doc.blocks) return names;
  for (const block of doc.blocks) {
    const step = block.step;
    if (!step || !step.sensitive) continue;
    for (const name of step.sensitive) {
      names.add(name);
      names.add(`${step.name}.${name}`);
    }
  }
  return names;
}

function renderVarsPanel(vars) {
  varsPanelEl.innerHTML = "";
  const entries = Object.entries(vars || {});
  if (!entries.length) {
    varsPanelEl.hidden = true;
    return;
  }
  varsPanelEl.hidden = false;

  const masked = sensitiveVarNames(lastDoc);
  for (const [name, value] of entries) {
    varsPanelEl.appendChild(renderVarChip(name, value, masked.has(name)));
  }
}

function renderVarChip(name, value, isMasked) {
  const chip = document.createElement("span");
  chip.className = "var-chip";

  const nameEl = document.createElement("span");
  nameEl.className = "var-chip-name";
  nameEl.textContent = `${name}=`;
  chip.appendChild(nameEl);

  const valueEl = document.createElement("span");
  valueEl.className = "var-chip-value";
  valueEl.textContent = isMasked ? "••••••" : value;
  chip.appendChild(valueEl);

  if (isMasked) {
    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "reveal-toggle";
    toggle.title = "Reveal value";
    toggle.textContent = "👁";
    toggle.addEventListener("click", () => {
      const revealed = valueEl.textContent !== "••••••";
      valueEl.textContent = revealed ? "••••••" : value;
    });
    chip.appendChild(toggle);
  }

  return chip;
}

function findLastRun(history, stepName) {
  for (let i = (history || []).length - 1; i >= 0; i--) {
    if (history[i].step_name === stepName) return history[i];
  }
  return null;
}

function formatLastRun(entry) {
  const status = entry.timed_out ? "timed out" : `exit code ${entry.exit_code}`;
  const when = entry.started_at ? new Date(entry.started_at).toLocaleString() : "";
  return `Last run: ${status} · ${entry.duration_ms}ms${when ? " · " + when : ""}`;
}

// renderLastRun builds (or, if el is given, updates) the .step-last-run row
// showing a step's most recent session.history entry, satisfying
// Requirement 2.2/2.4: rebuilt from server data, not accumulated in-memory
// state, so it survives a reload or a POST /api/open switch.
function renderLastRun(stepName, history, el) {
  const entry = findLastRun(history, stepName);
  const row = el || document.createElement("div");
  row.className = "step-last-run";
  row.innerHTML = "";
  row.hidden = !entry;
  if (!entry) return row;

  const text = document.createElement("span");
  text.textContent = formatLastRun(entry);
  row.appendChild(text);

  if (entry.log_path) {
    const viewBtn = document.createElement("button");
    viewBtn.type = "button";
    viewBtn.className = "btn btn-ghost btn-sm";
    viewBtn.textContent = "View log";
    const logView = document.createElement("pre");
    logView.className = "log-view";
    logView.hidden = true;
    viewBtn.addEventListener("click", () => void toggleLogView(entry.log_path, logView));
    row.appendChild(viewBtn);
    row.appendChild(logView);
  }

  return row;
}

async function toggleLogView(logPath, logView) {
  if (!logView.hidden) {
    logView.hidden = true;
    return;
  }
  const res = await fetch(`/api/logs?path=${encodeURIComponent(logPath)}`);
  logView.textContent = res.ok ? await res.text() : `failed to load log: ${res.statusText}`;
  logView.hidden = false;
}

function updateStepLastRun(card, history) {
  const existing = card.querySelector(".step-last-run");
  if (!existing) return;
  renderLastRun(card.dataset.stepName, history, existing);
}
