/**
 * chat.js — live message updates for Knoblauch.
 *
 * Tries SSE first; falls back to polling every 3 seconds if SSE is
 * unavailable (e.g. behind certain proxies).
 */

function initChat(channelPath, initialLastID) {
  const list = document.getElementById("message-list");
  const form = document.getElementById("post-form");
  const input = document.getElementById("msg-input");
  let lastID = initialLastID;

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
    list.appendChild(node);
    list.scrollTop = list.scrollHeight;
    // Update lastID from the new element's id attribute (msg-<id>).
    const id = parseInt(node.id?.replace("msg-", ""), 10);
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
          <p class="msg-body">${escHtml(msg.Body)}</p>
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
            <p class="msg-body">${escHtml(msg.Body)}</p>
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
    input.disabled = true;

    try {
      const res = await fetch(channelPath + "/post", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "HX-Request": "true",
        },
        body: "body=" + encodeURIComponent(body),
      });
      if (res.ok) {
        const html = await res.text();
        appendMessageHTML(html);
      }
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
