/*
 * sdd-flow.js — Componente de flujo SDD para la card del dashboard.
 *
 * Autónomo y desacoplado: no depende de focus.js / charts.js / portal.js.
 * Se monta sobre un contenedor [data-sdd-flow] y decide su vista según el
 * estado de la card que lo contiene:
 *   - card normal (chica)  -> vista LISTA (stepper vertical)
 *   - card .hero (centrada) -> vista GRAFO estilo n8n (nodos + conexiones + flujo)
 *
 * Diseño (SRP): modelo de datos, dos renderers (lista/grafo) intercambiables,
 * y un controlador que observa el DOM (clase .hero) y el tamaño (ResizeObserver).
 * Todo encapsulado en un IIFE: no expone nada al scope global.
 */
(function () {
  'use strict';

  /* ------------------------------------------------------------------ *
   *  Modelo: fases del pipeline SDD (Spec-Driven Development)
   * ------------------------------------------------------------------ */
  const PHASES = [
    { id: 'intake',    name: 'Intake',    icon: 'inbox',            desc: 'Recibe HU / issue' },
    { id: 'spec',      name: 'Spec',      icon: 'file-lines',       desc: 'Gherkin: criterios' },
    { id: 'proposal',  name: 'Proposal',  icon: 'lightbulb',        desc: 'Propuesta de cambio' },
    { id: 'design',    name: 'Design',    icon: 'compass-drafting', desc: 'Diseño técnico' },
    { id: 'plan',      name: 'Plan',      icon: 'list-check',       desc: 'Tareas / pasos' },
    { id: 'implement', name: 'Implement', icon: 'code',             desc: 'Codifica cambios' },
    { id: 'verify',    name: 'Verify',    icon: 'shield-halved',    desc: 'Checkpoints' },
    { id: 'review',    name: 'Review',    icon: 'magnifying-glass',  desc: 'Revisión' },
    { id: 'release',   name: 'Release',   icon: 'rocket',           desc: 'Merge / release' },
    { id: 'done',      name: 'Done',      icon: 'circle-check',     desc: 'Cerrado' },
  ];

  const STATE = Object.freeze({ DONE: 'done', ACTIVE: 'active', PENDING: 'pending' });

  /* ------------------------------------------------------------------ *
   *  Util DOM (mínimo, sin dependencias)
   * ------------------------------------------------------------------ */
  const SVG_NS = 'http://www.w3.org/2000/svg';

  function h(tag, cls, attrs) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (attrs) for (const k in attrs) node.setAttribute(k, attrs[k]);
    return node;
  }
  function s(tag, attrs) {
    const node = document.createElementNS(SVG_NS, tag);
    if (attrs) for (const k in attrs) node.setAttribute(k, attrs[k]);
    return node;
  }

  // Deriva el estado de cada fase a partir del índice activo.
  function statesFor(activeIdx) {
    return PHASES.map((p, i) => ({
      ...p,
      state: i < activeIdx ? STATE.DONE : i === activeIdx ? STATE.ACTIVE : STATE.PENDING,
    }));
  }

  /* ------------------------------------------------------------------ *
   *  Vista LISTA — stepper vertical (card chica)
   * ------------------------------------------------------------------ */
  const ListView = {
    mount(host) {
      host.innerHTML = '';
      const wrap = h('ul', 'sdf-list');
      this._rows = PHASES.map((p) => {
        const li = h('li', 'sdf-row', { 'data-id': p.id });
        li.innerHTML =
          '<span class="sdf-dot"></span>' +
          '<i class="sdf-ic fas fa-' + p.icon + '"></i>' +
          '<span class="sdf-name">' + p.name + '</span>' +
          '<span class="sdf-tag"></span>';
        wrap.appendChild(li);
        return li;
      });
      host.appendChild(wrap);
    },
    update(activeIdx) {
      if (!this._rows) return;
      statesFor(activeIdx).forEach((p, i) => {
        const li = this._rows[i];
        li.dataset.state = p.state;
        const tag = li.querySelector('.sdf-tag');
        tag.textContent = p.state === STATE.ACTIVE ? p.desc : '';
      });
    },
  };

  /* ------------------------------------------------------------------ *
   *  Vista GRAFO — nodos + conexiones bezier (card centrada, estilo n8n)
   * ------------------------------------------------------------------ */
  const NODE_W = 132, NODE_H = 48, GAP_X = 44, GAP_Y = 30, PAD = 10;

  const GraphView = {
    mount(host) {
      host.innerHTML = '';
      this.host = host;
      this.canvas = h('div', 'sdf-graph');
      this.svg = s('svg', { class: 'sdf-edges' });
      this.canvas.appendChild(this.svg);
      this._nodes = PHASES.map((p) => {
        const n = h('div', 'sdf-node', { 'data-id': p.id });
        n.innerHTML =
          '<span class="sdf-node-ic"><i class="fas fa-' + p.icon + '"></i></span>' +
          '<span class="sdf-node-txt"><b>' + p.name + '</b><em>' + p.desc + '</em></span>';
        this.canvas.appendChild(n);
        return n;
      });
      host.appendChild(this.canvas);
    },

    // Calcula posiciones en serpentina según el ancho disponible.
    layout() {
      if (!this.host) return;
      const width = this.host.clientWidth || 600;
      const cols = Math.max(1, Math.floor((width - PAD * 2 + GAP_X) / (NODE_W + GAP_X)));
      const rows = Math.ceil(PHASES.length / cols);
      this._pos = PHASES.map((_, i) => {
        const row = Math.floor(i / cols);
        const within = i % cols;
        const col = row % 2 === 0 ? within : cols - 1 - within; // boustrofedón
        return { x: PAD + col * (NODE_W + GAP_X), y: PAD + row * (NODE_H + GAP_Y), row };
      });
      this._nodes.forEach((n, i) => {
        n.style.transform = 'translate(' + this._pos[i].x + 'px,' + this._pos[i].y + 'px)';
      });
      const totalH = PAD * 2 + rows * NODE_H + (rows - 1) * GAP_Y;
      const totalW = PAD * 2 + cols * NODE_W + (cols - 1) * GAP_X;
      this.canvas.style.height = totalH + 'px';
      this.svg.setAttribute('viewBox', '0 0 ' + totalW + ' ' + totalH);
      this.svg.setAttribute('width', totalW);
      this.svg.setAttribute('height', totalH);
      this._drawEdges();
    },

    _drawEdges() {
      this.svg.innerHTML = '';
      this._edges = [];
      for (let i = 0; i < PHASES.length - 1; i++) {
        const a = this._pos[i], b = this._pos[i + 1];
        const sameRow = a.row === b.row;
        let x1, y1, x2, y2, d;
        if (sameRow) {
          const ltr = a.x < b.x;
          x1 = a.x + (ltr ? NODE_W : 0); y1 = a.y + NODE_H / 2;
          x2 = b.x + (ltr ? 0 : NODE_W); y2 = b.y + NODE_H / 2;
          const dx = (x2 - x1) * 0.5;
          d = 'M' + x1 + ',' + y1 + ' C' + (x1 + dx) + ',' + y1 + ' ' + (x2 - dx) + ',' + y2 + ' ' + x2 + ',' + y2;
        } else {
          x1 = a.x + NODE_W / 2; y1 = a.y + NODE_H;
          x2 = b.x + NODE_W / 2; y2 = b.y;
          const dy = (y2 - y1) * 0.5;
          d = 'M' + x1 + ',' + y1 + ' C' + x1 + ',' + (y1 + dy) + ' ' + x2 + ',' + (y2 - dy) + ' ' + x2 + ',' + y2;
        }
        const base = s('path', { class: 'sdf-edge', d: d });
        const flow = s('path', { class: 'sdf-edge-flow', d: d });
        this.svg.appendChild(base);
        this.svg.appendChild(flow);
        this._edges.push(flow);
      }
    },

    update(activeIdx) {
      if (!this._nodes) return;
      statesFor(activeIdx).forEach((p, i) => { this._nodes[i].dataset.state = p.state; });
      if (this._edges) {
        this._edges.forEach((edge, i) => {
          // la conexión "fluye" si su destino ya fue alcanzado o es el activo
          edge.classList.toggle('on', i + 1 <= activeIdx);
        });
      }
    },
  };

  /* ------------------------------------------------------------------ *
   *  Controlador — orquesta vista según estado de la card y el tamaño
   * ------------------------------------------------------------------ */
  function Controller(host) {
    const card = host.closest('.mosaic-item') || host.parentElement;
    let mode = null;            // 'list' | 'graph'
    let activeIdx = 5;          // fase en curso (Implement) al inicio
    let tickTimer = null;
    let view = null;

    function isHero() { return !!(card && card.classList.contains('hero')); }

    function render() {
      const next = isHero() ? 'graph' : 'list';
      if (next === mode) return;
      mode = next;
      host.dataset.mode = mode;
      view = mode === 'graph' ? GraphView : ListView;
      view.mount(host);
      if (mode === 'graph') view.layout();
      view.update(activeIdx);
    }

    function startTicking() {
      stopTicking();
      tickTimer = setInterval(() => {
        activeIdx = activeIdx + 1;
        if (activeIdx > PHASES.length - 1) activeIdx = 0;
        if (view) view.update(activeIdx);
      }, 2600);
    }
    function stopTicking() { if (tickTimer) { clearInterval(tickTimer); tickTimer = null; } }

    // Observa cambios de clase en la card (hero <-> normal) sin tocar focus.js
    const mo = new MutationObserver(() => render());
    if (card) mo.observe(card, { attributes: true, attributeFilter: ['class'] });

    // Recalcula el grafo cuando cambia el tamaño del contenedor
    const ro = new ResizeObserver(() => { if (mode === 'graph') view.layout(); });
    ro.observe(host);

    render();
    startTicking();
  }

  /* ------------------------------------------------------------------ *
   *  Bootstrap
   * ------------------------------------------------------------------ */
  function init() {
    document.querySelectorAll('[data-sdd-flow]').forEach((host) => Controller(host));
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
