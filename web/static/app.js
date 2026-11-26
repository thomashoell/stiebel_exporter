// Refreshes the values of the page every 10 seconds (the default poll interval of the exporter).
"use strict";

const refreshMs = 10000;

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
  document.getElementById("mode").textContent = data.mode || "–";
  const updated = document.getElementById("updated");
  const time = new Date(data.time);
  updated.dateTime = time.toISOString();
  updated.textContent = time.toLocaleTimeString("de-DE");
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
