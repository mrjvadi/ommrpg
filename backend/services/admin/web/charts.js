// Sparkline with hover crosshair + tooltip. Single series: the tile's label
// names it, so no legend. 2px line, recessive baseline, end marker.
function sparkline(el, points, fmt) {
  el.innerHTML = '';
  const w = el.clientWidth || 180, h = 40, pad = 4;
  if (!points.length) { el.innerHTML = '<div class="sub">—</div>'; return; }
  const xs = points.map(p => p.t), ys = points.map(p => p.v);
  const x0 = Math.min(...xs), x1 = Math.max(...xs) || x0 + 1;
  const ymax = Math.max(1, ...ys);
  const X = t => pad + (w - 2 * pad) * (x1 === x0 ? 1 : (t - x0) / (x1 - x0));
  const Y = v => h - pad - (h - 2 * pad) * (v / ymax);
  const d = points.map((p, i) => (i ? 'L' : 'M') + X(p.t).toFixed(1) + ' ' + Y(p.v).toFixed(1)).join(' ');
  const last = points[points.length - 1];
  const ns = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(ns, 'svg');
  svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
  svg.setAttribute('preserveAspectRatio', 'none');
  svg.innerHTML =
    `<line x1="${pad}" x2="${w - pad}" y1="${h - pad}" y2="${h - pad}" stroke="var(--axis)" stroke-width="1"/>` +
    `<path d="${d}" fill="none" stroke="var(--series-1)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>` +
    `<circle cx="${X(last.t)}" cy="${Y(last.v)}" r="3.5" fill="var(--series-1)" stroke="var(--surface)" stroke-width="2"/>` +
    `<line class="xh" y1="0" y2="${h}" stroke="var(--muted)" stroke-width="1" visibility="hidden"/>` +
    `<circle class="xd" r="4" fill="var(--series-1)" stroke="var(--surface)" stroke-width="2" visibility="hidden"/>`;
  el.appendChild(svg);
  const tip = document.createElement('div');
  tip.className = 'tip'; tip.style.display = 'none';
  el.appendChild(tip);
  const xh = svg.querySelector('.xh'), xd = svg.querySelector('.xd');
  const move = (ev) => {
    const r = svg.getBoundingClientRect();
    const px = (ev.touches ? ev.touches[0].clientX : ev.clientX) - r.left;
    let best = points[0];
    for (const p of points) if (Math.abs(X(p.t) - px) < Math.abs(X(best.t) - px)) best = p;
    const cx = X(best.t);
    xh.setAttribute('x1', cx); xh.setAttribute('x2', cx); xh.setAttribute('visibility', 'visible');
    xd.setAttribute('cx', cx); xd.setAttribute('cy', Y(best.v)); xd.setAttribute('visibility', 'visible');
    tip.textContent = fmt(best.v) + ' · ' + new Date(best.t).toLocaleTimeString('fa-IR');
    tip.style.display = 'block';
    tip.style.left = (cx / w * 100) + '%';
  };
  const leave = () => { xh.setAttribute('visibility', 'hidden'); xd.setAttribute('visibility', 'hidden'); tip.style.display = 'none'; };
  el.onmousemove = move; el.ontouchmove = move; el.onmouseleave = leave; el.ontouchend = leave;
}
