package admin

// dashboardHTML is the minimal, self-contained read-only web view served at
// /dashboard — the browser counterpart to `heramba watch`. It has no framework,
// no build step, and no external assets: a little inline JS polls /api/events
// and /metrics every few seconds and renders a live table plus the totals. Kept
// deliberately small; richer UI is out of scope for the OSS edge.
const dashboardHTML = `<!doctype html>
<html lang=en>
<head>
<meta charset=utf-8>
<meta name=viewport content="width=device-width,initial-scale=1">
<title>Heramba dashboard</title>
<style>
:root{--bg:#fff;--fg:#1a1a1a;--muted:#71717a;--line:#e5e7eb;--chip:#f4f4f5}
@media (prefers-color-scheme:dark){:root{--bg:#0f1115;--fg:#e8e8ea;--muted:#9a9aa5;--line:#262a33;--chip:#1b1f27}}
*{box-sizing:border-box}
body{font:14px/1.5 system-ui,-apple-system,sans-serif;margin:0;background:var(--bg);color:var(--fg)}
.wrap{max-width:960px;margin:0 auto;padding:16px}
h1{font-size:1.2rem;margin:.2rem 0}
.muted{color:var(--muted)}
.tiles{display:flex;gap:10px;flex-wrap:wrap;margin:12px 0}
.tile{background:var(--chip);border:1px solid var(--line);border-radius:8px;padding:10px 14px;min-width:120px}
.tile b{display:block;font-size:1.4rem}
table{border-collapse:collapse;width:100%;margin-top:8px}
th,td{text-align:left;padding:6px 8px;border-bottom:1px solid var(--line);white-space:nowrap}
td.detail{white-space:normal;color:var(--muted)}
.sev{font-weight:600}
.sev-critical{color:#ff6b6b}.sev-high{color:#f5a524}.sev-medium{color:#a78bfa}.sev-low{color:#57c7ff}.sev-info{color:var(--muted)}
code{background:var(--chip);padding:.05rem .3rem;border-radius:4px}
</style>
</head>
<body>
<div class=wrap>
<h1>Heramba — live detections</h1>
<div class=muted id=status>connecting…</div>
<div class=tiles id=tiles></div>
<table><thead><tr><th>time</th><th>sev</th><th>kind</th><th>source</th><th>target</th><th>detail</th></tr></thead>
<tbody id=rows></tbody></table>
<p class=muted>Read-only. Polls <code>/api/events</code> + <code>/metrics</code> every 3s.</p>
</div>
<script>
function esc(s){return String(s==null?'':s).replace(/[&<>"]/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]})}
function t(iso){try{return new Date(iso).toLocaleTimeString()}catch(e){return esc(iso)}}
function parseMetrics(txt){
  var out={total:0,sources:0,kinds:[]};
  txt.split('\n').forEach(function(l){
    if(l.indexOf('#')===0)return;
    var m=l.match(/^heramba_detections_total\s+(\d+)/);if(m)out.total=+m[1];
    var s=l.match(/^heramba_sources_total\s+(\d+)/);if(s)out.sources=+s[1];
    var k=l.match(/^heramba_detections_by_kind_total\{kind="([^"]+)"\}\s+(\d+)/);if(k)out.kinds.push([k[1],+k[2]]);
  });
  out.kinds.sort(function(a,b){return b[1]-a[1]});
  return out;
}
function renderTiles(m){
  var html='<div class=tile><span class=muted>detections</span><b>'+m.total+'</b></div>'+
           '<div class=tile><span class=muted>sources</span><b>'+m.sources+'</b></div>';
  m.kinds.slice(0,3).forEach(function(k){
    html+='<div class=tile><span class=muted>'+esc(k[0])+'</span><b>'+k[1]+'</b></div>';
  });
  document.getElementById('tiles').innerHTML=html;
}
function renderRows(evs){
  var rows=evs.map(function(e){
    var sev=esc(e.severity_label||'info');
    return '<tr><td>'+t(e.time)+'</td>'+
      '<td class="sev sev-'+sev+'">'+sev+'</td>'+
      '<td>'+esc(e.kind)+'</td>'+
      '<td><code>'+esc(e.source||'—')+'</code></td>'+
      '<td><code>'+esc(e.target||'—')+'</code></td>'+
      '<td class=detail>'+esc(e.detail||'')+'</td></tr>';
  }).join('');
  document.getElementById('rows').innerHTML=rows||'<tr><td colspan=6 class=muted>no detections yet</td></tr>';
}
function tick(){
  Promise.all([
    fetch('/api/events?n=100').then(function(r){return r.json()}),
    fetch('/metrics').then(function(r){return r.text()})
  ]).then(function(res){
    renderRows(res[0]);renderTiles(parseMetrics(res[1]));
    document.getElementById('status').textContent='updated '+new Date().toLocaleTimeString();
  }).catch(function(){document.getElementById('status').textContent='disconnected — retrying…'});
}
tick();setInterval(tick,3000);
</script>
</body>
</html>`
