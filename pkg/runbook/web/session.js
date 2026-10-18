// session.js renders session state (Requirement 2): the captured-vars chip
// list, each step's most-recent-run line, and the on-demand log viewer.
import { h, button, revealToggle } from "/ui/components.js";

const varsPanelEl = document.getElementById("vars-panel");
const MASK = "••••••";

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

// renderVarsPanel shows vars as chips, masking those doc declares sensitive;
// with no vars the panel is hidden.
export function renderVarsPanel(vars, doc) {
  const entries = Object.entries(vars || {});
  varsPanelEl.hidden = !entries.length;
  const masked = sensitiveVarNames(doc);
  varsPanelEl.replaceChildren(...entries.map(([name, value]) => renderVarChip(name, value, masked.has(name))));
}

function renderVarChip(name, value, isMasked) {
  const valueEl = h("span", { className: "var-chip-value" }, isMasked ? MASK : value);
  return h("span", { className: "var-chip" },
    h("span", { className: "var-chip-name" }, `${name}=`),
    valueEl,
    isMasked && revealToggle(() => {
      valueEl.textContent = valueEl.textContent === MASK ? value : MASK;
    }));
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
export function renderLastRun(stepName, history, el) {
  const entry = findLastRun(history, stepName);
  const row = el || h("div", { className: "step-last-run" });
  row.replaceChildren();
  row.hidden = !entry;
  if (!entry) return row;

  row.append(h("span", {}, formatLastRun(entry)));
  if (entry.log_path) {
    const logView = h("pre", { className: "log-view", hidden: true });
    row.append(button({ label: "View log", onClick: () => void toggleLogView(entry.log_path, logView) }), logView);
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

export function updateStepLastRun(card, history) {
  const existing = card.querySelector(".step-last-run");
  if (!existing) return;
  renderLastRun(card.dataset.stepName, history, existing);
}
