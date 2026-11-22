/* Heramba active-defense console — SPA logic.
   No framework, no external assets: polls the local JSON API and renders.
   Charts come from vendor/charts.js (window.HC). */
(function () {
  "use strict";

  var DEFAULT_POLL_MS = 2000;
  var LS_KEY = "heramba.settings.v1";
  var SEV = ["critical", "high", "medium", "low", "info"];
  var SEV_COLOR = {
    critical: "#ff6b6b", high: "#f5a524", medium: "#a78bfa", low: "#57c7ff", info: "#868d9c",
  };
  var KIND_COLOR = {
    decoy_hit: "#3ddc97", canary_triggered: "#ff6b6b", agentic_traffic: "#a78bfa",
    egress_blocked: "#57c7ff", rate_limited: "#f5a524", counter_deployed: "#6b7280",
    probe: "#868d9c",
  };

  var state = {
    window: "15m",
    token: "",
    paused: false,     // pipeline pause (control)
    mode: "",
    lastSeen: {},      // event signature -> true, to flag fresh rows
    online: false,
    // v2 UI prefs (persisted)
    refreshMs: DEFAULT_POLL_MS,
    autoPaused: false, // pause auto-refresh (UI)
    compact: false,
    sseEnabled: true,
    theme: "noir",
    // runtime
    es: null,          // EventSource
    sseLive: false,
    timer: null,
    drillSource: "",
  };

  // ---- persisted settings (localStorage, best-effort) ----
  function loadPrefs() {
    try {
      var raw = localStorage.getItem(LS_KEY);
      if (!raw) return;
      var p = JSON.parse(raw);
      if (p && typeof p === "object") {
        if (p.refreshMs) state.refreshMs = p.refreshMs;
        if (typeof p.compact === "boolean") state.compact = p.compact;
        if (typeof p.sseEnabled === "boolean") state.sseEnabled = p.sseEnabled;
        if (p.theme === "noir" || p.theme === "dim") state.theme = p.theme;
      }
    } catch (e) { /* private mode / disabled storage: use defaults */ }
  }
  function savePrefs() {
    try {
      localStorage.setItem(LS_KEY, JSON.stringify({
        refreshMs: state.refreshMs, compact: state.compact,
        sseEnabled: state.sseEnabled, theme: state.theme,
      }));
    } catch (e) { /* ignore */ }
  }

  // ---- helpers ----
  function $(id) { return document.getElementById(id); }
  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }
  function kindColor(k) { return KIND_COLOR[k] || "#6b7280"; }

  function relTime(iso) {
    var t = new Date(iso).getTime();
    if (isNaN(t)) return "";
    var s = Math.max(0, (Date.now() - t) / 1000);
    if (s < 2) return "now";
    if (s < 60) return Math.floor(s) + "s";
    if (s < 3600) return Math.floor(s / 60) + "m";
    if (s < 86400) return Math.floor(s / 3600) + "h";
    return Math.floor(s / 86400) + "d";
  }
  function clock(iso) {
    try { return new Date(iso).toLocaleTimeString([], { hour12: false }); }
    catch (e) { return esc(iso); }
  }

  // animate an integer from its current displayed value to target.
  function setNum(node, target) {
    if (!node) return;
    var from = parseInt(node.getAttribute("data-v") || "0", 10) || 0;
    if (from === target) { node.textContent = fmt(target); return; }
    node.setAttribute("data-v", String(target));
    var start = performance.now(), dur = 450;
    function step(now) {
      var p = Math.min(1, (now - start) / dur);
      var e = 1 - Math.pow(1 - p, 3);
      node.textContent = fmt(Math.round(from + (target - from) * e));
      if (p < 1) requestAnimationFrame(step);
    }
    requestAnimationFrame(step);
  }
  function fmt(n) { return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ","); }

  function fetchJSON(url, opts) {
    return fetch(url, opts).then(function (r) {
      if (!r.ok) throw { status: r.status };
      return r.json();
    });
  }
  function authHeaders() {
    var h = { "Content-Type": "application/json" };
    if (state.token) h["Authorization"] = "Bearer " + state.token;
    return h;
  }

  // ---- renderers ----
  function renderMetrics(m) {
    state.mode = m.mode || "";
    setNum($("kpi-total"), m.total || 0);
    setNum($("stat-sources"), m.sources || 0);
    setNum($("kpi-sources"), m.sources || 0);

    var bySev = m.by_severity || {};
    var crit = (bySev.critical || 0) + (bySev.high || 0);
    setNum($("kpi-crit"), crit);

    // mode pill
    var pill = $("mode-pill");
    pill.className = "pill mode-" + (state.paused ? "paused" : (m.mode || ""));
    $("mode-label").textContent = state.paused ? "PAUSED" : (m.mode ? m.mode.toUpperCase() : "—");

    // uptime
    $("uptime").textContent = fmtDur(m.uptime_seconds || 0);

    renderDonut(m.by_kind || {});
    renderSevBars(bySev, m.total || 0);
    renderHistogram(m.score || {});
    reflectModeSeg(m.mode);

    // threat index
    var threat = threatIndex(bySev, m.total || 0);
    renderGauge(threat);
    setNum($("nav-threat"), threat);
  }

  function fmtDur(s) {
    s = Math.floor(s);
    var d = Math.floor(s / 86400); s -= d * 86400;
    var h = Math.floor(s / 3600); s -= h * 3600;
    var m = Math.floor(s / 60); s -= m * 60;
    if (d) return d + "d " + h + "h";
    if (h) return h + "h " + m + "m";
    if (m) return m + "m " + s + "s";
    return s + "s";
  }

  function threatIndex(bySev, total) {
    if (!total) return 0;
    var w = { critical: 1, high: 0.7, medium: 0.4, low: 0.2, info: 0.05 };
    var num = 0, denom = 0;
    SEV.forEach(function (s) { num += (bySev[s] || 0) * w[s]; denom += (bySev[s] || 0); });
    if (!denom) return 0;
    var base = (num / denom) * 100;
    // volume pressure: more events nudges it up, saturating.
    var vol = Math.min(1, total / 300) * 15;
    return Math.min(100, Math.round(base + vol));
  }

  function tier(v) {
    if (v >= 80) return ["CRITICAL", "#ff6b6b"];
    if (v >= 60) return ["ELEVATED", "#f5a524"];
    if (v >= 35) return ["GUARDED", "#a78bfa"];
    if (v >= 15) return ["LOW", "#57c7ff"];
    return ["NOMINAL", "#3ddc97"];
  }

  function renderGauge(v) {
    var t = tier(v);
    HC.gauge($("gauge"), { value: v, max: 100, color: t[1], size: 230 });
    setNum($("gauge-num"), v);
    var tierEl = $("gauge-tier");
    tierEl.textContent = t[0];
    tierEl.style.color = t[1];
  }

  function renderDonut(byKind) {
    var entries = Object.keys(byKind).map(function (k) {
      return { label: k, value: byKind[k], color: kindColor(k) };
    }).filter(function (x) { return x.value > 0; })
      .sort(function (a, b) { return b.value - a.value; });
    HC.donut($("donut"), { segments: entries, size: 150, thickness: 22 });
    var ul = $("kind-legend");
    ul.innerHTML = entries.map(function (e) {
      return '<li><span class="sw" style="background:' + e.color + '"></span>' +
        '<span class="lk">' + esc(e.label) + '</span>' +
        '<span class="lv tnum">' + e.value + "</span></li>";
    }).join("") || '<li class="muted">no data</li>';
  }

  function renderSevBars(bySev, total) {
    var max = 0;
    SEV.forEach(function (s) { max = Math.max(max, bySev[s] || 0); });
    max = max || 1;
    var host = $("sev-bars");
    host.innerHTML = SEV.map(function (s) {
      var v = bySev[s] || 0;
      var pct = (v / max) * 100;
      return '<div class="sev-bar"><span class="lbl sev-' + s + '">' + s + "</span>" +
        '<span class="track"><span class="fill" style="width:' + pct + "%;background:" + SEV_COLOR[s] + '"></span></span>' +
        '<span class="val">' + v + "</span></div>";
    }).join("");
  }

  function renderHistogram(score) {
    var bounds = score.bounds || [];
    var cum = score.cum_counts || [];
    // de-cumulate into per-bucket counts.
    var counts = [], labels = [];
    for (var i = 0; i < cum.length; i++) {
      counts.push(cum[i] - (i > 0 ? cum[i - 1] : 0));
      var lo = i === 0 ? 0 : bounds[i - 1];
      var hi = i < bounds.length ? bounds[i] : "+";
      labels.push("[" + lo + "," + hi + ")");
    }
    HC.histogram($("histogram"), { counts: counts, labels: labels, color: "#a78bfa", height: 140 });
    $("hist-count").textContent = (score.count || 0) + " samples";
  }

  function renderTimeline(tl) {
    var pts = (tl.points || []).map(function (p) { return p.total; });
    HC.areaChart($("timeline"), { values: pts, color: "#3ddc97", height: 180, width: 700 });
  }

  function renderFeed(events) {
    var tb = $("feed-rows");
    var empty = $("feed-empty");
    if (!events.length) { tb.innerHTML = ""; empty.hidden = false; $("feed-count").textContent = ""; return; }
    empty.hidden = true;
    $("feed-count").textContent = events.length + " shown";
    var seen = {};
    tb.innerHTML = events.slice(0, 120).map(function (e) {
      var sev = e.severity_label || "info";
      var sig = e.time + "|" + e.kind + "|" + (e.source || "") + "|" + (e.target || "");
      seen[sig] = true;
      var fresh = state.online && !state.lastSeen[sig] ? " fresh" : "";
      return '<tr class="' + "" + fresh + '">' +
        '<td class="t" title="' + esc(clock(e.time)) + '">' + esc(relTime(e.time)) + "</td>" +
        '<td><span class="sevdot sev-' + sev + '"></span><span class="sev sev-' + sev + '">' + esc(sev) + "</span></td>" +
        '<td><span class="kind-tag" style="color:' + kindColor(e.kind) + '">' + esc(e.kind) + "</span></td>" +
        '<td class="src">' + esc(e.source || "—") + "</td>" +
        '<td><span class="tgt" title="' + esc(e.target || "") + '">' + esc(e.target || "—") + "</span></td>" +
        '<td class="col-detail">' + esc(e.detail || "") + "</td></tr>";
    }).join("");
    state.lastSeen = seen;
  }

  function renderSources(rows) {
    var tb = $("sources-rows");
    $("sources-empty").hidden = rows.length > 0;
    tb.innerHTML = rows.slice(0, 30).map(function (r) {
      var kinds = Object.keys(r.kinds || {}).sort(function (a, b) { return r.kinds[b] - r.kinds[a]; }).slice(0, 3);
      var ktags = kinds.map(function (k) { return '<span class="kind-tag" style="color:' + kindColor(k) + '">' + esc(k) + "</span>"; }).join(" ");
      var sev = r.max_severity || "info";
      return "<tr class='clickable' tabindex='0' role='button' data-source='" + esc(r.source) +
        "' aria-label='Drill into " + esc(r.source) + "'><td class='src'>" + esc(r.source) + "</td>" +
        "<td class='num tnum'>" + r.count + "</td>" +
        "<td><span class='sev sev-" + sev + "'>" + esc(sev) + "</span></td>" +
        "<td>" + ktags + "</td>" +
        "<td class='num tnum'>" + (r.max_confidence != null ? r.max_confidence.toFixed(2) : "—") + "</td></tr>";
    }).join("");
  }

  function renderSwarms(camps) {
    var host = $("swarm-list");
    $("swarms-empty").hidden = camps.length > 0;
    host.innerHTML = camps.slice(0, 20).map(function (c) {
      var ips = (c.sources || []).slice(0, 8).map(function (ip) { return '<span class="swarm-ip">' + esc(ip) + "</span>"; }).join("");
      var more = (c.source_count || 0) > 8 ? '<span class="swarm-ip">+' + (c.source_count - 8) + "</span>" : "";
      var badge = c.coordinated
        ? '<span class="swarm-badge coord">⚠ coordinated</span>'
        : '<span class="swarm-badge loose">cluster</span>';
      return '<div class="swarm' + (c.coordinated ? " coord" : "") + '">' +
        '<div class="swarm-top"><span class="swarm-id">' + esc(c.id || c.target || "campaign") + "</span>" + badge + "</div>" +
        '<div class="swarm-meta">' +
        "<span>sources <b>" + (c.source_count || 0) + "</b></span>" +
        "<span>events <b>" + (c.count || 0) + "</b></span>" +
        "<span>target <span class='swarm-tgt'>" + esc(c.target || "—") + "</span></span>" +
        "<span>sev <span class='sev sev-" + (c.max_severity || "info") + "'>" + esc(c.max_severity || "info") + "</span></span>" +
        "</div>" +
        '<div class="swarm-ips">' + ips + more + "</div></div>";
    }).join("");
  }

  function renderDecoys(rows) {
    var host = $("decoy-map");
    $("decoys-empty").hidden = rows.length > 0;
    var max = rows.reduce(function (a, r) { return Math.max(a, r.count); }, 1);
    host.innerHTML = rows.slice(0, 24).map(function (r) {
      var heat = Math.min(0.5, (r.count / max) * 0.5);
      var sev = r.max_severity || "info";
      return '<div class="decoy">' +
        '<div class="heat" style="background:linear-gradient(135deg, rgba(255,107,107,' + heat + '), transparent)"></div>' +
        '<div class="dtgt" title="' + esc(r.target) + '">' + esc(r.target) + "</div>" +
        '<div class="dcount tnum">' + r.count + "</div>" +
        '<div class="dmeta"><span>' + (r.source_count || 0) + " src</span><span class='sev sev-" + sev + "'>" + esc(sev) + "</span></div>" +
        "</div>";
    }).join("");
  }

  function renderBreaches(b) {
    b = b || {};
    var count = b.count || 0;
    setNum($("kpi-breaches"), count);
    var card = $("kpi-breach-card");
    if (card) card.classList.toggle("kpi-alert", count > 0);
    var foot = $("kpi-breaches-foot");
    if (foot) {
      foot.textContent = count > 0
        ? (b.sources || 0) + " source" + ((b.sources || 0) === 1 ? "" : "s") + " · canary triggers"
        : "canary triggers";
    }
  }

  function renderTargets(rows) {
    rows = rows || [];
    var host = $("targets-list");
    var empty = $("targets-empty");
    if (empty) empty.hidden = rows.length > 0;
    if (!host) return;
    var max = rows.reduce(function (a, r) { return Math.max(a, r.count || 0); }, 1);
    host.innerHTML = rows.slice(0, 8).map(function (r) {
      var pct = ((r.count || 0) / max) * 100;
      var sev = r.max_severity || "info";
      var color = SEV_COLOR[sev] || "#868d9c";
      return '<div class="tgt-row">' +
        '<span class="tgt-name" title="' + esc(r.target) + '">' + esc(r.target) + "</span>" +
        '<span class="tgt-val">' + (r.count || 0) + "</span>" +
        '<span class="tgt-track"><span class="tgt-fill" style="width:' + pct + "%;background:" + color + '"></span></span>' +
        "</div>";
    }).join("");
  }

  // renderSevTime draws the stacked severity-over-time chart from the timeline
  // buckets' per-severity counts, plus a small colour legend.
  function renderSevTime(tl) {
    var pts = (tl && tl.points) || [];
    var series = SEV.map(function (s) {
      return {
        color: SEV_COLOR[s], label: s,
        values: pts.map(function (p) { return (p.by_severity || {})[s] || 0; }),
      };
    }).filter(function (ser) {
      return ser.values.some(function (v) { return v > 0; });
    });
    HC.stackedArea($("sevtime"), { series: series, height: 180, width: 700 });
    var legend = $("sevtime-legend");
    if (legend) {
      legend.innerHTML = series.map(function (ser) {
        return '<span><i style="background:' + ser.color + '"></i>' + esc(ser.label) + "</span>";
      }).join("") || '<span class="muted">no data</span>';
    }
  }

  // ---- polling ----
  function updateLiveLabel() {
    var live = document.querySelector(".live");
    var lbl = $("live-label");
    if (!live || !lbl) return;
    if (state.autoPaused) { live.classList.add("stale"); lbl.textContent = "PAUSED"; return; }
    if (!state.online) { live.classList.add("stale"); lbl.textContent = "RECONNECT"; return; }
    live.classList.remove("stale");
    lbl.textContent = state.sseLive ? "LIVE" : "POLL";
  }
  function setOnline(ok) {
    state.online = ok;
    updateLiveLabel();
  }

  function poll() {
    if (state.autoPaused) return;
    Promise.all([
      fetchJSON("api/metrics"),
      fetchJSON("api/events?n=120"),
      fetchJSON("api/timeline?window=" + encodeURIComponent(state.window)),
      fetchJSON("api/sources"),
      fetchJSON("api/campaigns"),
      fetchJSON("api/decoys"),
      fetchJSON("api/breaches"),
    ]).then(function (res) {
      renderMetrics(res[0]);
      renderFeed(res[1]);
      setNum($("kpi-swarms"), res[4].filter(function (c) { return c.coordinated; }).length);
      renderTimeline(res[2]);
      renderSevTime(res[2]);
      renderSources(res[3]);
      renderSwarms(res[4]);
      renderDecoys(res[5]);
      renderTargets(res[5]);
      renderBreaches(res[6]);
      setOnline(true);
      // if the drill drawer is open, refresh its contents too.
      if (state.drillSource) loadDrill(state.drillSource);
    }).catch(function () {
      setOnline(false);
    });
  }

  // ---- controls ----
  function post(path, body) {
    var st = $("control-status");
    st.className = "control-status";
    st.textContent = "…";
    return fetch(path, { method: "POST", headers: authHeaders(), body: body ? JSON.stringify(body) : "" })
      .then(function (r) {
        return r.json().catch(function () { return {}; }).then(function (j) {
          if (!r.ok) {
            st.className = "control-status err";
            st.textContent = (j.error || ("error " + r.status)) + (r.status === 401 ? " — set a bearer token" : "");
            throw j;
          }
          st.className = "control-status ok";
          st.textContent = "applied";
          return j;
        });
      }).then(function (j) { poll(); return j; })
      .catch(function () { /* already surfaced */ });
  }

  function reflectModeSeg(mode) {
    document.querySelectorAll("#mode-seg button").forEach(function (b) {
      b.classList.toggle("on", b.getAttribute("data-mode") === mode);
    });
  }

  function wireControls() {
    document.querySelectorAll("#mode-seg button").forEach(function (b) {
      b.addEventListener("click", function () { post("api/control/mode", { mode: b.getAttribute("data-mode") }); });
    });
    $("btn-pause").addEventListener("click", function () {
      state.paused = !state.paused;
      this.classList.toggle("on", state.paused);
      this.textContent = state.paused ? "Resume" : "Pause";
      post("api/control/pause", { paused: state.paused });
    });
    $("btn-preset").addEventListener("click", function () {
      post("api/control/decoy-preset", { preset: $("preset-select").value });
    });
    $("btn-clear").addEventListener("click", function () {
      post("api/control/clear", null);
    });
    $("token-input").addEventListener("input", function () { state.token = this.value.trim(); });

    // window segmented control
    document.querySelectorAll("#window-seg button").forEach(function (b) {
      b.addEventListener("click", function () {
        state.window = b.getAttribute("data-window");
        document.querySelectorAll("#window-seg button").forEach(function (x) { x.classList.remove("on"); });
        b.classList.add("on");
        poll();
      });
    });

    // nav active state follows clicks
    document.querySelectorAll(".nav-item").forEach(function (a) {
      a.addEventListener("click", function () {
        document.querySelectorAll(".nav-item").forEach(function (x) { x.classList.remove("active"); });
        a.classList.add("active");
      });
    });
  }

  // ---- live streaming (SSE) with polling fallback ----
  function connectSSE() {
    if (!state.sseEnabled || state.autoPaused) return;
    if (typeof EventSource === "undefined") return; // graceful: polling covers it
    disconnectSSE();
    var es;
    try { es = new EventSource("api/stream"); }
    catch (e) { return; }
    state.es = es;
    es.onopen = function () { state.sseLive = true; setOnline(true); };
    es.onmessage = function (ev) {
      var frame;
      try { frame = JSON.parse(ev.data); } catch (e) { return; }
      if (!frame) return;
      if (frame.type === "event" && frame.event) {
        prependFeedRow(frame.event);
      } else if (frame.type === "metrics" && frame.metrics) {
        renderMetrics(frame.metrics);
      }
      setOnline(true);
    };
    es.onerror = function () {
      // Drop the stream and lean on polling; the browser may also auto-retry.
      state.sseLive = false;
      updateLiveLabel();
    };
  }
  function disconnectSSE() {
    if (state.es) { try { state.es.close(); } catch (e) {} state.es = null; }
    state.sseLive = false;
  }

  // prependFeedRow inserts a freshly-streamed event at the top of the feed with
  // a flash, so the UI updates the instant an event arrives (between polls).
  function prependFeedRow(e) {
    var tb = $("feed-rows");
    if (!tb) return;
    var empty = $("feed-empty");
    if (empty) empty.hidden = true;
    var sev = e.severity_label || "info";
    var tr = document.createElement("tr");
    tr.className = "fresh";
    tr.innerHTML =
      '<td class="t" title="' + esc(clock(e.time)) + '">' + esc(relTime(e.time)) + "</td>" +
      '<td><span class="sevdot sev-' + sev + '"></span><span class="sev sev-' + sev + '">' + esc(sev) + "</span></td>" +
      '<td><span class="kind-tag" style="color:' + kindColor(e.kind) + '">' + esc(e.kind) + "</span></td>" +
      '<td class="src">' + esc(e.source || "—") + "</td>" +
      '<td><span class="tgt" title="' + esc(e.target || "") + '">' + esc(e.target || "—") + "</span></td>" +
      '<td class="col-detail">' + esc(e.detail || "") + "</td>";
    tb.insertBefore(tr, tb.firstChild);
    // trim to keep the DOM bounded; the next poll re-syncs authoritatively.
    while (tb.children.length > 120) tb.removeChild(tb.lastChild);
  }

  // ---- managed refresh timer ----
  function scheduleTimer() {
    if (state.timer) { clearInterval(state.timer); state.timer = null; }
    if (state.autoPaused) return;
    state.timer = setInterval(poll, state.refreshMs);
  }

  // ---- theme + density ----
  function applyTheme() {
    document.documentElement.setAttribute("data-theme", state.theme);
    document.querySelectorAll("#theme-seg button").forEach(function (b) {
      b.classList.toggle("on", b.getAttribute("data-theme") === state.theme);
    });
  }
  function applyCompact() {
    var app = document.getElementById("app");
    if (app) app.classList.toggle("compact", state.compact);
  }

  // ---- source drill drawer ----
  function openDrill(source) {
    state.drillSource = source;
    $("drill-source").textContent = source;
    toggleDrawer("drill", true);
    loadDrill(source);
  }
  function loadDrill(source) {
    fetchJSON("api/sources/" + encodeURIComponent(source)).then(function (d) {
      renderDrill(d);
    }).catch(function () {
      $("drill-body").innerHTML = '<div class="empty">Could not load source detail.</div>';
    });
  }
  function renderDrill(d) {
    var body = $("drill-body");
    if (!body) return;
    var evs = d.events || [];
    var kinds = Object.keys(d.kinds || {}).sort(function (a, b) { return d.kinds[b] - d.kinds[a]; });
    var ktags = kinds.map(function (k) {
      return '<span class="kind-tag" style="color:' + kindColor(k) + '">' + esc(k) + " " + d.kinds[k] + "</span>";
    }).join(" ");
    var sev = d.max_severity || "info";
    var head =
      '<div class="swarm-meta" style="margin-bottom:12px">' +
      "<span>events <b>" + (d.count || 0) + "</b></span>" +
      "<span>targets <b>" + Object.keys(d.targets || {}).length + "</b></span>" +
      "<span>max sev <span class='sev sev-" + sev + "'>" + esc(sev) + "</span></span>" +
      "<span>max conf <b>" + (d.max_confidence != null ? d.max_confidence.toFixed(2) : "—") + "</b></span>" +
      "</div><div style='margin-bottom:12px'>" + (ktags || '<span class="muted">no activity</span>') + "</div>";
    var rows = evs.length
      ? ('<table class="feed"><thead><tr><th>time</th><th>sev</th><th>kind</th><th>target</th></tr></thead><tbody>' +
          evs.slice(0, 80).map(function (e) {
            var s = e.severity_label || "info";
            return "<tr><td class='t' title='" + esc(clock(e.time)) + "'>" + esc(relTime(e.time)) + "</td>" +
              "<td><span class='sev sev-" + s + "'>" + esc(s) + "</span></td>" +
              '<td><span class="kind-tag" style="color:' + kindColor(e.kind) + '">' + esc(e.kind) + "</span></td>" +
              "<td><span class='tgt'>" + esc(e.target || "—") + "</span></td></tr>";
          }).join("") + "</tbody></table>")
      : '<div class="empty rich"><span class="empty-glyph" aria-hidden="true">⌖</span>No events for this source.</div>';
    body.innerHTML = head + rows;
  }

  // ---- drawers (shared open/close) ----
  function toggleDrawer(name, open) {
    var drawer = $(name + "-drawer");
    var overlay = $(name + "-overlay");
    if (!drawer || !overlay) return;
    drawer.classList.toggle("open", open);
    overlay.classList.toggle("open", open);
    drawer.setAttribute("aria-hidden", open ? "false" : "true");
    if (!open && name === "drill") state.drillSource = "";
  }

  function wireSettings() {
    $("btn-settings").addEventListener("click", function () { toggleDrawer("settings", true); });
    $("settings-close").addEventListener("click", function () { toggleDrawer("settings", false); });
    $("settings-overlay").addEventListener("click", function () { toggleDrawer("settings", false); });
    $("drill-close").addEventListener("click", function () { toggleDrawer("drill", false); });
    $("drill-overlay").addEventListener("click", function () { toggleDrawer("drill", false); });
    document.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") { toggleDrawer("settings", false); toggleDrawer("drill", false); }
    });

    // refresh interval
    var rs = $("refresh-select");
    rs.value = String(state.refreshMs);
    rs.addEventListener("change", function () {
      state.refreshMs = parseInt(this.value, 10) || DEFAULT_POLL_MS;
      savePrefs();
      scheduleTimer();
    });

    // pause auto-refresh
    $("pause-refresh").addEventListener("change", function () {
      state.autoPaused = this.checked;
      if (state.autoPaused) { disconnectSSE(); if (state.timer) { clearInterval(state.timer); state.timer = null; } }
      else { scheduleTimer(); connectSSE(); poll(); }
      updateLiveLabel();
    });

    // compact density
    var ct = $("compact-toggle");
    ct.checked = state.compact;
    ct.addEventListener("change", function () {
      state.compact = this.checked;
      applyCompact();
      savePrefs();
    });

    // SSE toggle
    var st = $("sse-toggle");
    st.checked = state.sseEnabled;
    st.addEventListener("change", function () {
      state.sseEnabled = this.checked;
      savePrefs();
      if (state.sseEnabled) connectSSE(); else disconnectSSE();
      updateLiveLabel();
    });

    // theme segmented control (in drawer) + quick toggle button
    document.querySelectorAll("#theme-seg button").forEach(function (b) {
      b.addEventListener("click", function () {
        state.theme = b.getAttribute("data-theme");
        applyTheme(); savePrefs();
      });
    });
    $("btn-theme").addEventListener("click", function () {
      state.theme = state.theme === "noir" ? "dim" : "noir";
      applyTheme(); savePrefs();
    });
  }

  function wireDrill() {
    var tb = $("sources-rows");
    if (!tb) return;
    function handle(target) {
      var row = target.closest ? target.closest("tr.clickable") : null;
      if (row && row.getAttribute("data-source")) openDrill(row.getAttribute("data-source"));
    }
    tb.addEventListener("click", function (ev) { handle(ev.target); });
    tb.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter" || ev.key === " ") {
        var row = ev.target.closest && ev.target.closest("tr.clickable");
        if (row) { ev.preventDefault(); handle(ev.target); }
      }
    });
  }

  function start() {
    loadPrefs();
    applyTheme();
    applyCompact();
    wireControls();
    wireSettings();
    wireDrill();
    poll();
    scheduleTimer();
    connectSSE();
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
})();
