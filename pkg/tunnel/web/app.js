import { h, button, textInput, select, field, badge, card, logView } from "/ui/components.js";

const formEl = document.getElementById("open-form");
const tunnelsEl = document.getElementById("tunnels");

const AD_HOC = "__adhoc";
const STATE_TONES = { starting: "warning", connected: "success", closing: "warning", closed: "neutral", failed: "danger" };
const isActive = (state) => state === "starting" || state === "connected" || state === "closing";

let config = null;
let openBtn = null;
let formError = null;

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({ error: res.statusText }));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

function renderForm() {
  const databases = config.databases.map((db) => ({ value: db.name, label: `${db.name} — ${db.engine} ${db.host}:${db.port}` }));
  const target = select({ options: [...databases, { value: AD_HOC, label: "Ad hoc host…" }], value: databases[0]?.value || AD_HOC });
  const host = textInput({ placeholder: "db.internal" });
  const engine = select({ options: config.engines.map((e) => ({ value: e.name, label: e.name })) });
  const port = textInput({ type: "number", min: 1, max: 65535 });
  const localPort = textInput({ type: "number", min: 1, max: 65535 });
  const context = select({ options: config.contexts.map((c) => ({ value: c, label: c })), value: config.defaults.context });
  const namespace = textInput({ value: config.defaults.namespace || "" });

  const adHocFields = [field({ label: "Host", control: host }), field({ label: "Engine", control: engine }), field({ label: "Port", control: port })];
  const enginePort = () => config.engines.find((e) => e.name === engine.value)?.port;
  const update = () => {
    const adHoc = target.value === AD_HOC;
    for (const f of adHocFields) f.hidden = !adHoc;
    port.placeholder = enginePort();
    localPort.placeholder = adHoc ? port.value || enginePort() : config.databases.find((db) => db.name === target.value).port;
  };
  for (const el of [target, engine, port]) el.addEventListener("input", update);

  formError = h("div", { className: "form-error", role: "alert" });
  openBtn = button({
    label: "Open tunnel",
    variant: "primary",
    size: "md",
    onClick: () => {
      const adHoc = target.value === AD_HOC;
      void openTunnel({
        name: adHoc ? undefined : target.value,
        host: adHoc ? host.value.trim() : undefined,
        engine: adHoc ? engine.value : undefined,
        port: adHoc ? Number(port.value) || undefined : undefined,
        local_port: Number(localPort.value) || undefined,
        context: context.value,
        namespace: namespace.value.trim(),
      });
    },
  });

  formEl.replaceChildren(card({
    title: "Open a tunnel",
    body: [
      config.database_error && h("div", { className: "banner" }, `Configured databases are unavailable: ${config.database_error}`),
      h("div", { className: "form-grid" },
        field({ label: "Database", control: target }),
        adHocFields,
        field({ label: "Local port", control: localPort }),
        field({ label: "Kube context", control: context }),
        field({ label: "Namespace", control: namespace })),
    ],
    actions: [openBtn, formError],
  }));
  update();
}

async function openTunnel(req) {
  formError.textContent = "";
  openBtn.disabled = true;
  try {
    const { id } = await api("POST", "/api/tunnels", req);
    watch(id);
  } catch (err) {
    formError.textContent = err.message;
    openBtn.disabled = false;
  }
}

// watch renders a tunnel's card and keeps it current from the server's
// stream: output so far, then each change, until the tunnel ends.
function watch(id) {
  const output = logView();
  let el = null;
  const render = (t) => {
    const next = renderTunnel(t, output);
    if (el) el.replaceWith(next);
    else tunnelsEl.replaceChildren(next); // ponytail: one tunnel at a time; append once several are allowed
    el = next;
    openBtn.disabled = isActive(t.state);
  };

  const ws = new WebSocket(`ws://${location.host}/ws/tunnels/${id}`);
  ws.onmessage = (msg) => {
    const ev = JSON.parse(msg.data);
    if (ev.type === "output") {
      for (const line of ev.text.replace(/\n$/, "").split("\n")) output.append(line);
    } else {
      render(ev.tunnel);
    }
  };
  ws.onclose = () => void refresh(id, render);
}

// refresh shows the tunnel's final state if the stream ended without it.
async function refresh(id, render) {
  const tunnels = await api("GET", "/api/tunnels").catch(() => []);
  const t = tunnels.find((x) => x.id === id);
  if (t) render(t);
  else openBtn.disabled = false;
}

function renderTunnel(t, output) {
  const details = h("dl", { className: "details" },
    [["Pod", t.pod || "—"], ["Namespace", t.namespace], ["Context", t.context || "current"], ["Local address", t.local_addr]]
      .flatMap(([k, v]) => [h("dt", {}, k), h("dd", {}, v)]));

  const copy = button({ label: "Copy" });
  copy.addEventListener("click", async () => {
    await navigator.clipboard.writeText(t.hint);
    copy.textContent = "Copied";
  });

  return card({
    title: t.target,
    badges: [badge(t.state, STATE_TONES[t.state])],
    body: [
      details,
      t.state === "connected" && t.hint && h("div", { className: "hint" }, h("code", {}, t.hint), copy),
      t.error && h("pre", { className: "error-text" }, t.error),
      output.el,
    ],
    actions: isActive(t.state) && t.state !== "closing"
      ? [button({ label: "Close tunnel", variant: "danger", onClick: () => void api("DELETE", `/api/tunnels/${t.id}`) })]
      : [],
  });
}

async function load() {
  config = await api("GET", "/api/config");
  renderForm();
  const tunnels = await api("GET", "/api/tunnels");
  for (const t of tunnels) watch(t.id);
}

void load();
