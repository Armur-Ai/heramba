/*!
 * heramba-charts.js — a tiny dependency-free SVG charting micro-library.
 * Vendored for offline / air-gapped use. No CDN, no build step, no runtime deps.
 *
 * The MIT License (MIT)
 * Copyright (c) 2026 Armur AI — Heramba
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
(function (global) {
  "use strict";

  var NS = "http://www.w3.org/2000/svg";

  function el(name, attrs) {
    var node = document.createElementNS(NS, name);
    if (attrs) {
      for (var k in attrs) {
        if (attrs[k] != null) node.setAttribute(k, attrs[k]);
      }
    }
    return node;
  }

  function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
  }

  function svgRoot(host, w, h) {
    clear(host);
    var s = el("svg", {
      viewBox: "0 0 " + w + " " + h,
      preserveAspectRatio: "none",
      width: "100%",
      height: "100%",
      role: "img",
    });
    s.style.display = "block";
    s.style.overflow = "visible";
    host.appendChild(s);
    return s;
  }

  // smoothPath builds a Catmull-Rom-ish smoothed polyline through points.
  function smoothPath(pts) {
    if (!pts.length) return "";
    if (pts.length < 3) {
      return pts
        .map(function (p, i) {
          return (i ? "L" : "M") + p[0] + " " + p[1];
        })
        .join(" ");
    }
    var d = "M" + pts[0][0] + " " + pts[0][1];
    for (var i = 0; i < pts.length - 1; i++) {
      var p0 = pts[i === 0 ? 0 : i - 1];
      var p1 = pts[i];
      var p2 = pts[i + 1];
      var p3 = pts[i + 2 < pts.length ? i + 2 : i + 1];
      var cp1x = p1[0] + (p2[0] - p0[0]) / 6;
      var cp1y = p1[1] + (p2[1] - p0[1]) / 6;
      var cp2x = p2[0] - (p3[0] - p1[0]) / 6;
      var cp2y = p2[1] - (p3[1] - p1[1]) / 6;
      d += " C" + cp1x + " " + cp1y + " " + cp2x + " " + cp2y + " " + p2[0] + " " + p2[1];
    }
    return d;
  }

  // areaChart renders a smoothed area+line over an array of numeric values.
  // opts: { values:[n], color, fill, height, width, max }
  function areaChart(host, opts) {
    opts = opts || {};
    var vals = opts.values || [];
    var w = opts.width || 600;
    var h = opts.height || 160;
    var padT = 10, padB = 16, padL = 2, padR = 2;
    var color = opts.color || "#3ddc97";
    var s = svgRoot(host, w, h);
    if (!vals.length) return s;

    var max = opts.max != null ? opts.max : Math.max.apply(null, vals);
    if (max <= 0) max = 1;
    var innerW = w - padL - padR;
    var innerH = h - padT - padB;
    var step = vals.length > 1 ? innerW / (vals.length - 1) : 0;
    var pts = vals.map(function (v, i) {
      var x = padL + i * step;
      var y = padT + innerH - (Math.max(0, v) / max) * innerH;
      return [x, y];
    });

    // gridlines
    for (var g = 0; g <= 2; g++) {
      var gy = padT + (innerH / 2) * g;
      s.appendChild(el("line", { x1: padL, y1: gy, x2: w - padR, y2: gy, class: "hc-grid" }));
    }

    var gradId = "hc-grad-" + Math.random().toString(36).slice(2, 8);
    var defs = el("defs");
    var lg = el("linearGradient", { id: gradId, x1: 0, y1: 0, x2: 0, y2: 1 });
    lg.appendChild(el("stop", { offset: "0%", "stop-color": color, "stop-opacity": "0.35" }));
    lg.appendChild(el("stop", { offset: "100%", "stop-color": color, "stop-opacity": "0" }));
    defs.appendChild(lg);
    s.appendChild(defs);

    var line = smoothPath(pts);
    var area = line + " L" + pts[pts.length - 1][0] + " " + (padT + innerH) + " L" + pts[0][0] + " " + (padT + innerH) + " Z";
    s.appendChild(el("path", { d: area, fill: "url(#" + gradId + ")", stroke: "none" }));
    s.appendChild(el("path", { d: line, fill: "none", stroke: color, "stroke-width": 2, "stroke-linejoin": "round", "stroke-linecap": "round" }));

    // last point marker
    var last = pts[pts.length - 1];
    s.appendChild(el("circle", { cx: last[0], cy: last[1], r: 3, fill: color }));
    return s;
  }

  // donut renders a ring of proportional segments.
  // opts: { segments:[{label,value,color}], size, thickness }
  function donut(host, opts) {
    opts = opts || {};
    var segs = (opts.segments || []).filter(function (x) { return x.value > 0; });
    var size = opts.size || 180;
    var th = opts.thickness || 22;
    var s = svgRoot(host, size, size);
    var cx = size / 2, cy = size / 2;
    var r = (size - th) / 2 - 2;
    var total = segs.reduce(function (a, x) { return a + x.value; }, 0);
    if (total <= 0) {
      s.appendChild(el("circle", { cx: cx, cy: cy, r: r, fill: "none", stroke: "#23262f", "stroke-width": th }));
      return s;
    }
    var circ = 2 * Math.PI * r;
    var offset = 0;
    segs.forEach(function (seg) {
      var frac = seg.value / total;
      var len = frac * circ;
      var c = el("circle", {
        cx: cx, cy: cy, r: r, fill: "none",
        stroke: seg.color, "stroke-width": th,
        "stroke-dasharray": len + " " + (circ - len),
        "stroke-dashoffset": -offset,
        transform: "rotate(-90 " + cx + " " + cy + ")",
        "stroke-linecap": "butt",
      });
      c.appendChild(titleEl(seg.label + ": " + seg.value));
      s.appendChild(c);
      offset += len;
    });
    return s;
  }

  // bars renders horizontal proportional bars with labels baked by the caller.
  // opts: { items:[{value,color}], width, barH, gap, max }
  function bars(host, opts) {
    opts = opts || {};
    var items = opts.items || [];
    var w = opts.width || 300;
    var barH = opts.barH || 10;
    var gap = opts.gap || 6;
    var h = Math.max(1, items.length) * (barH + gap);
    var s = svgRoot(host, w, h);
    var max = opts.max != null ? opts.max : Math.max.apply(null, items.map(function (x) { return x.value; }).concat([1]));
    if (max <= 0) max = 1;
    items.forEach(function (it, i) {
      var y = i * (barH + gap);
      s.appendChild(el("rect", { x: 0, y: y, width: w, height: barH, rx: 3, class: "hc-track" }));
      var bw = Math.max(0, (it.value / max) * w);
      var r = el("rect", { x: 0, y: y, width: bw, height: barH, rx: 3, fill: it.color });
      r.appendChild(titleEl(String(it.value)));
      s.appendChild(r);
    });
    return s;
  }

  // gauge renders a semicircular threat gauge 0..1 (or 0..max).
  // opts: { value, max, size, color, track }
  function gauge(host, opts) {
    opts = opts || {};
    var size = opts.size || 200;
    var h = size * 0.62;
    var s = svgRoot(host, size, h);
    var cx = size / 2, cy = h - 6, r = size / 2 - 14, th = 14;
    var max = opts.max || 1;
    var v = Math.max(0, Math.min(1, (opts.value || 0) / max));
    function arc(frac, color, width) {
      var a0 = Math.PI, a1 = Math.PI + frac * Math.PI;
      var x0 = cx + r * Math.cos(a0), y0 = cy + r * Math.sin(a0);
      var x1 = cx + r * Math.cos(a1), y1 = cy + r * Math.sin(a1);
      var large = frac > 0.5 ? 1 : 0;
      return el("path", {
        d: "M" + x0 + " " + y0 + " A" + r + " " + r + " 0 " + large + " 1 " + x1 + " " + y1,
        fill: "none", stroke: color, "stroke-width": width, "stroke-linecap": "round",
      });
    }
    s.appendChild(arc(1, "#23262f", th));
    s.appendChild(arc(v, opts.color || "#3ddc97", th));
    return s;
  }

  // histogram renders a vertical-bar histogram from bucket counts.
  // opts: { counts:[n], labels:[s], color, width, height }
  function histogram(host, opts) {
    opts = opts || {};
    var counts = opts.counts || [];
    var w = opts.width || 320;
    var h = opts.height || 130;
    var padB = 16, padT = 8;
    var s = svgRoot(host, w, h);
    if (!counts.length) return s;
    var max = Math.max.apply(null, counts.concat([1]));
    var n = counts.length;
    var gap = 3;
    var bw = (w - gap * (n - 1)) / n;
    var innerH = h - padB - padT;
    counts.forEach(function (c, i) {
      var bh = (c / max) * innerH;
      var x = i * (bw + gap);
      var y = padT + innerH - bh;
      var rect = el("rect", { x: x, y: y, width: bw, height: Math.max(0, bh), rx: 2, fill: opts.color || "#a78bfa" });
      if (opts.labels && opts.labels[i] != null) rect.appendChild(titleEl(opts.labels[i] + ": " + c));
      s.appendChild(rect);
    });
    return s;
  }

  function titleEl(text) {
    var t = el("title");
    t.textContent = text;
    return t;
  }

  global.HC = {
    areaChart: areaChart,
    donut: donut,
    bars: bars,
    gauge: gauge,
    histogram: histogram,
  };
})(window);
