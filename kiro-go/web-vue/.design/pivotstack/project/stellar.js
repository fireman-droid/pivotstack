/* ============================================================
   Stellar Console · PivotStack User · stellar.js
   - starfield generator
   - tab switching
   - ECharts stellar theme + B1..B8
   - counters, copy, ticker, drawer, etc
   ============================================================ */

/* ============== 1. STARFIELD ============== */
(function buildStarfield() {
  const staticLayer = document.querySelector(".starfield__static");
  // Rotating layer is 200vmax square; render stars across that whole area
  // so rotation never reveals an edge.
  const w = 3200, h = 3200;
  const layers = [
    { count: 600, size: 1, alpha: 0.55 },
    { count: 350, size: 1, alpha: 0.72 },
    { count: 80,  size: 2, alpha: 0.88 },
  ];

  function rand(min, max) { return Math.random() * (max - min) + min; }
  function pickPos() { return [rand(0, w), rand(0, h)]; }

  const cvs = document.createElement("canvas");
  cvs.width = w; cvs.height = h;
  const ctx = cvs.getContext("2d");
  layers.forEach(({ count, size, alpha }) => {
    ctx.fillStyle = `rgba(255,255,255,${alpha})`;
    for (let i = 0; i < count; i++) {
      const [x, y] = pickPos();
      ctx.beginPath();
      ctx.arc(x, y, size / 2, 0, Math.PI * 2);
      ctx.fill();
    }
  });
  staticLayer.style.background = `url(${cvs.toDataURL("image/png")}) center / cover no-repeat, #000`;

  // animated twinkling stars placed across the full rotating area
  const tl = document.getElementById("twinkleLayer");
  const rotSize = Math.max(window.innerWidth, window.innerHeight) * 2;
  const N = 42;
  for (let i = 0; i < N; i++) {
    const s = document.createElement("i");
    s.className = "star";
    if (Math.random() < 0.18) s.classList.add("star--big");
    if (Math.random() < 0.06) { s.classList.add("star--xl"); s.classList.remove("star--big"); }
    s.style.left = rand(0, rotSize) + "px";
    s.style.top  = rand(0, rotSize) + "px";
    s.style.animationDuration = rand(3, 8) + "s";
    s.style.animationDelay    = rand(0, 5) + "s";
    tl.appendChild(s);
  }
})();

/* ============== 1b. METEORS ============== */
(function meteorSpawner() {
  const layer = document.getElementById("meteorLayer");
  if (!layer) return;
  function fire() {
    const m = document.createElement("div");
    m.className = "meteor";
    const vw = window.innerWidth, vh = window.innerHeight;
    // start position: anywhere in the upper-2/3 of viewport, away from dead center
    const startX = Math.random() * vw * 0.7 + (Math.random() < 0.5 ? 0 : vw * 0.2);
    const startY = Math.random() * vh * 0.55;
    // travel ~30deg below horizontal, randomly left-to-right or right-to-left
    const goLeft = startX > vw * 0.55; // bias direction toward away from edge
    const dir = goLeft ? -1 : 1;
    const angleDeg = 18 + Math.random() * 22;          // 18..40
    const angleRad = angleDeg * Math.PI / 180;
    const dist = Math.max(vw, vh) * 0.8;
    const dx = Math.cos(angleRad) * dist * dir;
    const dy = Math.sin(angleRad) * dist;
    // head of streak (right end of element) should point in (dx, dy)
    const rotateDeg = Math.atan2(dy, dx) * 180 / Math.PI;

    m.style.left = startX + "px";
    m.style.top  = startY + "px";
    m.style.rotate = rotateDeg + "deg";          // individual property — preserved by keyframes
    m.style.setProperty("--mx", dx + "px");
    m.style.setProperty("--my", dy + "px");
    layer.appendChild(m);
    requestAnimationFrame(() => m.classList.add("is-fire"));
    setTimeout(() => m.remove(), 1800);
  }
  function loop() {
    fire();
    const next = 12000 + Math.random() * 13000;   // 12-25s
    setTimeout(loop, next);
  }
  setTimeout(loop, 4000);
})();

/* ============== 2. ECHARTS STELLAR THEME ============== */
const stellarTheme = {
  color: ["#0bd470", "#52a8ff", "#f5a623", "#ff4d4d", "#a1a1a1", "#ededed"],
  backgroundColor: "transparent",
  textStyle: {
    color: "#a1a1a1",
    fontFamily: 'Geist, Inter, "PingFang SC", sans-serif',
    fontSize: 11,
  },
  title: { show: false },
  legend: {
    textStyle: { color: "#a1a1a1", fontSize: 11 },
    icon: "circle",
    itemWidth: 6, itemHeight: 6, itemGap: 16,
  },
  grid: { left: 0, right: 0, top: 12, bottom: 20, containLabel: true, borderColor: "transparent" },
  xAxis: {
    axisLine: { lineStyle: { color: "rgba(255,255,255,0.08)" } },
    axisTick: { show: false },
    axisLabel: { color: "#707070", fontSize: 10, margin: 12 },
    splitLine: { show: false },
  },
  yAxis: {
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: { color: "#707070", fontSize: 10 },
    splitLine: { lineStyle: { color: "rgba(255,255,255,0.04)", type: "dashed" } },
  },
  tooltip: {
    backgroundColor: "rgba(10,10,10,0.96)",
    borderColor: "rgba(255,255,255,0.10)",
    borderWidth: 1,
    padding: [8, 12],
    textStyle: { color: "#ededed", fontSize: 12, fontFamily: "Geist Mono, monospace" },
    extraCssText: "backdrop-filter: blur(8px); border-radius: 6px;",
  },
  line: {
    smooth: true,
    lineStyle: { width: 1.5 },
    symbol: "circle",
    symbolSize: 3,
    showSymbol: false,
  },
  bar: { itemStyle: { borderRadius: [3, 3, 0, 0] } },
};
echarts.registerTheme("stellar", stellarTheme);

/* ============== 3. CHART BUILDERS ============== */
const charts = {};
function resizeAll() { Object.values(charts).forEach(c => c && c.resize && c.resize()); }
window.addEventListener("resize", resizeAll);

// ---- B1: 7d trend line + area ----
function buildB1() {
  if (charts.b1) return;
  const dom = document.getElementById("chartB1");
  if (!dom) return;
  const c = echarts.init(dom, "stellar");
  charts.b1 = c;
  const data = [82, 95, 120, 88, 142, 156, 142];
  c.setOption({
    grid: { left: 8, right: 24, top: 24, bottom: 24, containLabel: true },
    xAxis: {
      type: "category",
      data: ["周一", "周二", "周三", "周四", "周五", "周六", "周日"],
      boundaryGap: false,
    },
    yAxis: { type: "value", splitNumber: 3 },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "line", lineStyle: { color: "rgba(255,255,255,0.15)", type: "dashed" } },
      formatter: p => {
        const v = p[0]; const prev = data[v.dataIndex - 1];
        const diff = prev ? Math.round(((v.value - prev) / prev) * 100) : 0;
        const arrow = diff >= 0 ? "↗" : "↘";
        const cls = diff >= 0 ? "#0bd470" : "#ff4d4d";
        return `<div style="font-family:Geist Mono">${v.name}&nbsp;&nbsp;<b>${v.value}</b> calls${prev?`&nbsp;&nbsp;<span style="color:${cls}">${arrow} ${diff>=0?'+':''}${diff}%</span>`:""}</div>`;
      },
    },
    series: [{
      type: "line",
      smooth: true,
      data,
      lineStyle: { width: 1.5, color: "#0bd470" },
      itemStyle: { color: "#0bd470" },
      showSymbol: false,
      emphasis: { showSymbol: true, itemStyle: { color: "#0bd470", borderColor: "#000", borderWidth: 2 } },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: "rgba(11,212,112,0.22)" },
          { offset: 1, color: "rgba(11,212,112,0)" },
        ]),
      },
      markPoint: {
        data: [{ type: "max", name: "峰值" }],
        symbol: "circle", symbolSize: 6,
        itemStyle: { color: "#0bd470", borderColor: "#000", borderWidth: 2 },
        label: {
          show: true, position: "top",
          color: "#ededed", fontSize: 11, fontFamily: "Geist Mono",
          formatter: "{c}",
        },
      },
    }],
  });
}

// ---- B2: donut ----
function buildB2() {
  if (charts.b2) return;
  const dom = document.getElementById("chartB2");
  if (!dom) return;
  const c = echarts.init(dom, "stellar");
  charts.b2 = c;
  c.setOption({
    tooltip: {
      formatter: p => `<div style="font-family:Geist Mono">${p.name}&nbsp;&nbsp;<b>${p.value}</b>&nbsp;&nbsp;${p.percent}%</div>`,
    },
    legend: { show: false },
    graphic: {
      type: "group",
      left: "center", top: "middle",
      children: [
        { type: "text", left: "center", top: "center",
          style: { text: "850", fill: "#ededed", font: '600 28px Geist Mono', textAlign: "center" } },
        { type: "text", left: "center", top: 28,
          style: { text: "CALLS", fill: "#707070", font: '500 9px Geist', letterSpacing: 1 } },
      ],
    },
    series: [{
      type: "pie",
      radius: ["55%", "78%"],
      center: ["50%", "50%"],
      avoidLabelOverlap: false,
      itemStyle: { borderColor: "#000", borderWidth: 2 },
      label: { show: false },
      emphasis: { scale: true, scaleSize: 4, label: { show: false } },
      data: [
        { value: 528, name: "opus-4-7",   itemStyle: { color: "#0bd470" } },
        { value: 238, name: "sonnet-4-6", itemStyle: { color: "#52a8ff" } },
        { value: 84,  name: "haiku-4-5",  itemStyle: { color: "#f5a623" } },
      ],
    }],
  });

  // legend hover -> highlight slice
  document.querySelectorAll(".donut-legend__item").forEach(el => {
    el.addEventListener("mouseenter", () => {
      c.dispatchAction({ type: "highlight", seriesIndex: 0, dataIndex: +el.dataset.idx });
    });
    el.addEventListener("mouseleave", () => {
      c.dispatchAction({ type: "downplay", seriesIndex: 0, dataIndex: +el.dataset.idx });
    });
  });
}

// ---- B3: mini sparklines for each upstream ----
const upstreams = [
  { name: "apijing",       status: "ok",   color: "#0bd470", lat: 142, err: "0.2%", errKind: null },
  { name: "aws稳定(自营)",  status: "ok",   color: "#0bd470", lat: 88,  err: "0.0%", errKind: null },
  { name: "cc",            status: "warn", color: "#f5a623", lat: 320, err: "4.1%", errKind: "warning" },
  { name: "kiro高缓存",     status: "ok",   color: "#0bd470", lat: 223, err: "0.0%", errKind: null },
  { name: "反重力pro",      status: "ok",   color: "#0bd470", lat: 178, err: "0.5%", errKind: null },
];
function genLat(base, jitter, n = 24) {
  const arr = [];
  for (let i = 0; i < n; i++) arr.push(Math.max(20, Math.round(base + (Math.random() - 0.5) * jitter)));
  return arr;
}
function buildHealthList() {
  const list = document.getElementById("healthList");
  if (!list || list.children.length) return;
  upstreams.forEach((u, i) => {
    const row = document.createElement("div");
    row.className = "health-row";
    const dotColor = u.status === "ok" ? "dot--green" : u.status === "warn" ? "dot--yellow" : "dot--red";
    row.innerHTML = `
      <span class="dot ${dotColor} ${u.status === 'ok' ? 'dot--pulse' : ''}"></span>
      <span class="health-row__name">${u.name}</span>
      <span class="health-row__lat">${u.lat}ms</span>
      <span class="chip ${u.errKind === 'warning' ? 'chip--warning' : 'chip--mono'}">${u.err}</span>
      <div class="health-row__spark" id="spark-${i}"></div>
    `;
    list.appendChild(row);
  });
  // mount sparklines
  upstreams.forEach((u, i) => {
    const dom = document.getElementById("spark-" + i);
    if (!dom) return;
    const c = echarts.init(dom, "stellar");
    charts["spark" + i] = c;
    const data = genLat(u.lat, u.lat * 0.4);
    c.setOption({
      animation: false,
      grid: { left: 0, right: 0, top: 2, bottom: 2 },
      xAxis: { show: false, type: "category", data: Array(24).fill("") },
      yAxis: { show: false, type: "value" },
      tooltip: { show: false },
      series: [{
        type: "line", smooth: true, symbol: "none",
        data,
        lineStyle: { width: 1, color: u.color },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: hexToRgba(u.color, 0.25) },
            { offset: 1, color: hexToRgba(u.color, 0) },
          ]),
        },
      }],
    });
  });
}
function hexToRgba(hex, a) {
  const m = hex.replace("#", "");
  const r = parseInt(m.substring(0,2), 16);
  const g = parseInt(m.substring(2,4), 16);
  const b = parseInt(m.substring(4,6), 16);
  return `rgba(${r},${g},${b},${a})`;
}

// ---- B4: heatmap ----
function buildB4() {
  if (charts.b4) return;
  const dom = document.getElementById("chartB4");
  if (!dom) return;
  const c = echarts.init(dom, "stellar");
  charts.b4 = c;

  const days = ["周日","周六","周五","周四","周三","周二","周一"];
  const data = [];
  for (let d = 0; d < 7; d++) {
    for (let h = 0; h < 24; h++) {
      let base = h >= 9 && h <= 23 ? 8 : 1;
      let val = Math.floor(base + Math.random() * 15);
      // weekend skew
      if (d <= 1 && h >= 13 && h <= 22) val += 6;
      data.push([h, d, val]);
    }
  }

  c.setOption({
    grid: { left: 36, right: 12, top: 8, bottom: 28, containLabel: false },
    tooltip: {
      formatter: p => `<div style="font-family:Geist Mono">${days[p.value[1]]} ${String(p.value[0]).padStart(2,"0")}:00<br/><b>${p.value[2]}</b> calls</div>`,
    },
    xAxis: {
      type: "category",
      data: Array.from({ length: 24 }, (_, i) => String(i).padStart(2, "0")),
      splitArea: { show: false },
      axisLine: { show: false },
      axisLabel: { color: "#707070", fontSize: 10 },
    },
    yAxis: {
      type: "category",
      data: days,
      splitArea: { show: false },
      axisLine: { show: false },
      axisLabel: { color: "#707070", fontSize: 10 },
    },
    visualMap: {
      min: 0, max: 30,
      show: false,
      calculable: false,
      inRange: { color: ["rgba(11,212,112,0.04)", "rgba(11,212,112,0.35)", "#0bd470"] },
    },
    series: [{
      type: "heatmap",
      data,
      itemStyle: { borderRadius: 2, borderColor: "#000", borderWidth: 1 },
      emphasis: { itemStyle: { borderColor: "#0bd470", borderWidth: 1 } },
    }],
  });
}

// ---- B5: stat ribbon mini charts ----
function buildB5() {
  // 24 buckets for each
  const calls = Array.from({ length: 24 }, () => Math.floor(Math.random() * 12 + 2));
  const lats  = Array.from({ length: 24 }, () => Math.round(120 + (Math.random() - 0.5) * 80));
  const errs  = Array.from({ length: 24 }, () => Math.random() < 0.85 ? 0 : Math.random() * 3);

  function mini(domId, data, color, type) {
    const dom = document.getElementById(domId);
    if (!dom || charts[domId]) return;
    const c = echarts.init(dom, "stellar");
    charts[domId] = c;
    const opt = {
      animation: false,
      grid: { left: 0, right: 0, top: 2, bottom: 2 },
      xAxis: { show: false, type: "category", data: Array(24).fill("") },
      yAxis: { show: false, type: "value" },
      tooltip: { show: false },
      series: [{
        type: type,
        data,
        smooth: type === "line",
        symbol: "none",
        barWidth: type === "bar" ? "70%" : undefined,
        itemStyle: { color, borderRadius: type === "bar" ? [2, 2, 0, 0] : 0 },
        lineStyle: type === "line" ? { width: 1, color } : undefined,
        areaStyle: type === "line" ? {
          color: new echarts.graphic.LinearGradient(0,0,0,1,[
            { offset: 0, color: hexToRgba(color, 0.25) },
            { offset: 1, color: hexToRgba(color, 0) },
          ]),
        } : undefined,
      }],
    };
    c.setOption(opt);
  }
  mini("chartB5a", calls, "#0bd470", "bar");
  mini("chartB5b", lats,  "#52a8ff", "line");
  mini("chartB5c", errs,  "#ff4d4d", "bar");
}

// ---- B6: latency histogram (drawer) ----
function buildB6() {
  const dom = document.getElementById("chartB6");
  if (!dom) return;
  if (charts.b6) charts.b6.dispose();
  const c = echarts.init(dom, "stellar");
  charts.b6 = c;
  const buckets = [4, 12, 28, 48, 64, 52, 30, 18, 9, 3];
  const currentBucket = 4;
  c.setOption({
    animation: true,
    grid: { left: 0, right: 0, top: 18, bottom: 18, containLabel: false },
    xAxis: {
      type: "category",
      data: ["0", "50", "100", "150", "200", "250", "300", "400", "500", "1s+"],
      axisLabel: { color: "#707070", fontSize: 9 },
      axisLine: { show: false },
    },
    yAxis: { show: false, type: "value" },
    tooltip: { show: false },
    series: [{
      type: "bar",
      barWidth: "65%",
      data: buckets.map((v, i) => ({
        value: v,
        itemStyle: { color: i === currentBucket ? "#0bd470" : "rgba(255,255,255,0.18)" },
        label: i === currentBucket ? {
          show: true, position: "top",
          color: "#0bd470", fontSize: 10, fontFamily: "Geist Mono",
          formatter: "本次",
        } : { show: false },
      })),
    }],
  });
}

// ---- B7: profit 30d ----
function buildB7() {
  if (charts.b7) return;
  const dom = document.getElementById("chartB7");
  if (!dom) return;
  const c = echarts.init(dom, "stellar");
  charts.b7 = c;
  const data = Array.from({ length: 30 }, (_, i) => {
    return +((Math.sin(i / 4) + 1.4) * 0.05 + Math.random() * 0.04 + 0.05).toFixed(3);
  });
  c.setOption({
    grid: { left: 8, right: 30, top: 18, bottom: 22, containLabel: true },
    xAxis: {
      type: "category",
      data: Array.from({ length: 30 }, (_, i) => `d${i + 1}`),
      boundaryGap: false,
      axisLabel: { interval: 5 },
    },
    yAxis: { type: "value", axisLabel: { formatter: "${value}" } },
    tooltip: {
      trigger: "axis",
      formatter: p => `<div style="font-family:Geist Mono">${p[0].name}&nbsp;&nbsp;<b>$${p[0].value.toFixed(2)}</b></div>`,
    },
    series: [{
      type: "line", smooth: true, showSymbol: false,
      data,
      lineStyle: { width: 1.5, color: "#0bd470" },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: "rgba(11,212,112,0.22)" },
          { offset: 1, color: "rgba(11,212,112,0)" },
        ]),
      },
      markLine: {
        symbol: "none",
        lineStyle: { color: "rgba(255,255,255,0.20)", type: "dashed", width: 1 },
        label: {
          position: "end",
          color: "#a1a1a1", fontSize: 10, fontFamily: "Geist Mono",
          formatter: p => `均值 $${p.value.toFixed(2)}`,
        },
        data: [{ type: "average" }],
      },
    }],
  });
}

// ---- B8: subkey balance horizontal bar ----
function buildB8() {
  if (charts.b8) return;
  const dom = document.getElementById("chartB8");
  if (!dom) return;
  const c = echarts.init(dom, "stellar");
  charts.b8 = c;
  const items = [
    { name: "agent-team-a",   v: 2.40 },
    { name: "client-acme",    v: 1.80 },
    { name: "test-sandbox",   v: 0.50 },
    { name: "client-zenith",  v: 3.20 },
    { name: "archive-2026q1", v: 0.00 },
  ];
  c.setOption({
    grid: { left: 10, right: 50, top: 4, bottom: 4, containLabel: true },
    xAxis: { show: false, type: "value" },
    yAxis: {
      type: "category",
      data: items.map(i => i.name),
      inverse: true,
      axisLine: { show: false },
      axisLabel: { color: "#a1a1a1", fontSize: 11, fontFamily: "Geist Mono" },
    },
    tooltip: {
      formatter: p => `<div style="font-family:Geist Mono">${p.name}<br/><b>$${p.value.toFixed(2)}</b></div>`,
    },
    series: [{
      type: "bar",
      data: items.map(i => ({
        value: i.v,
        itemStyle: { color: i.v === 0 ? "#4d4d4d" : "#0bd470", borderRadius: [0, 3, 3, 0] },
      })),
      barWidth: 10,
      label: {
        show: true, position: "right",
        color: "#ededed", fontSize: 11, fontFamily: "Geist Mono",
        formatter: p => "$" + p.value.toFixed(2),
      },
    }],
  });
}

/* ============== 4. TAB SWITCHING ============== */
const tabInitMap = {
  dashboard: () => { buildB1(); buildB2(); buildB4(); buildHealthList(); renderRecentCalls(); },
  recharge: () => { renderRechargeList(); },
  logs:     () => { buildB5(); renderLogsTable(); buildB6(); },
  api:      () => {},
  reseller: () => { buildB7(); buildB8(); renderSubkeyTable(); },
};
function switchTab(name) {
  document.querySelectorAll(".nav__item").forEach(b => b.classList.toggle("is-active", b.dataset.tab === name));
  document.querySelectorAll(".tab").forEach(t => t.classList.toggle("is-active", t.dataset.tabPanel === name));
  if (tabInitMap[name]) tabInitMap[name]();
  requestAnimationFrame(() => { setTimeout(resizeAll, 50); setTimeout(resizeAll, 450); });
}
document.querySelectorAll(".nav__item").forEach(b => {
  b.addEventListener("click", () => switchTab(b.dataset.tab));
});
document.querySelectorAll("[data-go]").forEach(b => {
  b.addEventListener("click", () => switchTab(b.dataset.go));
});

/* ============== 5. RECENT CALLS · LATEST 10 ============== */
const RECENT_CALLS = [
  ["10:42:31","abc12345","claude-opus-4-7","aws稳定",   3512,728,4240,0.12,142,"ok"],
  ["10:39:18","def67890","claude-sonnet-4-6","cc",       824,252,1076,0.03,95,"ok"],
  ["10:38:02","ghi23456","claude-opus-4-7","aws稳定",   6521,1234,7755,0.25,188,"ok"],
  ["10:32:45","jkl78901","claude-opus-4-7","aws稳定",   2432,689,3121,0.09,126,"ok"],
  ["10:30:11","mno34567","claude-haiku-4-5","apijing",  482,158,640,0.006,62,"ok"],
  ["10:28:53","pqr89012","claude-opus-4-7","aws稳定",   4820,982,5802,0.18,204,"ok"],
  ["10:25:01","stu45678","claude-sonnet-4-6","cc",      1201,320,1521,0.05,287,"warn"],
  ["10:22:14","vwx90123","claude-opus-4-7","aws稳定",   3889,712,4601,0.14,151,"ok"],
  ["10:19:47","yz123456","claude-haiku-4-5","apijing",  321,98,419,0.004,58,"ok"],
  ["10:18:32","abc78901","claude-opus-4-7","aws稳定",   5210,1012,6222,0.22,178,"ok"],
];
function fmtNum(n) { return n.toLocaleString("en-US"); }
function dollar(n) { return "$" + (n < 0.01 ? n.toFixed(3) : n.toFixed(2)); }
function statusDot(s) {
  if (s === "ok") return `<span class="dot dot--green"></span>`;
  if (s === "warn") return `<span class="dot dot--yellow"></span>`;
  if (s === "err") return `<span class="dot dot--red"></span>`;
  return `<span class="dot"></span>`;
}
function latencyColor(ms) {
  if (ms < 150) return "num--green";
  if (ms < 280) return "";
  return "num--warn";
}
function renderRecentCalls() {
  const tbody = document.getElementById("recentCallsTable");
  if (!tbody || tbody.children.length) return;
  tbody.innerHTML = RECENT_CALLS.map(r => rowHtml(r, false)).join("");
}
function rowHtml(r, withChev) {
  const [time, rid, model, upstream, inT, outT, total, cost, lat, status] = r;
  return `
    <div class="table__row" data-rid="${rid}">
      <div class="time" style="width:80px">${time}</div>
      <div style="width:120px"><span class="mono-id">${rid}<i data-lucide="copy" style="width:10px;height:10px;opacity:.5"></i></span></div>
      <div style="flex:1">${model}</div>
      <div style="width:100px"><span class="chip chip--mono">${upstream}</span></div>
      <div class="num" style="width:64px;text-align:right">${fmtNum(inT)}</div>
      <div class="num" style="width:64px;text-align:right">${fmtNum(outT)}</div>
      <div class="num num--strong" style="width:72px;text-align:right">${fmtNum(total)}</div>
      <div class="num num--green" style="width:72px;text-align:right">${dollar(cost)}</div>
      <div class="num ${latencyColor(lat)}" style="width:80px;text-align:right">${lat}ms</div>
      <div style="width:40px;text-align:center">${statusDot(status)}</div>
      ${withChev ? '<i class="row-chev" data-lucide="chevron-right"></i>' : ''}
    </div>
  `;
}

/* ============== 6. RECHARGE TABLE ============== */
const RECHARGE_HIST = [
  ["2026-05-15", "微信",   52,   "ok"],
  ["2026-05-12", "兑换码", 10,   "ok"],
  ["2026-05-08", "支付宝", 108,  "ok"],
  ["2026-04-30", "微信",   52,   "ok"],
  ["2026-04-22", "支付宝", 10,   "ok"],
  ["2026-04-12", "微信",   108,  "ok"],
  ["2026-03-30", "兑换码", 50,   "ok"],
  ["2026-03-18", "微信",   10,   "ok"],
];
function renderRechargeList() {
  const list = document.getElementById("rechargeList");
  if (!list || list.children.length) return;
  list.innerHTML = RECHARGE_HIST.map(([d, m, a, s]) => `
    <div class="recharge-row">
      <span class="time">${d}</span>
      <span class="chip chip--mono">${m}</span>
      <span class="amt">+$${a.toFixed(2)}</span>
      <span>${statusDot(s)}</span>
    </div>
  `).join("");
}

/* ============== 7. LOGS TABLE (40 rows) ============== */
function genLogsData() {
  const models = ["claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5"];
  const ups    = ["aws稳定", "cc", "apijing", "kiro高缓存", "反重力pro"];
  const out = [];
  let hh = 10, mm = 42, ss = 31;
  for (let i = 0; i < 40; i++) {
    if (i > 0) {
      const sec = Math.floor(20 + Math.random() * 200);
      ss -= sec;
      while (ss < 0) { ss += 60; mm -= 1; }
      while (mm < 0) { mm += 60; hh -= 1; }
      if (hh < 0) hh = 0;
    }
    const time = `${String(hh).padStart(2,"0")}:${String(mm).padStart(2,"0")}:${String(ss).padStart(2,"0")}`;
    const rid = Math.random().toString(36).substring(2, 10);
    const model = models[Math.floor(Math.random() * models.length)];
    const up = ups[Math.floor(Math.random() * ups.length)];
    const inT = Math.floor(400 + Math.random() * 7000);
    const outT = Math.floor(80 + Math.random() * 1400);
    const total = inT + outT;
    const cost = +(total * 0.00003 + 0.001).toFixed(model === "claude-haiku-4-5" ? 4 : 3);
    const lat = Math.floor(50 + Math.random() * 350);
    let status = "ok";
    if (Math.random() < 0.08) status = "warn";
    if (Math.random() < 0.02) status = "err";
    out.push([time, rid, model, up, inT, outT, total, cost, lat, status]);
  }
  return out;
}
const LOGS_DATA = genLogsData();
function renderLogsTable() {
  const tbody = document.getElementById("logsTable");
  if (!tbody || tbody.children.length) return;
  tbody.innerHTML = LOGS_DATA.map(r => rowHtml(r, true)).join("");
  if (window.lucide) lucide.createIcons();
  // row click -> drawer
  tbody.querySelectorAll(".table__row").forEach(row => {
    row.addEventListener("click", e => {
      if (e.target.closest(".mono-id")) return;
      const rid = row.dataset.rid;
      const data = LOGS_DATA.find(r => r[1] === rid);
      if (!data) return;
      document.getElementById("drawerRid").textContent = rid + "-...-e3a2";
      document.getElementById("drawerTime").textContent = "2026-05-17 " + data[0] + " UTC+8";
      document.getElementById("drawerModel").textContent = data[2];
      document.getElementById("drawerUp").textContent = data[3];
      document.getElementById("drawerRetry").textContent = Math.random() < 0.85 ? "否 · 1 次命中" : "是 · 重试 1 次";
      buildB6();
    });
  });
}

/* ============== 8. SUBKEY TABLE ============== */
const SUBKEYS = [
  ["agent-team-a",   2.40, 282, "2 分钟前", "ok"],
  ["client-acme",    1.80, 142, "5 分钟前", "ok"],
  ["test-sandbox",   0.50,  88, "2 小时前", "ok"],
  ["client-zenith",  3.20, 421, "1 天前",   "ok"],
  ["mobile-prod",    1.10, 187, "3 小时前", "ok"],
  ["batch-runner",   2.20, 612, "12 分钟前","warn"],
  ["webhook-relay",  0.90,  42, "6 小时前", "ok"],
  ["archive-2026q1", 0.00,   0, "30 天前",  "gray"],
];
function renderSubkeyTable() {
  const tbody = document.getElementById("subkeyTable");
  if (!tbody || tbody.children.length) return;
  tbody.innerHTML = SUBKEYS.map(([alias, bal, calls, when, status]) => {
    const sCls = status === "ok" ? "dot--green" : status === "warn" ? "dot--yellow" : "dot--gray";
    return `
      <div class="table__row">
        <div style="flex:1;min-width:140px"><span class="t-body-strong mono">${alias}</span></div>
        <div class="num num--strong" style="width:80px;text-align:right">$${bal.toFixed(2)}</div>
        <div class="num" style="width:96px;text-align:right">${fmtNum(calls)}</div>
        <div class="time" style="width:96px">${when}</div>
        <div style="width:40px;text-align:center"><span class="dot ${sCls}"></span></div>
        <div style="width:96px;display:flex;gap:4px">
          <button class="btn btn--ghost btn--icon btn--sm" title="充值"><i data-lucide="plus"></i></button>
          <button class="btn btn--ghost btn--icon btn--sm" title="暂停"><i data-lucide="pause"></i></button>
          <button class="btn btn--ghost btn--icon btn--sm" title="删除"><i data-lucide="trash-2"></i></button>
        </div>
      </div>
    `;
  }).join("");
  if (window.lucide) lucide.createIcons();
}

/* ============== 9. COUNTER ANIMATION ============== */
function animateCounter(el) {
  const target  = parseFloat(el.dataset.counter);
  const prefix  = el.dataset.prefix || "";
  const suffix  = el.dataset.suffix || "";
  const decimals = parseInt(el.dataset.decimals || "0", 10);
  const dur = 900;
  const start = performance.now();
  function step(now) {
    const t = Math.min(1, (now - start) / dur);
    const eased = 1 - Math.pow(1 - t, 3);
    const v = target * eased;
    el.textContent = prefix + v.toFixed(decimals) + suffix;
    if (t < 1) requestAnimationFrame(step);
    else el.textContent = prefix + target.toFixed(decimals) + suffix;
  }
  requestAnimationFrame(step);
}

/* ============== 10. COPY ============== */
function attachCopy(el) {
  el.addEventListener("click", async e => {
    e.stopPropagation();
    const val = el.dataset.copy || el.closest("[data-copy]")?.dataset.copy;
    if (!val) return;
    try { await navigator.clipboard.writeText(val); } catch (_) {}
    const icon = el.querySelector("i") || el;
    if (icon.dataset && icon.dataset.lucide) {
      const prev = icon.dataset.lucide;
      icon.setAttribute("data-lucide", "check");
      el.classList.add("is-copied");
      if (window.lucide) lucide.createIcons();
      showToast("已复制");
      setTimeout(() => {
        // reset icon
        const fresh = el.querySelector("i") || el;
        fresh.setAttribute("data-lucide", prev);
        el.classList.remove("is-copied");
        if (window.lucide) lucide.createIcons();
      }, 1500);
    }
  });
}
function showToast(msg) {
  const t = document.getElementById("toast");
  t.innerHTML = `<i data-lucide="check"></i><span>${msg}</span>`;
  if (window.lucide) lucide.createIcons();
  t.classList.add("is-show");
  clearTimeout(showToast._t);
  showToast._t = setTimeout(() => t.classList.remove("is-show"), 1500);
}

/* ============== 11. ROUTE / AMOUNT / PAY interactions ============== */
function bindGroupSelect(container, selector) {
  container.querySelectorAll(selector).forEach(chip => {
    chip.addEventListener("click", () => {
      container.querySelectorAll(selector).forEach(c => c.classList.remove("is-selected", "chip--selected"));
      chip.classList.add("is-selected");
      // re-add the green dot dynamically for route chips
      chip.classList.add("chip-bounce");
      setTimeout(() => chip.classList.remove("chip-bounce"), 240);
    });
  });
}

/* ============== 12. INIT ============== */
function init() {
  // lucide icons first
  if (window.lucide) lucide.createIcons();

  // initial tab
  switchTab("dashboard");

  // counters
  setTimeout(() => document.querySelectorAll("[data-counter]").forEach(animateCounter), 200);

  // copies
  document.querySelectorAll(".copy-btn, [data-copy-code]").forEach(attachCopy);

  // route row chip select (each route-row independent)
  document.querySelectorAll(".route-row").forEach(rr => {
    rr.querySelectorAll(".route-row__chips .chip").forEach(c => {
      c.addEventListener("click", e => {
        e.stopPropagation();
        rr.querySelectorAll(".route-row__chips .chip").forEach(x => {
          x.classList.remove("chip--selected", "is-selected");
          x.classList.add("chip--mono");
          // strip prepended dot
          const dot = x.querySelector(".dot");
          if (dot) dot.remove();
        });
        c.classList.remove("chip--mono");
        c.classList.add("chip--selected", "chip-bounce");
        // prepend dot
        if (!c.querySelector(".dot")) {
          const d = document.createElement("span");
          d.className = "dot dot--green";
          c.insertBefore(d, c.firstChild);
        }
        setTimeout(() => c.classList.remove("chip-bounce"), 240);
      });
    });
  });

  // amount chips
  const amountChips = document.getElementById("amountChips");
  if (amountChips) {
    amountChips.querySelectorAll(".chip--amount").forEach(c => {
      c.addEventListener("click", () => {
        amountChips.querySelectorAll(".chip--amount").forEach(x => x.classList.remove("is-selected"));
        c.classList.add("is-selected", "chip-bounce");
        document.getElementById("customAmount").value = c.dataset.amount;
        updateAmount();
        setTimeout(() => c.classList.remove("chip-bounce"), 240);
      });
    });
    const amountInput = document.getElementById("customAmount");
    amountInput.addEventListener("input", updateAmount);
    function updateAmount() {
      const v = parseFloat(amountInput.value) || 0;
      // bonus mapping
      let bonus = 1.0;
      if (v >= 500) bonus = 1.14;
      else if (v >= 100) bonus = 1.08;
      else if (v >= 50)  bonus = 1.04;
      const got = (v * bonus).toFixed(2);
      document.getElementById("amountConvert").textContent = `= $${got} 虚拟$`;
      document.getElementById("rechargeCta").textContent = `确认充值 ¥${v} →`;
    }
  }

  // pay tiles
  document.querySelectorAll(".pay-grid .pay").forEach(p => {
    p.addEventListener("click", () => {
      document.querySelectorAll(".pay-grid .pay").forEach(x => {
        x.classList.remove("is-selected");
        const corner = x.querySelector(".pay__corner");
        if (corner) corner.remove();
      });
      p.classList.add("is-selected");
      if (!p.querySelector(".pay__corner")) {
        const c = document.createElement("span");
        c.className = "pay__corner";
        c.innerHTML = '<i data-lucide="check"></i>';
        p.appendChild(c);
        if (window.lucide) lucide.createIcons();
      }
    });
  });

  // ctabs
  document.querySelectorAll(".ctab").forEach(t => {
    t.addEventListener("click", () => {
      document.querySelectorAll(".ctab").forEach(x => x.classList.remove("is-active"));
      t.classList.add("is-active");
      document.querySelectorAll("[data-cpanel]").forEach(p => p.hidden = p.dataset.cpanel !== t.dataset.ctab);
    });
  });

  // api key reveal
  const apiKeyMasked = document.getElementById("apiKeyMasked");
  const apiKeyToggle = document.getElementById("apiKeyToggle");
  if (apiKeyToggle) {
    let revealed = false;
    apiKeyToggle.addEventListener("click", () => {
      revealed = !revealed;
      apiKeyMasked.textContent = revealed ? "sk-XYZ987654321ABCDEFGHe3a2" : "sk-•••••••••••••••e3a2";
      apiKeyToggle.innerHTML = revealed
        ? '<i data-lucide="eye-off"></i>隐藏'
        : '<i data-lucide="eye"></i>显示完整';
      if (window.lucide) lucide.createIcons();
    });
  }

  // test send
  const tSend = document.getElementById("testSendBtn");
  if (tSend) tSend.addEventListener("click", () => {
    tSend.innerHTML = '<i data-lucide="loader"></i>发送中...';
    if (window.lucide) lucide.createIcons();
    setTimeout(() => {
      document.getElementById("testResult").classList.remove("is-hidden");
      tSend.innerHTML = '<i data-lucide="send"></i>发送测试';
      if (window.lucide) lucide.createIcons();
      showToast("测试成功");
    }, 700);
  });

  // switch (auto refresh)
  document.querySelectorAll(".switch__track").forEach(s => {
    s.addEventListener("click", () => s.classList.toggle("is-on"));
  });

  // drawer close
  const drawerClose = document.getElementById("drawerClose");
  if (drawerClose) drawerClose.addEventListener("click", () => {
    document.getElementById("logDrawer").style.transform = "translateX(20px)";
    document.getElementById("logDrawer").style.opacity = "0.5";
    setTimeout(() => {
      document.getElementById("logDrawer").style.transform = "";
      document.getElementById("logDrawer").style.opacity = "";
    }, 300);
  });

  // ticker simulation: prepend new row every 8s on dashboard
  setInterval(() => {
    const tbody = document.getElementById("recentCallsTable");
    if (!tbody) return;
    const onDash = document.querySelector('[data-tab-panel="dashboard"]').classList.contains("is-active");
    if (!onDash) return;
    const now = new Date();
    const time = `${String(now.getHours()).padStart(2,"0")}:${String(now.getMinutes()).padStart(2,"0")}:${String(now.getSeconds()).padStart(2,"0")}`;
    const rid = Math.random().toString(36).substring(2, 10);
    const models = ["claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5"];
    const ups = ["aws稳定", "cc", "apijing"];
    const m = models[Math.floor(Math.random() * models.length)];
    const u = ups[Math.floor(Math.random() * ups.length)];
    const inT = Math.floor(400 + Math.random() * 5000);
    const outT = Math.floor(80 + Math.random() * 1000);
    const total = inT + outT;
    const cost = +(total * 0.00003 + 0.001).toFixed(3);
    const lat = Math.floor(50 + Math.random() * 300);
    const row = document.createElement("div");
    row.innerHTML = rowHtml([time, rid, m, u, inT, outT, total, cost, lat, "ok"], false);
    const newRow = row.firstElementChild;
    newRow.classList.add("is-new");
    tbody.insertBefore(newRow, tbody.firstChild);
    if (tbody.children.length > 10) tbody.removeChild(tbody.lastElementChild);
    if (window.lucide) lucide.createIcons();
  }, 8000);

  // live ago tick
  let ago = 12;
  setInterval(() => {
    ago = (ago + 1) % 60;
    const el = document.getElementById("liveAgo");
    if (el) el.textContent = `· ${ago}s ago`;
  }, 1000);

  // re-create lucide icons after dynamic content
  if (window.lucide) lucide.createIcons();
}

document.addEventListener("DOMContentLoaded", init);
