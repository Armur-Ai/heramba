/* Heramba active-defense console — SPA logic.
   No framework, no external assets: polls the local JSON API and renders.
   Charts come from vendor/charts.js (window.HC). */
(function () {
  "use strict";

  var POLL_MS = 2000;
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
    paused: false,
    mode: "",
    lastSeen: {}, // event signature -> true, to flag fresh rows
    online: false,
  };

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
      return "<tr><td class='src'>" + esc(r.source) + "</td>" +
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

  // ---- polling ----
  function setOnline(ok) {
    state.online = ok;
    var live = document.querySelector(".live");
    var lbl = $("live-label");
    if (ok) { live.classList.remove("stale"); lbl.textContent = "LIVE"; }
    else { live.classList.add("stale"); lbl.textContent = "RECONNECT"; }
  }

  function poll() {
    Promise.all([
      fetchJSON("api/metrics"),
      fetchJSON("api/events?n=120"),
      fetchJSON("api/timeline?window=" + encodeURIComponent(state.window)),
      fetchJSON("api/sources"),
      fetchJSON("api/campaigns"),
      fetchJSON("api/decoys"),
    ]).then(function (res) {
      renderMetrics(res[0]);
      renderFeed(res[1]);
      setNum($("kpi-swarms"), res[4].filter(function (c) { return c.coordinated; }).length);
      renderTimeline(res[2]);
      renderSources(res[3]);
      renderSwarms(res[4]);
      renderDecoys(res[5]);
      setOnline(true);
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

  function start() {
    wireControls();
    poll();
    setInterval(poll, POLL_MS);
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
})();
