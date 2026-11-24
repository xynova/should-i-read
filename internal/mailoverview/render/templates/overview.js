(function () {
  var tabs = document.querySelectorAll(".month-tabs button[data-month]");
  var monthMore = document.querySelector(".month-more");
  var panels = document.querySelectorAll(".tab-panel");
  var storageKey = "sir-mail-overview-theme";
  var navKey = "sir-mail-overview-nav";
  var pageButtons = document.querySelectorAll(".page-tabs button");
  var monthTabs = document.querySelector(".month-tabs");
  var restoringNav = false;
  var navSaveTimer = 0;
  var onBoardMonth = function () {};

  function readNav() {
    try {
      var raw = localStorage.getItem(navKey);
      if (!raw) {
        return {};
      }
      var o = JSON.parse(raw);
      if (!o || typeof o !== "object") {
        return {};
      }
      return o;
    } catch (err) {
      return {};
    }
  }

  function writeNav(patch) {
    if (restoringNav) {
      return;
    }
    var cur = readNav();
    for (var k in patch) {
      if (Object.prototype.hasOwnProperty.call(patch, k)) {
        cur[k] = patch[k];
      }
    }
    try {
      localStorage.setItem(navKey, JSON.stringify(cur));
    } catch (err) {}
  }

  function currentMonthKey() {
    var tab = document.querySelector(".month-tabs button[data-month][aria-selected='true']");
    return tab ? tab.getAttribute("data-month") || "" : "";
  }

  function saveNavNow() {
    navSaveTimer = 0;
    writeNav({
      page: document.body.getAttribute("data-page") || "overview",
      month: currentMonthKey(),
      scrollY: window.scrollY || window.pageYOffset || 0,
      monthScroll: monthTabs ? monthTabs.scrollLeft : 0
    });
  }

  function scheduleNavSave() {
    if (restoringNav) {
      return;
    }
    if (navSaveTimer) {
      clearTimeout(navSaveTimer);
    }
    navSaveTimer = setTimeout(saveNavNow, 120);
  }

  function monthButton(key) {
    if (!key) {
      return null;
    }
    for (var i = 0; i < tabs.length; i++) {
      if (tabs[i].getAttribute("data-month") === key) {
        return tabs[i];
      }
    }
    return null;
  }

  function activatePage(page) {
    if (page !== "report" && page !== "actioned") {
      page = "overview";
    }
    document.body.setAttribute("data-page", page);
    for (var i = 0; i < pageButtons.length; i++) {
      var b = pageButtons[i];
      b.setAttribute("aria-selected", b.getAttribute("data-page") === page ? "true" : "false");
    }
    paintMonthTabCounts();
    onBoardMonth(currentMonthKey());
    scheduleNavSave();
  }

  function paintMonthTabCounts() {
    var page = document.body.getAttribute("data-page");
    var els = document.querySelectorAll(".month-tabs .tab-count");
    for (var i = 0; i < els.length; i++) {
      var el = els[i];
      var full = el.getAttribute("data-full-count");
      var live = el.getAttribute("data-live-count");
      if (page === "report" && full) {
        el.textContent = full;
        continue;
      }
      if (page === "actioned") {
        el.textContent = el.getAttribute("data-actioned-count") || "0";
        continue;
      }
      if (live) {
        el.textContent = live;
      }
    }
  }

  for (var pb = 0; pb < pageButtons.length; pb++) {
    pageButtons[pb].addEventListener("click", function (ev) {
      activatePage(ev.currentTarget.getAttribute("data-page"));
    });
  }

  function closeMonthMore() {
    if (monthMore) {
      monthMore.open = false;
    }
  }

  function syncMonthMore() {
    if (!monthMore) {
      return;
    }
    var selected = monthMore.querySelector("button[data-month][aria-selected='true']");
    var label = monthMore.querySelector(".month-more-label");
    var toggle = monthMore.querySelector(".month-more-toggle");
    var copy = selected ? (selected.textContent || "").replace(/\s+/g, " ").trim() : "…";
    if (selected) {
      monthMore.classList.add("is-selected");
    } else {
      monthMore.classList.remove("is-selected");
    }
    if (label) {
      label.textContent = copy;
    }
    if (toggle) {
      toggle.setAttribute("aria-label", selected ? copy : "Previous months");
    }
    closeMonthMore();
  }

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
    syncMonthMore();
    onBoardMonth(key);
    scheduleNavSave();
  }

  for (var k = 0; k < tabs.length; k++) {
    tabs[k].addEventListener("click", function (ev) {
      activateTab(ev.currentTarget.getAttribute("data-month"));
    });
  }

  document.addEventListener("click", function (ev) {
    if (!monthMore || !monthMore.open) {
      return;
    }
    if (monthMore.contains(ev.target)) {
      return;
    }
    closeMonthMore();
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
      closeMonthMore();
    }
  });

  function parseStamp(iso) {
    var t = String(iso || "").trim();
    if (/^\d{4}-\d{2}-\d{2}$/.test(t)) {
      var y = parseInt(t.slice(0, 4), 10);
      var m = parseInt(t.slice(5, 7), 10);
      var d = parseInt(t.slice(8, 10), 10);
      if (!y || m < 1 || d < 1) {
        return null;
      }
      return new Date(y, m - 1, d);
    }
    var dt = new Date(t);
    if (isNaN(dt.getTime())) {
      return null;
    }
    return dt;
  }

  function formatLocalBoard(iso) {
    var d = parseStamp(iso);
    if (!d) {
      return "";
    }
    var wd = d.toLocaleDateString(undefined, { weekday: "short" }).replace(/[.,]$/g, "");
    var tm = d.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
    return wd + " " + String(d.getDate()) + " " + tm;
  }

  function formatLocalBoardTitle(iso) {
    var d = parseStamp(iso);
    if (!d) {
      return "";
    }
    return d.toLocaleString(undefined, {
      weekday: "long",
      day: "numeric",
      month: "short",
      year: "numeric",
      hour: "numeric",
      minute: "2-digit"
    });
  }

  function setButtonTip(el, aria, tip) {
    if (!el) {
      return;
    }
    el.setAttribute("aria-label", aria);
    el.setAttribute("data-tooltip", tip || aria);
  }

  function formatLocalDate(iso) {
    var d = parseStamp(iso);
    if (!d) {
      return "";
    }
    var opts = { day: "numeric", month: "short" };
    if (d.getFullYear() !== new Date().getFullYear()) {
      opts.year = "numeric";
    }
    return d.toLocaleDateString(undefined, opts);
  }

  function localizeTimes() {
    var nodes = document.querySelectorAll("time[datetime]");
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      var iso = el.getAttribute("datetime") || "";
      var next = el.getAttribute("data-local") === "board" ? formatLocalBoard(iso) : formatLocalDate(iso);
      if (next) {
        el.textContent = next;
      }
      if (el.getAttribute("data-local") === "board") {
        var title = formatLocalBoardTitle(iso);
        var btn = document.getElementById("mail-sync");
        if (btn && next && btn.getAttribute("aria-busy") !== "true") {
          var syncName = "Sync mail, last built " + next;
          setButtonTip(btn, syncName, title ? "Sync mail. Last built " + title : syncName);
        }
      }
    }
  }

  function restoreNav() {
    var stored = readNav();
    restoringNav = true;
    var month = stored.month;
    if (!monthButton(month)) {
      var initial = document.querySelector(".month-tabs button[data-month][aria-selected='true']");
      month = initial ? initial.getAttribute("data-month") : "";
    }
    if (month) {
      activateTab(month);
    }
    if (stored.page) {
      activatePage(stored.page);
    }
    function applyScroll() {
      if (monthTabs && typeof stored.monthScroll === "number") {
        monthTabs.scrollLeft = stored.monthScroll;
      }
      if (typeof stored.scrollY === "number") {
        window.scrollTo(0, stored.scrollY);
      }
      restoringNav = false;
    }
    if (window.requestAnimationFrame) {
      window.requestAnimationFrame(function () {
        window.requestAnimationFrame(applyScroll);
      });
    } else {
      applyScroll();
    }
  }

  restoreNav();
  localizeTimes();
  window.addEventListener("scroll", scheduleNavSave, { passive: true });
  if (monthTabs) {
    monthTabs.addEventListener("scroll", scheduleNavSave, { passive: true });
  }
  window.addEventListener("beforeunload", saveNavNow);

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

  function toggleDeleteEntries(btn) {
    var open = btn.getAttribute("aria-expanded") === "true";
    var next = !open;
    btn.setAttribute("aria-expanded", next ? "true" : "false");
    btn.textContent = next ? "−" : "+";
    var id = btn.getAttribute("aria-controls") || "";
    if (!id) {
      return;
    }
    var rows = document.querySelectorAll('tr.delete-entry[data-parent="' + id + '"]');
    for (var i = 0; i < rows.length; i++) {
      rows[i].hidden = !next;
    }
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
        if (!show) {
          var exp = extra[i].querySelector(".delete-expand");
          if (exp && exp.getAttribute("aria-expanded") === "true") {
            toggleDeleteEntries(exp);
          }
        }
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
    var panel = document.getElementById("intent-panel");
    var backdrop = document.getElementById("intent-backdrop");
    var titleEl = document.getElementById("intent-title");
    var statusEl = document.getElementById("intent-status");
    var textEl = document.getElementById("intent-text");
    var closeBtn = document.getElementById("intent-close");
    var markBtn = document.getElementById("intent-mark");
    var ackBtn = document.getElementById("intent-ack");
    var prevBtn = document.getElementById("intent-prev");
    var nextBtn = document.getElementById("intent-next");
    var outliersWrap = document.getElementById("intent-outliers-wrap");
    var outliersList = document.getElementById("intent-outliers");
    var worthBox = document.getElementById("intent-worth");
    if (!panel || !statusEl || !textEl) {
      return;
    }

    var lastItems = [];
    var lastActioned = {};
    var lastActionedRows = null;
    var lastStagedSet = {};
    var lastAckedSet = {};
    var previewHash = "";
    var previewQueue = [];
    var previewIndex = -1;
    var hashIndex = null;
    var cartItemCap = 200;
    var worthBusy = false;
    var panelSeq = 0;
    var intentMemo = {};
    var lastIntentKey = "";

    function intentMemoKey(month, selected) {
      return (month || "") + "\n" + (selected || "");
    }

    function forgetIntentMemo() {
      if (lastIntentKey) {
        delete intentMemo[lastIntentKey];
        lastIntentKey = "";
      }
    }

    function setIntentOpen(open) {
      panel.hidden = !open;
      if (!open) {
        panel.classList.remove("intent-html");
      }
      if (backdrop) {
        backdrop.hidden = !open;
      }
    }

    function dismissPanel() {
      panelSeq += 1;
      setIntentOpen(false);
      clearOutlierBullets();
      clearPreviewQueue();
    }

    function beginPanel() {
      panelSeq += 1;
      setIntentOpen(true);
      return panelSeq;
    }

    function panelIsCurrent(seq) {
      return seq === panelSeq;
    }

    if (closeBtn) {
      closeBtn.addEventListener("click", function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        dismissPanel();
      });
    }
    if (backdrop) {
      backdrop.addEventListener("click", function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        dismissPanel();
      });
    }

    function clickOpensIntent(el) {
      if (!el || !el.closest) {
        return false;
      }
      if (el.closest("table.fluff-senders tbody tr[data-hashes]")) {
        return true;
      }
      if (el.closest("table.action-table tbody tr")) {
        return true;
      }
      if (el.closest("tr.delete-entry[data-hashes], tr.delete-parent[data-hashes]")) {
        return true;
      }
      if (el.closest(".ktree .krow")) {
        return true;
      }
      return false;
    }

    function onOutsideIntent(ev) {
      if (panel.hidden || !ev.target) {
        return;
      }
      if (panel.contains(ev.target)) {
        return;
      }
      if (backdrop && (ev.target === backdrop || (backdrop.contains && backdrop.contains(ev.target)))) {
        return;
      }
      if (ev.target.closest && ev.target.closest("#stage-panel")) {
        return;
      }
      if (clickOpensIntent(ev.target)) {
        return;
      }
      dismissPanel();
    }

    function activeMonth() {
      var tab = document.querySelector(".month-tabs button[data-month][aria-selected='true']");
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

    function isSvgNode(el) {
      if (!el) {
        return false;
      }
      if (el.ownerSVGElement) {
        return true;
      }
      var ns = el.namespaceURI || "";
      return ns.indexOf("svg") >= 0 || String(el.tagName || "").toLowerCase() === "svg";
    }

    // Heatmap cells paint the count and also put it in <title>. textContent
    // concatenates both, so 7 becomes 77. Prefer the tooltip label.
    function svgTitleLabel(el) {
      if (!el || !el.getElementsByTagName) {
        return "";
      }
      var titles = el.getElementsByTagName("title");
      if (!titles || !titles.length) {
        return "";
      }
      var parts = [];
      var seen = {};
      for (var i = 0; i < titles.length; i++) {
        var t = normalizeText(titles[i].textContent);
        if (!t || seen[t]) {
          continue;
        }
        seen[t] = true;
        parts.push(t);
      }
      return parts.join("\n");
    }

    function readableText(el) {
      if (!el) {
        return "";
      }
      if (isSvgNode(el)) {
        var titled = svgTitleLabel(el);
        if (titled) {
          return titled;
        }
      }
      return normalizeText(el.innerText || el.textContent);
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
                hashIndex[m.object_hash] = {
                  sender: m.sender || m.sender_name || "",
                  subject: m.subject || "",
                  date: m.date || "",
                  about: m.about || "",
                  needs_reply: m.needs_reply || 0,
                  decision: m.decision || 0,
                  urgency: m.urgency || 0,
                  outlier_first: !!m.outlier_first
                };
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
        if (!h || seen[h] || lastActioned[h]) {
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

    function kindRowOf(el) {
      if (!el || !el.closest) {
        return null;
      }
      var row = el.closest(".krow");
      if (!row || !row.closest(".ktree")) {
        return null;
      }
      return row;
    }

    // Parent kind rows put every descendant hash on the wrapping <li>.
    // Only a leaf krow carries its own data-hashes; never walk up to Notification.
    function itemsFromKindRow(krow) {
      if (!krow || !krow.closest || !krow.getAttribute) {
        return [];
      }
      if (!krow.closest(".kleaf")) {
        return [];
      }
      if (!krow.getAttribute("data-hashes")) {
        return [];
      }
      return itemsFromEl(krow);
    }

    function collectFromRect(box) {
      var empty = { text: "", items: [] };
      var root = document.querySelector(".tab-panel.active") || document.querySelector("main");
      if (!root) {
        return empty;
      }
      var sel = "h2, h3, p, .need-tile, .need-hint, td, th, .ktree .krow, figcaption, caption, [data-hash], [data-hashes]";
      var nodes = root.querySelectorAll(sel);
      var seen = {};
      var parts = [];
      var items = [];
      var seenHash = {};
      for (var i = 0; i < nodes.length; i++) {
        var el = nodes[i];
        if (el.closest && el.closest(".ktree") && !(el.classList && el.classList.contains("krow"))) {
          continue;
        }
        if (!visible(el) || !intersects(box, el.getBoundingClientRect())) {
          continue;
        }
        var t = readableText(el);
        if (t && !seen[t]) {
          seen[t] = true;
          parts.push(t);
        }
        var got = el.classList && el.classList.contains("krow") ? itemsFromKindRow(el) : itemsFromEl(el);
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
      if (el.closest && el.closest(".kind-tree")) {
        var krow = kindRowOf(el);
        if (!krow) {
          return { text: "", items: [] };
        }
        return { text: readableText(krow), items: itemsFromKindRow(krow) };
      }
      var items = itemsFromEl(el);
      var block = el.closest("[data-hash], [data-hashes], .need-tile, tr, .panel, section, svg, .table-wrap, li");
      if (!block) {
        block = el;
      }
      return { text: readableText(block), items: items };
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

    function setWorthEnabled(on) {
      if (!worthBox) {
        return;
      }
      worthBox.disabled = !on;
      if (!on) {
        worthBox.checked = false;
      }
    }

    function setMarkEnabled(on) {
      if (!markBtn) {
        return;
      }
      markBtn.disabled = !on;
    }

    function setAckEnabled(on) {
      if (!ackBtn) {
        return;
      }
      ackBtn.disabled = !on;
    }

    function updatePreviewNav() {
      var prevOn = previewIndex > 0;
      var nextOn = previewIndex >= 0 && previewIndex < previewQueue.length - 1;
      if (prevBtn) {
        prevBtn.disabled = !prevOn;
      }
      if (nextBtn) {
        nextBtn.disabled = !nextOn;
      }
    }

    function previewItemUsable(h, keepHash) {
      if (!h) {
        return false;
      }
      if (keepHash && h === keepHash) {
        return true;
      }
      if (lastActioned[h] || lastStagedSet[h] || lastAckedSet[h]) {
        return false;
      }
      return true;
    }

    function collectRowItems(rows, keepHash) {
      var out = [];
      var seen = {};
      for (var r = 0; r < rows.length; r++) {
        var got = enrichItems(itemsFromEl(rows[r]));
        for (var g = 0; g < got.length; g++) {
          var it = got[g];
          var h = it && it.object_hash;
          if (!h || seen[h] || !previewItemUsable(h, keepHash)) {
            continue;
          }
          seen[h] = true;
          out.push(it);
        }
      }
      return out;
    }

    function collectPreviewQueue(tr, items, keepHash) {
      var rows = [];
      if (tr && tr.classList && tr.classList.contains("delete-entry")) {
        var parent = tr.getAttribute("data-parent") || "";
        var all = document.querySelectorAll("tr.delete-entry[data-hashes]");
        for (var i = 0; i < all.length; i++) {
          if ((all[i].getAttribute("data-parent") || "") === parent) {
            rows.push(all[i]);
          }
        }
      } else if (tr && tr.closest) {
        var table = tr.closest("table.action-table");
        if (table) {
          var arows = table.querySelectorAll("tbody tr[data-hashes]");
          for (var a = 0; a < arows.length; a++) {
            rows.push(arows[a]);
          }
        }
      }
      var out = [];
      if (rows.length) {
        out = collectRowItems(rows, keepHash);
      }
      if (!out.length) {
        var seed = enrichItems(items || []);
        var seen = {};
        for (var s = 0; s < seed.length; s++) {
          var it = seed[s];
          var h = it && it.object_hash;
          if (!h || seen[h] || !previewItemUsable(h, keepHash)) {
            continue;
          }
          seen[h] = true;
          out.push(it);
        }
      }
      return out;
    }

    function collectOutlierQueue(keepHash) {
      if (!outliersList) {
        return [];
      }
      var cards = outliersList.querySelectorAll(".outlier-card[data-hash]");
      var rows = [];
      for (var i = 0; i < cards.length; i++) {
        rows.push(cards[i]);
      }
      return collectRowItems(rows, keepHash);
    }

    function setPreviewQueue(items, hash) {
      previewQueue = items || [];
      previewIndex = -1;
      if (hash) {
        for (var i = 0; i < previewQueue.length; i++) {
          if (previewQueue[i].object_hash === hash) {
            previewIndex = i;
            break;
          }
        }
      }
      updatePreviewNav();
    }

    function clearPreviewQueue() {
      previewQueue = [];
      previewIndex = -1;
      updatePreviewNav();
    }

    function openPreviewIndex(i) {
      if (i < 0 || i >= previewQueue.length) {
        return;
      }
      var it = previewQueue[i];
      previewIndex = i;
      previewHash = it.object_hash || "";
      lastItems = [it];
      lastIntentKey = "";
      setMarkEnabled(!!previewHash);
      setAckEnabled(!!previewHash);
      setWorthEnabled(!!previewHash);
      updatePreviewNav();
      var seq = beginPanel();
      loadMessagePreview(previewHash, {
        sender: it.sender || "",
        subject: it.subject || "",
        date: it.date || ""
      }, seq, false);
    }

    function stepPreview(delta) {
      openPreviewIndex(previewIndex + delta);
    }

    function advancePreviewAfterStage() {
      if (previewIndex < 0 || !previewHash || previewQueue.length < 2) {
        clearPreviewQueue();
        return false;
      }
      var drop = previewHash;
      var nextItems = [];
      for (var i = 0; i < previewQueue.length; i++) {
        if (previewQueue[i].object_hash !== drop) {
          nextItems.push(previewQueue[i]);
        }
      }
      if (!nextItems.length) {
        clearPreviewQueue();
        return false;
      }
      var nextIdx = previewIndex;
      if (nextIdx >= nextItems.length) {
        nextIdx = nextItems.length - 1;
      }
      previewQueue = nextItems;
      openPreviewIndex(nextIdx);
      return true;
    }

    function detailCandidate(meta) {
      if (!meta) {
        return false;
      }
      if (meta.outlier_first) {
        return true;
      }
      if ((meta.needs_reply || 0) >= 0.45) {
        return true;
      }
      if ((meta.decision || 0) >= 0.45) {
        return true;
      }
      if ((meta.urgency || 0) >= 0.45) {
        return true;
      }
      var subj = String(meta.subject || "").toLowerCase();
      if (/fail|failure|security|urgent|action required|release|mention|breaking/.test(subj)) {
        return true;
      }
      return false;
    }

    function outlierScore(meta) {
      if (!meta) {
        return 0;
      }
      var s = Math.max(meta.needs_reply || 0, meta.decision || 0, meta.urgency || 0);
      if (meta.outlier_first) {
        s += 1.5;
      }
      if (detailCandidate(meta)) {
        s += 0.15;
      }
      return s;
    }

    function shortIntentDate(raw) {
      var t = String(raw || "").trim();
      if (t.length < 10) {
        return "";
      }
      var y = parseInt(t.slice(0, 4), 10);
      var m = parseInt(t.slice(5, 7), 10);
      var d = parseInt(t.slice(8, 10), 10);
      if (!y || m < 1 || m > 12 || d < 1) {
        return t.slice(0, 10);
      }
      var names = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
      return names[m - 1] + " " + d;
    }

    function dateStamp(raw) {
      var t = String(raw || "").trim();
      if (t.length < 10) {
        return 0;
      }
      var n = Date.parse(t);
      return isNaN(n) ? 0 : n;
    }

    function parseOutlierSubject(raw) {
      var subj = String(raw || "").replace(/\s+/g, " ").trim();
      var repo = "";
      var rest = subj;
      var m = subj.match(/^\[([^\]]+)\]\s*(.*)$/);
      if (m) {
        repo = m[1];
        rest = (m[2] || "").trim();
      }
      rest = rest.replace(/\s*\([0-9a-f]{7,40}\)\s*$/i, "");
      rest = rest.replace(/\s+-\s+(main|master|develop|trunk)\s*$/i, "");
      rest = rest.replace(/^Run failed:\s*/i, "Failed: ");
      rest = rest.replace(/^PR run failed:\s*/i, "PR failed: ");
      if (rest.length > 72) {
        rest = rest.slice(0, 71) + "…";
      }
      return { repo: repo, title: rest || subj, key: (repo + "\n" + rest).toLowerCase() };
    }

    function dateRangeLabel(rawDates) {
      var dated = [];
      for (var i = 0; i < rawDates.length; i++) {
        var raw = rawDates[i];
        if (!raw) {
          continue;
        }
        dated.push({ raw: raw, stamp: dateStamp(raw) });
      }
      if (!dated.length) {
        return "";
      }
      dated.sort(function (a, b) {
        return a.stamp - b.stamp;
      });
      var first = shortIntentDate(dated[0].raw);
      var last = shortIntentDate(dated[dated.length - 1].raw);
      if (!first) {
        return "";
      }
      if (!last || first === last) {
        return first;
      }
      return first + "–" + last;
    }

    function clearOutlierBullets() {
      if (outliersList) {
        outliersList.innerHTML = "";
      }
      if (outliersWrap) {
        outliersWrap.hidden = true;
      }
    }

    function paintOutlierBullets(items) {
      if (!outliersWrap || !outliersList) {
        return;
      }
      if (!items || items.length < 2 || previewHash) {
        clearOutlierBullets();
        return;
      }
      var groups = {};
      var order = [];
      for (var i = 0; i < items.length; i++) {
        var it = items[i];
        var h = it && it.object_hash;
        var meta = h ? messageByHash(h) : null;
        if (!meta) {
          meta = {
            subject: (it && it.subject) || "",
            date: (it && it.date) || "",
            needs_reply: 0,
            decision: 0,
            urgency: 0,
            outlier_first: false
          };
        }
        if (!detailCandidate(meta)) {
          continue;
        }
        var parsed = parseOutlierSubject(meta.subject || (it && it.subject) || "");
        var g = groups[parsed.key];
        if (!g) {
          g = {
            repo: parsed.repo,
            title: parsed.title,
            count: 0,
            score: 0,
            dates: [],
            hashes: [],
            latestStamp: -1,
            latestHash: "",
            latestDate: "",
            latestSubject: "",
            latestSender: ""
          };
          groups[parsed.key] = g;
          order.push(g);
        }
        g.count += 1;
        g.score = Math.max(g.score, outlierScore(meta));
        var whenRaw = meta.date || (it && it.date) || "";
        g.dates.push(whenRaw);
        if (h) {
          g.hashes.push(h);
        }
        var stamp = dateStamp(whenRaw);
        if (h && stamp >= g.latestStamp) {
          g.latestStamp = stamp;
          g.latestHash = h;
          g.latestDate = whenRaw;
          g.latestSubject = meta.subject || (it && it.subject) || parsed.title;
          g.latestSender = meta.sender || (it && it.sender) || "";
        }
      }
      order.sort(function (a, b) {
        if (b.score !== a.score) {
          return b.score - a.score;
        }
        return b.count - a.count;
      });
      if (order.length > 5) {
        order = order.slice(0, 5);
      }
      if (order.length === 0) {
        clearOutlierBullets();
        return;
      }
      var html = "";
      for (var j = 0; j < order.length; j++) {
        var row = order[j];
        html += '<li class="outlier-card"';
        if (row.latestHash) {
          html += ' data-hash="' + escapeHtml(row.latestHash) + '"';
        }
        if (row.hashes.length) {
          html += ' data-hashes="' + escapeHtml(row.hashes.join(",")) + '"';
        }
        if (row.latestSender) {
          html += ' data-sender="' + escapeHtml(row.latestSender) + '"';
        }
        if (row.latestSubject) {
          html += ' data-subject="' + escapeHtml(row.latestSubject) + '"';
        }
        if (row.latestDate) {
          html += ' data-date="' + escapeHtml(row.latestDate) + '"';
        }
        html += ' role="button" tabindex="0"><div class="outlier-title">';
        if (row.repo) {
          html += '<span class="outlier-repo">' + escapeHtml(row.repo) + "</span>";
        }
        if (row.title) {
          html += '<span class="outlier-kind">' + escapeHtml(row.title) + "</span>";
        }
        html += '</div><div class="outlier-meta">';
        var when = dateRangeLabel(row.dates);
        if (when) {
          html += escapeHtml(when);
        }
        if (row.count > 1) {
          html += (when ? " · " : "") + "×" + row.count + " · latest";
        } else {
          html += (when ? " · " : "") + "Open preview";
        }
        html += "</div></li>";
      }
      outliersList.innerHTML = html;
      outliersWrap.hidden = false;
    }

    function setPanelTitle(title) {
      if (titleEl) {
        titleEl.textContent = title || "Message";
      }
    }

    function showPanel(status, body, seq) {
      if (seq !== undefined && !panelIsCurrent(seq)) {
        return;
      }
      setIntentOpen(true);
      panel.removeAttribute("aria-busy");
      statusEl.textContent = status;
      textEl.innerHTML = markdownify(body);
      if (status === "Polypus TLDR") {
        paintOutlierBullets(lastItems);
      } else {
        clearOutlierBullets();
      }
    }

    function showWait(message, seq, keepOutliers) {
      if (seq !== undefined && !panelIsCurrent(seq)) {
        return;
      }
      setIntentOpen(true);
      panel.setAttribute("aria-busy", "true");
      statusEl.textContent = "";
      textEl.innerHTML = '<div class="intent-wait" role="status"><span class="intent-spinner" aria-hidden="true"></span><p>' + escapeHtml(message) + "</p></div>";
      if (!keepOutliers) {
        clearOutlierBullets();
      }
    }

    function sizeMessageHTMLFrame(frame) {
      try {
        var doc = frame.contentDocument;
        if (!doc || !doc.documentElement) {
          return;
        }
        var h = Math.max(
          doc.documentElement.scrollHeight || 0,
          doc.documentElement.offsetHeight || 0,
          doc.body ? doc.body.scrollHeight : 0,
          doc.body ? doc.body.offsetHeight : 0
        );
        if (h < 80) {
          h = 80;
        }
        if (h > 8000) {
          h = 8000;
        }
        frame.style.height = h + "px";
      } catch (err) {
        /* sandboxed document may be unread */
      }
    }

    function watchMessageHTMLFrame(frame) {
      sizeMessageHTMLFrame(frame);
      try {
        var doc = frame.contentDocument;
        if (!doc) {
          return;
        }
        var imgs = doc.images || [];
        for (var i = 0; i < imgs.length; i++) {
          imgs[i].addEventListener("load", function () {
            sizeMessageHTMLFrame(frame);
          });
          imgs[i].addEventListener("error", function () {
            sizeMessageHTMLFrame(frame);
          });
        }
      } catch (err) {
        /* ignore */
      }
      window.setTimeout(function () {
        sizeMessageHTMLFrame(frame);
      }, 250);
      window.setTimeout(function () {
        sizeMessageHTMLFrame(frame);
      }, 1000);
    }

    function showMessagePreview(data, fallback, seq, keepOutliers) {
      if (seq !== undefined && !panelIsCurrent(seq)) {
        return;
      }
      setIntentOpen(true);
      panel.removeAttribute("aria-busy");
      setPanelTitle("Message");
      var subject = (data && data.subject) || fallback.subject || "";
      var from = (data && data.from) || fallback.sender || "";
      var date = (data && data.date) || fallback.date || "";
      var body = (data && data.body) || "";
      var mimeHTML = (data && data.html) || "";
      panel.classList.toggle("intent-html", !!mimeHTML);
      statusEl.textContent = "";
      var html = '<div class="msg-preview">';
      if (subject) {
        html += '<p class="msg-subject">' + escapeHtml(subject) + "</p>";
      }
      if (from || date) {
        html += '<p class="msg-meta">' + escapeHtml(from) + (date ? " · " + escapeHtml(date) : "") + "</p>";
      }
      html += "</div>";
      textEl.innerHTML = html;
      var wrap = textEl.querySelector(".msg-preview") || textEl;
      if (mimeHTML) {
        var frame = document.createElement("iframe");
        frame.className = "msg-html";
        frame.setAttribute("sandbox", "allow-same-origin");
        frame.referrerPolicy = "no-referrer";
        frame.title = "Message HTML";
        frame.addEventListener("load", function () {
          watchMessageHTMLFrame(frame);
        });
        frame.srcdoc = mimeHTML;
        wrap.appendChild(frame);
      } else if (body) {
        var pre = document.createElement("pre");
        pre.className = "msg-body";
        pre.textContent = body;
        wrap.appendChild(pre);
      } else {
        var empty = document.createElement("p");
        empty.className = "msg-meta";
        empty.textContent = "No body on this page. Serve overview to load the message.";
        wrap.appendChild(empty);
      }
      if (data && data.truncated) {
        var trunc = document.createElement("p");
        trunc.className = "msg-trunc";
        trunc.textContent = "Preview truncated.";
        wrap.appendChild(trunc);
      }
      if (worthBox) {
        worthBox.checked = !!(data && data.kept);
      }
      if (!keepOutliers) {
        clearOutlierBullets();
      }
    }

    function askIntent(selected, items, title) {
      var raw = items || [];
      lastItems = enrichItems(raw);
      previewHash = "";
      lastIntentKey = "";
      clearPreviewQueue();
      setMarkEnabled(lastItems.length > 0);
      setAckEnabled(lastItems.length > 0);
      setWorthEnabled(false);
      clearOutlierBullets();
      var seq = beginPanel();
      var month = activeMonth();
      if (!selected) {
        setPanelTitle("Message");
        showPanel("Nothing selected.", "Click a Reply or Decide row to preview it.", seq);
        return;
      }
      setPanelTitle(title || "Kind");
      if (raw.length && lastItems.length === 0) {
        showPanel("Already in Trash.", "Those messages are already in Trash.", seq);
        return;
      }
      var key = intentMemoKey(month, selected);
      lastIntentKey = key;
      if (intentMemo[key]) {
        showPanel("Polypus TLDR", intentMemo[key], seq);
        return;
      }
      var waitLabel = "Summarizing this selection…";
      if ((title || "") === "Sender") {
        waitLabel = "Summarizing this sender…";
      } else if (title) {
        waitLabel = "Summarizing this group…";
      }
      showWait(waitLabel, seq);
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
        if (!panelIsCurrent(seq)) {
          return;
        }
        if (out.ok && out.data && out.data.intent) {
          intentMemo[key] = out.data.intent;
          showPanel("Polypus TLDR", out.data.intent, seq);
          return;
        }
        var err = (out.data && out.data.error) || "Intent guess failed.";
        if (!out.ok && (err.indexOf("Failed to fetch") >= 0 || err.indexOf("JSON") >= 0)) {
          showPanel("Serve this report to ask Polypus.", "Run should-i-read mail overview serve and open the printed URL.", seq);
          return;
        }
        showPanel("Intent guess failed.", err, seq);
      }).catch(function () {
        if (!panelIsCurrent(seq)) {
          return;
        }
        showPanel("Serve this report to ask Polypus.", "Run should-i-read mail overview serve and open the printed URL.", seq);
      });
    }

    function loadMessagePreview(hash, fallback, seq, keepOutliers) {
      if (!hash) {
        showMessagePreview(null, fallback, seq, keepOutliers);
        return;
      }
      setPanelTitle("Message");
      showWait("Opening the message…", seq, keepOutliers);
      fetch("/preview?hash=" + encodeURIComponent(hash)).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        }).catch(function () {
          return { ok: false, data: {} };
        });
      }).then(function (out) {
        if (!panelIsCurrent(seq)) {
          return;
        }
        if (out.ok && out.data && out.data.ok) {
          showMessagePreview(out.data, fallback, seq, keepOutliers);
          return;
        }
        showMessagePreview(null, fallback, seq, keepOutliers);
        if (out.data && out.data.error) {
          statusEl.textContent = "Headers only. The message file is not in the mail store.";
        }
      }).catch(function () {
        showMessagePreview(null, fallback, seq, keepOutliers);
      });
    }

    function openActionRow(tr) {
      var items = enrichItems(itemsFromEl(tr));
      lastItems = items;
      lastIntentKey = "";
      previewHash = items.length ? items[0].object_hash : "";
      setPreviewQueue(collectPreviewQueue(tr, items, previewHash), previewHash);
      setMarkEnabled(items.length > 0);
      setAckEnabled(items.length > 0);
      setWorthEnabled(!!previewHash);
      clearOutlierBullets();
      var seq = beginPanel();
      var fallback = {
        sender: tr.getAttribute("data-sender") || "",
        subject: tr.getAttribute("data-subject") || "",
        date: tr.getAttribute("data-date") || ""
      };
      loadMessagePreview(items.length ? items[0].object_hash : "", fallback, seq, false);
    }

    function openOutlierCard(card) {
      var hash = (card.getAttribute("data-hash") || "").trim();
      if (!hash) {
        return;
      }
      previewHash = hash;
      setPreviewQueue(collectOutlierQueue(hash), hash);
      if (previewIndex >= 0) {
        lastItems = [previewQueue[previewIndex]];
      } else {
        lastItems = enrichItems(itemsFromEl(card));
      }
      setWorthEnabled(true);
      setMarkEnabled(lastItems.length > 0);
      setAckEnabled(lastItems.length > 0);
      var cards = outliersList ? outliersList.querySelectorAll(".outlier-card") : [];
      for (var i = 0; i < cards.length; i++) {
        if (cards[i] === card) {
          cards[i].classList.add("is-open");
        } else {
          cards[i].classList.remove("is-open");
        }
      }
      var seq = beginPanel();
      loadMessagePreview(hash, {
        sender: card.getAttribute("data-sender") || "",
        subject: card.getAttribute("data-subject") || "",
        date: card.getAttribute("data-date") || ""
      }, seq, true);
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

    var mailMonthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

    function parseMailDay(s) {
      var t = String(s || "").trim();
      if (t.length < 10 || t.charAt(4) !== "-" || t.charAt(7) !== "-") {
        return 0;
      }
      var y = parseInt(t.slice(0, 4), 10);
      var m = parseInt(t.slice(5, 7), 10);
      var d = parseInt(t.slice(8, 10), 10);
      if (!y || m < 1 || m > 12 || d < 1 || d > 31) {
        return 0;
      }
      return y * 10000 + m * 100 + d;
    }

    function formatMailDay(n, refYear) {
      if (!n) {
        return "";
      }
      var y = Math.floor(n / 10000);
      var m = Math.floor((n % 10000) / 100);
      var d = n % 100;
      var label = d + " " + mailMonthNames[m - 1];
      if (y !== refYear) {
        return label + " " + y;
      }
      return label;
    }

    function folderWhen(items) {
      var min = 0;
      var max = 0;
      for (var i = 0; i < items.length; i++) {
        var n = parseMailDay(items[i].date);
        if (!n) {
          continue;
        }
        if (!min || n < min) {
          min = n;
        }
        if (!max || n > max) {
          max = n;
        }
      }
      if (!min) {
        return "";
      }
      var year = (new Date()).getFullYear();
      if (min === max) {
        return formatMailDay(min, year);
      }
      return formatMailDay(min, year) + " - " + formatMailDay(max, year);
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
      var when = formatMailDay(parseMailDay(item.date), (new Date()).getFullYear());
      if (when) {
        var time = document.createElement("div");
        time.className = "cart-when";
        time.textContent = when;
        meta.appendChild(time);
      }
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

    function paintStageEmpty(list, kind) {
      if (!list) {
        return;
      }
      var wrap = document.createElement("div");
      wrap.className = "cart-empty";
      wrap.setAttribute("role", "status");
      var ns = "http://www.w3.org/2000/svg";
      var svg = document.createElementNS(ns, "svg");
      svg.setAttribute("class", "cart-empty-icon");
      svg.setAttribute("viewBox", "0 0 32 32");
      svg.setAttribute("width", "36");
      svg.setAttribute("height", "36");
      svg.setAttribute("aria-hidden", "true");
      svg.setAttribute("focusable", "false");
      var tray = document.createElementNS(ns, "path");
      tray.setAttribute("fill", "none");
      tray.setAttribute("stroke", "currentColor");
      tray.setAttribute("stroke-width", "1.8");
      tray.setAttribute("stroke-linejoin", "round");
      tray.setAttribute("d", "M4.8 13h22.4l-2.1 11.4c-.2 1.1-1.2 1.9-2.3 1.9H9.2c-1.1 0-2.1-.8-2.3-1.9L4.8 13z");
      var flap = document.createElementNS(ns, "path");
      flap.setAttribute("fill", "none");
      flap.setAttribute("stroke", "currentColor");
      flap.setAttribute("stroke-width", "1.8");
      flap.setAttribute("stroke-linecap", "round");
      flap.setAttribute("stroke-linejoin", "round");
      flap.setAttribute("d", "M11 13l5-6.2L21 13");
      svg.appendChild(tray);
      svg.appendChild(flap);
      var title = document.createElement("p");
      title.className = "cart-empty-title";
      title.textContent = "Empty";
      var hint = document.createElement("p");
      hint.className = "cart-empty-hint";
      hint.textContent = kind === "read"
        ? "Nothing waiting to mark as seen."
        : "Nothing waiting to move to Trash.";
      wrap.appendChild(svg);
      wrap.appendChild(title);
      wrap.appendChild(hint);
      list.appendChild(wrap);
    }

    function renderCart(data) {
      var countEl = document.getElementById("cart-count");
      var list = document.getElementById("cart-list");
      var n = (data && data.count) || 0;
      if (countEl) {
        countEl.textContent = String(n);
      }
      updateStageCount();
      updateStageHelp();
      if (!list) {
        applyStaged(data);
        return;
      }
      list.innerHTML = "";
      var items = (data && data.items) || [];
      if (!items.length) {
        paintStageEmpty(list, "trash");
        applyStaged(data);
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
        var when = folderWhen(group.items);
        if (when) {
          var span = document.createElement("div");
          span.className = "cart-folder-when";
          span.textContent = when;
          head.appendChild(span);
        }
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
      applyStaged(data);
    }

    function kindTreeBranchLi(el) {
      return !!(el && el.closest && el.closest(".ktree") && el.tagName === "LI" && el.querySelector && el.querySelector(":scope > details"));
    }

    function hashesMatchStaged(parts, set, needAll) {
      var any = false;
      for (var p = 0; p < parts.length; p++) {
        var hash = parts[p].trim();
        if (!hash) {
          continue;
        }
        any = true;
        if (set[hash]) {
          if (!needAll) {
            return true;
          }
        } else if (needAll) {
          return false;
        }
      }
      return needAll ? any : false;
    }

    function applyStaged(data) {
      var set = {};
      var items = (data && data.items) || [];
      for (var i = 0; i < items.length; i++) {
        var h = items[i] && items[i].object_hash;
        if (h) {
          set[h] = true;
        }
      }
      lastStagedSet = set;
      var nodes = document.querySelectorAll("[data-hashes]");
      for (var n = 0; n < nodes.length; n++) {
        var el = nodes[n];
        if (el.closest && (el.closest("#sec-actioned") || el.closest("#sec-actioned-full") || el.closest("#stage-panel"))) {
          continue;
        }
        var parts = String(el.getAttribute("data-hashes") || "").split(",");
        var staged = hashesMatchStaged(parts, set, kindTreeBranchLi(el));
        if (staged) {
          el.classList.add("is-staged");
          el.setAttribute("title", "Staged in the deletion cart");
        } else {
          el.classList.remove("is-staged");
          if (el.getAttribute("title") === "Staged in the deletion cart") {
            el.removeAttribute("title");
          }
        }
      }
      var rows = document.querySelectorAll("tr.delete-parent, tr.delete-entry, table.action-table tbody tr[data-hashes]");
      for (var p = 0; p < rows.length; p++) {
        syncExclusiveRow(rows[p]);
      }
    }

    function parentHasFreeHashes(tr) {
      var parts = String((tr && tr.getAttribute("data-hashes")) || "").split(",");
      for (var p = 0; p < parts.length; p++) {
        var hash = parts[p].trim();
        if (!hash) {
          continue;
        }
        if (!lastStagedSet[hash]) {
          return true;
        }
      }
      return false;
    }

    function syncExclusiveRow(row) {
      if (!row) {
        return;
      }
      var del = row.querySelector(".delete-mark");
      var ack = row.querySelector(".ack-mark");
      if (del) {
        del.disabled = !parentHasFreeHashes(row);
      }
      if (ack) {
        ack.disabled = row.classList.contains("is-acked");
      }
    }

    var mailSync = { poll: null, busy: false, status: "idle", awaitingBoard: false };
    var mailSyncBtn = document.getElementById("mail-sync");
    var mailSyncStripe = document.getElementById("mail-sync-stripe");

    function mailSyncIsRunning() {
      return mailSync.status === "running" || mailSync.busy;
    }

    function mailSyncIsErrorText(text, status) {
      if (status === "failed") {
        return true;
      }
      var t = String(text || "").toLowerCase();
      return t.indexOf("fail") >= 0 || t.indexOf("needs review") >= 0 || t.indexOf("error") >= 0;
    }

    function showMailSyncProgress(text) {
      if (!mailSyncStripe) {
        return;
      }
      var t = String(text || "").replace(/^\s*·\s*/, "").trim();
      if (!t) {
        t = "Syncing…";
      }
      mailSyncStripe.classList.add("is-progress");
      mailSyncStripe.hidden = false;
      mailSyncStripe.setAttribute("role", "status");
      mailSyncStripe.textContent = t;
    }

    function showMailSyncStatus(text) {
      if (!mailSyncStripe) {
        return;
      }
      if (mailSyncIsRunning()) {
        showMailSyncProgress(text);
        return;
      }
      var t = String(text || "").replace(/^\s*·\s*/, "").trim();
      mailSyncStripe.classList.remove("is-progress");
      if (!t || !mailSyncIsErrorText(t, mailSync.status)) {
        mailSyncStripe.hidden = true;
        mailSyncStripe.textContent = "";
        mailSyncStripe.removeAttribute("role");
        return;
      }
      mailSyncStripe.hidden = false;
      mailSyncStripe.setAttribute("role", "alert");
      mailSyncStripe.textContent = t;
    }

    function applyHostEvent(body) {
      if (!body || !body.line) {
        return;
      }
      if (mailSyncIsRunning() || body.source === "essence") {
        showMailSyncProgress(body.line);
      }
    }

    function startHostEvents() {
      if (!window.EventSource) {
        return;
      }
      var es = new EventSource("/events");
      es.onmessage = function (ev) {
        try {
          applyHostEvent(JSON.parse(ev.data));
        } catch (e) {}
      };
    }

    function mailSyncBlocked() {
      var applying = cartJob.status === "applying" || (ackJob && ackJob.status === "applying");
      return mailSyncIsRunning() || applying || cartJob.busy || ackJob.busy;
    }

    function paintSyncBusy(btn, running, blocked) {
      if (!btn) {
        return;
      }
      btn.disabled = blocked;
      btn.setAttribute("aria-busy", running ? "true" : "false");
      var spin = btn.querySelector(".mail-sync-spinner");
      if (spin) {
        spin.hidden = !running;
      }
    }

    function updateMailSyncButton() {
      var running = mailSyncIsRunning();
      var blocked = mailSyncBlocked();
      paintSyncBusy(mailSyncBtn, running, blocked);
      if (!mailSyncBtn) {
        return;
      }
      if (running) {
        setButtonTip(mailSyncBtn, "Syncing mail");
      } else {
        var whenEl = mailSyncBtn.querySelector(".mail-sync-when");
        var iso = whenEl ? whenEl.getAttribute("datetime") || "" : "";
        var whenText = whenEl && whenEl.textContent ? whenEl.textContent.trim() : "";
        var idleName = whenText ? "Sync mail, last built " + whenText : "Sync mail with the configured account";
        var longTitle = iso ? formatLocalBoardTitle(iso) : "";
        setButtonTip(mailSyncBtn, idleName, longTitle ? "Sync mail. Last built " + longTitle : idleName);
      }
    }

    function stopMailSyncPoll() {
      if (mailSync.poll) {
        clearInterval(mailSync.poll);
        mailSync.poll = null;
      }
    }

    function startMailSyncPoll() {
      stopMailSyncPoll();
      mailSync.poll = setInterval(refreshMailSync, 1000);
    }

    function reloadBoardIfSyncDone() {
      if (!mailSync.awaitingBoard || mailSync.status !== "done") {
        if (mailSync.status === "failed") {
          mailSync.awaitingBoard = false;
        }
        return false;
      }
      mailSync.awaitingBoard = false;
      stopMailSyncPoll();
      window.location.reload();
      return true;
    }

    function refreshMailSync() {
      fetch("/sync").then(function (res) {
        return res.json();
      }).then(function (body) {
        if (!body) {
          return;
        }
        mailSync.status = body.status || "idle";
        updateMailSyncButton();
        if (mailSyncIsRunning()) {
          showMailSyncStatus(body.line || "Syncing…");
        } else if (body.line) {
          showMailSyncStatus(body.line);
        } else if (body.error) {
          showMailSyncStatus(body.error);
        } else if (mailSync.status === "idle") {
          showMailSyncStatus("");
        }
        if (reloadBoardIfSyncDone()) {
          return;
        }
        if (mailSync.status !== "running") {
          stopMailSyncPoll();
        }
      }).catch(function () {});
    }

    function startMailSync() {
      if (mailSyncBlocked()) {
        return;
      }
      mailSync.busy = true;
      mailSync.awaitingBoard = true;
      updateMailSyncButton();
      fetch("/sync", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({})
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, status: res.status, data: data };
        });
      }).then(function (out) {
        mailSync.busy = false;
        if (!out.ok && out.status !== 202 && out.status !== 200) {
          mailSync.awaitingBoard = false;
          showMailSyncStatus((out.data && out.data.error) || "Sync failed.");
          updateMailSyncButton();
          return;
        }
        if (out.data && out.data.status) {
          mailSync.status = out.data.status;
        }
        updateMailSyncButton();
        if (mailSyncIsRunning()) {
          showMailSyncStatus((out.data && out.data.line) || "Syncing…");
        } else if (out.data && out.data.line) {
          showMailSyncStatus(out.data.line);
        }
        if (reloadBoardIfSyncDone()) {
          return;
        }
        if (mailSync.status === "running") {
          startMailSyncPoll();
        }
      }).catch(function () {
        mailSync.busy = false;
        mailSync.awaitingBoard = false;
        showMailSyncStatus("Sync needs overview serve with Neverest.");
        updateMailSyncButton();
      });
    }

    var cartJob = { id: "", status: "", poll: null, linePoll: null, last: null, busy: false };
    var applySession = false;
    var applyWantedAck = false;
    var applyWantedCart = false;
    var stageTab = "read";
    var cartJobConfirm = "trash-at-source";
    var cartJobStatusEl = document.getElementById("cart-job-status");
    var applySourceBtn = document.getElementById("apply-source");
    var applySourceStripe = document.getElementById("apply-source-stripe");

    function cartCountNow() {
      var el = document.getElementById("cart-count");
      return el ? parseInt(el.textContent, 10) || 0 : 0;
    }

    function updateStageCount() {
      var n = cartCountNow() + ackCountNow();
      var el = document.getElementById("stage-count");
      if (el) {
        el.textContent = String(n);
      }
      var btn = document.getElementById("stage-toggle");
      if (btn) {
        setButtonTip(btn, "Open Read and Trash staging, " + n);
      }
    }

    function updateStageHelp() {
      var staged = cartCountNow() + ackCountNow();
      var stageHelp = document.getElementById("stage-help");
      var ackHelp = document.getElementById("ack-help");
      var cartHelp = document.getElementById("cart-help");
      if (stageHelp) {
        stageHelp.hidden = staged === 0;
      }
      if (ackHelp) {
        ackHelp.hidden = stageTab !== "read" || ackCountNow() === 0;
      }
      if (cartHelp) {
        cartHelp.hidden = stageTab !== "trash" || cartCountNow() === 0;
      }
    }

    function paintStageJob(el, onTab, job, message, kind) {
      if (!el) {
        return;
      }
      el.textContent = "";
      var note = String(message || "").trim();
      if (!job && !note) {
        el.hidden = true;
        return;
      }
      if (note) {
        var msg = document.createElement("span");
        msg.className = "stage-job-state";
        msg.textContent = note;
        el.appendChild(msg);
        el.hidden = !onTab;
        return;
      }
      var status = String(job.status || "");
      var title = document.createElement("span");
      title.className = "stage-job-state";
      title.textContent = status ? status.charAt(0).toUpperCase() + status.slice(1) : "Job";
      el.appendChild(title);
      var c = job.counts || {};
      var rows = kind === "read"
        ? [["planned", c.planned], ["unresolved", c.unresolved], ["refused", c.refused], ["read", c.seen], ["failed", c.failed]]
        : [["planned", c.planned], ["unresolved", c.unresolved], ["refused", c.refused], ["trashed", c.trashed], ["failed", c.failed]];
      var hideZeros = status === "done" || status === "failed";
      for (var i = 0; i < rows.length; i++) {
        var n = rows[i][1] || 0;
        if (hideZeros && n === 0) {
          continue;
        }
        var chip = document.createElement("span");
        chip.className = "stage-job-count";
        chip.textContent = rows[i][0] + " " + n;
        el.appendChild(chip);
      }
      if (kind !== "read" && (c.refused || 0) >= 1) {
        var extra = document.createElement("span");
        extra.className = "stage-job-note";
        extra.textContent = c.refused + " already in Trash.";
        el.appendChild(extra);
      }
      if (job.error) {
        var err = document.createElement("span");
        err.className = "stage-job-note";
        err.textContent = job.error;
        el.appendChild(err);
      }
      el.hidden = !onTab;
      if (!el.hidden && el.childNodes.length) {
        var listId = el.id === "ack-job-status" ? "ack-list" : "cart-list";
        var list = document.getElementById(listId);
        var empty = list ? list.querySelector(".cart-empty") : null;
        if (empty) {
          empty.remove();
        }
      }
    }

    function clearStageJob(el) {
      if (!el) {
        return;
      }
      el.textContent = "";
      el.hidden = true;
    }

    function jobResultBits(job, kind) {
      if (!job || !job.counts) {
        return [];
      }
      var c = job.counts;
      var bits = [];
      if (kind === "read") {
        if ((c.seen || 0) >= 1) {
          bits.push("read " + c.seen);
        }
      } else if ((c.trashed || 0) >= 1) {
        bits.push("trashed " + c.trashed);
      }
      if (kind !== "read" && (c.refused || 0) >= 1) {
        bits.push(c.refused + " already in Trash");
      }
      if ((c.failed || 0) >= 1) {
        bits.push("failed " + c.failed);
      }
      return bits;
    }

    function appliedSourceLine() {
      var bits = [];
      if (applyWantedAck) {
        bits = bits.concat(jobResultBits(ackJob ? ackJob.last : null, "read"));
      }
      if (applyWantedCart) {
        bits = bits.concat(jobResultBits(cartJob.last, "trash"));
      }
      if (!bits.length) {
        return "Applied at source.";
      }
      return "Applied at source. " + bits.join(" · ");
    }

    function applyJobsFailed() {
      if (applyWantedCart && cartJob.last && cartJob.last.status === "failed") {
        return true;
      }
      if (applyWantedAck && ackJob && ackJob.last && ackJob.last.status === "failed") {
        return true;
      }
      return false;
    }

    function maybePaintApplyStripe() {
      if (!applySession) {
        return;
      }
      if (applyWantedCart && cartJob && (cartJob.busy || cartJob.status === "applying")) {
        return;
      }
      if (applyWantedAck && ackJob && (ackJob.busy || ackJob.status === "applying")) {
        return;
      }
      showApplySourceStripe(appliedSourceLine(), applyJobsFailed() ? "error" : "success");
    }

    function showCartJobStatus(text) {
      clearStageJob(cartJobStatusEl);
      var t = String(text || "").trim();
      if (!t || t.indexOf("Checking out") === 0) {
        return;
      }
      showApplySourceStripe(t, "error");
    }

    function showCartJob(job) {
      clearStageJob(cartJobStatusEl);
      if (job && job.status !== "applying") {
        maybePaintApplyStripe();
      }
    }

    function jobLeftover(job) {
      var planned = job && job.counts ? job.counts.planned : 0;
      var failed = job && job.counts ? job.counts.failed : 0;
      return planned >= 1 || failed >= 1;
    }

    function cartHasWork(cartCount) {
      return (cartCount || 0) >= 1 || jobLeftover(cartJob.last);
    }

    function ackHasWork(ackCount) {
      return (ackCount || 0) >= 1 || jobLeftover(ackJob.last);
    }

    function showApplySourceStripe(text, tone) {
      if (!applySourceStripe) {
        return;
      }
      var t = String(text || "").trim();
      applySourceStripe.classList.remove("is-progress", "is-success");
      if (!t) {
        applySourceStripe.hidden = true;
        applySourceStripe.textContent = "";
        applySourceStripe.removeAttribute("role");
        return;
      }
      if (tone === "success") {
        applySourceStripe.classList.add("is-success");
        applySourceStripe.setAttribute("role", "status");
      } else if (tone === "error") {
        applySourceStripe.setAttribute("role", "alert");
      } else {
        applySourceStripe.classList.add("is-progress");
        applySourceStripe.setAttribute("role", "status");
      }
      applySourceStripe.hidden = false;
      applySourceStripe.textContent = t;
    }

    function applySourceState() {
      var applying = (cartJob && cartJob.status === "applying") || (ackJob && ackJob.status === "applying");
      var checkingOut = !!(cartJob && cartJob.busy) || !!(ackJob && ackJob.busy);
      var needed = cartHasWork(cartCountNow()) || ackHasWork(ackCountNow());
      var busy = applying || checkingOut;
      return {
        applying: applying,
        checkingOut: checkingOut,
        needed: needed,
        busy: busy,
        disabled: busy || !needed || mailSyncIsRunning()
      };
    }

    function paintApplyButton(btn, state, hideWhenIdle) {
      if (!btn) {
        return;
      }
      if (hideWhenIdle) {
        btn.hidden = !state.needed && !state.busy;
      }
      btn.disabled = state.disabled;
      btn.setAttribute("aria-busy", state.busy ? "true" : "false");
      var spin = btn.querySelector(".apply-source-spinner");
      if (spin) {
        spin.hidden = !state.busy;
      }
    }

    function updateApplySourceButton() {
      var state = applySourceState();
      var applyAria = state.applying ? "Applying at source" : (state.checkingOut ? "Checking out" : "Apply at source");
      var applyTip = state.busy ? applyAria : "Mark Read as seen and move Trash at the mail source";
      paintApplyButton(applySourceBtn, state, true);
      if (applySourceBtn) {
        setButtonTip(applySourceBtn, applyAria, applyTip);
      }
      var stageApply = document.getElementById("stage-apply");
      paintApplyButton(stageApply, state, false);
      if (stageApply) {
        setButtonTip(stageApply, applyAria, applyTip);
      }
      updateMailSyncButton();
    }

    function updateCartJobButtons(job, cartCount) {
      updateApplySourceButton();
    }

    function jobCountsLine(job) {
      if (!job || !job.counts) {
        return "";
      }
      var c = job.counts;
      return "planned " + c.planned + " · unresolved " + c.unresolved + " · refused " + c.refused + " · trashed " + c.trashed + " · failed " + c.failed;
    }

    function jobStatusLine(job, cartCount) {
      if (!job) {
        return "";
      }
      var line = "Job " + job.status + ". " + jobCountsLine(job);
      var refused = job.counts ? job.counts.refused || 0 : 0;
      if (refused >= 1) {
        line += " " + refused + " already in Trash.";
      }
      if (job.error) {
        line += " " + job.error;
      }
      return line;
    }

    function stopCartJobPoll() {
      if (cartJob.poll) {
        clearInterval(cartJob.poll);
        cartJob.poll = null;
      }
    }

    function stopCartLinePoll() {
      if (cartJob.linePoll) {
        clearInterval(cartJob.linePoll);
        cartJob.linePoll = null;
      }
    }

    function startCartLinePoll() {
      stopCartLinePoll();
      cartJob.linePoll = setInterval(function () {
        if (cartJob.busy || cartJob.status === "applying") {
          return;
        }
        refreshJob(cartCountNow());
      }, 2000);
    }

    function startCartJobPoll(id) {
      if (cartJob.poll && cartJob.pollDone) {
        return cartJob.pollDone;
      }
      stopCartJobPoll();
      cartJob.pollDone = new Promise(function (resolve) {
        function tick() {
          fetch("/cart/jobs/" + encodeURIComponent(id)).then(function (res) {
            return res.json();
          }).then(function (body) {
            if (!body || !body.job) {
              return;
            }
            cartJob.id = body.job.id;
            cartJob.status = body.job.status;
            cartJob.last = body.job;
            showCartJob(body.job);
            updateCartJobButtons(body.job, cartCountNow());
            refreshActioned();
            if (body.job.status !== "applying") {
              stopCartJobPoll();
              refreshCart().then(function () {
                resolve(body.job);
              });
            }
          }).catch(function () {});
        }
        tick();
        cartJob.poll = setInterval(tick, 1000);
      });
      return cartJob.pollDone;
    }

    function checkoutTrashAtSource() {
      cartJob.busy = true;
      showCartJobStatus("Checking out deletion cart…");
      updateApplySourceButton();
      return fetch("/cart/checkout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: cartJobConfirm })
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, status: res.status, data: data };
        });
      }).then(function (out) {
        cartJob.busy = false;
        if (!out.ok && out.status !== 202 && out.status !== 200) {
          showCartJobStatus((out.data && out.data.error) || "Checkout failed.");
          if (out.data && out.data.job) {
            cartJob.id = out.data.job.id;
            cartJob.status = out.data.job.status;
            cartJob.last = out.data.job;
          }
          updateApplySourceButton();
          return out;
        }
        var job = out.data && out.data.job;
        if (!job) {
          showCartJobStatus("Checkout had no job.");
          updateApplySourceButton();
          return out;
        }
        cartJob.id = job.id;
        cartJob.status = job.status;
        cartJob.last = job;
        showCartJob(job);
        updateApplySourceButton();
        if (job.status === "applying") {
          return startCartJobPoll(job.id).then(function () {
            return out;
          });
        }
        return refreshCart().then(function () {
          return out;
        });
      }).catch(function () {
        cartJob.busy = false;
        showCartJobStatus("Deletion checkout needs overview serve with pimdir.");
        updateApplySourceButton();
        return { ok: false, data: { error: "Deletion checkout needs overview serve with pimdir." } };
      });
    }

    function actionedWhen(row) {
      if (!row || !row.from) {
        return "";
      }
      if (!row.until || row.until === row.from) {
        return row.from;
      }
      return row.from + " through " + row.until;
    }

    function allActionedRows() {
      if (lastActionedRows !== null) {
        return lastActionedRows;
      }
      return overviewModel().actioned || [];
    }

    function actionedRowsForMonth(rows, key) {
      rows = rows || [];
      if (!key || key === "all") {
        return rows;
      }
      var out = [];
      for (var i = 0; i < rows.length; i++) {
        if ((rows[i].month_key || "") === key) {
          out.push(rows[i]);
        }
      }
      return out;
    }

    function actionedCountForMonth(rows, key) {
      var slice = actionedRowsForMonth(rows, key);
      var n = 0;
      for (var i = 0; i < slice.length; i++) {
        n += slice[i].count || 0;
      }
      return n;
    }

    function writeActionedTabCounts(rows) {
      var buttons = document.querySelectorAll(".month-tabs button[data-month]");
      for (var i = 0; i < buttons.length; i++) {
        var key = buttons[i].getAttribute("data-month") || "";
        var el = buttons[i].querySelector(".tab-count");
        if (el) {
          el.setAttribute("data-actioned-count", String(actionedCountForMonth(rows, key)));
        }
      }
      paintMonthTabCounts();
    }

    function fillActionedTable(bodyId, secId, rows) {
      var body = document.getElementById(bodyId);
      var sec = document.getElementById(secId);
      if (!body || !sec) {
        return;
      }
      body.textContent = "";
      for (var r = 0; r < rows.length; r++) {
        var row = rows[r];
        var tr = document.createElement("tr");
        tr.setAttribute("data-hashes", (row.hashes || []).join(","));
        tr.setAttribute("data-sender", row.sender || "");
        if (row.month_key) {
          tr.setAttribute("data-month", row.month_key);
        }
        var tdS = document.createElement("td");
        tdS.textContent = row.sender || "";
        var tdC = document.createElement("td");
        tdC.textContent = String(row.count || 0);
        var tdW = document.createElement("td");
        tdW.textContent = actionedWhen(row);
        var tdO = document.createElement("td");
        tdO.textContent = row.outcome || "";
        tr.appendChild(tdS);
        tr.appendChild(tdC);
        tr.appendChild(tdW);
        tr.appendChild(tdO);
        body.appendChild(tr);
      }
      sec.hidden = rows.length === 0;
    }

    function paintActionedView(key) {
      var rows = actionedRowsForMonth(allActionedRows(), key);
      fillActionedTable("actioned-body", "sec-actioned", rows);
      paintActionedCharts("sec-actioned-report", "actioned-outcome-bars", "actioned-sender-bars", rows);
    }

    function paintActionedFull() {
      var rows = allActionedRows();
      fillActionedTable("actioned-full-body", "sec-actioned-full", rows);
      paintActionedCharts("sec-actioned-full-report", "actioned-full-outcome-bars", "actioned-full-sender-bars", rows);
    }

    function applyActioned(rows) {
      var set = {};
      lastActionedRows = rows || [];
      for (var i = 0; i < lastActionedRows.length; i++) {
        var hs = (lastActionedRows[i] && lastActionedRows[i].hashes) || [];
        for (var j = 0; j < hs.length; j++) {
          if (hs[j]) {
            set[hs[j]] = true;
          }
        }
      }
      var nodes = document.querySelectorAll("[data-hashes], [data-hash]");
      for (var n = 0; n < nodes.length; n++) {
        var el = nodes[n];
        if (el.closest && (el.closest("#sec-actioned") || el.closest("#sec-actioned-full") || el.closest(".page-report"))) {
          continue;
        }
        var liveN = liveHashCount(el, set);
        var any = hashParts(el).length > 0;
        if (any && liveN === 0) {
          el.classList.add("is-actioned");
        } else {
          el.classList.remove("is-actioned");
        }
        var drill = el.getAttribute && el.getAttribute("data-hash");
        if (drill) {
          var drillRow = document.getElementById("drill-" + drill);
          if (drillRow) {
            if (set[drill]) {
              drillRow.classList.add("is-actioned");
            } else {
              drillRow.classList.remove("is-actioned");
            }
          }
        }
      }
      paintActionedView(currentMonthKey());
      paintActionedFull();
      lastActioned = set;
      writeActionedTabCounts(lastActionedRows);
      syncLiveChrome(set);
      hideEmptyFluffSenders();
    }

    onBoardMonth = function (key) {
      paintActionedView(key);
    };
    paintActionedView(currentMonthKey());
    paintActionedFull();
    writeActionedTabCounts(allActionedRows());

    function fillReportBars(el, items) {
      if (!el) {
        return;
      }
      el.textContent = "";
      var max = 1;
      for (var i = 0; i < items.length; i++) {
        if (items[i].count > max) {
          max = items[i].count;
        }
      }
      for (var j = 0; j < items.length; j++) {
        var item = items[j];
        var pct = Math.round((item.count * 100) / max);
        if (item.count > 0 && pct < 2) {
          pct = 2;
        }
        var row = document.createElement("div");
        row.className = "report-bar-row";
        var lab = document.createElement("span");
        lab.className = "report-bar-lab";
        lab.textContent = item.label;
        lab.setAttribute("title", item.label);
        var track = document.createElement("span");
        track.className = "report-bar-track";
        var fill = document.createElement("span");
        fill.className = "report-bar-fill";
        fill.style.width = pct + "%";
        track.appendChild(fill);
        var n = document.createElement("span");
        n.className = "report-bar-n";
        n.textContent = String(item.count);
        row.appendChild(lab);
        row.appendChild(track);
        row.appendChild(n);
        el.appendChild(row);
      }
    }

    function paintActionedCharts(secId, outcomeId, senderId, rows) {
      var sec = document.getElementById(secId);
      var outcomeEl = document.getElementById(outcomeId);
      var senderEl = document.getElementById(senderId);
      rows = rows || [];
      var outcomes = {};
      var senders = {};
      var order = ["trashed at source", "already in Trash", "marked read at source"];
      for (var i = 0; i < rows.length; i++) {
        var row = rows[i];
        var oc = String((row && row.outcome) || "").trim();
        var sender = String((row && row.sender) || "").trim();
        var n = row && row.count ? row.count : 0;
        if (oc && n > 0) {
          outcomes[oc] = (outcomes[oc] || 0) + n;
        }
        if (sender && n > 0) {
          senders[sender] = (senders[sender] || 0) + n;
        }
      }
      var outcomeItems = [];
      var seen = {};
      for (var o = 0; o < order.length; o++) {
        if (outcomes[order[o]]) {
          outcomeItems.push({ label: order[o], count: outcomes[order[o]] });
          seen[order[o]] = true;
        }
      }
      var extra = [];
      for (var k in outcomes) {
        if (outcomes.hasOwnProperty(k) && !seen[k]) {
          extra.push(k);
        }
      }
      extra.sort();
      for (var e = 0; e < extra.length; e++) {
        outcomeItems.push({ label: extra[e], count: outcomes[extra[e]] });
      }
      var senderItems = [];
      for (var s in senders) {
        if (senders.hasOwnProperty(s)) {
          senderItems.push({ label: s, count: senders[s] });
        }
      }
      senderItems.sort(function (a, b) {
        if (b.count !== a.count) {
          return b.count - a.count;
        }
        return a.label < b.label ? -1 : 1;
      });
      if (senderItems.length > 8) {
        senderItems = senderItems.slice(0, 8);
      }
      fillReportBars(outcomeEl, outcomeItems);
      fillReportBars(senderEl, senderItems);
      if (sec) {
        sec.hidden = outcomeItems.length === 0;
      }
    }

    function hideEmptyFluffSenders() {
      var tables = document.querySelectorAll("table.fluff-senders");
      for (var i = 0; i < tables.length; i++) {
        var table = tables[i];
        var rows = table.querySelectorAll("tbody tr");
        var live = 0;
        for (var r = 0; r < rows.length; r++) {
          if (!rows[r].classList.contains("is-actioned")) {
            live++;
          }
        }
        var wrap = table.closest(".table-wrap") || table;
        wrap.hidden = live === 0;
        var heading = wrap.previousElementSibling;
        if (heading && heading.classList && heading.classList.contains("sub")) {
          heading.hidden = live === 0;
        }
        var panel = table.closest("section.panel");
        if (panel) {
          panel.hidden = live === 0;
        }
      }
    }

    function hashParts(el) {
      if (!el || !el.getAttribute) {
        return [];
      }
      var many = el.getAttribute("data-hashes");
      var one = el.getAttribute("data-hash");
      var raw = many || one || "";
      var parts = String(raw).split(",");
      var out = [];
      for (var i = 0; i < parts.length; i++) {
        var h = parts[i].trim();
        if (h) {
          out.push(h);
        }
      }
      return out;
    }

    function liveHashCount(el, actioned) {
      var parts = hashParts(el);
      var n = 0;
      var seen = {};
      for (var i = 0; i < parts.length; i++) {
        var h = parts[i];
        if (seen[h] || (actioned && actioned[h])) {
          continue;
        }
        seen[h] = true;
        n++;
      }
      return n;
    }

    function uniqueLiveIn(root, actioned) {
      if (!root || !root.querySelectorAll) {
        return 0;
      }
      var seen = {};
      var n = 0;
      var nodes = root.querySelectorAll("[data-hashes], [data-hash]");
      for (var i = 0; i < nodes.length; i++) {
        var el = nodes[i];
        if (el.closest && (el.closest("#sec-actioned") || el.closest("#sec-actioned-full"))) {
          continue;
        }
        var parts = hashParts(el);
        for (var p = 0; p < parts.length; p++) {
          var h = parts[p];
          if (seen[h] || (actioned && actioned[h])) {
            continue;
          }
          seen[h] = true;
          n++;
        }
      }
      return n;
    }

    var overviewModelCache;
    function overviewModel() {
      if (overviewModelCache) {
        return overviewModelCache;
      }
      var raw = document.getElementById("overview-model");
      if (!raw || !raw.textContent) {
        overviewModelCache = {};
        return overviewModelCache;
      }
      try {
        overviewModelCache = JSON.parse(raw.textContent);
      } catch (err) {
        overviewModelCache = {};
      }
      return overviewModelCache;
    }

    function sectionMessages(key) {
      var model = overviewModel();
      if (key === "all" && model.all) {
        return model.all.messages || [];
      }
      var months = model.months || [];
      for (var i = 0; i < months.length; i++) {
        if (months[i] && months[i].key === key) {
          return months[i].messages || [];
        }
      }
      return [];
    }

    function setNeedTile(tile, n) {
      if (!tile) {
        return;
      }
      var countEl = tile.querySelector(".need-count");
      if (countEl) {
        countEl.textContent = String(n);
      }
      if (n === 0) {
        tile.classList.add("is-zero");
      } else {
        tile.classList.remove("is-zero");
      }
    }

    function shareLabel(share) {
      if (share < 0.01) {
        return "<1%";
      }
      return Math.round(share * 100) + "%";
    }

    function syncKindTree(panel, actioned) {
      var tree = panel.querySelector(".page-overview .kind-tree");
      if (!tree) {
        return;
      }
      var items = tree.querySelectorAll("li[data-hashes]");
      var top = [];
      var maxTop = 1;
      for (var i = 0; i < items.length; i++) {
        var li = items[i];
        var n = liveHashCount(li, actioned);
        var row = li.querySelector(":scope > details > summary .krow, :scope > .kleaf .krow");
        if (row) {
          var countEl = row.querySelector(".kcount");
          if (countEl) {
            countEl.textContent = String(n);
          }
        }
        if (li.parentElement && li.parentElement.classList.contains("ktree")) {
          top.push({ li: li, n: n, row: row });
          if (n > maxTop) {
            maxTop = n;
          }
        }
      }
      var tabLive = uniqueLiveIn(panel, actioned);
      if (tabLive < 1) {
        tabLive = 1;
      }
      for (var t = 0; t < items.length; t++) {
        var node = items[t];
        var live = liveHashCount(node, actioned);
        var krow = node.querySelector(":scope > details > summary .krow, :scope > .kleaf .krow");
        if (!krow) {
          continue;
        }
        var parentLi = node.parentElement ? node.parentElement.closest("li[data-hashes]") : null;
        var parentN = parentLi ? liveHashCount(parentLi, actioned) : tabLive;
        if (parentN < 1) {
          parentN = 1;
        }
        var shareEl = krow.querySelector(".kshare");
        if (shareEl) {
          shareEl.textContent = shareLabel(live / parentN);
        }
        var fill = krow.querySelector(".kfill");
        if (fill) {
          fill.style.width = (live / maxTop * 100).toFixed(2) + "%";
        }
      }
      var hint = tree.querySelector(".ktree-hint");
      if (hint) {
        hint.textContent = hint.textContent.replace(/\d+ messages\.?$/, uniqueLiveIn(tree, actioned) + " messages.");
      }
    }

    function syncLiveChrome(actioned) {
      var panels = document.querySelectorAll(".tab-panel");
      for (var p = 0; p < panels.length; p++) {
        var panel = panels[p];
        var key = panel.getAttribute("data-month") || "";
        var tabLive = uniqueLiveIn(panel, actioned);
        var tabCount = document.querySelector('.month-tabs button[data-month="' + key + '"] .tab-count');
        if (tabCount) {
          if (!tabCount.getAttribute("data-full-count")) {
            tabCount.setAttribute("data-full-count", tabCount.textContent || "0");
          }
          tabCount.setAttribute("data-live-count", String(tabLive));
        }
        var replyLive = 0;
        var decideLive = 0;
        var lists = panel.querySelectorAll(".action-list");
        for (var l = 0; l < lists.length; l++) {
          var list = lists[l];
          var live = uniqueLiveIn(list, actioned);
          if (list.classList.contains("tone-reply")) {
            replyLive = live;
          }
          if (list.classList.contains("tone-decide")) {
            decideLive = live;
          }
          var countEl = list.querySelector(".list-count");
          if (countEl) {
            var rowsLeft = 0;
            var rows = list.querySelectorAll("tbody tr[data-hashes]");
            for (var r = 0; r < rows.length; r++) {
              var remain = liveHashCount(rows[r], actioned);
              if (remain > 0) {
                rowsLeft++;
              }
              var badge = rows[r].querySelector(".repeat");
              if (badge) {
                if (remain <= 1) {
                  badge.hidden = true;
                } else {
                  badge.hidden = false;
                  badge.textContent = "×" + remain;
                }
              }
              var ncell = rows[r].querySelector(".delete-count");
              if (ncell) {
                ncell.textContent = String(remain);
              }
              var mcell = rows[r].querySelector(".delete-minutes");
              if (mcell) {
                mcell.textContent = (remain * 5 / 60).toFixed(2);
              }
            }
            countEl.textContent = live + " messages" + (rowsLeft < live ? ", repeats grouped" : "");
          }
          list.hidden = live === 0;
          var extraLive = 0;
          var extra = list.querySelectorAll("tr.extra");
          for (var e = 0; e < extra.length; e++) {
            if (liveHashCount(extra[e], actioned) > 0) {
              extraLive++;
            }
          }
          var more = list.querySelector(".more-btn");
          if (more) {
            more.hidden = extraLive === 0;
            more.setAttribute("data-more-label", "Show " + extraLive + " more");
            if (more.getAttribute("aria-expanded") !== "true") {
              more.textContent = "Show " + extraLive + " more";
            }
          }
        }
        var reply = 0;
        var decide = 0;
        var urgent = 0;
        var quiet = 0;
        var msgs = sectionMessages(key);
        if (msgs.length) {
          for (var m = 0; m < msgs.length; m++) {
            var h = msgs[m] && msgs[m].object_hash;
            if (!h || (actioned && actioned[h])) {
              continue;
            }
            var nr = msgs[m].needs_reply || 0;
            var dec = msgs[m].decision || 0;
            var urg = msgs[m].urgency || 0;
            if (nr >= 0.5) {
              reply++;
            } else if (dec >= 0.5) {
              decide++;
            } else if (urg >= 0.5) {
              urgent++;
            } else {
              quiet++;
            }
          }
        } else {
          reply = replyLive;
          decide = decideLive;
        }
        var tiles = panel.querySelectorAll(".need-tile");
        for (var ti = 0; ti < tiles.length; ti++) {
          var tile = tiles[ti];
          if (tile.classList.contains("tone-reply")) {
            setNeedTile(tile, reply);
          } else if (tile.classList.contains("tone-decide")) {
            setNeedTile(tile, decide);
          } else if (tile.classList.contains("tone-urgent")) {
            setNeedTile(tile, urgent);
          } else if (tile.classList.contains("tone-safe")) {
            setNeedTile(tile, quiet);
          }
        }
        syncKindTree(panel, actioned);
      }
      paintMonthTabCounts();
    }

    function refreshActioned() {
      return fetch("/actioned").then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        }).catch(function () {
          return { ok: false, data: {} };
        });
      }).then(function (out) {
        if (out.ok && out.data && out.data.ok) {
          applyActioned(out.data.rows || []);
        }
      }).catch(function () {});
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
          return refreshJob(out.data.count || 0).then(function () {
            return refreshActioned();
          });
        }
        return refreshActioned();
      }).catch(function () {});
    }

    function refreshJob(cartCount) {
      var url = cartJob.id ? "/cart/jobs/" + encodeURIComponent(cartJob.id) : "/cart/jobs";
      return fetch(url).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        }).catch(function () {
          return { ok: false, data: {} };
        });
      }).then(function (out) {
        if (!out.ok || !out.data || !out.data.job) {
          return;
        }
        cartJob.id = out.data.job.id;
        cartJob.status = out.data.job.status;
        cartJob.last = out.data.job;
        showCartJob(out.data.job);
        updateCartJobButtons(out.data.job, cartCount);
        if (out.data.job.status === "applying") {
          if (!cartJob.poll) {
            startCartJobPoll(out.data.job.id);
          }
        } else {
          stopCartJobPoll();
        }
      }).catch(function () {});
    }

    var cartPanel = document.getElementById("stage-panel");
    var stageToggle = document.getElementById("stage-toggle");
    var stageClose = document.getElementById("stage-close");
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

    function setStageTab(tab) {
      stageTab = tab === "trash" ? "trash" : "read";
      var readBtn = document.getElementById("stage-tab-read");
      var trashBtn = document.getElementById("stage-tab-trash");
      var ackList = document.getElementById("ack-list");
      var trashList = document.getElementById("cart-list");
      if (readBtn) {
        readBtn.setAttribute("aria-selected", stageTab === "read" ? "true" : "false");
      }
      if (trashBtn) {
        trashBtn.setAttribute("aria-selected", stageTab === "trash" ? "true" : "false");
      }
      if (ackList) {
        ackList.hidden = stageTab !== "read";
      }
      if (trashList) {
        trashList.hidden = stageTab !== "trash";
      }
      updateStageHelp();
      clearStageJob(document.getElementById("ack-job-status"));
      clearStageJob(document.getElementById("cart-job-status"));
    }

    function setStageOpen(on, tab) {
      if (!cartPanel) {
        return;
      }
      if (tab) {
        setStageTab(tab);
      }
      cartPanel.hidden = !on;
      document.body.classList.toggle("cart-open", on);
      if (stageToggle) {
        stageToggle.setAttribute("aria-expanded", on ? "true" : "false");
      }
      if (on) {
        if (stageTab === "trash") {
          refreshCart();
          startCartLinePoll();
          writeNav({ side: "trash" });
        } else {
          stopCartLinePoll();
          refreshAck();
          writeNav({ side: "read" });
        }
      } else {
        stopCartLinePoll();
        var side = readNav().side;
        if (side === "cart" || side === "ack" || side === "trash" || side === "read") {
          writeNav({ side: "" });
        }
      }
    }

    function setCartOpen(on) {
      if (on) {
        setStageOpen(true, "trash");
        return;
      }
      if (cartPanel && !cartPanel.hidden && stageTab === "trash") {
        setStageOpen(false);
      }
    }

    if (stageToggle && cartPanel) {
      stageToggle.addEventListener("click", function () {
        if (!cartPanel.hidden) {
          setStageOpen(false);
          return;
        }
        var openTab = "read";
        if (cartCountNow() && !ackCountNow()) {
          openTab = "trash";
        } else if (stageTab === "trash") {
          openTab = "trash";
        }
        setStageOpen(true, openTab);
      });
    }
    if (stageClose && cartPanel) {
      stageClose.addEventListener("click", function () {
        setStageOpen(false);
      });
    }
    var stageTabs = document.querySelector(".stage-tabs");
    if (stageTabs) {
      stageTabs.addEventListener("click", function (ev) {
        var btn = ev.target.closest && ev.target.closest("[data-stage-tab]");
        if (!btn) {
          return;
        }
        ev.preventDefault();
        setStageOpen(true, btn.getAttribute("data-stage-tab"));
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
          updateCartJobButtons(cartJob.last, (data && data.count) || 0);
        }).catch(function () {});
      });
    }
    if (worthBox) {
      worthBox.addEventListener("change", function () {
        if (worthBusy || !previewHash) {
          return;
        }
        worthBusy = true;
        var want = worthBox.checked;
        var req;
        if (want) {
          var row = lastItems[0] || { object_hash: previewHash };
          req = fetch("/keep", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ items: [{
              object_hash: previewHash,
              sender: row.sender || "",
              subject: row.subject || "",
              date: row.date || ""
            }] })
          });
        } else {
          req = fetch("/keep?hash=" + encodeURIComponent(previewHash), { method: "DELETE" });
        }
        req.then(function (res) {
          if (!res) {
            return { ok: true, data: {} };
          }
          return res.json().then(function (data) {
            return { ok: res.ok, data: data };
          }).catch(function () {
            return { ok: false, data: {} };
          });
        }).then(function (out) {
          worthBusy = false;
          if (!out.ok) {
            worthBox.checked = !want;
            statusEl.textContent = (out.data && out.data.error) || "Could not save worth reading.";
            return;
          }
        }).catch(function () {
          worthBusy = false;
          worthBox.checked = !want;
          statusEl.textContent = "Serve this report to save worth reading.";
        });
      });
    }
    function postCart(items, opts) {
      opts = opts || {};
      items = enrichItems(items || []);
      var markEl = opts.markBtn || null;
      if (!items.length) {
        if (markEl) {
          markEl.disabled = true;
        }
        if (opts.fromPanel) {
          showPanel("Already in Trash.", "Those messages are already in Trash.");
          setMarkEnabled(false);
        }
        return;
      }
      if (markEl) {
        markEl.disabled = true;
      }
      var tldr = opts.tldr;
      if (!tldr) {
        tldr = cartTldr();
      }
      fetch("/cart", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ items: items, tldr: tldr, group_label: folderLabel(items, tldr) })
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        });
      }).then(function (out) {
        if (!out.ok) {
          if (markEl) {
            markEl.disabled = false;
          }
          showPanel("Could not add to the cart.", (out.data && out.data.error) || "Cart add failed.");
          return;
        }
        renderCart(out.data);
        setCartOpen(true);
        refreshAck();
        refreshActioned();
        var skipped = (out.data && (out.data.skipped_actioned || 0)) || 0;
        var staged = (out.data && out.data.count) || 0;
        if (opts.fromPanel) {
          setMarkEnabled(false);
          forgetIntentMemo();
        }
        if (skipped && !staged) {
          showPanel("Already in Trash.", skipped + " already in Trash. Nothing added to the cart.");
          return;
        }
        if (opts.fromPanel) {
          if (advancePreviewAfterStage()) {
            return;
          }
          dismissPanel();
        }
      }).catch(function () {
        if (markEl) {
          markEl.disabled = false;
        }
        showPanel("Could not add to the cart.", "Serve this report to use the deletion cart.");
      });
    }

    function stageDeleteParent(tr) {
      var nameEl = tr.querySelector(".sender-name");
      var name = (nameEl && nameEl.textContent.trim()) || tr.getAttribute("data-sender") || "Delete";
      var items = itemsFromEl(tr);
      var n = enrichItems(items).length;
      var tldr = name;
      if (n) {
        tldr = name + " · " + n + (n === 1 ? " message" : " messages");
      }
      postCart(items, { tldr: tldr, markBtn: tr.querySelector(".delete-mark") });
    }

    function stageAckParent(tr) {
      postAck(itemsFromEl(tr), tr.querySelector(".ack-mark"));
    }

    var ackJob = { id: "", status: "", poll: null, last: null, busy: false };
    var ackJobConfirm = "seen-at-source";
    var ackJobStatusEl = document.getElementById("ack-job-status");

    function ackCountNow() {
      var el = document.getElementById("ack-count");
      return el ? parseInt(el.textContent, 10) || 0 : 0;
    }

    function applyAcked(data) {
      var set = {};
      var items = (data && data.items) || [];
      for (var i = 0; i < items.length; i++) {
        var h = items[i] && items[i].object_hash;
        if (h) {
          set[h] = true;
        }
      }
      lastAckedSet = set;
      var nodes = document.querySelectorAll("table.action-table tbody tr[data-hashes], tr.delete-parent[data-hashes], tr.delete-entry[data-hashes]");
      for (var n = 0; n < nodes.length; n++) {
        var el = nodes[n];
        var parts = String(el.getAttribute("data-hashes") || "").split(",");
        var staged = hashesMatchStaged(parts, set, el.classList.contains("delete-parent"));
        if (staged) {
          el.classList.add("is-acked");
          el.setAttribute("title", "Staged to mark as read");
        } else {
          el.classList.remove("is-acked");
          if (el.getAttribute("title") === "Staged to mark as read") {
            el.removeAttribute("title");
          }
        }
        syncExclusiveRow(el);
      }
    }

    function renderAck(data) {
      var countEl = document.getElementById("ack-count");
      var list = document.getElementById("ack-list");
      var n = (data && data.count) || 0;
      if (countEl) {
        countEl.textContent = String(n);
      }
      updateStageCount();
      updateStageHelp();
      applyAcked(data);
      if (!list) {
        return;
      }
      list.innerHTML = "";
      var items = (data && data.items) || [];
      if (!items.length) {
        paintStageEmpty(list, "read");
        return;
      }
      for (var i = 0; i < items.length; i++) {
        var it = items[i];
        var row = document.createElement("div");
        row.className = "cart-item";
        var meta = document.createElement("div");
        meta.textContent = (it.sender || "Unknown") + " · " + (it.subject || "(no subject)");
        var rm = document.createElement("button");
        rm.type = "button";
        rm.className = "theme-toggle";
        rm.setAttribute("data-hash", it.object_hash);
        rm.textContent = "Remove";
        row.appendChild(meta);
        row.appendChild(rm);
        list.appendChild(row);
      }
    }

    function showAckJobStatus(text) {
      clearStageJob(ackJobStatusEl);
      var t = String(text || "").trim();
      if (!t || t.indexOf("Checking out") === 0) {
        return;
      }
      showApplySourceStripe(t, "error");
    }

    function showAckJob(job) {
      clearStageJob(ackJobStatusEl);
      if (job && job.status !== "applying") {
        maybePaintApplyStripe();
      }
    }

    function ackJobCountsLine(job) {
      if (!job || !job.counts) {
        return "";
      }
      var c = job.counts;
      return "planned " + c.planned + " · unresolved " + c.unresolved + " · refused " + c.refused + " · read " + c.seen + " · failed " + c.failed;
    }

    function ackJobStatusLine(job) {
      if (!job) {
        return "";
      }
      var line = "Job " + job.status + ". " + ackJobCountsLine(job);
      if (job.error) {
        line += " " + job.error;
      }
      return line;
    }

    function updateAckJobButtons(job, cartCount) {
      updateApplySourceButton();
    }

    function stopAckJobPoll() {
      if (ackJob.poll) {
        clearInterval(ackJob.poll);
        ackJob.poll = null;
      }
    }

    function startAckJobPoll(id) {
      if (ackJob.poll && ackJob.pollDone) {
        return ackJob.pollDone;
      }
      stopAckJobPoll();
      ackJob.pollDone = new Promise(function (resolve) {
        function tick() {
          fetch("/ack/jobs/" + encodeURIComponent(id)).then(function (res) {
            return res.json();
          }).then(function (body) {
            if (!body || !body.job) {
              return;
            }
            ackJob.id = body.job.id;
            ackJob.status = body.job.status;
            ackJob.last = body.job;
            showAckJob(body.job);
            updateAckJobButtons(body.job, ackCountNow());
            refreshActioned();
            if (body.job.status !== "applying") {
              stopAckJobPoll();
              refreshAck().then(function () {
                resolve(body.job);
              });
            }
          }).catch(function () {});
        }
        tick();
        ackJob.poll = setInterval(tick, 1000);
      });
      return ackJob.pollDone;
    }

    function refreshAckJob(cartCount) {
      var url = ackJob.id ? "/ack/jobs/" + encodeURIComponent(ackJob.id) : "/ack/jobs";
      return fetch(url).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        }).catch(function () {
          return { ok: false, data: {} };
        });
      }).then(function (out) {
        if (!out.ok || !out.data || !out.data.job) {
          updateAckJobButtons(ackJob.last, cartCount);
          return;
        }
        ackJob.id = out.data.job.id;
        ackJob.status = out.data.job.status;
        ackJob.last = out.data.job;
        showAckJob(out.data.job);
        updateAckJobButtons(out.data.job, cartCount);
        if (out.data.job.status === "applying") {
          if (!ackJob.poll) {
            startAckJobPoll(out.data.job.id);
          }
        }
      }).catch(function () {});
    }

    function refreshAck() {
      return fetch("/ack").then(function (res) {
        return res.json();
      }).then(function (data) {
        renderAck(data);
        return refreshAckJob((data && data.count) || 0);
      }).catch(function () {});
    }

    function setAckOpen(on) {
      if (on) {
        setStageOpen(true, "read");
        return;
      }
      if (cartPanel && !cartPanel.hidden && stageTab === "read") {
        setStageOpen(false);
      }
    }

    function postAck(items, markBtn, opts) {
      opts = opts || {};
      items = enrichItems(items);
      if (!items.length) {
        if (opts.fromPanel) {
          setAckEnabled(false);
        }
        return;
      }
      if (markBtn) {
        markBtn.disabled = true;
      }
      fetch("/ack", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ items: items })
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, data: data };
        });
      }).then(function (out) {
        if (!out.ok) {
          if (markBtn) {
            markBtn.disabled = false;
          }
          if (opts.fromPanel) {
            statusEl.textContent = (out.data && out.data.error) || "Could not add to Acked.";
          }
          return;
        }
        renderAck(out.data);
        setAckOpen(true);
        refreshAckJob((out.data && out.data.count) || 0);
        refreshCart();
        refreshActioned();
        if (opts.fromPanel) {
          if (advancePreviewAfterStage()) {
            return;
          }
          dismissPanel();
        }
      }).catch(function () {
        if (markBtn) {
          markBtn.disabled = false;
        }
        if (opts.fromPanel) {
          statusEl.textContent = "Serve this report to use Acked.";
        }
      });
    }

    function checkoutSeenAtSource() {
      ackJob.busy = true;
      showAckJobStatus("Checking out Read list…");
      updateApplySourceButton();
      return fetch("/ack/checkout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: ackJobConfirm })
      }).then(function (res) {
        return res.json().then(function (data) {
          return { ok: res.ok, status: res.status, data: data };
        });
      }).then(function (out) {
        ackJob.busy = false;
        if (!out.ok && out.status !== 202 && out.status !== 200) {
          showAckJobStatus((out.data && out.data.error) || "Checkout failed.");
          if (out.data && out.data.job) {
            ackJob.id = out.data.job.id;
            ackJob.status = out.data.job.status;
            ackJob.last = out.data.job;
          }
          updateApplySourceButton();
          return out;
        }
        var job = out.data && out.data.job;
        if (!job) {
          showAckJobStatus("Checkout had no job.");
          updateApplySourceButton();
          return out;
        }
        ackJob.id = job.id;
        ackJob.status = job.status;
        ackJob.last = job;
        showAckJob(job);
        updateApplySourceButton();
        if (job.status === "applying") {
          return startAckJobPoll(job.id).then(function () {
            return out;
          });
        }
        return refreshAck().then(function () {
          return out;
        });
      }).catch(function () {
        ackJob.busy = false;
        showAckJobStatus("Acked checkout needs overview serve with pimdir.");
        updateApplySourceButton();
        return { ok: false, data: { error: "Acked checkout needs overview serve with pimdir." } };
      });
    }

    function applyAtSource() {
      if (applySourceState().disabled) {
        return;
      }
      applySession = true;
      var doAck = ackHasWork(ackCountNow());
      var doCart = cartHasWork(cartCountNow());
      applyWantedAck = doAck;
      applyWantedCart = doCart;
      if (!doAck && !doCart) {
        return;
      }
      var parts = [];
      if (doAck) {
        parts.push("Read as seen");
      }
      if (doCart) {
        parts.push("Trash at source");
      }
      showApplySourceStripe("Applying " + parts.join(" and ") + "…");
      var chain = Promise.resolve();
      if (doAck) {
        chain = chain.then(checkoutSeenAtSource);
      }
      if (doCart) {
        chain = chain.then(function (ackOut) {
          if (doAck && ackOut && !ackOut.ok && ackOut.status !== 202 && ackOut.status !== 200) {
            showApplySourceStripe((ackOut.data && ackOut.data.error) || "Acked checkout failed. Cart was not applied.", "error");
            updateApplySourceButton();
            return ackOut;
          }
          return checkoutTrashAtSource();
        });
      }
      chain.then(function (last) {
        if (last && !last.ok && last.status !== 202 && last.status !== 200) {
          showApplySourceStripe((last.data && last.data.error) || "Apply at source failed.", "error");
          updateApplySourceButton();
          return;
        }
        maybePaintApplyStripe();
        updateApplySourceButton();
      }).catch(function () {
        showApplySourceStripe("Apply at source needs overview serve with pimdir.", "error");
        updateApplySourceButton();
      });
    }

    if (applySourceBtn) {
      applySourceBtn.addEventListener("click", applyAtSource);
    }
    var stageApplyBtn = document.getElementById("stage-apply");
    if (stageApplyBtn) {
      stageApplyBtn.addEventListener("click", applyAtSource);
    }

    var ackList = document.getElementById("ack-list");
    if (ackList) {
      ackList.addEventListener("click", function (ev) {
        var btn = ev.target.closest && ev.target.closest("button[data-hash]");
        if (!btn) {
          return;
        }
        ev.preventDefault();
        fetch("/ack?hash=" + encodeURIComponent(btn.getAttribute("data-hash")), { method: "DELETE" }).then(function (res) {
          return res.json();
        }).then(function (data) {
          renderAck(data);
          updateAckJobButtons(ackJob.last, (data && data.count) || 0);
        }).catch(function () {});
      });
    }

    if (outliersList) {
      outliersList.addEventListener("click", function (ev) {
        var card = ev.target && ev.target.closest ? ev.target.closest(".outlier-card") : null;
        if (!card || !outliersList.contains(card)) {
          return;
        }
        ev.preventDefault();
        ev.stopPropagation();
        openOutlierCard(card);
      });
      outliersList.addEventListener("keydown", function (ev) {
        if (ev.key !== "Enter" && ev.key !== " ") {
          return;
        }
        var card = ev.target && ev.target.closest ? ev.target.closest(".outlier-card") : null;
        if (!card) {
          return;
        }
        ev.preventDefault();
        openOutlierCard(card);
      });
    }

    if (markBtn) {
      markBtn.addEventListener("click", function () {
        postCart(lastItems, { fromPanel: true });
      });
    }
    if (ackBtn) {
      ackBtn.addEventListener("click", function () {
        var items = lastItems;
        if (previewHash) {
          items = [{
            object_hash: previewHash,
            sender: (lastItems[0] && lastItems[0].sender) || "",
            subject: (lastItems[0] && lastItems[0].subject) || "",
            date: (lastItems[0] && lastItems[0].date) || ""
          }];
        }
        postAck(items, ackBtn, { fromPanel: true });
      });
    }
    if (prevBtn) {
      prevBtn.addEventListener("click", function () {
        stepPreview(-1);
      });
    }
    if (nextBtn) {
      nextBtn.addEventListener("click", function () {
        stepPreview(1);
      });
    }
    refreshCart();
    refreshAck();
    var storedSide = readNav().side;
    if (storedSide === "cart" || storedSide === "trash") {
      setCartOpen(true);
    } else if (storedSide === "ack" || storedSide === "read") {
      setAckOpen(true);
    }
    refreshMailSync();
    startHostEvents();
    if (mailSyncBtn) {
      mailSyncBtn.addEventListener("click", startMailSync);
    }

    function onKindTreeClick(ev) {
      if (!ev.target || !ev.target.closest) {
        return;
      }
      if (!ev.target.closest(".kind-tree") || ev.target.closest(".kind-tree summary")) {
        return;
      }
      var krow = kindRowOf(ev.target);
      if (!krow || !krow.closest(".kleaf")) {
        return;
      }
      askIntent(readableText(krow), itemsFromKindRow(krow));
    }

    function onActionTableClick(ev) {
      if (!ev.target || !ev.target.closest) {
        return;
      }
      var ack = ev.target.closest(".ack-mark");
      if (ack) {
        ev.preventDefault();
        ev.stopPropagation();
        var row = ack.closest("table.action-table tbody tr[data-hashes]");
        if (row) {
          postAck(itemsFromEl(row), ack);
        }
        return;
      }
      var tr = ev.target.closest("table.action-table tbody tr");
      if (!tr) {
        return;
      }
      openActionRow(tr);
    }

    function onFluffSenderClick(ev) {
      if (!ev.target || !ev.target.closest) {
        return;
      }
      var tr = ev.target.closest("table.fluff-senders tbody tr[data-hashes]");
      if (!tr) {
        return;
      }
      askIntent(readableText(tr), itemsFromEl(tr), "Sender");
    }

    function onDeleteListClick(ev) {
      if (!ev.target || !ev.target.closest) {
        return;
      }
      var btn = ev.target.closest(".delete-expand");
      if (btn) {
        ev.preventDefault();
        ev.stopPropagation();
        toggleDeleteEntries(btn);
        return;
      }
      var ack = ev.target.closest(".ack-mark");
      if (ack) {
        ev.preventDefault();
        ev.stopPropagation();
        var ackEntry = ack.closest("tr.delete-entry[data-hashes]");
        if (ackEntry) {
          stageAckParent(ackEntry);
        }
        return;
      }
      var mark = ev.target.closest(".delete-mark");
      if (mark) {
        ev.preventDefault();
        ev.stopPropagation();
        var parent = mark.closest("tr.delete-parent[data-hashes]");
        if (parent) {
          stageDeleteParent(parent);
        }
        return;
      }
      var entry = ev.target.closest("tr.delete-entry[data-hashes]");
      if (entry) {
        openActionRow(entry);
      }
    }

    window.addEventListener("click", onKindTreeClick, true);
    window.addEventListener("click", onActionTableClick, true);
    window.addEventListener("click", onFluffSenderClick, true);
    window.addEventListener("click", onDeleteListClick, true);
    window.addEventListener("click", onOutsideIntent);
    window.addEventListener("keydown", function (ev) {
      if (panel.hidden) {
        return;
      }
      if (ev.key === "Escape") {
        dismissPanel();
        return;
      }
      var tag = ev.target && ev.target.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
        return;
      }
      if (ev.key === "ArrowLeft") {
        ev.preventDefault();
        stepPreview(-1);
        return;
      }
      if (ev.key === "ArrowRight") {
        ev.preventDefault();
        stepPreview(1);
      }
    });
  }
})();
