(function () {
  var tabs = document.querySelectorAll(".month-tabs button");
  var panels = document.querySelectorAll(".tab-panel");
  var storageKey = "sir-mail-overview-theme";

  function activateTab(key) {
    for (var i = 0; i < panels.length; i++) {
      panels[i].classList.remove("active");
    }
    for (var j = 0; j < tabs.length; j++) {
      var t = tabs[j];
      var sel = t.getAttribute("data-month") === key;
      t.setAttribute("aria-selected", sel ? "true" : "false");
    }
    var p = document.querySelector('.tab-panel[data-month="' + key + '"]');
    if (p) {
      p.classList.add("active");
    }
  }

  for (var k = 0; k < tabs.length; k++) {
    tabs[k].addEventListener("click", function (ev) {
      activateTab(ev.currentTarget.getAttribute("data-month"));
    });
  }

  var initial = document.querySelector(".month-tabs button[aria-selected='true']");
  if (initial) {
    activateTab(initial.getAttribute("data-month"));
  }

  function applyStoredTheme() {
    var pref = localStorage.getItem(storageKey);
    if (pref === "light" || pref === "dark") {
      document.documentElement.setAttribute("data-theme", pref);
    }
  }

  var toggle = document.getElementById("theme-toggle");
  if (toggle) {
    applyStoredTheme();
    toggle.addEventListener("click", function () {
      var cur = document.documentElement.getAttribute("data-theme");
      var next = cur === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      localStorage.setItem(storageKey, next);
    });
  }

  function wireTable(tableId) {
    var table = document.getElementById(tableId);
    if (!table) {
      return;
    }
    var tbody = table.querySelector("tbody");
    if (!tbody) {
      return;
    }
    var search = document.getElementById(tableId + "-search");
    var rows = Array.prototype.slice.call(tbody.querySelectorAll("tr"));

    function rowText(tr) {
      return tr.innerText.toLowerCase();
    }

    function applyFilter() {
      var q = search ? search.value.toLowerCase().trim() : "";
      for (var i = 0; i < rows.length; i++) {
        var show = !q || rowText(rows[i]).indexOf(q) >= 0;
        rows[i].style.display = show ? "" : "none";
      }
    }

    if (search) {
      search.addEventListener("input", applyFilter);
    }

    var headers = table.querySelectorAll("thead th[data-sort]");
    for (var h = 0; h < headers.length; h++) {
      headers[h].style.cursor = "pointer";
      headers[h].addEventListener("click", function (ev) {
        var col = parseInt(ev.currentTarget.getAttribute("data-sort"), 10);
        var dir = ev.currentTarget.getAttribute("data-dir") === "asc" ? "desc" : "asc";
        ev.currentTarget.setAttribute("data-dir", dir);
        rows.sort(function (a, b) {
          var ca = a.cells[col] ? a.cells[col].innerText : "";
          var cb = b.cells[col] ? b.cells[col].innerText : "";
          if (dir === "asc") {
            return ca.localeCompare(cb);
          }
          return cb.localeCompare(ca);
        });
        for (var r = 0; r < rows.length; r++) {
          tbody.appendChild(rows[r]);
        }
      });
    }

    for (var d = 0; d < rows.length; d++) {
      rows[d].addEventListener("click", function (ev) {
        var tr = ev.currentTarget;
        if (!tr.classList.contains("msg-row")) {
          return;
        }
        var id = tr.getAttribute("data-hash");
        if (!id) {
          return;
        }
        var drill = document.getElementById("drill-" + id);
        if (!drill) {
          return;
        }
        var show = drill.style.display === "none" || drill.style.display === "";
        drill.style.display = show ? "table-row" : "none";
      });
    }
  }

  var tables = document.querySelectorAll("table.data[data-overview-table]");
  for (var t = 0; t < tables.length; t++) {
    wireTable(tables[t].id);
  }

  var moreButtons = document.querySelectorAll(".more-btn[data-more-for]");
  for (var mb = 0; mb < moreButtons.length; mb++) {
    moreButtons[mb].addEventListener("click", function (ev) {
      var btn = ev.currentTarget;
      var table = document.getElementById(btn.getAttribute("data-more-for"));
      if (!table) {
        return;
      }
      var extra = table.querySelectorAll("tr.extra");
      var show = btn.getAttribute("aria-expanded") !== "true";
      for (var i = 0; i < extra.length; i++) {
        extra[i].hidden = !show;
      }
      btn.setAttribute("aria-expanded", show ? "true" : "false");
      btn.textContent = show ? "Show fewer" : btn.getAttribute("data-more-label");
    });
  }

  var audits = document.querySelectorAll("details.audit");
  window.addEventListener("beforeprint", function () {
    for (var i = 0; i < audits.length; i++) {
      audits[i].setAttribute("data-was-open", audits[i].open ? "1" : "0");
      audits[i].open = true;
    }
  });
  window.addEventListener("afterprint", function () {
    for (var i = 0; i < audits.length; i++) {
      if (audits[i].getAttribute("data-was-open") !== "1") {
        audits[i].open = false;
      }
    }
  });

  wireHighlight();

  function wireHighlight() {
    var toggle = document.getElementById("highlight-toggle");
    var layer = document.getElementById("highlight-layer");
    var rectEl = document.getElementById("highlight-rect");
    var panel = document.getElementById("intent-panel");
    var statusEl = document.getElementById("intent-status");
    var textEl = document.getElementById("intent-text");
    var closeBtn = document.getElementById("intent-close");
    var markBtn = document.getElementById("intent-mark");
    if (!toggle || !layer || !rectEl || !panel || !statusEl || !textEl) {
      return;
    }

    var drag = null;
    var lastItems = [];
    var hashIndex = null;
    var cartItemCap = 200;
    var hint = document.getElementById("highlight-hint");

    function highlightOn() {
      return toggle.getAttribute("aria-pressed") === "true";
    }

    function setHighlight(on) {
      toggle.setAttribute("aria-pressed", on ? "true" : "false");
      toggle.textContent = on ? "Highlighting" : "Highlight";
      layer.hidden = !on;
      document.body.classList.toggle("highlight-on", on);
      if (hint) {
        hint.hidden = !on;
      }
      if (!on) {
        rectEl.hidden = true;
      }
    }

    toggle.addEventListener("click", function () {
      setHighlight(!highlightOn());
    });

    setHighlight(true);

    if (closeBtn) {
      closeBtn.addEventListener("click", function () {
        panel.hidden = true;
      });
    }

    function activeMonth() {
      var tab = document.querySelector(".month-tabs button[aria-selected='true']");
      if (!tab) {
        return "";
      }
      var label = tab.textContent || "";
      return label.replace(/\s+/g, " ").trim();
    }

    function intersects(a, b) {
      return !(a.right < b.left || a.left > b.right || a.bottom < b.top || a.top > b.bottom);
    }

    function visible(el) {
      if (!el || !el.getBoundingClientRect) {
        return false;
      }
      var r = el.getBoundingClientRect();
      if (r.width < 2 || r.height < 2) {
        return false;
      }
      var st = window.getComputedStyle(el);
      return st.display !== "none" && st.visibility !== "hidden";
    }

    function normalizeText(s) {
      return (s || "").replace(/\s+/g, " ").trim();
    }

    function messageByHash(h) {
      if (!hashIndex) {
        hashIndex = {};
        var raw = document.getElementById("overview-model");
        if (!raw || !raw.textContent) {
          return null;
        }
        try {
          var model = JSON.parse(raw.textContent);
          function ingest(sec) {
            if (!sec || !sec.messages) {
              return;
            }
            for (var i = 0; i < sec.messages.length; i++) {
              var m = sec.messages[i];
              if (m && m.object_hash) {
                hashIndex[m.object_hash] = m;
              }
            }
          }
          ingest(model.all);
          var months = model.months || [];
          for (var j = 0; j < months.length; j++) {
            ingest(months[j]);
          }
        } catch (err) {
          hashIndex = {};
        }
      }
      return hashIndex[h] || null;
    }

    function enrichItems(items) {
      var out = [];
      var seen = {};
      for (var i = 0; i < items.length; i++) {
        var it = items[i];
        var h = (it && it.object_hash) || "";
        if (!h || seen[h]) {
          continue;
        }
        seen[h] = true;
        var m = messageByHash(h);
        if (m) {
          if (!it.sender) {
            it.sender = m.sender || "";
          }
          if (!it.subject) {
            it.subject = m.subject || "";
          }
          if (!it.date) {
            it.date = m.date || "";
          }
        }
        out.push(it);
        if (out.length >= cartItemCap) {
          break;
        }
      }
      return out;
    }

    function itemsFromEl(el) {
      var out = [];
      var seen = {};
      while (el && el !== document.body) {
        var hashes = [];
        var one = el.getAttribute && el.getAttribute("data-hash");
        var many = el.getAttribute && el.getAttribute("data-hashes");
        if (one) {
          hashes.push(one);
        }
        if (many) {
          hashes = hashes.concat(many.split(","));
        }
        if (hashes.length) {
          var sender = el.getAttribute("data-sender") || "";
          var subject = el.getAttribute("data-subject") || "";
          var date = el.getAttribute("data-date") || "";
          for (var i = 0; i < hashes.length; i++) {
            var h = (hashes[i] || "").trim();
            if (!h || seen[h]) {
              continue;
            }
            seen[h] = true;
            out.push({ object_hash: h, sender: sender, subject: subject, date: date });
          }
          break;
        }
        el = el.parentElement;
      }
      return out;
    }

    function collectFromRect(box) {
      var empty = { text: "", items: [] };
      var root = document.querySelector(".tab-panel.active") || document.querySelector("main");
      if (!root) {
        return empty;
      }
      var sel = "h2, h3, p, .need-tile, .need-hint, td, th, .kname, .kcount, li, figcaption, caption, [data-hash], [data-hashes]";
      var nodes = root.querySelectorAll(sel);
      var seen = {};
      var parts = [];
      var items = [];
      var seenHash = {};
      for (var i = 0; i < nodes.length; i++) {
        var el = nodes[i];
        if (!visible(el) || !intersects(box, el.getBoundingClientRect())) {
          continue;
        }
        var t = normalizeText(el.innerText || el.textContent);
        if (t && !seen[t]) {
          seen[t] = true;
          parts.push(t);
        }
        var got = itemsFromEl(el);
        for (var j = 0; j < got.length; j++) {
          if (seenHash[got[j].object_hash]) {
            continue;
          }
          seenHash[got[j].object_hash] = true;
          items.push(got[j]);
        }
      }
      return { text: parts.join("\n"), items: items };
    }

    function blockFromPoint(x, y) {
      layer.style.pointerEvents = "none";
      var el = document.elementFromPoint(x, y);
      layer.style.pointerEvents = "";
      if (!el) {
        return { text: "", items: [] };
      }
      var items = itemsFromEl(el);
      var block = el.closest("[data-hash], [data-hashes], .need-tile, tr, .panel, section, svg, .table-wrap, .kind-tree, li");
      if (!block) {
        block = el;
      }
      return { text: normalizeText(block.innerText || block.textContent), items: items };
    }

    function escapeHtml(s) {
      return String(s)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
    }

    function markdownify(s) {
      var html = escapeHtml(s || "");
      html = html.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
      html = html.replace(/\n/g, "<br>");
      return html;
    }

    function setMarkEnabled(on) {
      if (!markBtn) {
        return;
      }
      markBtn.disabled = !on;
    }

    function showPanel(status, body) {
      panel.hidden = false;
      statusEl.textContent = status;
      textEl.innerHTML = markdownify(body);
    }

    function askIntent(selected, items) {
      lastItems = enrichItems(items || []);
      setMarkEnabled(lastItems.length > 0);
      var month = activeMonth();
      if (!selected) {
        showPanel("Nothing readable in that region.", "Drag a box over tiles, charts, or table text, or click one message row.");
        return;
      }
      showPanel("Asking Polypus…", selected.slice(0, 280));
      fetch("/intent", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ month: month, text: selected })
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        }).catch(function () {
          return { ok: false, data: { error: "The intent reply was not JSON." } };
        });
      }).then(function (out) {
        if (out.ok && out.data && out.data.intent) {
          showPanel("Polypus TLDR", out.data.intent);
          return;
        }
        var err = (out.data && out.data.error) || "Intent guess failed.";
        if (!out.ok && (err.indexOf("Failed to fetch") >= 0 || err.indexOf("JSON") >= 0)) {
          showPanel("Serve this report to ask Polypus.", "Run should-i-read mail overview serve and open the printed URL. Highlight still works on this file; the model hop needs that local page.");
          return;
        }
        showPanel("Intent guess failed.", err);
      }).catch(function () {
        showPanel("Serve this report to ask Polypus.", "Run should-i-read mail overview serve and open the printed URL. Highlight still works on this file; the model hop needs that local page.");
      });
    }

    function ignoreHighlightTarget(el) {
      if (!el || !el.closest) {
        return false;
      }
      return !!el.closest("header.site-header, #intent-panel, #delete-cart");
    }

    function onPointerDown(ev) {
      if (drag || !highlightOn() || ev.button !== 0) {
        return;
      }
      if (document.body.classList.contains("cart-resizing")) {
        return;
      }
      if (ignoreHighlightTarget(ev.target)) {
        return;
      }
      ev.preventDefault();
      drag = { x: ev.clientX, y: ev.clientY };
      rectEl.hidden = false;
      rectEl.style.left = drag.x + "px";
      rectEl.style.top = drag.y + "px";
      rectEl.style.width = "0px";
      rectEl.style.height = "0px";
      if (ev.pointerId != null && ev.target && ev.target.setPointerCapture) {
        try {
          ev.target.setPointerCapture(ev.pointerId);
        } catch (err) {}
      }
    }

    function onPointerMove(ev) {
      if (!drag) {
        return;
      }
      var x = Math.min(ev.clientX, drag.x);
      var y = Math.min(ev.clientY, drag.y);
      var w = Math.abs(ev.clientX - drag.x);
      var h = Math.abs(ev.clientY - drag.y);
      rectEl.style.left = x + "px";
      rectEl.style.top = y + "px";
      rectEl.style.width = w + "px";
      rectEl.style.height = h + "px";
    }

    function finishPointer(ev) {
      if (!drag) {
        return;
      }
      var start = drag;
      drag = null;
      var w = Math.abs(ev.clientX - start.x);
      var h = Math.abs(ev.clientY - start.y);
      if (!highlightOn()) {
        rectEl.hidden = true;
        return;
      }
      if (w < 6 && h < 6) {
        rectEl.hidden = true;
        var clicked = blockFromPoint(start.x, start.y);
        askIntent(clicked.text, clicked.items);
        return;
      }
      var box = {
        left: Math.min(start.x, ev.clientX),
        top: Math.min(start.y, ev.clientY),
        right: Math.max(start.x, ev.clientX),
        bottom: Math.max(start.y, ev.clientY)
      };
      var dragged = collectFromRect(box);
      askIntent(dragged.text, dragged.items);
    }

    function cartTldr() {
      return normalizeText(textEl.innerText || textEl.textContent || "");
    }

    function tidyFolderLabel(label) {
      var t = normalizeText(label || "");
      t = t.replace(/^TLDR:\s*/i, "");
      t = t.split(/Mark as deletable/i)[0];
      t = normalizeText(t);
      if (t.length > 72) {
        t = t.slice(0, 71) + "…";
      }
      return t;
    }

    function folderLabel(items, tldr) {
      var t = tidyFolderLabel(tldr);
      if (t) {
        return t;
      }
      if (items.length && items[0].sender) {
        return items[0].sender;
      }
      return items.length + " messages";
    }

    function groupCartItems(items) {
      var order = [];
      var by = {};
      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        var gid = item.group_id || item.object_hash;
        if (!by[gid]) {
          by[gid] = {
            id: gid,
            label: item.group_label || item.sender || "Marked mail",
            items: []
          };
          order.push(gid);
        }
        by[gid].items.push(item);
      }
      var groups = [];
      for (var j = 0; j < order.length; j++) {
        groups.push(by[order[j]]);
      }
      return groups;
    }

    function cartItemRow(item) {
      var li = document.createElement("li");
      var meta = document.createElement("div");
      meta.className = "cart-meta";
      var from = document.createElement("div");
      from.className = "cart-from";
      from.textContent = item.sender || item.object_hash;
      var sub = document.createElement("div");
      sub.className = "cart-subject";
      sub.textContent = item.subject || "";
      meta.appendChild(from);
      meta.appendChild(sub);
      var rm = document.createElement("button");
      rm.type = "button";
      rm.className = "theme-toggle";
      rm.setAttribute("data-hash", item.object_hash);
      rm.textContent = "Remove";
      li.appendChild(meta);
      li.appendChild(rm);
      return li;
    }

    function renderCart(data) {
      var countEl = document.getElementById("cart-count");
      var list = document.getElementById("cart-list");
      var n = (data && data.count) || 0;
      if (countEl) {
        countEl.textContent = String(n);
      }
      if (!list) {
        return;
      }
      list.innerHTML = "";
      var items = (data && data.items) || [];
      if (!items.length) {
        var empty = document.createElement("p");
        empty.className = "cart-empty";
        empty.textContent = "Nothing staged yet.";
        list.appendChild(empty);
        return;
      }
      var groups = groupCartItems(items);
      for (var g = 0; g < groups.length; g++) {
        var group = groups[g];
        var folder = document.createElement("details");
        folder.className = "cart-folder";
        if (g === 0) {
          folder.open = true;
        }
        var summary = document.createElement("summary");
        var chev = document.createElement("span");
        chev.className = "chev";
        chev.setAttribute("aria-hidden", "true");
        var head = document.createElement("div");
        head.className = "cart-folder-head";
        var title = document.createElement("div");
        title.className = "cart-folder-title";
        title.textContent = tidyFolderLabel(group.label) || group.label;
        var count = document.createElement("div");
        count.className = "cart-folder-count";
        count.textContent = group.items.length === 1 ? "1 message" : group.items.length + " messages";
        head.appendChild(title);
        head.appendChild(count);
        var rmAll = document.createElement("button");
        rmAll.type = "button";
        rmAll.className = "theme-toggle";
        rmAll.setAttribute("data-group", group.id);
        rmAll.textContent = "Remove all";
        summary.appendChild(chev);
        summary.appendChild(head);
        summary.appendChild(rmAll);
        var ul = document.createElement("ul");
        for (var i = 0; i < group.items.length; i++) {
          ul.appendChild(cartItemRow(group.items[i]));
        }
        folder.appendChild(summary);
        folder.appendChild(ul);
        list.appendChild(folder);
      }
    }

    var cartJob = { id: "", status: "", poll: null, last: null };
    var cartJobConfirm = "trash-at-source";
    var cartJobStartBtn = document.getElementById("cart-job-start");
    var cartJobTrashBtn = document.getElementById("cart-job-trash");
    var cartJobStatusEl = document.getElementById("cart-job-status");

    function showCartJobStatus(text) {
      if (!cartJobStatusEl) {
        return;
      }
      var t = String(text || "").trim();
      if (!t) {
        cartJobStatusEl.hidden = true;
        cartJobStatusEl.textContent = "";
        return;
      }
      cartJobStatusEl.hidden = false;
      cartJobStatusEl.textContent = t;
    }

    function updateCartJobButtons(job, cartCount) {
      var planned = job && job.counts ? job.counts.planned : 0;
      var applying = job && job.status === "applying";
      if (cartJobStartBtn) {
        cartJobStartBtn.disabled = !cartCount || applying;
      }
      if (cartJobTrashBtn) {
        cartJobTrashBtn.disabled = !job || !job.id || planned < 1 || applying;
      }
    }

    function jobCountsLine(job) {
      if (!job || !job.counts) {
        return "";
      }
      var c = job.counts;
      return "planned " + c.planned + " · unresolved " + c.unresolved + " · refused " + c.refused + " · trashed " + c.trashed + " · failed " + c.failed;
    }

    function stopCartJobPoll() {
      if (cartJob.poll) {
        clearInterval(cartJob.poll);
        cartJob.poll = null;
      }
    }

    function startCartJobPoll(id) {
      stopCartJobPoll();
      cartJob.poll = setInterval(function () {
        fetch("/cart/jobs/" + encodeURIComponent(id)).then(function (res) {
          return res.json();
        }).then(function (body) {
          if (!body || !body.job) {
            return;
          }
          cartJob.id = body.job.id;
          cartJob.status = body.job.status;
          cartJob.last = body.job;
          showCartJobStatus("Job " + body.job.status + ". " + jobCountsLine(body.job));
          updateCartJobButtons(body.job, document.getElementById("cart-count") ? parseInt(document.getElementById("cart-count").textContent, 10) || 0 : 0);
          if (body.job.status !== "applying") {
            stopCartJobPoll();
            refreshCart();
          }
        }).catch(function () {});
      }, 1000);
    }

    function startDeletionJob() {
      fetch("/cart/jobs", { method: "POST" }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, status: res.status, data: data };
        });
      }).then(function (out) {
        if (!out.ok || !out.data || !out.data.job) {
          showCartJobStatus((out.data && out.data.error) || "Could not start deletion job.");
          return;
        }
        cartJob.id = out.data.job.id;
        cartJob.status = out.data.job.status;
        cartJob.last = out.data.job;
        showCartJobStatus("Job " + out.data.job.status + ". " + jobCountsLine(out.data.job));
        var count = document.getElementById("cart-count") ? parseInt(document.getElementById("cart-count").textContent, 10) || 0 : 0;
        updateCartJobButtons(out.data.job, count);
      }).catch(function () {
        showCartJobStatus("Deletion jobs need overview serve with pimdir.");
      });
    }

    function applyTrashAtSource() {
      if (!cartJob.id) {
        return;
      }
      fetch("/cart/jobs/" + encodeURIComponent(cartJob.id) + "/apply", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: cartJobConfirm })
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, status: res.status, data: data };
        });
      }).then(function (out) {
        if (!out.ok && out.status !== 202 && out.status !== 200) {
          showCartJobStatus((out.data && out.data.error) || "Apply failed.");
          return;
        }
        cartJob.status = "applying";
        showCartJobStatus("Moving to Trash at source…");
        startCartJobPoll(cartJob.id);
        updateCartJobButtons({ id: cartJob.id, status: "applying", counts: { planned: 1 } }, 0);
      }).catch(function () {
        showCartJobStatus("Could not apply deletion job.");
      });
    }

    if (cartJobStartBtn) {
      cartJobStartBtn.addEventListener("click", startDeletionJob);
    }
    if (cartJobTrashBtn) {
      cartJobTrashBtn.addEventListener("click", applyTrashAtSource);
    }

    function refreshCart() {
      return fetch("/cart").then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        }).catch(function () {
          return { ok: false, data: {} };
        });
      }).then(function (out) {
        if (out.ok) {
          renderCart(out.data);
          updateCartJobButtons(cartJob.last, out.data.count || 0);
        }
      }).catch(function () {});
    }

    var cartPanel = document.getElementById("delete-cart");
    var cartToggle = document.getElementById("cart-toggle");
    var cartClose = document.getElementById("cart-close");
    var cartWidthKey = "sir-mail-overview-cart-width";
    var cartWidthMin = 240;
    var cartWidthMaxFrac = 0.6;

    function clampCartWidth(px) {
      var max = Math.floor(window.innerWidth * cartWidthMaxFrac);
      var min = Math.min(cartWidthMin, max);
      if (px < min) {
        return min;
      }
      if (px > max) {
        return max;
      }
      return px;
    }

    function applyCartWidth(px) {
      var w = clampCartWidth(px);
      document.documentElement.style.setProperty("--cart-width", w + "px");
      return w;
    }

    function restoreCartWidth() {
      var raw = localStorage.getItem(cartWidthKey);
      var n = raw ? parseInt(raw, 10) : 0;
      if (n > 0) {
        applyCartWidth(n);
      }
    }

    function wireCartResize() {
      var handle = document.getElementById("cart-resize");
      if (!handle || !cartPanel) {
        return;
      }
      var dragW = null;
      handle.addEventListener("pointerdown", function (ev) {
        if (ev.button !== 0) {
          return;
        }
        ev.preventDefault();
        ev.stopPropagation();
        dragW = { startX: ev.clientX, startW: cartPanel.getBoundingClientRect().width };
        document.body.classList.add("cart-resizing");
        if (handle.setPointerCapture && ev.pointerId != null) {
          try {
            handle.setPointerCapture(ev.pointerId);
          } catch (err) {}
        }
      });
      window.addEventListener("pointermove", function (ev) {
        if (!dragW) {
          return;
        }
        applyCartWidth(dragW.startW + (dragW.startX - ev.clientX));
      });
      function endDrag() {
        if (!dragW) {
          return;
        }
        dragW = null;
        document.body.classList.remove("cart-resizing");
        localStorage.setItem(cartWidthKey, String(Math.round(cartPanel.getBoundingClientRect().width)));
      }
      window.addEventListener("pointerup", endDrag);
      window.addEventListener("pointercancel", endDrag);
    }

    restoreCartWidth();
    wireCartResize();

    function setCartOpen(on) {
      if (!cartPanel) {
        return;
      }
      cartPanel.hidden = !on;
      document.body.classList.toggle("cart-open", on);
      if (cartToggle) {
        cartToggle.setAttribute("aria-expanded", on ? "true" : "false");
      }
      if (on) {
        refreshCart();
      }
    }

    if (cartToggle && cartPanel) {
      cartToggle.addEventListener("click", function () {
        setCartOpen(cartPanel.hidden);
      });
    }
    if (cartClose && cartPanel) {
      cartClose.addEventListener("click", function () {
        setCartOpen(false);
      });
    }
    var cartList = document.getElementById("cart-list");
    if (cartList) {
      cartList.addEventListener("click", function (ev) {
        var btn = ev.target.closest && ev.target.closest("button[data-hash], button[data-group]");
        if (!btn) {
          return;
        }
        ev.preventDefault();
        ev.stopPropagation();
        var group = btn.getAttribute("data-group");
        var hash = btn.getAttribute("data-hash");
        var url = group ? "/cart?group=" + encodeURIComponent(group) : "/cart?hash=" + encodeURIComponent(hash);
        fetch(url, { method: "DELETE" }).then(function (res) {
          return res.json();
        }).then(function (data) {
          renderCart(data);
        }).catch(function () {});
      });
    }
    if (markBtn) {
      markBtn.addEventListener("click", function () {
        if (!lastItems.length) {
          return;
        }
        var tldr = cartTldr();
        fetch("/cart", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ items: lastItems, tldr: tldr, group_label: folderLabel(lastItems, tldr) })
        }).then(function (res) {
          return res.json().then(function (data) {
            return { ok: res.ok, data: data };
          });
        }).then(function (out) {
          if (!out.ok) {
            showPanel("Could not add to the cart.", (out.data && out.data.error) || "Cart add failed.");
            return;
          }
          renderCart(out.data);
          setCartOpen(true);
          setMarkEnabled(false);
        }).catch(function () {
          showPanel("Could not add to the cart.", "Serve this report to use the deletion cart.");
        });
      });
    }
    refreshCart();

    window.addEventListener("pointerdown", onPointerDown, true);
    window.addEventListener("pointermove", onPointerMove, true);
    window.addEventListener("pointerup", finishPointer, true);
    window.addEventListener("pointercancel", finishPointer, true);
    window.addEventListener("mousedown", onPointerDown, true);
    window.addEventListener("mousemove", onPointerMove, true);
    window.addEventListener("mouseup", finishPointer, true);
  }
})();
