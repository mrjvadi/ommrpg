'use strict';
// OMMRPG admin panel: hash-routed SPA, Persian RTL, live via Centrifugo.

const S = { token: localStorage.getItem('adm_token') || '', user: null, rt: null, live: 'offline', dash: null, feed: [], history: [] };
const fa = (n, d = 0) => Number(n || 0).toLocaleString('fa-IR', { maximumFractionDigits: d });
const ton = (nano) => fa(Number(nano || 0) / 1e9, 3) + ' تون';
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const time = (t) => t ? new Date(t).toLocaleString('fa-IR') : '—';
const clock = (t) => t ? new Date(t).toLocaleTimeString('fa-IR') : '';
const $ = (sel, root = document) => root.querySelector(sel);
const RAR = { common: 'معمولی', uncommon: 'غیرمعمول', rare: 'کمیاب', epic: 'حماسی', legendary: 'افسانه‌ای', mythic: 'اسطوره‌ای' };
const rar = (r) => `<span class="badge"><span class="dot rar-${esc(r)}"></span>${RAR[r] || esc(r)}</span>`;
const STATUS = { pending: ['warn', 'در انتظار'], approved: ['info', 'تأیید شده'], sending: ['warn', 'در حال ارسال'], sent: ['good', 'ارسال شد'], failed: ['bad', 'ناموفق'], rejected: ['bad', 'رد شد'], active: ['good', 'فعال'], sold: ['info', 'فروخته شد'], cancelled: ['bad', 'لغو شد'], settling: ['warn', 'در حال تسویه'], vault: ['info', 'خزانه'], bound: ['good', 'روی کاراکتر'], burned: ['bad', 'سوزانده'] };
const status = (s) => { const [c, l] = STATUS[s] || ['info', s]; return `<span class="badge"><span class="dot ${c}"></span>${esc(l)}</span>`; };

const ERR = { invalid: 'ورودی نامعتبر', unauthorized: 'احراز هویت ناموفق', forbidden: 'دسترسی کافی ندارید', not_found: 'پیدا نشد', conflict: 'تداخل وضعیت؛ دوباره تلاش کنید', insufficient_funds: 'موجودی کافی نیست', cooldown: 'کمی صبر کنید', rate_limited: 'تلاش‌های زیاد؛ کمی بعد دوباره امتحان کنید', unavailable: 'سرویس در دسترس نیست', internal: 'خطای داخلی' };
const errText = (e) => e ? `${ERR[e.code] || e.code}${e.message ? ' — ' + e.message : ''}` : 'خطا';

function toast(msg, err) {
  const el = document.createElement('div');
  el.className = 'toast' + (err ? ' err' : '');
  el.textContent = msg;
  $('#toasts').appendChild(el);
  setTimeout(() => el.remove(), 4000);
}

async function api(method, path, body) {
  const res = await fetch('/admin/api' + path, {
    method, headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + S.token },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) { logout(); throw new Error('نشست منقضی شد'); }
  if (!res.ok) throw new Error(data.error ? errText(data.error) : res.statusText);
  return data;
}
async function run(fn, okMsg) {
  try { const r = await fn(); if (okMsg) toast(okMsg); return r; } catch (e) { toast(e.message, true); }
}

// ---------------------------------------------------------------- shell

const PAGES = [
  ['dashboard', 'داشبورد زنده'], ['players', 'بازیکنان'], ['market', 'بازار NFT'], ['withdrawals', 'برداشت‌های TON'],
  ['chain', 'زنجیره OMM'], ['policy', 'سیاست بازار'], ['audit', 'گزارش ممیزی'], ['admins', 'مدیران'],
];

function shell(title, inner) {
  const route = location.hash.slice(2).split('/')[0] || 'dashboard';
  const liveCls = S.live === 'live' ? 'good' : S.live === 'connecting' ? 'warn' : 'bad';
  const liveTxt = S.live === 'live' ? 'زنده' : S.live === 'connecting' ? 'در حال اتصال' : 'قطع';
  $('#app').innerHTML = `
  <div class="shell">
    <aside class="side">
      <div class="brand">OMMRPG<small>پنل مدیریت</small></div>
      <nav class="nav">${PAGES.map(([id, name]) => `<a href="#/${id}" class="${route === id ? 'active' : ''}">${name}</a>`).join('')}</nav>
    </aside>
    <main class="main">
      <div class="topbar">
        <h1>${title}</h1>
        <span class="pill" id="live"><span class="dot ${liveCls}"></span>${liveTxt}</span>
        <span class="pill">${esc(S.user.username)} · ${esc(S.user.role)}</span>
        <button class="btn small" id="theme">تم</button>
        <button class="btn small" id="logout">خروج</button>
      </div>
      <div id="page">${inner}</div>
    </main>
  </div>`;
  $('#logout').onclick = logout;
  $('#theme').onclick = () => {
    const cur = document.documentElement.dataset.theme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    document.documentElement.dataset.theme = cur === 'dark' ? 'light' : 'dark';
    localStorage.setItem('adm_theme', document.documentElement.dataset.theme);
  };
}

function setLive(state) {
  S.live = state;
  const el = $('#live');
  if (el) {
    const m = { live: ['good', 'زنده'], connecting: ['warn', 'در حال اتصال'], offline: ['bad', 'قطع'] }[state];
    el.innerHTML = `<span class="dot ${m[0]}"></span>${m[1]}`;
  }
}

function logout() {
  localStorage.removeItem('adm_token');
  S.token = ''; S.user = null;
  if (S.rt) S.rt.close();
  loginPage();
}

function loginPage() {
  $('#app').innerHTML = `
  <div class="card login">
    <h2 style="font-size:20px;color:var(--text)">ورود به پنل مدیریت</h2>
    <div class="field"><label>نام کاربری</label><input id="u" autocomplete="username"></div>
    <div class="field"><label>رمز عبور</label><input id="p" type="password" autocomplete="current-password"></div>
    <button class="btn primary" id="go" style="width:100%">ورود</button>
  </div>`;
  const go = async () => {
    try {
      const r = await fetch('/admin/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: $('#u').value, password: $('#p').value }) });
      const d = await r.json();
      if (!r.ok) throw new Error(errText(d.error));
      S.token = d.token; localStorage.setItem('adm_token', d.token);
      localStorage.setItem('adm_rt', JSON.stringify(d.realtime));
      start();
    } catch (e) { toast(e.message, true); }
  };
  $('#go').onclick = go;
  $('#p').onkeydown = (e) => { if (e.key === 'Enter') go(); };
}

function wsURL(u) {
  if (u.startsWith('ws')) return u;
  return (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + u;
}

async function start() {
  try { S.user = await api('GET', '/me'); } catch { return loginPage(); }
  const rt = JSON.parse(localStorage.getItem('adm_rt') || 'null');
  if (rt && !S.rt) S.rt = new Realtime(wsURL(rt.url), rt.token, onPush, setLive);
  route();
}

function onPush(channel, data) {
  if (channel === 'admin:metrics') {
    S.history.push(data);
    if (S.history.length > 300) S.history.shift();
    if (currentRoute() === 'dashboard') renderMetrics();
  } else if (channel === 'admin:feed') {
    S.feed.unshift(data);
    if (S.feed.length > 200) S.feed.pop();
    if (currentRoute() === 'dashboard') prependFeed(data);
  }
}

const currentRoute = () => location.hash.slice(2).split('/')[0] || 'dashboard';

function route() {
  if (!S.user) return;
  const [page, arg] = location.hash.slice(2).split('/');
  ({ dashboard, players, player, market, withdrawals, chain, block, policy, audit, admins }[page || 'dashboard'] || dashboard)(arg);
}
window.addEventListener('hashchange', route);

// ---------------------------------------------------------------- dashboard

const KPIS = [
  ['online', 'بازیکن آنلاین', v => fa(v)],
  ['kills_per_min', 'کشتار در دقیقه', v => fa(v)],
  ['attacks_per_min', 'حمله در دقیقه', v => fa(v)],
  ['moves_per_sec', 'حرکت در ثانیه', v => fa(v, 1)],
  ['events_per_min', 'رویداد در دقیقه', v => fa(v)],
  ['rejected_moves_per_min', 'حرکت ردشده (ضدتقلب) در دقیقه', v => fa(v)],
];

async function dashboard() {
  shell('داشبورد زنده', `
    <div class="grid kpis" id="kpis">${KPIS.map(([k, label]) => `
      <div class="card tile"><div class="label">${label}</div><div class="value num" id="v-${k}">—</div><div class="spark" id="s-${k}"></div></div>`).join('')}
    </div>
    <div class="grid kpis econ" id="econ"></div>
    <div class="grid cols-2">
      <div class="card"><h2>رویدادهای زنده</h2><ul class="feed" id="feed"></ul></div>
      <div class="grid" style="align-content:start">
        <div class="card"><h2>سلامت سرویس‌ها</h2><div class="svc" id="svc"></div></div>
        <div class="card"><h2>اعلان همگانی به بازیکنان</h2>
          <div class="row"><input id="ann" placeholder="متن اعلان" style="flex:1"><button class="btn primary" id="annb">ارسال</button></div></div>
        <div class="card"><h2>برترین بازیکنان</h2><div class="table-wrap" id="top"></div></div>
        <div class="card"><h2>جمعیت نواحی</h2><div class="table-wrap" id="zones"></div></div>
      </div>
    </div>`);
  $('#annb').onclick = () => run(() => api('POST', '/announce', { text: $('#ann').value }), 'اعلان ارسال شد');
  const d = await run(() => api('GET', '/dashboard'));
  if (!d) return;
  if (S.history.length < d.history.length) S.history = d.history;
  S.feed = d.feed;
  $('#feed').innerHTML = S.feed.map(feedItem).join('') || '<li class="empty">هنوز رویدادی نیست</li>';
  $('#top').innerHTML = table(['نام', 'سطح', 'کلاس'], d.top.map(c => [`<a href="#/player/${c.id}">${esc(c.name)}</a>`, fa(c.level), esc(c.class || '—')]));
  renderMetrics();
}

function renderMetrics() {
  const h = S.history;
  if (!h.length || !$('#kpis')) return;
  const last = h[h.length - 1];
  for (const [k, , fmt] of KPIS) {
    $('#v-' + k).textContent = fmt(last[k]);
    sparkline($('#s-' + k), h.map(p => ({ t: p.t, v: p[k] || 0 })), fmt);
  }
  const a = last.asset;
  if (a) {
    const tiles = [
      ['حجم معاملات TON (۲۴ساعت)', ton(a.volume_24h_ton), `${fa(a.sales_24h)} فروش`],
      ['حجم معاملات طلا (۲۴ساعت)', fa(a.volume_24h_gold) + ' طلا', ''],
      ['آگهی‌های فعال', fa(a.active_ton + a.active_gold), `${fa(a.active_ton)} TON · ${fa(a.active_gold)} طلا`],
      ['موجودی TON کاربران', ton(a.user_balances), `قفل‌شده: ${ton(a.locked_total)}`],
      ['کارمزد جمع‌شده', ton(a.fees_ton), ''],
      ['برداشت‌های منتظر', fa(a.pending_withdrawals), a.pending_withdrawals ? '<a href="#/withdrawals">بررسی ›</a>' : ''],
      ['NFTها', fa(Object.values(a.tokens || {}).reduce((x, y) => x + y, 0)), Object.entries(a.tokens || {}).map(([k, v]) => `${(STATUS[k] || [0, k])[1]}: ${fa(v)}`).join(' · ')],
      ['ارتفاع زنجیره', fa(a.chain_height), '<a href="#/chain">مرورگر ›</a>'],
    ];
    $('#econ').innerHTML = tiles.map(([l, v, s]) => `<div class="card tile"><div class="label">${l}</div><div class="value num">${v}</div><div class="sub">${s}</div></div>`).join('');
  }
  if (last.services) {
    $('#svc').innerHTML = Object.keys(last.services).sort().map(k => `<div><span class="dot ${last.services[k] ? 'good' : 'bad'}"></span>${k} ${last.services[k] ? '' : '— قطع'}</div>`).join('');
  }
  const z = Object.entries(last.zones || {}).sort((x, y) => y[1] - x[1]).slice(0, 8);
  $('#zones').innerHTML = z.length ? table(['ناحیه', 'بازیکن'], z.map(([k, v]) => [`<span class="mono">${esc(k)}</span>`, fa(v)])) : '<div class="empty">کسی آنلاین نیست</div>';
}

function feedItem(it, fresh) {
  const cls = { good: 'good', warn: 'warn', money: 'money', info: 'info' }[it.level] || 'info';
  const txt = it.kind === 'audit' ? `مدیر ${esc(it.admin)}: ${esc(it.action)} ${esc(it.target || '')} ${it.ok ? '' : '(ناموفق)'}` : esc(it.text);
  return `<li class="${fresh ? 'new' : ''}"><span class="t">${clock(it.time)}</span><span class="dot ${it.kind === 'audit' ? 'warn' : cls}"></span><span>${txt}</span></li>`;
}
function prependFeed(it) {
  const f = $('#feed');
  if (!f) return;
  if (f.querySelector('.empty')) f.innerHTML = '';
  f.insertAdjacentHTML('afterbegin', feedItem(it, true));
  while (f.children.length > 200) f.lastChild.remove();
}

function table(head, rows, onClick) {
  if (!rows.length) return '<div class="empty">موردی نیست</div>';
  return `<table><thead><tr>${head.map(h => `<th>${h}</th>`).join('')}</tr></thead><tbody>${rows.map((r, i) => `<tr ${onClick ? `class="click" data-i="${i}"` : ''}>${r.map(c => `<td>${c}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
}

// ---------------------------------------------------------------- players

async function players() {
  shell('بازیکنان', `<div class="card"><div class="row"><input id="q" placeholder="جستجو با نام، شناسه کاراکتر یا شناسه حساب" style="flex:1"><button class="btn primary" id="s">جستجو</button></div><div class="table-wrap" id="list"></div></div>`);
  const load = async () => {
    const r = await run(() => api('GET', '/players?q=' + encodeURIComponent($('#q').value)));
    if (!r) return;
    $('#list').innerHTML = table(['نام', 'سطح', 'کلاس', 'ساخته شده'], r.characters.map(c => [esc(c.name), fa(c.level), esc(c.class || '—'), time(c.created_at)]), true);
    $('#list').querySelectorAll('tr.click').forEach(tr => tr.onclick = () => location.hash = '#/player/' + r.characters[tr.dataset.i].id);
  };
  $('#s').onclick = load;
  $('#q').onkeydown = e => { if (e.key === 'Enter') load(); };
  load();
}

async function player(id) {
  shell('بازیکن', '<div class="empty">در حال بارگذاری…</div>');
  const p = await run(() => api('GET', '/players/' + id));
  if (!p) return;
  const c = p.character, acc = p.account || {};
  const inv = (p.inventory && p.inventory.items) || [];
  $('#page').innerHTML = `
  <div class="grid cols-2">
    <div class="card"><h2>${esc(c.name)}</h2><dl class="kv">
      <dt>سطح</dt><dd>${fa(c.level)} — ${fa(c.xp)} از ${fa(c.xp_to_next)} امتیاز تجربه</dd>
      <dt>کلاس</dt><dd>${esc(c.class || '—')}</dd>
      <dt>ویژگی‌ها</dt><dd>قدرت ${fa(c.attributes.str)} · چابکی ${fa(c.attributes.agi)} · هوش ${fa(c.attributes.int)} · بنیه ${fa(c.attributes.vit)}</dd>
      <dt>جان</dt><dd>${p.vitals ? fa(p.vitals.hp) + ' / ' + fa(p.vitals.max_hp) : '—'}</dd>
      <dt>مکان</dt><dd>${p.position ? '<bdi class="mono">' + esc(p.position.zone) + ' (' + p.position.x.toFixed(1) + ', ' + p.position.y.toFixed(1) + ')</bdi>' : 'آفلاین'}</dd>
      <dt>حساب</dt><dd>${esc(acc.display_name || '')} ${acc.telegram_id ? '· تلگرام ' + acc.telegram_id : ''} ${acc.banned ? status('cancelled') + ' مسدود: ' + esc(acc.ban_reason || '') : ''}</dd>
      <dt>طلا / اسنس</dt><dd>${fa(p.inventory ? p.inventory.wallet.gold : 0)} / ${fa(p.inventory ? p.inventory.wallet.essence : 0)}</dd>
      <dt>TON</dt><dd>${p.ton ? ton(p.ton.balance) + ' (قفل: ' + ton(p.ton.locked) + ') · memo <span class="mono">' + esc(p.ton.deposit_memo) + '</span>' : '—'}</dd>
    </dl></div>
    <div class="card"><h2>اقدامات</h2>
      <div class="row"><button class="btn" id="respawn">انتقال به نقطه شروع</button><button class="btn" id="kick">قطع اتصال</button>
      <button class="btn ${acc.banned ? '' : 'danger'}" id="ban">${acc.banned ? 'رفع مسدودیت' : 'مسدود کردن حساب'}</button></div>
      <h2 style="margin-top:14px">اعطا (با ثبت در گزارش ممیزی)</h2>
      <div class="form-grid">
        <div class="field"><label>طلا (+/-)</label><input id="g-gold" type="number" value="0"></div>
        <div class="field"><label>اسنس (+/-)</label><input id="g-ess" type="number" value="0"></div>
        <div class="field"><label>آیتم (پایه)</label><select id="g-base"><option value="">— هیچ —</option>${['sword','axe','mace','spear','dagger','bow','crossbow','staff','wand','shield','tome','helm','hood','plate','leather','robe','greaves','pants','boots','sabatons','gauntlets','gloves','ring','amulet'].map(b => `<option>${b}</option>`).join('')}</select></div>
        <div class="field"><label>کمیابی</label><select id="g-rar">${Object.entries(RAR).map(([k, v]) => `<option value="${k}">${v}</option>`).join('')}</select></div>
        <div class="field"><label>سطح آیتم</label><input id="g-lvl" type="number" value="${c.level}"></div>
        <div class="field"><label>ارتقا (+N)</label><input id="g-enh" type="number" value="0"></div>
      </div>
      <div class="row"><input id="g-reason" placeholder="دلیل (الزامی)" style="flex:1"><button class="btn primary" id="grant">اعطا</button></div>
    </div>
  </div>
  <div class="card" style="margin-top:12px"><h2>کوله‌پشتی (${fa(inv.length)})</h2><div class="table-wrap">${table(['آیتم', 'کمیابی', 'اسلات', 'سطح', 'ارتقا', 'مجهز'], inv.map(i => [esc(i.display_name), rar(i.item.rarity), esc(i.item.slot), fa(i.state.level), '+' + fa(i.state.enhance), i.equipped ? '✓' : '']))}</div></div>
  <div class="card" style="margin-top:12px"><h2>NFTهای حساب</h2><div class="table-wrap">${table(['#', 'آیتم', 'کمیابی', 'وضعیت'], p.tokens.map(t => ['#' + fa(t.id), esc((t.item && t.item.display_name) || ''), rar(t.rarity), status(t.state)]))}</div></div>`;
  $('#respawn').onclick = () => run(() => api('POST', `/players/${id}/respawn`), 'منتقل شد');
  $('#kick').onclick = () => run(() => api('POST', `/players/${id}/kick`), 'اتصال قطع شد');
  $('#ban').onclick = async () => {
    const reason = acc.banned ? '' : prompt('دلیل مسدودسازی؟');
    if (!acc.banned && !reason) return;
    await run(() => api('POST', `/accounts/${c.account_id}/ban`, { banned: !acc.banned, reason }), 'انجام شد');
    player(id);
  };
  $('#grant').onclick = async () => {
    const body = { gold: +$('#g-gold').value, essence: +$('#g-ess').value, base: $('#g-base').value, rarity: $('#g-rar').value, item_level: +$('#g-lvl').value, enhance: +$('#g-enh').value, reason: $('#g-reason').value };
    if (await run(() => api('POST', `/players/${id}/grant`, body), 'اعطا شد')) player(id);
  };
}

// ---------------------------------------------------------------- market

async function market() {
  shell('بازار NFT', `<div class="tabs" id="tabs">${['active', 'settling', 'sold', 'cancelled'].map(s => `<button data-s="${s}">${STATUS[s][1]}</button>`).join('')}</div><div class="card"><div class="table-wrap" id="list"></div></div>`);
  const load = async (st) => {
    document.querySelectorAll('#tabs button').forEach(b => b.classList.toggle('on', b.dataset.s === st));
    const r = await run(() => api('GET', '/market?status=' + st));
    if (!r) return;
    $('#list').innerHTML = table(['آگهی', 'NFT', 'آیتم', 'کمیابی', 'قیمت', 'وضعیت', 'زمان', ''], r.listings.map(l => [
      '#' + fa(l.id), `<a href="#" data-tok="${l.token_id}">#${fa(l.token_id)}</a>`, esc(l.token.item.display_name || ''), rar(l.token.rarity),
      l.currency === 'TON' ? ton(l.price) : fa(l.price) + ' طلا', status(l.status), time(l.created_at),
      l.status === 'active' ? `<button class="btn small danger" data-cancel="${l.id}">لغو</button>` : '']));
    $('#list').querySelectorAll('[data-cancel]').forEach(b => b.onclick = async () => {
      if (confirm('این آگهی لغو شود؟')) { await run(() => api('POST', `/market/${b.dataset.cancel}/cancel`), 'لغو شد'); load(st); }
    });
    $('#list').querySelectorAll('[data-tok]').forEach(a => a.onclick = (e) => { e.preventDefault(); showToken(a.dataset.tok); });
  };
  document.querySelectorAll('#tabs button').forEach(b => b.onclick = () => load(b.dataset.s));
  load('active');
}

async function showToken(id) {
  const v = await run(() => api('GET', '/tokens/' + id));
  if (!v) return;
  alert(`NFT #${id} — ${v.token.item.display_name}\n\nتاریخچه روی زنجیره:\n` + v.provenance.map(t => `${time(t.created_at)}  ${t.kind}${t.amount ? '  ' + (t.currency === 'TON' ? ton(t.amount) : fa(t.amount) + ' ' + t.currency) : ''}  بلاک ${t.block_height ?? '—'}`).join('\n'));
}

// ---------------------------------------------------------------- withdrawals

async function withdrawals() {
  shell('برداشت‌های TON', `
    <div class="tabs" id="tabs">${['pending', 'approved', 'sending', 'failed', 'sent', 'rejected', ''].map(s => `<button data-s="${s}">${s ? STATUS[s][1] : 'همه'}</button>`).join('')}</div>
    <div class="card"><div class="table-wrap" id="list"></div></div>
    <div class="card" style="margin-top:12px"><h2>واریز آزمایشی (فقط در حالت mock شبکه TON)</h2>
      <div class="row"><input id="memo" placeholder="memo واریز حساب" class="mono"><input id="amt" type="number" step="0.1" placeholder="مقدار TON"><button class="btn" id="dep">شبیه‌سازی واریز</button></div></div>`);
  $('#dep').onclick = () => run(() => api('POST', '/ton/mock-deposit', { memo: $('#memo').value, amount: Math.round(+$('#amt').value * 1e9) }), 'واریز شبیه‌سازی شد');
  const load = async (st) => {
    document.querySelectorAll('#tabs button').forEach(b => b.classList.toggle('on', b.dataset.s === st));
    const r = await run(() => api('GET', '/withdrawals?status=' + st));
    if (!r) return;
    $('#list').innerHTML = table(['#', 'حساب', 'مقصد', 'مقدار', 'کارمزد', 'وضعیت', 'بررسی‌کننده', 'تراکنش', 'زمان', ''], r.withdrawals.map(w => [
      fa(w.id), `<span class="mono">${esc(w.account_id.slice(0, 8))}</span>`, `<span class="mono">${esc(w.to_address)}</span>`, ton(w.amount), ton(w.fee),
      status(w.status), esc(w.reviewer || ''), w.tx_hash ? `<span class="mono">${esc(w.tx_hash.slice(0, 12))}…</span>` : '', time(w.created_at),
      (w.status === 'pending' || w.status === 'failed') ? `<button class="btn small primary" data-ok="${w.id}">تأیید</button> <button class="btn small danger" data-no="${w.id}">رد و بازگشت وجه</button>` : '']));
    $('#list').querySelectorAll('[data-ok]').forEach(b => b.onclick = async () => { if (confirm('ارسال این مبلغ روی شبکه TON تأیید شود؟')) { await run(() => api('POST', `/withdrawals/${b.dataset.ok}/review`, { approve: true }), 'تأیید شد'); load(st); } });
    $('#list').querySelectorAll('[data-no]').forEach(b => b.onclick = async () => { const note = prompt('دلیل رد؟') || ''; await run(() => api('POST', `/withdrawals/${b.dataset.no}/review`, { approve: false, note }), 'رد شد'); load(st); });
  };
  document.querySelectorAll('#tabs button').forEach(b => b.onclick = () => load(b.dataset.s));
  load('pending');
}

// ---------------------------------------------------------------- chain

async function chain() {
  shell('زنجیره OMM', '<div class="empty">…</div>');
  const r = await run(() => api('GET', '/chain'));
  if (!r) return;
  $('#page').innerHTML = `
  <div class="grid kpis">
    <div class="card tile"><div class="label">ارتفاع</div><div class="value num">${fa(r.info.height)}</div></div>
    <div class="card tile"><div class="label">کل تراکنش‌ها</div><div class="value num">${fa(r.info.txs)}</div></div>
    <div class="card tile"><div class="label">در انتظار بلاک</div><div class="value num">${fa(r.info.pending_txs)}</div></div>
    <div class="card tile"><div class="label">کلید عمومی امضا (ed25519)</div><div class="sub mono" style="word-break:break-all">${esc(r.info.public_key)}</div></div>
  </div>
  <div class="card"><h2>بررسی تراکنش</h2><div class="row"><input id="tx" class="mono" placeholder="hash تراکنش" style="flex:1"><button class="btn primary" id="verify">بررسی اثبات Merkle</button></div><div id="proof"></div></div>
  <div class="card" style="margin-top:12px"><h2>آخرین بلاک‌ها</h2><div class="table-wrap">${table(['ارتفاع', 'تراکنش', 'hash', 'ریشه Merkle', 'زمان'], r.blocks.map(b => [fa(b.height), fa(b.tx_count), `<span class="mono">${esc(b.hash.slice(0, 16))}…</span>`, `<span class="mono">${esc(b.merkle_root.slice(0, 16))}…</span>`, time(b.created_at)]), true)}</div></div>`;
  document.querySelectorAll('tr.click').forEach(tr => tr.onclick = () => location.hash = '#/block/' + r.blocks[tr.dataset.i].height);
  $('#verify').onclick = async () => {
    const p = await run(() => api('GET', '/chain/tx/' + $('#tx').value.trim()));
    if (!p) return;
    $('#proof').innerHTML = `<p>${p.valid ? '<span class="badge"><span class="dot good"></span>معتبر: تراکنش در بلاک ' + fa(p.block.height) + ' با امضای معتبر ثبت شده است</span>' : '<span class="badge"><span class="dot warn"></span>هنوز در بلاک نیست یا نامعتبر</span>'}</p>
      <dl class="kv"><dt>نوع</dt><dd>${esc(p.tx.kind)}</dd><dt>NFT</dt><dd>${p.tx.token_id ? '#' + fa(p.tx.token_id) : '—'}</dd><dt>مسیر اثبات</dt><dd class="mono">${(p.proof || []).map(s => (s.left ? 'L:' : 'R:') + s.hash.slice(0, 10)).join(' → ') || '—'}</dd></dl>`;
  };
}

async function block(h) {
  shell('بلاک ' + fa(h), '<div class="empty">…</div>');
  const b = await run(() => api('GET', '/chain/blocks/' + h));
  if (!b) return;
  $('#page').innerHTML = `<div class="card"><dl class="kv"><dt>hash</dt><dd class="mono">${esc(b.hash)}</dd><dt>قبلی</dt><dd class="mono">${esc(b.prev_hash)}</dd><dt>ریشه Merkle</dt><dd class="mono">${esc(b.merkle_root)}</dd><dt>امضا</dt><dd class="mono" style="word-break:break-all">${esc(b.signature)}</dd></dl></div>
  <div class="card" style="margin-top:12px"><h2>تراکنش‌ها</h2><div class="table-wrap">${table(['نوع', 'NFT', 'از', 'به', 'مقدار', 'hash'], (b.txs || []).map(t => [esc(t.kind), t.token_id ? '#' + fa(t.token_id) : '', `<span class="mono">${esc((t.from || '').slice(0, 10))}</span>`, `<span class="mono">${esc((t.to || '').slice(0, 10))}</span>`, t.amount ? (t.currency === 'TON' ? ton(t.amount) : fa(t.amount) + ' ' + esc(t.currency)) : '', `<span class="mono">${esc(t.hash.slice(0, 14))}…</span>`]))}</div></div>`;
}

// ---------------------------------------------------------------- policy

async function policy() {
  shell('سیاست بازار', '<div class="empty">…</div>');
  const p = await run(() => api('GET', '/policy'));
  if (!p) return;
  const rsel = (id, v) => `<select id="${id}">${Object.entries(RAR).map(([k, n]) => `<option value="${k}" ${k === v ? 'selected' : ''}>${n}</option>`).join('')}</select>`;
  $('#page').innerHTML = `<div class="card"><div class="form-grid">
    <div class="field"><label>حداقل کمیابی برای ساخت NFT</label>${rsel('mint', p.mint_min_rarity)}</div>
    <div class="field"><label>حداقل کمیابی برای فروش با طلا</label>${rsel('gold', p.gold_min_rarity)}</div>
    <div class="field"><label>حداقل کمیابی برای فروش با TON (پول واقعی)</label>${rsel('tonr', p.ton_min_rarity)}</div>
    <div class="field"><label>کارمزد بازار (درصد)</label><input id="fee" type="number" step="0.1" value="${p.fee_bps / 100}"></div>
    <div class="field"><label>حداقل قیمت TON</label><input id="minton" type="number" step="0.01" value="${p.min_price_ton / 1e9}"></div>
    <div class="field"><label>حداقل قیمت طلا</label><input id="mingold" type="number" value="${p.min_price_gold}"></div>
    <div class="field"><label>حداقل برداشت TON</label><input id="wmin" type="number" step="0.1" value="${p.withdraw_min / 1e9}"></div>
    <div class="field"><label>کارمزد شبکه برای برداشت (TON)</label><input id="wfee" type="number" step="0.01" value="${p.withdraw_fee / 1e9}"></div>
    <div class="field"><label>تأیید خودکار برداشت تا (TON، صفر = همیشه دستی)</label><input id="auto" type="number" step="0.1" value="${p.auto_approve_max / 1e9}"></div>
  </div>
  <div class="row"><label><input type="checkbox" id="trade" ${p.trading_enabled ? 'checked' : ''}> معاملات فعال</label>
  <label><input type="checkbox" id="wd" ${p.withdraw_enabled ? 'checked' : ''}> برداشت فعال</label></div>
  <button class="btn primary" id="save">ذخیره</button></div>`;
  $('#save').onclick = () => run(() => api('PUT', '/policy', {
    mint_min_rarity: $('#mint').value, gold_min_rarity: $('#gold').value, ton_min_rarity: $('#tonr').value,
    fee_bps: Math.round(+$('#fee').value * 100), min_price_ton: Math.round(+$('#minton').value * 1e9), min_price_gold: +$('#mingold').value,
    withdraw_min: Math.round(+$('#wmin').value * 1e9), withdraw_fee: Math.round(+$('#wfee').value * 1e9), auto_approve_max: Math.round(+$('#auto').value * 1e9),
    trading_enabled: $('#trade').checked, withdraw_enabled: $('#wd').checked,
  }), 'ذخیره شد');
}

// ---------------------------------------------------------------- audit & admins

async function audit() {
  shell('گزارش ممیزی', '<div class="empty">…</div>');
  const r = await run(() => api('GET', '/audit'));
  if (!r) return;
  $('#page').innerHTML = `<div class="card"><div class="table-wrap">${table(['زمان', 'مدیر', 'اقدام', 'هدف', 'نتیجه', 'جزئیات'], r.entries.map(e => [time(e.time), esc(e.admin), esc(e.action), `<span class="mono">${esc(e.target || '')}</span>`, e.ok ? status('sent') : `<span class="badge"><span class="dot bad"></span>${esc(e.error)}</span>`, `<span class="mono">${esc(e.details ? JSON.stringify(e.details).slice(0, 80) : '')}</span>`]))}</div></div>`;
}

async function admins() {
  shell('مدیران', '<div class="empty">…</div>');
  const r = await run(() => api('GET', '/admins'));
  if (!r) return;
  $('#page').innerHTML = `<div class="card"><div class="table-wrap">${table(['نام کاربری', 'نقش', 'وضعیت', 'آخرین ورود'], r.admins.map(a => [esc(a.username), esc(a.role), a.disabled ? status('cancelled') : status('active'), time(a.last_login_at)]))}</div></div>
  <div class="card" style="margin-top:12px"><h2>افزودن یا ویرایش مدیر</h2><div class="form-grid">
    <div class="field"><label>نام کاربری</label><input id="au"></div>
    <div class="field"><label>رمز عبور (حداقل ۱۰ کاراکتر؛ برای ویرایش خالی بگذارید)</label><input id="ap" type="password"></div>
    <div class="field"><label>نقش</label><select id="ar"><option value="viewer">viewer (فقط مشاهده)</option><option value="moderator">moderator</option><option value="admin">admin</option><option value="owner">owner</option></select></div>
  </div><div class="row"><label><input type="checkbox" id="ad"> غیرفعال</label><button class="btn primary" id="as">ذخیره</button></div></div>`;
  $('#as').onclick = async () => { if (await run(() => api('POST', '/admins', { username: $('#au').value, password: $('#ap').value, role: $('#ar').value, disabled: $('#ad').checked }), 'ذخیره شد')) admins(); };
}

// ---------------------------------------------------------------- boot
const savedTheme = localStorage.getItem('adm_theme');
if (savedTheme) document.documentElement.dataset.theme = savedTheme;
if (!location.hash) location.hash = '#/dashboard';
if (S.token) start(); else loginPage();
