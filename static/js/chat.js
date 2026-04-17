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
  const loadOlderBtn = document.getElementById("load-older");
  const integrationSet = new Set((integrations || []).map(s => s.toLowerCase()));
  let lastID = initialLastID;
  let oldestID = parseInt(list.dataset.oldestId || "0", 10);

  function renderMessageHTML(msg) {
    const timeStr = timeAgo(new Date(msg.CreatedAt));
    const userHTML = msg.DisplayName
      ? `${escHtml(msg.DisplayName)} <span class="msg-username">${escHtml(msg.Username)}</span>`
      : escHtml(msg.Username);
    return `<div class="message" id="msg-${msg.ID}">
      <span class="msg-user">${userHTML}</span>
      <span class="msg-time">${timeStr}</span>
      <div class="msg-body">${msg.BodyHTML}</div>
    </div>`;
  }

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
        appendMessageHTML(renderMessageHTML(msg));
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
          appendMessageHTML(renderMessageHTML(msg));
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

  // "Load older messages" — keyset pagination via /history?before=<oldestID>.
  const PAGE_SIZE = 50;
  if (loadOlderBtn) {
    loadOlderBtn.addEventListener("click", async () => {
      if (!oldestID) return;
      loadOlderBtn.disabled = true;
      const prevHeight = list.scrollHeight;
      const prevTop = list.scrollTop;
      try {
        const res = await fetch(`${channelPath}/history?before=${oldestID}&limit=${PAGE_SIZE}`);
        if (!res.ok) return;
        const msgs = await res.json();
        if (!msgs || msgs.length === 0) {
          loadOlderBtn.remove();
          return;
        }
        // Insert oldest-first, each right after the button, in reverse so final
        // DOM order matches server-side chronological order.
        for (let i = msgs.length - 1; i >= 0; i--) {
          if (document.getElementById("msg-" + msgs[i].ID)) continue;
          const div = document.createElement("div");
          div.innerHTML = renderMessageHTML(msgs[i]).trim();
          loadOlderBtn.after(div.firstChild);
        }
        oldestID = msgs[0].ID;
        list.dataset.oldestId = String(oldestID);
        list.scrollTop = prevTop + (list.scrollHeight - prevHeight);
        if (msgs.length < PAGE_SIZE) loadOlderBtn.remove();
      } catch (_) {
      } finally {
        loadOlderBtn.disabled = false;
      }
    });
  }

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
    if (sub === "audio")     return rest ? `audio player for ${rest}` : "audio <lang> <guid> [form]";
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
