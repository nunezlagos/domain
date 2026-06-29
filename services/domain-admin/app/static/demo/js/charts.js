let charts = {};

function initTokenChart() {
  const canvas = document.getElementById('chartTokens');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const days = ['Lun','Mar','Mie','Jue','Vie','Sab','Dom'];
  charts.tokens = new Chart(ctx, {
    type: 'line',
    data: {
      labels: days,
      datasets: [
        { label: 'Input Tokens',
          data: [12400,13800,11200,15600,14200,9800,16500],
          borderColor: DJ.c1, backgroundColor: DJ.c1a10,
          fill: true, tension: 0, stepped: 'before',
          pointRadius: 5, pointHoverRadius: 7,
          pointBorderColor: DJ.c1, pointBackgroundColor: '#ffffff',
          pointBorderWidth: 2.5, borderWidth: 2.5 },
        { label: 'Output Tokens',
          data: [8900,9200,7800,10500,9800,7200,11200],
          borderColor: DJ.c4, backgroundColor: DJ.c4a10,
          fill: true, tension: 0, stepped: 'before',
          pointRadius: 5, pointHoverRadius: 7,
          pointBorderColor: DJ.c4, pointBackgroundColor: '#ffffff',
          pointBorderWidth: 2.5, borderWidth: 2.5 },
      ]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: {
        legend: { labels: { color: DJ.textMuted, font: { size: 10 }, boxWidth: 10, padding: 10, usePointStyle: true, pointStyle: 'circle' } }
      },
      scales: {
        x: { ticks: { color: DJ.textDim, font: { size: 9 } }, grid: { color: DJ.gridLine } },
        y: { ticks: { color: DJ.textDim, font: { size: 9 } }, grid: { color: DJ.gridLine }, beginAtZero: true }
      }
    }
  });
}

function initRequestsChart() {
  const reqLabels = ['Agents','Skills','Flows','Prompts','Crons'];
  const reqData = [
    MOCK_DATA.agents.reduce((s,d) => s + (d.calls||0), 0),
    MOCK_DATA.skills.reduce((s,d) => s + (d.calls||0), 0),
    MOCK_DATA.flows.reduce((s,d) => s + (d.runs||0), 0),
    MOCK_DATA.prompts.reduce((s,d) => s + (d.uses||0), 0),
    MOCK_DATA.crons.length * 100,
  ];
  const ctx = document.getElementById('chartRequests').getContext('2d');
  const totalReqs = reqData.reduce((a, b) => a + b, 0);
  charts.requests = new Chart(ctx, {
    type: 'doughnut',
    data: {
      labels: reqLabels,
      datasets: [{
        data: reqData,
        backgroundColor: [DJ.c1a70, DJ.c2a70, DJ.c3a70, DJ.c4a70, DJ.c5a70],
        borderColor:     [DJ.c1,    DJ.c2,    DJ.c3,    DJ.c4,    DJ.c5],
        borderWidth: 1.5,
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      cutout: '72%',
      plugins: {
        legend: { position: 'right', labels: { color: DJ.textMuted, font: { size: 10 }, boxWidth: 10, padding: 8, usePointStyle: true, pointStyle: 'circle' } }
      }
    },
    plugins: [{
      id: 'centerText',
      beforeDraw: function(chart) {
        const {ctx, chartArea: {top, left, right, bottom}} = chart;
        const cx = (left + right) / 2, cy = (top + bottom) / 2;
        ctx.save();
        ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
        ctx.font = '600 18px system-ui, sans-serif';
        ctx.fillStyle = DJ.textStrong;
        ctx.fillText(totalReqs.toLocaleString(), cx, cy - 6);
        ctx.font = '9px system-ui, sans-serif';
        ctx.fillStyle = DJ.textDim;
        ctx.fillText('total', cx, cy + 14);
        ctx.restore();
      }
    }]
  });
}

function initFrequencyChart() {
  const ctx = document.getElementById('chartFrequency').getContext('2d');
  const hours = Array.from({length: 24}, (_, i) => String(i).padStart(2,'0') + 'h');
  charts.frequency = new Chart(ctx, {
    type: 'bar',
    data: {
      labels: hours,
      datasets: [{
        label: 'Requests',
        data: [12,8,5,3,2,1,4,18,42,56,48,38,45,52,58,44,62,78,85,72,54,38,28,18],
        backgroundColor: DJ.c1a40,
        borderColor: DJ.c1,
        borderWidth: 1.5,
        borderRadius: { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 },
        borderSkipped: false,
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: DJ.textDim, font: { size: 8 }, maxTicksLimit: 12 }, grid: { display: false } },
        y: { ticks: { color: DJ.textDim, font: { size: 9 } }, grid: { color: DJ.gridLine }, beginAtZero: true }
      }
    }
  });
}

function initFlowsChart() {
  const flowLabels = MOCK_DATA.flows.map(d => d.name);
  const flowRuns = MOCK_DATA.flows.map(d => d.runs);
  const ctx = document.getElementById('chartFlows').getContext('2d');
  charts.flows = new Chart(ctx, {
    type: 'bar',
    data: {
      labels: flowLabels,
      datasets: [{
        label: 'Runs', data: flowRuns,
        backgroundColor: [DJ.c3a60, DJ.c2a60],
        borderColor:     [DJ.c3,    DJ.c2],
        borderWidth: 1.5,
        borderRadius: { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 },
        borderSkipped: false,
      }]
    },
    options: {
      indexAxis: 'y', responsive: true, maintainAspectRatio: false,
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: DJ.textFaint, font: { size: 10 } }, grid: { color: DJ.gridLine }, beginAtZero: true },
        y: { ticks: { color: DJ.textMid,   font: { size: 11 } }, grid: { display: false } }
      }
    }
  });
}

function initSuccessChart() {
  const ctx = document.getElementById('chartSuccess').getContext('2d');
  const days = ['L','M','X','J','V','S','D'];
  charts.success = new Chart(ctx, {
    type: 'line',
    data: {
      labels: days,
      datasets: [{
        label: 'Success Rate',
        data: [94, 96, 95, 97, 98, 92, 97.3],
        borderColor: DJ.success,
        backgroundColor: (ctx) => {
          const c = ctx.chart.ctx.createLinearGradient(0, 0, 0, ctx.chart.height);
          c.addColorStop(0, DJ.successA25);
          c.addColorStop(1, 'rgba(74,222,128,0)');
          return c;
        },
        fill: true, tension: 0.4,
        pointRadius: 3, pointHoverRadius: 5,
        pointBackgroundColor: DJ.success,
        pointBorderColor: '#ffffff',
        pointBorderWidth: 1.5, borderWidth: 2,
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { display: false } },
      scales: {
        x: { display: false },
        y: {
          min: 80, max: 100,
          ticks: { color: DJ.textMuted, font: { size: 9 }, callback: (v) => v + '%' },
          grid: { color: DJ.successGrid }
        }
      }
    }
  });
}

function setupGraphCanvas() {
  const canvas = document.getElementById('chartHealth');
  if (!canvas || !canvas.parentElement) return null;
  const rect = canvas.parentElement.getBoundingClientRect();
  const W = rect.width, H = rect.height;
  if (!W || W < 10 || !H || H < 10) return null;
  const dpr = window.devicePixelRatio || 1;
  canvas.width = W * dpr;
  canvas.height = H * dpr;
  canvas.style.width = W + 'px';
  canvas.style.height = H + 'px';
  const ctx = canvas.getContext('2d');
  ctx.scale(dpr, dpr);
  return { ctx, W, H };
}

function positionNodes(nodes, nodeMap, agents, skills, flows, W, H) {
  const colX = { flow: W * 0.15, agent: W * 0.5, skill: W * 0.85 };
  const spacing = H / (Math.max(agents.length, skills.length, flows.length) + 1);
  agents.forEach((a, i) => {
    const n = { id: a.slug, label: a.name.split(' ')[0], type: 'agent', color: DJ.c1, x: colX.agent, y: spacing * (i + 1), vx: 0, vy: 0 };
    nodes.push(n); nodeMap[a.slug] = n;
  });
  skills.forEach((s, i) => {
    const n = { id: s.slug, label: s.name.split(' ')[0], type: 'skill', color: DJ.c2, x: colX.skill, y: spacing * (i + 1), vx: 0, vy: 0 };
    nodes.push(n); nodeMap[s.slug] = n;
  });
  flows.forEach((f, i) => {
    const n = { id: f.slug, label: f.name.split(' ')[0], type: 'flow', color: DJ.c3, x: colX.flow, y: spacing * (i + 1.5), vx: 0, vy: 0 };
    nodes.push(n); nodeMap[f.slug] = n;
  });
}

function simulateForces(nodes, edges, nodeMap) {
  for (let iter = 0; iter < 60; iter++) {
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const dx = nodes[j].x - nodes[i].x, dy = nodes[j].y - nodes[i].y;
        const dist = Math.sqrt(dx*dx + dy*dy) || 1;
        const f = 3000 / (dist * dist);
        nodes[i].vx -= (dx/dist) * f; nodes[i].vy -= (dy/dist) * f;
        nodes[j].vx += (dx/dist) * f; nodes[j].vy += (dy/dist) * f;
      }
    }
    for (const [f, t] of edges) {
      const a = nodeMap[f], b = nodeMap[t];
      if (!a || !b) continue;
      const dx = b.x - a.x, dy = b.y - a.y, dist = Math.sqrt(dx*dx + dy*dy) || 1;
      const force = dist * 0.015;
      a.vx += dx * force; a.vy += dy * force;
      b.vx -= dx * force; b.vy -= dy * force;
    }
    for (const n of nodes) { n.vx *= 0.45; n.vy *= 0.45; n.x += n.vx; n.y += n.vy; }
  }
}

function drawHexagon(ctx, x, y, r) {
  ctx.beginPath();
  for (let i = 0; i < 6; i++) {
    const a = Math.PI / 3 * i - Math.PI / 6, px = x + r * Math.cos(a), py = y + r * Math.sin(a);
    i === 0 ? ctx.moveTo(px, py) : ctx.lineTo(px, py);
  }
  ctx.closePath();
}

function renderEdges(ctx, edges, nodeMap) {
  for (const [f, t] of edges) {
    const a = nodeMap[f], b = nodeMap[t];
    if (!a || !b) continue;
    const mx = (a.x + b.x) / 2;
    ctx.beginPath(); ctx.moveTo(a.x, a.y);
    ctx.lineTo(mx, a.y); ctx.lineTo(mx, b.y); ctx.lineTo(b.x, b.y);
    ctx.strokeStyle = a.color + '20'; ctx.lineWidth = 2; ctx.stroke();
    ctx.beginPath(); ctx.moveTo(a.x, a.y);
    ctx.lineTo(mx, a.y); ctx.lineTo(mx, b.y); ctx.lineTo(b.x, b.y);
    ctx.strokeStyle = DJ.gridLine; ctx.lineWidth = 1; ctx.setLineDash([3, 4]); ctx.stroke(); ctx.setLineDash([]);
  }
}

function renderNodes(ctx, nodes) {
  for (const n of nodes) {
    if (!isFinite(n.x) || !isFinite(n.y)) continue;
    const g = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, 16);
    g.addColorStop(0, n.color + '60'); g.addColorStop(1, n.color + '00');
    ctx.fillStyle = g; drawHexagon(ctx, n.x, n.y, 16); ctx.fill();
  }
  for (const n of nodes) {
    if (!isFinite(n.x) || !isFinite(n.y)) continue;
    drawHexagon(ctx, n.x, n.y, 6);
    ctx.fillStyle = n.color; ctx.fill();
    ctx.strokeStyle = n.color + '80'; ctx.lineWidth = 1.5; ctx.stroke();
  }
  ctx.fillStyle = DJ.textMid; ctx.font = '9px system-ui, sans-serif';
  ctx.textAlign = 'center'; ctx.textBaseline = 'top';
  for (const n of nodes) {
    if (!isFinite(n.x) || !isFinite(n.y)) continue;
    ctx.fillText(n.label, n.x, n.y + 9);
  }
}

function drawNetworkGraph() {
  const canvas = document.getElementById('chartHealth');
  const parent = canvas.parentElement;
  if (!parent) return;
  const result = setupGraphCanvas();
  if (!result) { requestAnimationFrame(drawNetworkGraph); return; }
  const { ctx, W, H } = result;
  const nodes = [], nodeMap = {};
  const edges = [
    ['soporte-bot', 'web-search'], ['code-reviewer', 'query-db'], ['sdd-generator', 'send-email'],
    ['sdd-pipeline-v1', 'code-reviewer'], ['sdd-pipeline-v1', 'sdd-generator'], ['issue-intake', 'soporte-bot'],
  ];
  const agents = MOCK_DATA.agents.filter(a => a.status === 'active');
  const skills = MOCK_DATA.skills;
  const flows = MOCK_DATA.flows.filter(f => f.status === 'active');
  positionNodes(nodes, nodeMap, agents, skills, flows, W, H);
  simulateForces(nodes, edges, nodeMap);
  ctx.clearRect(0, 0, W, H);
  renderEdges(ctx, edges, nodeMap);
  renderNodes(ctx, nodes);
}

function initNetworkGraph() {
  window.drawNetworkGraph = drawNetworkGraph;
  drawNetworkGraph();
  window.addEventListener('resize', drawNetworkGraph);
}

function initCharts() {
  initTokenChart();
  initRequestsChart();
  initFrequencyChart();
  initFlowsChart();
  initSuccessChart();
  initNetworkGraph();
}

function resizeAllCharts() {
  Object.values(charts).forEach(c => { try { c.resize(); } catch (e) {} });
  if (typeof window.drawNetworkGraph === 'function') window.drawNetworkGraph();
}
