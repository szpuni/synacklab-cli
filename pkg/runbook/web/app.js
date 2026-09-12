const docEl = document.getElementById("doc");
const cwdEl = document.getElementById("cwd");

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
  cwdEl.textContent = doc.session.cwd;
  for (const block of doc.blocks) {
    docEl.appendChild(block.kind === "step" ? renderStep(block.step) : renderProse(block.prose));
  }
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
  header.innerHTML = `<span class="step-name">${escapeHTML(step.name)}</span><span class="step-lang">${escapeHTML(step.lang)}</span>`;
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
      const label = document.createElement("label");
      label.textContent = name;
      const input = document.createElement("input");
      input.type = "text";
      inputEls[name] = input;
      label.appendChild(input);
      inputsDiv.appendChild(label);
    }
    card.appendChild(inputsDiv);
  }

  let confirmBanner = null;
  if (step.requires_confirm) {
    confirmBanner = document.createElement("div");
    confirmBanner.className = "confirm-banner";
    confirmBanner.textContent = `Requires confirmation: ${step.confirm_reason}`;
    card.appendChild(confirmBanner);
  }

  const runBtn = document.createElement("button");
  runBtn.type = "button";
  runBtn.textContent = "Run";
  if (step.requires_confirm) runBtn.className = "danger";
  card.appendChild(runBtn);

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

void loadDoc();
