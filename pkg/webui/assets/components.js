// components.js: the shared component kit for serve UIs. Each factory returns
// a DOM element, or {el, ...methods} when the component has behavior. Text is
// always set as text content, never parsed as HTML.

// h builds an element. props: className, on ({event: handler}), and any
// other key, set as a DOM property when the element has one (value, type,
// disabled, hidden, ...) else as an attribute (data-*, aria-*). Children may
// be strings, elements, arrays, or null/false (skipped).
export function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  const { className, on, ...rest } = props;
  if (className) el.className = className;
  for (const [event, handler] of Object.entries(on || {})) el.addEventListener(event, handler);
  for (const [key, value] of Object.entries(rest)) {
    if (value == null || value === false) continue;
    if (key in el) el[key] = value;
    else el.setAttribute(key, value === true ? "" : value);
  }
  for (const child of children.flat()) {
    if (child != null && child !== false) el.append(child);
  }
  return el;
}

// button variants: primary, ghost, danger; sizes: sm, md.
export function button({ label, variant = "ghost", size = "sm", onClick, ...props }) {
  return h("button", {
    type: "button",
    className: `btn btn-${size} btn-${variant}`,
    on: onClick ? { click: onClick } : undefined,
    ...props,
  }, label);
}

// confirmButton arms on the first click (showing armLabel) and calls
// onConfirm on the second. reset() disarms it, e.g. once the action is done.
export function confirmButton({ label, armLabel, onConfirm, ...props }) {
  let armed = false;
  const el = button({
    label,
    ...props,
    onClick: () => {
      if (!armed) {
        armed = true;
        el.textContent = armLabel;
        return;
      }
      onConfirm();
    },
  });
  return {
    el,
    reset() {
      armed = false;
      el.textContent = label;
    },
  };
}

export function textInput(props = {}) {
  return h("input", { className: "field-input", type: "text", ...props });
}

// select options: [{value, label}].
export function select({ options, value, ...props }) {
  return h("select", { className: "field-input", ...props },
    options.map((o) => h("option", { value: o.value, selected: o.value === value }, o.label)));
}

// revealToggle is the eye button that shows/hides a secret value.
export function revealToggle(onToggle) {
  return h("button", { type: "button", className: "reveal-toggle", title: "Reveal value", on: { click: onToggle } }, "👁");
}

// field labels a control. With secret, the control is masked and gets a
// reveal toggle.
export function field({ label, control, secret = false }) {
  const row = h("div", { className: "field-input-row" }, control);
  if (secret) {
    control.type = "password";
    control.autocomplete = "off";
    row.append(revealToggle(() => {
      control.type = control.type === "password" ? "text" : "password";
    }));
  }
  return h("div", { className: "field" }, h("label", { className: "field-label" }, label), row);
}

// badge tones: neutral, success, warning, danger.
export function badge(text, tone = "neutral") {
  return h("span", { className: tone === "neutral" ? "badge" : `badge badge-${tone}` }, text);
}

export function card({ title, badges = [], body = [], actions = [], ...props }) {
  const className = props.className ? `card ${props.className}` : "card";
  return h("section", { ...props, className },
    h("div", { className: "card-header" },
      h("span", { className: "card-title" }, title),
      h("span", { className: "card-badges" }, badges)),
    body,
    actions.length ? h("div", { className: "card-actions" }, actions) : null);
}

// logView is a streaming output pane. append(text, kind) adds a line; kind
// "stderr" or "status" styles it. It stays scrolled to the bottom unless the
// user scrolled up.
export function logView(props = {}) {
  const el = h("div", { className: "output", ...props });
  return {
    el,
    append(text, kind) {
      const pinned = el.scrollTop + el.clientHeight >= el.scrollHeight - 4;
      el.append(h("div", { className: kind ? `${kind}-line` : undefined }, text));
      if (pinned) el.scrollTop = el.scrollHeight;
    },
    clear() {
      el.textContent = "";
    },
  };
}

export function emptyState({ icon, text }) {
  return h("div", { className: "empty-state" },
    h("div", { className: "empty-state-icon" }, icon),
    h("div", {}, text));
}
