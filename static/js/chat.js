/**
 * chat.js — live message updates for Knoblauch.
 *
 * Tries SSE first; falls back to polling every 3 seconds if SSE is
 * unavailable (e.g. behind certain proxies).
 */

function initChat(channelPath, initialLastID, integrations) {
  const list = document.getElementById("message-list");
  const form = document.getElementById("post-form");
  const input = document.getElementById("msg-input");
  const hint = document.getElementById("msg-hint");
  const integrationSet = new Set((integrations || []).map(s => s.toLowerCase()));
  let lastID = initialLastID;

  // Show a hint when the message starts with a known @mention.
  input.addEventListener("input", () => {
    const val = input.value;
    if (!val.startsWith("@")) { hint.textContent = ""; return; }
    const name = val.slice(1).split(" ")[0].toLowerCase();
    if (name && integrationSet.has(name)) {
      hint.textContent = "@" + name + ": " + describeQuery(name, val.slice(name.length + 1).trim());
    } else {
      hint.textContent = "";
    }
  });

  // Remove the "no messages" placeholder once we receive real ones.
  function ensureNoEmpty() {
    const empty = list.querySelector("p.empty");
    if (empty) empty.remove();
  }

  function appendMessageHTML(html) {
    ensureNoEmpty();
    const div = document.createElement("div");
    div.innerHTML = html.trim();
    const node = div.firstChild;
    // Deduplicate: skip if a message with this ID is already in the list.
    const id = parseInt(node.id?.replace("msg-", ""), 10);
    if (!isNaN(id) && document.getElementById("msg-" + id)) return;
    list.appendChild(node);
    list.scrollTop = list.scrollHeight;
    if (!isNaN(id) && id > lastID) lastID = id;
  }

  // SSE path
  function startSSE() {
    const es = new EventSource(channelPath + "/events");
    es.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        // Build the message HTML client-side to avoid a round-trip.
        const timeStr = timeAgo(new Date(msg.CreatedAt));
        const html = `<div class="message" id="msg-${msg.ID}">
          <span class="msg-user">${escHtml(msg.Username)}</span>
          <span class="msg-time">${timeStr}</span>
          <div class="msg-body">${msg.BodyHTML}</div>
        </div>`;
        appendMessageHTML(html);
        lastID = msg.ID;
      } catch (_) {}
    };
    es.onerror = () => {
      es.close();
      // Fall back to polling.
      startPolling();
    };
    return es;
  }

  // Polling fallback
  function startPolling() {
    setInterval(async () => {
      try {
        const res = await fetch(`${channelPath}/poll?after=${lastID}`);
        if (!res.ok) return;
        const msgs = await res.json();
        if (!msgs) return;
        for (const msg of msgs) {
          const timeStr = timeAgo(new Date(msg.CreatedAt));
          const html = `<div class="message" id="msg-${msg.ID}">
            <span class="msg-user">${escHtml(msg.Username)}</span>
            <span class="msg-time">${timeStr}</span>
            <div class="msg-body">${msg.BodyHTML}</div>
          </div>`;
          appendMessageHTML(html);
          lastID = msg.ID;
        }
      } catch (_) {}
    }, 3000);
  }

  // Submit via fetch so we get the rendered fragment back (no full reload).
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const body = input.value.trim();
    if (!body) return;
    input.value = "";
    hint.textContent = "";
    input.disabled = true;

    try {
      const res = await fetch(channelPath + "/post", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: "body=" + encodeURIComponent(body),
      });
      // SSE delivers the message; no need to append here.
    } catch (_) {
      // Network error — the message may have been lost; user can retry.
    } finally {
      input.disabled = false;
      input.focus();
    }
  });

  // Scroll to bottom on load.
  list.scrollTop = list.scrollHeight;

  // Start live updates.
  if (typeof EventSource !== "undefined") {
    startSSE();
  } else {
    startPolling();
  }
}

// describeQuery returns a short hint string for a known integration query.
function describeQuery(name, query) {
  if (!query) return "type a query to send";
  const parts = query.trim().split(/\s+/);
  const sub = parts[0].toLowerCase();
  const rest = parts.slice(1).join(" ");
  if (name === "barsukas") {
    if (sub === "help")      return "show all commands";
    if (sub === "search")    return rest ? `search for "${rest}"` : "search for a lemma";
    if (sub === "info")      return rest ? `look up "${rest}"` : "look up a word";
    if (sub === "translate") return rest ? `translate ${rest}` : "translate <lang> <word>";
    if (sub === "forms")     return rest ? `forms of ${rest}` : "forms <lang> <guid>";
    if (sub === "grammar")   return rest ? `grammar for ${rest}` : "grammar <lang> <guid>";
    if (sub === "sentences") return rest ? `sentences for ${rest}` : "sentences <lang> <guid>";
    if (sub === "status")    return rest ? `status of ${rest}` : "status <guid>";
    if (sub === "stats")     return rest ? `corpus stats for ${rest}` : "corpus stats (all languages)";
    return `look up "${query}"`;
  }
  return `query ${name}`;
}

function escHtml(s) {
  return s.replace(/&/g, "&amp;")
          .replace(/</g, "&lt;")
          .replace(/>/g, "&gt;")
          .replace(/"/g, "&quot;");
}

function timeAgo(date) {
  const d = Date.now() - date.getTime();
  if (d < 60000) return "just now";
  if (d < 3600000) return Math.floor(d / 60000) + "m ago";
  if (d < 86400000) return Math.floor(d / 3600000) + "h ago";
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}
