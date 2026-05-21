/* ============================================================
   PivotStack · Workbench Edition · page JS
   Mega-tab, login directions, admin views, notification flows,
   ECharts (reusing stellar theme registered by stellar.js).
   ============================================================ */

/* ============== DATA ============== */

const NOTIFS = [
  {
    id: "ntf_001",
    title: "Claude group 上游维护通告",
    body: "**Claude** group 将于今晚 02:00 - 03:00 进行上游 key 轮换。\n\n期间可能出现短暂调用失败,请已经在生产的客户提前规避。\n\n建议切换到 `cc` 或 `kiro高缓存` 渠道作为备份。",
    level: "warn",
    targetType: "group", targetValue: ["claude"],
    status: "published",
    publishAt: 1779091200, expireAt: 1779177600,
    dismissible: true,
    stats: { targetCount: 42, readCount: 18, dismissedCount: 3, unreadCount: 24 },
    publishStr: "05-18 02:00",
  },
  {
    id: "ntf_002",
    title: "充值优惠活动",
    body: "本月充值 ¥500 多送 **14%** 虚拟$(=$70)。\n\n活动至 5 月 31 日 23:59 结束。",
    level: "info",
    targetType: "plan", targetValue: ["credit", "hybrid"],
    status: "published",
    publishAt: 1779000000, expireAt: 1779600000,
    dismissible: true,
    stats: { targetCount: 120, readCount: 89, dismissedCount: 4, unreadCount: 27 },
    publishStr: "05-15 18:00",
  },
  {
    id: "ntf_003",
    title: "🚨 紧急维护:网关重启",
    body: "PivotStack 网关将于 **15 分钟后** 重启,预计 30 秒不可用。\n\n请等待后重试。",
    level: "critical",
    targetType: "all", targetValue: null,
    status: "expired",
    publishAt: 1778800000, expireAt: 1778803600,
    dismissible: false,
    stats: { targetCount: 356, readCount: 356, dismissedCount: 0, unreadCount: 0 },
    publishStr: "05-13 09:00",
  },
  {
    id: "ntf_004",
    title: "即将到期:续费提醒",
    body: "您的订阅将于 7 天后到期,[立即续费](https://pivotstack.cn/recharge)。",
    level: "warn",
    targetType: "userIds", targetValue: ["key_abc", "key_def"],
    status: "draft",
    publishAt: 0, expireAt: 0,
    dismissible: true,
    stats: { targetCount: 2, readCount: 0, dismissedCount: 0, unreadCount: 0 },
    publishStr: "—",
  },
];

const USER_NOTIFS = [
  // for user dropdown / center: 4 items (3 active + 1 expired)
  { id: "ntf_001", level: "warn", title: "Claude group 上游维护通告", excerpt: "Claude group 将于今晚 02:00 - 03:00 进行上游 key 轮换...", time: "2 分钟前", unread: true, dismissible: true },
  { id: "ntf_002", level: "info", title: "充值优惠活动", excerpt: "本月充 ¥500 多送 14% 虚拟$,活动至 5 月 31 日 23:59 结束", time: "1 天前", unread: true, dismissible: true },
  { id: "ntf_003", level: "critical", title: "🚨 紧急维护:网关重启", excerpt: "网关将于 15 分钟后重启,预计 30 秒不可用 (已过期)", time: "3 天前", unread: false, expired: true, dismissible: false },
  { id: "welcome", level: "info", title: "欢迎使用 PivotStack", excerpt: "新版用户控制台已上线,所有功能 stelar-console.cn/changelog", time: "1 月前", unread: false, dismissible: true },
];

const KEYS = [
  { sel: false, note: "玉米地大佬",          masked: "sk-•••e3a2", plan: "hybrid", balance: 195.40, gift: 12.00, requests: 2847, last: "刚刚",   status: "ok" },
  { sel: false, note: "半岛沉静的芝士",      masked: "sk-•••f5e1", plan: "credit", balance: 65.71,  gift: 5.00,  requests: 1932, last: "2 分钟前", status: "ok" },
  { sel: false, note: "勺子?",               masked: "sk-•••a8b3", plan: "credit", balance: 22.56,  gift: 0.00,  requests: 1421, last: "5 分钟前", status: "ok" },
  { sel: false, note: "agent-team-a",        masked: "sk-•••c9d1", plan: "timed",  balance: 2.40,   gift: 0.00,  requests: 282,  last: "12 分钟前", status: "ok" },
  { sel: false, note: "client-acme",         masked: "sk-•••b2e7", plan: "hybrid", balance: 1.80,   gift: 0.50,  requests: 142,  last: "1 小时前", status: "ok" },
  { sel: false, note: "test-sandbox",        masked: "sk-•••0f9a", plan: "free",   balance: 0.50,   gift: 0.50,  requests: 88,   last: "2 小时前", status: "warn" },
  { sel: false, note: "batch-runner",        masked: "sk-•••7c4d", plan: "credit", balance: 2.20,   gift: 0.00,  requests: 612,  last: "30 分钟前", status: "warn" },
  { sel: false, note: "client-zenith",       masked: "sk-•••dd11", plan: "hybrid", balance: 3.20,   gift: 1.00,  requests: 421,  last: "1 天前", status: "ok" },
  { sel: false, note: "webhook-relay",       masked: "sk-•••8801", plan: "timed",  balance: 0.90,   gift: 0.00,  requests: 42,   last: "6 小时前", status: "ok" },
  { sel: false, note: "archive-2026q1",      masked: "sk-•••deff", plan: "free",   balance: 0.00,   gift: 0.00,  requests: 0,    last: "30 天前", status: "idle" },
  { sel: false, note: "mobile-prod",         masked: "sk-•••3309", plan: "credit", balance: 1.10,   gift: 0.10,  requests: 187,  last: "3 小时前", status: "ok" },
  { sel: false, note: "key_xxx (suspect)",   masked: "sk-•••bad1", plan: "free",   balance: 0.10,   gift: 0.00,  requests: 218,  last: "刚刚",   status: "err" },
];

const ADMIN_TREE = {
  dashboard: { label: "DASHBOARD", items: [
    { route: "overview", text: "总览" },
  ]},
  channels: { label: "CHANNELS", items: [
    { route: "channels-groups", text: "分组管理" },
    { route: "channels-newapi", text: "NewAPI Provider" },
    { route: "channels-direct", text: "自营直连" },
    { route: "channels-reconcile", text: "对账中心" },
  ]},
  billing: { label: "KEYS & BILLING", items: [
    { route: "billing-keys", text: "API Keys" },
    { route: "billing-codes", text: "兑换码" },
    { route: "billing-pricing", text: "定价配置" },
    { route: "billing-unit", text: "单位换算" },
  ]},
  ops: { label: "OPERATIONS", items: [
    { route: "ops-logs", text: "调用日志" },
    { route: "ops-abuse", text: "Abuse Monitor" },
  ]},
  system: { label: "SYSTEM", items: [
    { route: "system-settings", text: "系统设置" },
    { route: "system-experimental", text: "实验性功能" },
    { route: "notifications", text: "通知管理", icon: "bell" },
  ]},
  reseller: { label: "RESELLER", items: [
    { route: "reseller-summary", text: "代理总览" },
    { route: "reseller-keys", text: "子 Key 管理" },
  ]},
};

const ROUTE_TO_VIEW = {
  "overview": "overview",
  "billing-keys": "billing-keys",
  "notifications": "notifications",
};

/* ============== UTILITIES ============== */
function fmt(n, d = 0) { return Number(n).toLocaleString("en-US", { minimumFractionDigits: d, maximumFractionDigits: d }); }
function dollar(n, d = 2) { return "$" + Number(n).toFixed(d); }
function escHtml(s) { return s.replace(/[&<>]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c])); }
function mdToHtml(src) {
  // tiny safe markdown subset: **bold**, `code`, [link](url), - lists, paragraphs
  const esc = escHtml(src);
  const lines = esc.split("\n");
  const out = [];
  let inList = false;
  for (const ln of lines) {
    const m = ln.match(/^- (.+)$/);
    if (m) {
      if (!inList) { out.push("<ul>"); inList = true; }
      out.push("<li>" + inlineMd(m[1]) + "</li>");
      continue;
    } else if (inList) { out.push("</ul>"); inList = false; }
    if (!ln.trim()) { out.push(""); continue; }
    out.push("<p>" + inlineMd(ln) + "</p>");
  }
  if (inList) out.push("</ul>");
  return out.join("\n");
}
function inlineMd(s) {
  return s
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/`(.+?)`/g, "<code>$1</code>")
    .replace(/\[(.+?)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
}
function nlvlChip(level) {
  if (level === "critical") return `<span class="nlvl nlvl--crit"><span class="nlvl__dot"></span>CRIT</span>`;
  if (level === "warn")     return `<span class="nlvl nlvl--warn"><span class="nlvl__dot"></span>WARN</span>`;
  return `<span class="nlvl nlvl--info"><span class="nlvl__dot"></span>INFO</span>`;
}
function statusTag(s) {
  const map = {
    published: ["atag--ok", "PUBLISHED"],
    draft:     ["atag--draft", "DRAFT"],
    expired:   ["atag--idle", "EXPIRED"],
    ok:        ["atag--ok", "ACTIVE"],
    warn:      ["atag--warn", "WARN"],
    err:       ["atag--err", "DOWN"],
    idle:      ["atag--idle", "IDLE"],
  };
  const [cls, label] = map[s] || ["atag--idle", s.toUpperCase()];
  return `<span class="atag ${cls}"><span class="atag__dot"></span>${label}</span>`;
}
function targetLabel(n) {
  if (n.targetType === "all") return `<span class="chip chip--mono">all</span>`;
  if (n.targetType === "plan") return `<span class="chip chip--mono">plan:${n.targetValue.join(",")}</span>`;
  if (n.targetType === "group") return `<span class="chip chip--mono">group:${n.targetValue.join(",")}</span>`;
  return `<span class="chip chip--mono">users:${n.targetValue.length}</span>`;
}
function readRate(s) {
  if (!s.targetCount) return "—";
  return `<span class="mono">${s.readCount}/${s.targetCount}</span>`;
}
function refreshIcons() { if (window.lucide) lucide.createIcons(); }

function toast(msg, icon = "check") {
  const t = document.getElementById("toast");
  t.innerHTML = `<i data-lucide="${icon}"></i><span>${msg}</span>`;
  refreshIcons();
  t.classList.add("is-show");
  clearTimeout(toast._t);
  toast._t = setTimeout(() => t.classList.remove("is-show"), 1600);
}

/* ============== MEGA-TAB ============== */
function switchMega(name) {
  document.querySelectorAll(".mega-tab").forEach(b => b.classList.toggle("is-active", b.dataset.mega === name));
  document.querySelectorAll(".mega-section").forEach(s => {
    const on = s.dataset.megaPanel === name;
    s.classList.toggle("is-active", on);
    s.hidden = !on;
  });
  // lazy init per panel
  if (name === "admin") setTimeout(initAdminCharts, 50);
  if (name === "notif") setTimeout(initNotifSectionCharts, 50);
  // resize acharts when they're visible
  setTimeout(() => Object.values(acharts).forEach(c => c && c.resize && c.resize()), 80);
}
document.querySelectorAll(".mega-tab").forEach(b => b.addEventListener("click", () => switchMega(b.dataset.mega)));

/* ============== LOGIN — direction switcher ============== */
function switchDir(name) {
  document.querySelectorAll(".dir-tab").forEach(b => b.classList.toggle("is-active", b.dataset.dir === name));
  document.querySelectorAll("[data-dir-panel]").forEach(p => {
    const on = p.dataset.dirPanel === name;
    p.hidden = !on;
  });
  if (name === "boot") setTimeout(() => bootSequence.start(), 100);
  if (name === "warp") setTimeout(() => warpScene.start(), 100);
  if (name === "singularity") setTimeout(() => orbScene.start(), 100);
}
document.querySelectorAll(".dir-tab").forEach(b => b.addEventListener("click", () => switchDir(b.dataset.dir)));

/* ============== LOGIN — Singularity ============== */
const orbScene = (function() {
  let dustCtx = null, dustRaf = null, particles = [];
  function init() {
    const canvas = document.getElementById("orbDust");
    if (!canvas) return;
    function fit() {
      const r = canvas.parentElement.getBoundingClientRect();
      canvas.width = r.width * 2; canvas.height = r.height * 2;
      canvas.style.width = r.width + "px";
      canvas.style.height = r.height + "px";
    }
    fit();
    window.addEventListener("resize", fit);
    dustCtx = canvas.getContext("2d");
    dustCtx.scale(2, 2);
    spawn();
    loop();
    document.getElementById("orb").addEventListener("click", () => {
      document.getElementById("loginCardA").classList.add("is-open");
      document.querySelector("[data-dir-panel='singularity'] .orb-scene")?.classList.add("is-hidden");
    });
    // click outside card to dismiss
    document.addEventListener("click", e => {
      const wrap = document.getElementById("loginCardA");
      if (!wrap || !wrap.classList.contains("is-open")) return;
      if (e.target.closest(".login-card")) return;
      if (e.target.closest(".orb")) return;
      if (e.target.closest("[data-dir]")) return;
      wrap.classList.remove("is-open");
      document.querySelector("[data-dir-panel='singularity'] .orb-scene")?.classList.remove("is-hidden");
    });
  }
  function spawn() {
    const c = document.getElementById("orbDust");
    if (!c) return;
    const w = c.clientWidth, h = c.clientHeight;
    particles = [];
    for (let i = 0; i < 14; i++) {
      const angle = Math.random() * Math.PI * 2;
      const r = 180 + Math.random() * 200;
      particles.push({
        x: w / 2 + Math.cos(angle) * r,
        y: h / 2 + Math.sin(angle) * r,
        cx: w / 2, cy: h / 2,
        speed: 0.05 + Math.random() * 0.10,
        life: 1.0,
        size: Math.random() * 1.2 + 0.6,
      });
    }
  }
  function loop() {
    if (!dustCtx) return;
    const c = document.getElementById("orbDust");
    if (!c) return;
    const w = c.clientWidth, h = c.clientHeight;
    dustCtx.clearRect(0, 0, w, h);
    for (const p of particles) {
      const dx = p.cx - p.x, dy = p.cy - p.y;
      const d = Math.hypot(dx, dy);
      if (d < 50) {
        p.life -= 0.02;
        if (p.life <= 0) {
          const a = Math.random() * Math.PI * 2;
          const r = 220 + Math.random() * 160;
          p.x = p.cx + Math.cos(a) * r;
          p.y = p.cy + Math.sin(a) * r;
          p.life = 1.0;
        }
      } else {
        p.x += (dx / d) * p.speed;
        p.y += (dy / d) * p.speed;
      }
      dustCtx.fillStyle = `rgba(11,212,112,${0.35 * p.life})`;
      dustCtx.beginPath();
      dustCtx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
      dustCtx.fill();
    }
    dustRaf = requestAnimationFrame(loop);
  }
  return { init, start: () => { if (!dustCtx) init(); else { spawn(); } } };
})();

/* ============== LOGIN — Boot Sequence ============== */
const bootSequence = (function() {
  let started = false;
  const lines = [
    { delay:  50, html: '<span class="label">▌PIVOTSTACK GATEWAY OS · v6.0</span>' },
    { delay: 100, html: '<span class="dim">▌─────────────────────────────────────</span>' },
    { delay: 250, html: '▌<span class="ok">[ OK ]</span> starfield engine loaded' },
    { delay: 200, html: '▌<span class="ok">[ OK ]</span> auth module ready (v2)' },
    { delay: 220, html: '▌<span class="ok">[ OK ]</span> gateway <span class="info">:8990</span> listening' },
    { delay: 200, html: '▌<span class="ok">[ OK ]</span> notifications · polling 60s · 2 unread' },
    { delay: 240, html: '▌<span class="ok">[ OK ]</span> welcome to <span class="label">pivotstack</span>' },
    { delay: 240, html: '▌' },
    { delay: 200, html: '▌<span class="label">Sign in to continue.</span>' },
    { delay: 240, html: '▌' },
  ];
  function start() {
    if (started) return;
    started = true;
    const body = document.getElementById("bootBody");
    if (!body) return;
    body.innerHTML = "";
    let acc = 0;
    lines.forEach(L => {
      acc += L.delay;
      setTimeout(() => {
        const div = document.createElement("div");
        div.className = "boot-line";
        div.innerHTML = L.html;
        body.appendChild(div);
      }, acc);
    });
    setTimeout(() => renderPrompt("user"), acc + 100);
  }
  function renderPrompt(mode) {
    const body = document.getElementById("bootBody");
    if (!body) return;
    // remove any existing prompt
    body.querySelectorAll(".boot-input-line, .boot-help, .boot-pwd-line").forEach(el => el.remove());
    if (mode === "user") {
      const line1 = document.createElement("div");
      line1.className = "boot-input-line";
      line1.innerHTML = `<span class="dim">▌</span><span class="boot-prompt">pivot login</span> <span class="boot-flag">--user</span> <input class="boot-input" id="bootUserInput" placeholder="you@example.com" value="founder@pivotstack.cn" autocomplete="off">`;
      body.appendChild(line1);
      const line2 = document.createElement("div");
      line2.className = "boot-input-line boot-pwd-line";
      line2.innerHTML = `<span class="dim">▌                  </span><input class="boot-input" type="password" id="bootPwdInput" placeholder="password" value="••••••••" autocomplete="off">`;
      body.appendChild(line2);
    } else {
      const line1 = document.createElement("div");
      line1.className = "boot-input-line";
      line1.innerHTML = `<span class="dim">▌</span><span class="boot-prompt">pivot login</span> <span class="boot-flag">--key</span> <input class="boot-input mono" id="bootKeyInput" placeholder="sk-..." value="sk-XYZ987654321ABCDEFGHe3a2" autocomplete="off">`;
      body.appendChild(line1);
    }
    const help = document.createElement("div");
    help.className = "boot-help";
    help.innerHTML = `▌ <kbd>Enter</kbd> 登录 &nbsp;&nbsp; <kbd>Tab</kbd> 切换到 <span class="boot-flag">--${mode === "user" ? "key" : "user"}</span> &nbsp;&nbsp; <kbd>Esc</kbd> 重启<br>▌<span class="boot-cursor"></span>`;
    body.appendChild(help);
    body.scrollTop = body.scrollHeight;

    const firstInput = body.querySelector(".boot-input");
    firstInput && firstInput.focus();
    body.querySelectorAll(".boot-input").forEach(inp => {
      inp.addEventListener("keydown", e => {
        if (e.key === "Tab") { e.preventDefault(); renderPrompt(mode === "user" ? "key" : "user"); }
        else if (e.key === "Enter") {
          e.preventDefault();
          toast("登录成功 · 跳转 user/dashboard", "check");
        }
        else if (e.key === "Escape") { started = false; start(); }
      });
    });
  }
  return { start };
})();

/* ============== LOGIN — Warp Tunnel ============== */
const warpScene = (function() {
  let raf = null, ctx = null, stars = [], phase = "idle", phaseStart = 0;
  const TOTAL_STARS = 480;
  function init() {
    const c = document.getElementById("warpCanvas");
    if (!c) return false;
    ctx = c.getContext("2d");
    fit();
    window.addEventListener("resize", fit);
    return true;
  }
  function fit() {
    const c = document.getElementById("warpCanvas");
    if (!c) return;
    const r = c.parentElement.getBoundingClientRect();
    c.width = r.width * 2; c.height = r.height * 2;
    c.style.width = r.width + "px"; c.style.height = r.height + "px";
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.scale(2, 2);
  }
  function spawnStars() {
    stars = [];
    const c = document.getElementById("warpCanvas");
    const maxR = Math.max(c.clientWidth, c.clientHeight) * 0.75;
    for (let i = 0; i < TOTAL_STARS; i++) {
      stars.push({
        a: Math.random() * Math.PI * 2,
        r: Math.random() * maxR + 5,
        speed: 0.6 + Math.random() * 2.4,
        maxR,
      });
    }
  }
  function loop(now) {
    if (!ctx) return;
    const c = document.getElementById("warpCanvas");
    if (!c) return;
    const w = c.clientWidth, h = c.clientHeight;
    const cx = w / 2, cy = h / 2;
    ctx.clearRect(0, 0, w, h);
    const elapsed = (now - phaseStart) / 1000;
    let mult = 1;
    if (phase === "warp") {
      // 0..1.5s accelerate, 1.5..2s decelerate
      mult = elapsed < 1.5 ? (3 + elapsed * 6) : Math.max(0.35, 1.8 - (elapsed - 1.5) * 2.9);
    } else if (phase === "idle") {
      mult = 0.5;
    }
    for (const s of stars) {
      s.r += s.speed * mult;
      if (s.r > s.maxR) {
        s.r = 5 + Math.random() * 30;
        s.a = Math.random() * Math.PI * 2;
        s.speed = 0.6 + Math.random() * 2.4;
      }
      // streak length scales with both phase mult AND distance from center
      // closer-to-edge stars stretch dramatically more
      const distFactor = 0.4 + (s.r / s.maxR) * 1.6;
      const streakLen = s.speed * mult * 6 * distFactor;
      const x = cx + Math.cos(s.a) * s.r;
      const y = cy + Math.sin(s.a) * s.r;
      const tx = cx + Math.cos(s.a) * Math.max(0, s.r - streakLen);
      const ty = cy + Math.sin(s.a) * Math.max(0, s.r - streakLen);
      // brightness: dim near center, bright at edges; even brighter during warp
      const alpha = Math.min(1, (s.r / s.maxR) * 1.2);
      const intensity = phase === "warp" ? alpha : alpha * 0.75;
      ctx.strokeStyle = `rgba(255,255,255,${intensity})`;
      ctx.lineWidth = phase === "warp" ? (1.0 + (s.r / s.maxR) * 1.2) : 0.7;
      ctx.lineCap = "round";
      ctx.beginPath();
      ctx.moveTo(tx, ty);
      ctx.lineTo(x, y);
      ctx.stroke();
    }
    // transition
    if (phase === "warp" && elapsed > 2.0) {
      phase = "idle";
      document.getElementById("warpCardWrap").classList.add("is-visible");
    }
    raf = requestAnimationFrame(loop);
  }
  function start() {
    if (!ctx && !init()) return;
    fit();
    spawnStars();
    document.getElementById("warpCardWrap").classList.remove("is-visible");
    phase = "warp";
    phaseStart = performance.now();
    cancelAnimationFrame(raf);
    raf = requestAnimationFrame(loop);
  }
  return { start };
})();
document.getElementById("warpReplay")?.addEventListener("click", () => warpScene.start());

/* ============== LOGIN — form tab + eye toggle (delegated) ============== */
document.querySelectorAll(".login-tab").forEach(t => {
  t.addEventListener("click", () => {
    const root = t.closest(".login-card");
    const sel = t.dataset.ltab || t.dataset.ltabWarp;
    root.querySelectorAll(".login-tab").forEach(x => x.classList.remove("is-active"));
    t.classList.add("is-active");
    root.querySelectorAll("[data-lpanel],[data-lpanel-warp]").forEach(p => {
      const key = p.dataset.lpanel || p.dataset.lpanelWarp;
      p.hidden = key !== sel;
    });
  });
});
document.querySelectorAll(".login-switch").forEach(b => {
  b.addEventListener("click", () => {
    const target = b.dataset.switchTo;
    const root = b.closest(".login-card");
    root.querySelectorAll(".login-tab").forEach(x => x.classList.toggle("is-active", x.dataset.ltab === target));
    root.querySelectorAll("[data-lpanel]").forEach(p => p.hidden = p.dataset.lpanel !== target);
  });
});
document.querySelectorAll(".login-eye").forEach(b => {
  b.addEventListener("click", e => {
    e.preventDefault();
    const input = b.parentElement.querySelector("input");
    if (!input) return;
    input.type = input.type === "password" ? "text" : "password";
    b.querySelector("i").setAttribute("data-lucide", input.type === "password" ? "eye" : "eye-off");
    refreshIcons();
  });
});

/* ============== ADMIN — rail + tree + view switch ============== */
function renderTree(railName) {
  const tree = document.getElementById("adminTree");
  if (!tree) return;
  // Build sections grouped by rail "domain"
  // rail names: dashboard/channels/billing/ops/system/reseller
  const map = ADMIN_TREE[railName] || ADMIN_TREE.dashboard;
  // current view
  const currentRoute = window.__currentRoute || "overview";
  tree.innerHTML = `
    <div class="tree__section">
      <div class="tree__label">${map.label}</div>
      <div class="tree__items">
        ${map.items.map(it => `
          <div class="tree__item ${it.route === currentRoute ? "is-active" : ""}" data-route="${it.route}">
            <span>${it.text}</span>
          </div>
        `).join("")}
      </div>
    </div>
  `;
  tree.querySelectorAll(".tree__item").forEach(el => {
    el.addEventListener("click", () => {
      const r = el.dataset.route;
      window.__currentRoute = r;
      renderTree(railName);
      switchAdminView(r);
    });
  });
}
function switchAdminView(route) {
  const view = ROUTE_TO_VIEW[route];
  if (!view) {
    // Show a "stub" view for non-implemented routes — keep current view but flash a toast
    toast(`路由 ${route} 在原型中未画 · 用统一模板覆盖`, "info");
    return;
  }
  document.querySelectorAll(".admin-view").forEach(v => {
    const on = v.dataset.viewPanel === view;
    v.classList.toggle("is-active", on);
    v.hidden = !on;
  });
  setTimeout(initAdminCharts, 30);
  setTimeout(() => Object.values(acharts).forEach(c => c && c.resize && c.resize()), 80);
}
document.querySelectorAll(".rail__item[data-rail]").forEach(b => {
  b.addEventListener("click", () => {
    const name = b.dataset.rail;
    document.querySelectorAll(".rail__item").forEach(x => x.classList.remove("is-active"));
    b.classList.add("is-active");
    if (name === "notif-rail") {
      // jump to notification management in admin
      const sysBtn = document.querySelector('.rail__item[data-rail="system"]');
      sysBtn?.classList.add("is-active");
      b.classList.remove("is-active");
      window.__currentRoute = "notifications";
      renderTree("system");
      switchAdminView("notifications");
      return;
    }
    // pick first route in this domain
    const firstRoute = (ADMIN_TREE[name] || ADMIN_TREE.dashboard).items[0].route;
    window.__currentRoute = firstRoute;
    renderTree(name);
    switchAdminView(firstRoute);
  });
});
// initial tree render
window.__currentRoute = "overview";
renderTree("dashboard");

document.querySelectorAll("[data-rail-jump]").forEach(b => {
  b.addEventListener("click", () => {
    const target = b.dataset.railJump;
    const railBtn = document.querySelector(`.rail__item[data-rail="${target}"]`);
    railBtn?.click();
  });
});

/* ============== ADMIN — keys table ============== */
function renderKeys() {
  const tbody = document.getElementById("keysTable");
  if (!tbody) return;
  tbody.innerHTML = KEYS.map((k, i) => `
    <div class="atable__row ${k.sel ? "is-selected" : ""}" data-idx="${i}">
      <div style="width:24px"><span class="chk ${k.sel ? "is-on" : ""}" data-row-chk="${i}"></span></div>
      <div style="flex:1;min-width:140px">${escHtml(k.note)}</div>
      <div style="width:140px" class="mono">${k.masked}</div>
      <div style="width:80px"><span class="chip chip--mono">${k.plan}</span></div>
      <div style="width:90px;text-align:right;font-family:var(--font-mono);color:${k.balance > 0 ? "var(--text-pri)" : "var(--text-ter)"}">${dollar(k.balance)}</div>
      <div style="width:72px;text-align:right" class="mono ${k.gift ? "" : ""}">${k.gift ? "+" + dollar(k.gift) : "—"}</div>
      <div style="width:80px;text-align:right" class="mono">${fmt(k.requests)}</div>
      <div style="width:100px" class="mono" style="color:var(--text-ter)">${k.last}</div>
      <div style="width:80px">${statusTag(k.status)}</div>
      <div style="width:40px;text-align:right"><i class="row-chev" data-lucide="chevron-right"></i></div>
    </div>
  `).join("");
  refreshIcons();
  attachKeysRowEvents();
  updateBulkBar();
}
function attachKeysRowEvents() {
  document.querySelectorAll(".atable--keys .atable__row").forEach(row => {
    row.addEventListener("click", e => {
      if (e.target.closest("[data-row-chk]")) return;
      toast(`打开 drawer · ${row.querySelector("div:nth-child(2)").textContent.trim()}`, "panel-right-open");
    });
  });
  document.querySelectorAll("[data-row-chk]").forEach(c => {
    c.addEventListener("click", e => {
      e.stopPropagation();
      const i = +c.dataset.rowChk;
      KEYS[i].sel = !KEYS[i].sel;
      renderKeys();
    });
  });
}
function updateBulkBar() {
  const n = KEYS.filter(k => k.sel).length;
  const bar = document.getElementById("bulkBar");
  const all = document.getElementById("bulkAll");
  if (!bar) return;
  if (n > 0) {
    bar.hidden = false;
    document.getElementById("bulkCount").textContent = n;
    all.classList.toggle("is-on", n === KEYS.length);
  } else {
    bar.hidden = true;
    all.classList.remove("is-on");
  }
}
document.getElementById("bulkAll")?.addEventListener("click", () => {
  const allOn = KEYS.every(k => k.sel);
  KEYS.forEach(k => k.sel = !allOn);
  renderKeys();
});
document.getElementById("bulkCancel")?.addEventListener("click", () => {
  KEYS.forEach(k => k.sel = false);
  renderKeys();
});
renderKeys();

/* ============== ADMIN — notifications table ============== */
function renderNotifTable(targetId = "notifTable") {
  const tbody = document.getElementById(targetId);
  if (!tbody) return;
  tbody.innerHTML = NOTIFS.map(n => `
    <div class="atable__row" data-notif-id="${n.id}">
      <div style="flex:1;min-width:200px">
        <div style="display:inline-flex;align-items:center;gap:8px">
          ${nlvlChip(n.level)}
          <span style="color:var(--text-pri);font-weight:500">${escHtml(n.title)}</span>
        </div>
      </div>
      <div style="width:160px">${targetLabel(n)}</div>
      <div style="width:80px">${nlvlChip(n.level)}</div>
      <div style="width:100px">${statusTag(n.status)}</div>
      <div style="width:140px" class="mono" style="color:var(--text-ter)">${n.publishStr}</div>
      <div style="width:90px;text-align:right">${readRate(n.stats)}</div>
      <div style="width:64px;text-align:right;display:flex;gap:4px;justify-content:flex-end">
        <button class="btn btn--ghost btn--icon btn--sm" title="编辑"><i data-lucide="pencil"></i></button>
        <button class="btn btn--ghost btn--icon btn--sm" title="删除" style="color:var(--text-ter)"><i data-lucide="trash-2"></i></button>
      </div>
    </div>
  `).join("");
  refreshIcons();
  tbody.querySelectorAll(".atable__row").forEach(row => {
    row.addEventListener("click", e => {
      if (e.target.closest("button")) return;
      openStatsDrawer(row.dataset.notifId);
    });
  });
}
renderNotifTable("notifTable");
renderNotifTable("notifTable2");

/* ============== STATS DRAWER ============== */
function openStatsDrawer(id) {
  const n = NOTIFS.find(x => x.id === id);
  if (!n) return;
  document.getElementById("statsDrawerBd").classList.add("is-open");
  document.getElementById("statsDrawer").classList.add("is-open");
  document.getElementById("statsTitle").textContent = n.title;
  document.getElementById("statsId").textContent = `${n.id} · ${n.targetType}${n.targetValue ? ":" + n.targetValue.join(",") : ""}`;
  document.querySelectorAll(".stats-tile").forEach((tile, i) => {
    const nums = [n.stats.targetCount, n.stats.readCount, n.stats.dismissedCount, n.stats.unreadCount];
    const num = tile.querySelector(".metric-tile__num");
    if (num) num.textContent = nums[i];
  });
  setTimeout(buildNotifStatsChart, 50);
}
function closeStatsDrawer() {
  document.getElementById("statsDrawerBd").classList.remove("is-open");
  document.getElementById("statsDrawer").classList.remove("is-open");
}
document.getElementById("statsDrawerBd")?.addEventListener("click", closeStatsDrawer);
document.getElementById("statsDrawerClose")?.addEventListener("click", closeStatsDrawer);

/* ============== NOTIFICATION SECTION ============== */
function switchNotifSub(name) {
  document.querySelectorAll(".sub-tab").forEach(t => t.classList.toggle("is-active", t.dataset.notifSub === name));
  document.querySelectorAll("[data-notif-panel]").forEach(p => {
    const on = p.dataset.notifPanel === name;
    p.classList.toggle("is-active", on);
    p.hidden = !on;
  });
}
document.querySelectorAll(".sub-tab").forEach(t => t.addEventListener("click", () => switchNotifSub(t.dataset.notifSub)));
document.querySelectorAll("[data-notif-sub-jump]").forEach(b => b.addEventListener("click", () => switchNotifSub(b.dataset.notifSubJump)));

/* bell dropdown */
function renderBellItems() {
  const list = document.getElementById("bellItems");
  if (!list) return;
  list.innerHTML = USER_NOTIFS.map(u => `
    <div class="ndrop__item ${u.unread ? "is-unread is-" + (u.level === "critical" ? "crit" : u.level) : ""} ${u.expired ? "is-expired" : ""}" data-notif-id="${u.id}">
      <span class="ndrop__lvldot" style="background:${u.level==='critical'?'#ff4d4d':u.level==='warn'?'#f5a623':'#52a8ff'}"></span>
      <div class="ndrop__body">
        <div class="ndrop__title">${escHtml(u.title)}</div>
        <div class="ndrop__excerpt">${escHtml(u.excerpt)}</div>
      </div>
      <span class="ndrop__time">${u.time}</span>
    </div>
  `).join("");
  list.querySelectorAll(".ndrop__item").forEach(el => {
    el.addEventListener("click", () => openNotifModal(el.dataset.notifId));
  });
}
renderBellItems();
function toggleBellDrop(force) {
  const d = document.getElementById("bellDrop");
  if (!d) return;
  const on = force !== undefined ? force : !d.classList.contains("is-open");
  d.classList.toggle("is-open", on);
}
document.getElementById("userBell")?.addEventListener("click", e => { e.stopPropagation(); toggleBellDrop(); });
document.addEventListener("click", e => {
  if (e.target.closest(".bell-wrap")) return;
  toggleBellDrop(false);
});

document.getElementById("markAllRead")?.addEventListener("click", e => {
  e.stopPropagation();
  USER_NOTIFS.forEach(u => u.unread = false);
  renderBellItems();
  refreshBellBadge();
  toast("已标记全部为已读");
});
document.getElementById("markAllRead2")?.addEventListener("click", () => {
  USER_NOTIFS.forEach(u => u.unread = false);
  renderUserNotifCenter();
  renderBellItems();
  refreshBellBadge();
  toast("已标记全部为已读");
});

function refreshBellBadge() {
  const unread = USER_NOTIFS.filter(u => u.unread).length;
  const badge = document.getElementById("bellBadge");
  if (!badge) return;
  const anyCrit = USER_NOTIFS.some(u => u.unread && u.level === "critical");
  badge.style.display = unread ? "flex" : "none";
  badge.textContent = unread > 9 ? "9+" : unread;
  badge.classList.toggle("bell__badge--crit", anyCrit);
  if (!anyCrit && unread > 0) badge.style.background = "var(--success)";
  if (anyCrit) badge.style.background = "";
}
refreshBellBadge();

/* user notification center */
function renderUserNotifCenter(filter = "all") {
  const list = document.getElementById("userNotifList");
  if (!list) return;
  const filtered = USER_NOTIFS.filter(u => filter === "all" || (filter === "unread" && u.unread) || (filter === "read" && !u.unread));
  list.innerHTML = filtered.map(u => `
    <div class="user-notif ${u.unread ? "is-unread is-" + (u.level === "critical" ? "crit" : u.level) : ""} ${u.expired ? "is-expired" : ""}" data-notif-id="${u.id}">
      <span class="user-notif__unread"></span>
      ${nlvlChip(u.level)}
      <div class="user-notif__body">
        <div class="user-notif__title">${escHtml(u.title)}${u.expired ? ' <span class="t-label tertiary">(已过期)</span>' : ""}</div>
        <div class="user-notif__excerpt">${escHtml(u.excerpt)}</div>
      </div>
      <span class="user-notif__time">${u.time}</span>
      <button class="user-notif__action">详情 →</button>
    </div>
  `).join("");
  list.querySelectorAll(".user-notif").forEach(el => {
    el.addEventListener("click", () => openNotifModal(el.dataset.notifId));
  });
}
renderUserNotifCenter();
document.querySelectorAll("[data-cfilter]").forEach(b => {
  b.addEventListener("click", () => {
    document.querySelectorAll("[data-cfilter]").forEach(x => x.classList.remove("is-selected"));
    b.classList.add("is-selected");
    renderUserNotifCenter(b.dataset.cfilter);
  });
});

/* ============== NOTIFICATION MODAL ============== */
function openNotifModal(id) {
  let item = USER_NOTIFS.find(u => u.id === id);
  let source = item;
  if (!item) {
    const n = NOTIFS.find(x => x.id === id);
    if (n) source = { ...n, time: n.publishStr };
  }
  if (!source) return;
  const ref = NOTIFS.find(x => x.id === id);
  const body = ref ? ref.body : (source.body || source.excerpt);
  document.getElementById("notifModalTitle").textContent = source.title;
  document.getElementById("notifModalLvl").innerHTML = nlvlChip(source.level);
  document.getElementById("notifModalMeta").textContent = `发布于 ${source.time || source.publishStr} · 来自 admin`;
  document.getElementById("notifModalBody").innerHTML = mdToHtml(body);
  document.getElementById("notifModalDismiss").hidden = !(source.dismissible !== false);
  document.getElementById("notifModalBd").classList.add("is-open");
  // mark as read
  if (item && item.unread) {
    item.unread = false;
    renderBellItems();
    renderUserNotifCenter(document.querySelector("[data-cfilter].is-selected")?.dataset.cfilter || "all");
    refreshBellBadge();
  }
}
function closeNotifModal() { document.getElementById("notifModalBd").classList.remove("is-open"); }
document.getElementById("notifModalBd")?.addEventListener("click", e => { if (e.target.id === "notifModalBd") closeNotifModal(); });
document.getElementById("notifModalClose")?.addEventListener("click", closeNotifModal);
document.getElementById("notifModalCloseFoot")?.addEventListener("click", closeNotifModal);
document.getElementById("notifModalDismiss")?.addEventListener("click", () => {
  toast("已忽略,不再提示");
  closeNotifModal();
});
document.querySelectorAll("[data-open-notif]").forEach(b => b.addEventListener("click", () => openNotifModal(b.dataset.openNotif)));

/* ============== CREATE NOTIFICATION MODAL ============== */
function openCreate() {
  document.getElementById("createNotifBd").classList.add("is-open");
}
function closeCreate() { document.getElementById("createNotifBd").classList.remove("is-open"); }
document.getElementById("openCreateNotif")?.addEventListener("click", openCreate);
document.querySelectorAll("[data-open-create-2]").forEach(b => b.addEventListener("click", openCreate));
document.getElementById("createNotifClose")?.addEventListener("click", closeCreate);
document.getElementById("cfCancel")?.addEventListener("click", closeCreate);
document.getElementById("createNotifBd")?.addEventListener("click", e => { if (e.target.id === "createNotifBd") closeCreate(); });
document.getElementById("cfPublish")?.addEventListener("click", () => {
  closeCreate();
  toast("通知已发布 · 已读率开始统计", "send");
});

document.querySelectorAll("[data-cf-level]").forEach(b => {
  b.addEventListener("click", () => {
    document.querySelectorAll("[data-cf-level]").forEach(x => x.classList.remove("is-selected"));
    b.classList.add("is-selected");
  });
});
document.querySelectorAll("[data-cf-target]").forEach(b => {
  b.addEventListener("click", () => {
    document.querySelectorAll("[data-cf-target]").forEach(x => x.classList.remove("is-selected"));
    b.classList.add("is-selected");
    const t = b.dataset.cfTarget;
    document.querySelectorAll("[data-target-panel]").forEach(p => p.hidden = p.dataset.targetPanel !== t);
  });
});
// preview toggle
document.getElementById("cfPreview")?.addEventListener("click", () => {
  toast("Markdown preview 切换 (原型中省略实现)", "eye");
});

// checkboxes
document.querySelectorAll(".chk").forEach(c => {
  if (c.dataset.rowChk !== undefined) return; // those handled by row events
  if (c.id === "bulkAll") return;
  c.addEventListener("click", () => c.classList.toggle("is-on"));
});

// switch
document.querySelectorAll(".switch__track").forEach(s => {
  s.addEventListener("click", () => s.classList.toggle("is-on"));
});

/* ============== CRITICAL BANNER (admin) ============== */
const critBanner = document.getElementById("critBannerAdmin");
function refreshCrit() {
  const anyCrit = USER_NOTIFS.some(u => u.unread && u.level === "critical");
  if (critBanner) critBanner.classList.toggle("is-shown", anyCrit);
}
refreshCrit();
document.querySelectorAll("[data-view-crit]").forEach(b => b.addEventListener("click", () => openNotifModal("ntf_003")));

/* ============================================================== */
/* ECHARTS                                                        */
/* ============================================================== */
const acharts = {};
window.addEventListener("resize", () => Object.values(acharts).forEach(c => c && c.resize && c.resize()));

let adminChartsInitialized = false;
function initAdminCharts() {
  if (adminChartsInitialized) return;
  // can only init when DOM is visible
  if (!document.querySelector(".admin-view.is-active")) return;
  buildAdminRev();
  buildAdminModel();
  buildAdminCalls();
  buildAdminHeat();
  buildAdminHealth();
  adminChartsInitialized = true;
}
function buildAdminRev() {
  const dom = document.getElementById("chartAdminRev");
  if (!dom || acharts.rev) return;
  const c = echarts.init(dom, "stellar");
  acharts.rev = c;
  const data = [240, 280, 305, 340, 320, 410, 535];
  c.setOption({
    grid: { left: 8, right: 12, top: 16, bottom: 22, containLabel: true },
    xAxis: { type: "category", data: ["d-6","d-5","d-4","d-3","d-2","d-1","今"], boundaryGap: false },
    yAxis: { type: "value", axisLabel: { formatter: "¥{value}" }, splitNumber: 3 },
    tooltip: { trigger: "axis", formatter: p => `<div style="font-family:Geist Mono">${p[0].name} &nbsp;<b>¥${p[0].value}</b></div>` },
    series: [{
      type: "line", smooth: true, showSymbol: false,
      data,
      lineStyle: { width: 1.5, color: "#0bd470" },
      areaStyle: { color: new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:"rgba(11,212,112,0.22)"},{offset:1,color:"rgba(11,212,112,0)"}]) },
      markPoint: { data: [{ type:"max", name:"max" }], symbol: "circle", symbolSize: 6, itemStyle:{ color:"#0bd470", borderColor:"#000", borderWidth:2 }, label:{ show:true, position:"top", color:"#ededed", fontSize:10, fontFamily:"Geist Mono", formatter: p=>"¥"+p.value }, },
    }],
  });
}
function buildAdminModel() {
  const dom = document.getElementById("chartAdminModel");
  if (!dom || acharts.model) return;
  const c = echarts.init(dom, "stellar");
  acharts.model = c;
  c.setOption({
    tooltip: { formatter: p => `<div style="font-family:Geist Mono">${p.name}&nbsp;<b>${p.value}</b>&nbsp;${p.percent}%</div>` },
    graphic: { type:"group", left:"center", top:"middle", children:[
      { type:"text", left:"center", top:"center", style: { text:"9.8k", fill:"#ededed", font:"600 22px Geist Mono", textAlign:"center" } },
      { type:"text", left:"center", top: 24, style: { text:"CALLS", fill:"#707070", font:"500 9px Geist", letterSpacing:1 } },
    ]},
    series: [{
      type: "pie", radius: ["55%", "78%"], center: ["50%", "50%"],
      itemStyle: { borderColor: "#000", borderWidth: 2 },
      label: { show: false }, emphasis: { scale: true, scaleSize: 4, label: { show: false } },
      data: [
        { value: 6210, name: "opus-4-7",   itemStyle: { color: "#0bd470" } },
        { value: 2743, name: "sonnet-4-6", itemStyle: { color: "#52a8ff" } },
        { value:  902, name: "haiku-4-5",  itemStyle: { color: "#f5a623" } },
      ],
    }],
  });
}
function buildAdminCalls() {
  const dom = document.getElementById("chartAdminCalls");
  if (!dom || acharts.calls) return;
  const c = echarts.init(dom, "stellar");
  acharts.calls = c;
  const calls = [820, 950, 1240, 880, 1432, 1660, 1455];
  const errs  = [0.5, 0.4, 0.7, 0.3, 0.6, 0.42, 0.42];
  c.setOption({
    grid: { left: 8, right: 32, top: 24, bottom: 22, containLabel: true },
    legend: { show: true, top: 0, right: 0, textStyle:{ color:"#a1a1a1", fontSize:10 }, data:[{name:"调用量", icon:"circle"}, {name:"错误率%", icon:"circle"}] },
    xAxis: { type: "category", data: ["周一","周二","周三","周四","周五","周六","周日"], boundaryGap: false },
    yAxis: [
      { type: "value", name: "调用", nameTextStyle:{color:"#707070",fontSize:10} },
      { type: "value", name: "错误率%", position: "right", nameTextStyle:{color:"#707070",fontSize:10}, axisLabel:{ formatter:"{value}%" } },
    ],
    tooltip: { trigger: "axis" },
    series: [
      { name: "调用量", type: "line", smooth: true, showSymbol: false, data: calls,
        lineStyle: { width: 1.5, color: "#0bd470" },
        areaStyle: { color: new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:"rgba(11,212,112,0.18)"},{offset:1,color:"rgba(11,212,112,0)"}]) }, yAxisIndex: 0 },
      { name: "错误率%", type: "line", smooth: true, showSymbol: false, data: errs,
        lineStyle: { width: 1.2, color: "#ff4d4d" }, yAxisIndex: 1 },
    ],
  });
}
function buildAdminHeat() {
  const dom = document.getElementById("chartAdminHeat");
  if (!dom || acharts.heat) return;
  const c = echarts.init(dom, "stellar");
  acharts.heat = c;
  const days = ["周日","周六","周五","周四","周三","周二","周一"];
  const data = [];
  for (let d=0;d<7;d++) for (let h=0;h<24;h++) {
    let base = h >= 9 && h <= 23 ? 12 : 2;
    if (h>=2 && h<=4) base = 25; // peak per data note
    let v = Math.floor(base + Math.random()*15);
    if (d <= 1 && h >= 13 && h <= 22) v += 6;
    data.push([h, d, v]);
  }
  c.setOption({
    grid: { left: 28, right: 8, top: 8, bottom: 24 },
    tooltip: { formatter: p => `<div style="font-family:Geist Mono">${days[p.value[1]]} ${String(p.value[0]).padStart(2,"0")}:00<br/><b>${p.value[2]}</b> calls</div>` },
    xAxis: { type:"category", data: Array.from({length:24},(_,i)=>String(i).padStart(2,"0")), axisLine:{show:false}, axisLabel:{color:"#707070", fontSize:9} },
    yAxis: { type:"category", data: days, axisLine:{show:false}, axisLabel:{color:"#707070", fontSize:9} },
    visualMap: { min:0, max:45, show:false, inRange:{ color: ["rgba(11,212,112,0.04)","rgba(11,212,112,0.35)","#0bd470"] } },
    series: [{ type:"heatmap", data, itemStyle:{ borderRadius:2, borderColor:"#000", borderWidth:1 } }],
  });
}
function buildAdminHealth() {
  const list = document.getElementById("adminHealth");
  if (!list || list.children.length) return;
  const ups = [
    { name:"apijing",     status:"ok",   color:"#0bd470", lat:142, err:"0.2%", errChip:"chip--mono" },
    { name:"aws稳定(自营)", status:"ok",   color:"#0bd470", lat:88,  err:"0.0%", errChip:"chip--mono" },
    { name:"cc",          status:"warn", color:"#f5a623", lat:320, err:"4.1%", errChip:"chip--warning" },
    { name:"kiro高缓存",   status:"ok",   color:"#0bd470", lat:223, err:"0.0%", errChip:"chip--mono" },
    { name:"反重力pro",    status:"ok",   color:"#0bd470", lat:178, err:"0.5%", errChip:"chip--mono" },
  ];
  list.innerHTML = ups.map((u,i) => `
    <div class="health-row">
      <span class="dot ${u.status==='ok'?'dot--green dot--pulse':'dot--yellow'}"></span>
      <span class="health-row__name">${u.name}</span>
      <span class="health-row__lat">${u.lat}ms</span>
      <span class="chip ${u.errChip}">${u.err}</span>
      <div class="health-row__spark" id="admHealth-${i}"></div>
    </div>
  `).join("");
  ups.forEach((u, i) => {
    const dom = document.getElementById("admHealth-" + i);
    if (!dom) return;
    const c = echarts.init(dom, "stellar");
    acharts["admHealth" + i] = c;
    const data = Array.from({length:24}, () => Math.max(20, Math.round(u.lat + (Math.random()-0.5)*u.lat*0.5)));
    const rgb = u.color === "#0bd470" ? "11,212,112" : "245,166,35";
    c.setOption({
      animation: false,
      grid: { left: 0, right: 0, top: 2, bottom: 2 },
      xAxis: { show: false, type: "category", data: Array(24).fill("") },
      yAxis: { show: false, type: "value" },
      tooltip: { show: false },
      series: [{ type:"line", smooth:true, symbol:"none", data,
        lineStyle:{ width:1, color: u.color },
        areaStyle:{ color: new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:`rgba(${rgb},0.25)`},{offset:1,color:`rgba(${rgb},0)`}]) },
      }],
    });
  });
}

function buildNotifStatsChart() {
  const dom = document.getElementById("chartNotifStats");
  if (!dom) return;
  if (acharts.notifStats) acharts.notifStats.dispose();
  const c = echarts.init(dom, "stellar");
  acharts.notifStats = c;
  // 24h reads (every hour)
  const data = [0, 0, 4, 6, 3, 1, 0, 1, 0, 0, 2, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
  c.setOption({
    grid: { left: 0, right: 12, top: 12, bottom: 18, containLabel: true },
    xAxis: { type:"category", data: Array.from({length:24},(_,i)=>String(i).padStart(2,"0")), axisLine:{show:false}, axisLabel:{color:"#707070", fontSize:9} },
    yAxis: { show: false, type:"value" },
    tooltip: { trigger:"axis", formatter: p => `<div style="font-family:Geist Mono">${p[0].name}:00 &nbsp;<b>${p[0].value}</b></div>` },
    series: [{ type:"line", smooth:true, symbol:"none", data,
      lineStyle:{ width: 1.2, color: "#0bd470" },
      areaStyle: { color: new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:"rgba(11,212,112,0.30)"},{offset:1,color:"rgba(11,212,112,0)"}]) },
    }],
  });
}

let notifSecCharts = false;
function initNotifSectionCharts() {
  if (notifSecCharts) return;
  notifSecCharts = true;
  // no admin acharts in notif section; placeholder for future
}

/* ============== INIT ============== */
function init() {
  refreshIcons();
  // counter animations
  document.querySelectorAll("[data-counter]").forEach(el => {
    const target = parseFloat(el.dataset.counter);
    const prefix = el.dataset.prefix || "";
    const suffix = el.dataset.suffix || "";
    const decimals = parseInt(el.dataset.decimals || "0", 10);
    const start = performance.now();
    function step(now) {
      const t = Math.min(1, (now - start) / 900);
      const e = 1 - Math.pow(1 - t, 3);
      const v = target * e;
      el.textContent = prefix + v.toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals }) + suffix;
      if (t < 1) requestAnimationFrame(step);
      else el.textContent = prefix + target.toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals }) + suffix;
    }
    requestAnimationFrame(step);
  });
  // start with login → singularity
  setTimeout(() => orbScene.start(), 200);
}
document.addEventListener("DOMContentLoaded", init);
if (document.readyState !== "loading") init();
