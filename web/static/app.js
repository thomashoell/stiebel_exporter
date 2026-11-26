// Refreshes the values of the page every 10 seconds (the default poll interval of the exporter) and, if the
// exporter runs with -web-write, sends operating mode changes.
"use strict";

const refreshMs = 10000;
// the values show a new operating mode only after the next poll of the exporter
const modeHoldMs = 15000;

const modeForm = document.getElementById("mode-form");
let modeDirty = false; // the user picked a mode that is not sent yet
let modePending = false;
let modeHoldUntil = 0;

function render(data) {
  for (const [key, item] of Object.entries(data.items)) {
    // a value can appear twice: in a tile at the top and in its section
    for (const el of document.querySelectorAll(`[data-key="${CSS.escape(key)}"]`)) {
      const value = el.querySelector("[data-value]");
      if (value) value.textContent = item.text;
      if (item.state) {
        el.dataset.state = item.state;
      } else {
        delete el.dataset.state;
      }
    }
  }
  if (Date.now() >= modeHoldUntil) {
    document.getElementById("mode").textContent = data.mode || "–";
    syncModeSelect(data.mode);
  }
  const updated = document.getElementById("updated");
  const time = new Date(data.time);
  updated.dateTime = time.toISOString();
  updated.textContent = time.toLocaleTimeString("de-DE");
}

// shows the current mode in the select unless the user is choosing one
function syncModeSelect(mode) {
  if (!modeForm || modeDirty || modePending) return;
  const select = modeForm.elements.mode;
  if (document.activeElement === select) return;
  const option = [...select.options].find((o) => o.value === mode);
  if (option) select.value = mode;
}

function setModeStatus(text, state) {
  const status = document.getElementById("mode-status");
  status.textContent = text;
  if (state) {
    status.dataset.state = state;
  } else {
    delete status.dataset.state;
  }
}

async function submitMode(event) {
  event.preventDefault();
  const select = modeForm.elements.mode;
  const button = modeForm.querySelector("button");
  const mode = select.value;
  if (!mode || modePending) return;
  if (!window.confirm(`Betriebsart auf „${mode}“ ändern?`)) return;
  modePending = true;
  select.disabled = button.disabled = true;
  setModeStatus("Wird gesetzt …");
  try {
    const resp = await fetch("/api/v1/operating-mode", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mode }),
    });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok) throw new Error(data.error || resp.statusText);
    document.getElementById("mode").textContent = data.mode;
    modeDirty = false;
    modeHoldUntil = Date.now() + modeHoldMs;
    setModeStatus(`Übernommen: ${data.mode}`, "on");
  } catch (err) {
    setModeStatus(`Nicht übernommen: ${err.message}`, "warn");
  } finally {
    modePending = false;
    select.disabled = button.disabled = false;
  }
}

if (modeForm) {
  modeForm.elements.mode.addEventListener("change", () => {
    modeDirty = true;
    setModeStatus("");
  });
  modeForm.addEventListener("submit", submitMode);
}

async function refresh() {
  const offline = document.getElementById("offline");
  try {
    const resp = await fetch("/api/v1/values", { cache: "no-store" });
    if (!resp.ok) throw new Error(resp.statusText);
    render(await resp.json());
    offline.hidden = true;
  } catch {
    offline.hidden = false;
  }
}

// no refresh while the tab is hidden; catch up when it becomes visible again
let timer;
function schedule() {
  clearInterval(timer);
  if (!document.hidden) {
    refresh();
    timer = setInterval(refresh, refreshMs);
  }
}
document.addEventListener("visibilitychange", schedule);
timer = setInterval(refresh, refreshMs);
