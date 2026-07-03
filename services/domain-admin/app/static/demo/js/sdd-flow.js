(function () {
  'use strict';

  /* ================================================================== *
   *  NODES
   * ================================================================== */
  var NODES = [
    { id:'start',    name:'Inicio',    icon:'play',             x:8,   y:180, type:'start',
      desc:'Pipeline iniciado por commit, PR o comando manual.',
      tools:['domain_session_bootstrap','domain_orchestrate'],
      ops:[{type:'read',label:'config, policies'}], output:'flow_run_id, plan[]',
      spec:'Verificar contexto de sesión activo y orquestar el plan de fases.',
      tasks:[{id:'T-001',label:'Inicializar contexto de sesión',status:'done',assignee:'sistema'}], issues:[] },
    { id:'explore',  name:'Explorar',  icon:'magnifying-glass', x:96,  y:180, type:'phase',
      desc:'Analiza prompt: intención, alcance y módulos afectados desde el grafo de código vivo.',
      tools:['domain_code_graph','domain_code_explore','domain_mem_search'],
      ops:[{type:'search',label:'code graph'},{type:'read',label:'observaciones'}], output:'intent, scope, multi_concern, affected_paths[]',
      spec:'Identificar intent, scope y paths afectados partiendo del code graph.',
      tasks:[{id:'T-002',label:'Analizar prompt del usuario',status:'pending',assignee:'agente'},{id:'T-003',label:'Consultar grafo de código',status:'pending',assignee:'agente'}], issues:[] },
    { id:'spec',     name:'Spec',      icon:'file-lines',       x:184, y:70,  type:'phase',
      desc:'Wizard adaptativo interactivo: produce issue draft con Gherkin (RFC 2119).',
      tools:['domain_issue_create_start','domain_issue_create_answer','domain_issue_create_commit'],
      ops:[{type:'write',label:'issue draft'},{type:'ask',label:'AskUserQuestion'}], output:'issue_slug, issue_md',
      spec:'Redactar spec con Gherkin scenarios; preguntar solo slots no inferibles.',
      tasks:[{id:'T-004',label:'Completar wizard de slots',status:'pending',assignee:'agente'},{id:'T-005',label:'Confirmar non_goals con el humano',status:'pending',assignee:'humano'},{id:'T-006',label:'Commit del issue draft',status:'pending',assignee:'agente'}],
      issues:[{id:'I-001',label:'SDD sin criterios de éxito',severity:'medium',status:'open'}] },
    { id:'propose',  name:'Proponer',  icon:'lightbulb',        x:184, y:290, type:'phase',
      desc:'Genera proposal.md (scope, enfoque, riesgos) y lo sincroniza BD↔repo.',
      tools:['domain_openspec_export','domain_openspec_apply','domain_knowledge_save'],
      ops:[{type:'write',label:'proposal.md'},{type:'save',label:'knowledge_doc'}], output:'proposal_md, status=draft',
      spec:'Proponer enfoque alto-level sin código; sync openspec como contrato de fase.',
      tasks:[{id:'T-007',label:'Redactar proposal.md',status:'pending',assignee:'agente'},{id:'T-008',label:'Sync openspec BD↔repo',status:'pending',assignee:'agente'}],
      issues:[{id:'I-002',label:'Propuesta sin validación técnica',severity:'low',status:'open'}] },
    { id:'design',   name:'Disenar',   icon:'compass-drafting', x:280, y:180, type:'phase',
      desc:'ADRs por decisión técnica + plan TDD/sabotage; sincroniza design.md BD↔repo.',
      tools:['domain_openspec_export','domain_openspec_apply','domain_mem_save'],
      ops:[{type:'write',label:'design.md, ADRs'},{type:'save',label:'adr (Required)'}], output:'adrs[], tdd_plan[]',
      spec:'Producir ADRs y plan TDD; al menos 1 ADR persistido en memoria.',
      tasks:[{id:'T-009',label:'Redactar ADRs',status:'pending',assignee:'agente'},{id:'T-010',label:'Definir plan TDD + sabotage',status:'pending',assignee:'agente'}],
      issues:[{id:'I-003',label:'Diseño sin breaking changes',severity:'high',status:'open'}] },
    { id:'tasks',    name:'Tareas',    icon:'list-check',       x:368, y:180, type:'phase',
      desc:'Descompone en tareas atómicas (≤2h) con parallel_groups; sincroniza tasks.md.',
      tools:['domain_openspec_export','domain_openspec_apply','domain_knowledge_save'],
      ops:[{type:'write',label:'tasks.md'},{type:'save',label:'knowledge_doc'}], output:'tasks[{section,parallel_group,max_hours}]',
      spec:'Crear task list atómica ordenada con parallel_groups.',
      tasks:[{id:'T-011',label:'Descomponer en tareas ≤2h',status:'pending',assignee:'agente'},{id:'T-012',label:'Asignar parallel_groups',status:'pending',assignee:'agente'}],
      issues:[{id:'I-004',label:'Tareas sin dependencias',severity:'medium',status:'open'}] },
    { id:'apply',    name:'Aplicar',   icon:'code',             x:456, y:180, type:'phase',
      desc:'Implementa vía TDD estricto (rojo→mínima→refactor); commits conventional.',
      tools:['domain_mem_save'],
      ops:[{type:'write',label:'code changes'},{type:'save',label:'code_reference (Required)'}], output:'commit_sha, files_changed[], test_result',
      spec:'Implementar por task con TDD; persistir code_reference.',
      tasks:[{id:'T-013',label:'Test rojo → impl mínima',status:'pending',assignee:'agente'},{id:'T-014',label:'Refactor + commit semántico',status:'pending',assignee:'agente'}],
      issues:[{id:'I-005',label:'Cambio sin seguir el diseño',severity:'critical',status:'open'}] },
    { id:'verify',   name:'Verificar', icon:'shield-halved',    x:544, y:70,  type:'phase',
      desc:'Valida TODOS los Gherkin scenarios del spec; reporte escéptico evidence-based.',
      tools:['domain_verify_start','domain_verify_update_item','domain_verify_complete'],
      ops:[{type:'read',label:'test output'},{type:'check',label:'scenarios'}], output:'scenarios_passed, scenarios_failed[], verdict',
      spec:'Mapear cada Gherkin scenario a un test y verificar server-side.',
      tasks:[{id:'T-015',label:'Ejecutar verify por scenario',status:'pending',assignee:'agente'},{id:'T-016',label:'Reportar cobertura',status:'pending',assignee:'agente'}],
      issues:[{id:'I-006',label:'Test fallando en CI',severity:'critical',status:'open'}] },
    { id:'judge',    name:'Juzgar',    icon:'gavel',            x:544, y:290, type:'phase',
      desc:'Panel adversarial: tests de sabotaje que rompen invariantes y validan la regresión.',
      tools:['domain_mem_save'],
      ops:[{type:'check',label:'sabotage tests'},{type:'save',label:'sabotage_record (Required)'}], output:'sabotages[], audit_gaps[], verdict',
      spec:'Romper invariantes → validar que el test atrapa la regresión → restaurar.',
      tasks:[{id:'T-017',label:'Correr sabotage tests',status:'pending',assignee:'agente'},{id:'T-018',label:'Audit checklist',status:'pending',assignee:'agente'}],
      issues:[{id:'I-007',label:'Review bloqueada',severity:'medium',status:'open'}] },
    { id:'review',   name:'Revisar',   icon:'clipboard-check',  x:640, y:70,  type:'phase',
      desc:'Auditoría read-only contra políticas/skills del proyecto (resolver project→platform).',
      tools:['domain_project_policy_list','domain_verify_start','domain_verify_update_item','domain_verify_complete'],
      ops:[{type:'read',label:'policies, skills'},{type:'check',label:'compliance'}], output:'verdict: compliant | violations_found',
      spec:'Auditar el cambio contra las policies; violations_found bloquea archive.',
      tasks:[{id:'T-019',label:'Resolver policies jerárquicas',status:'pending',assignee:'agente'},{id:'T-020',label:'Registrar checkpoint de review',status:'pending',assignee:'agente'}],
      issues:[{id:'I-008',label:'Violación de policy sin resolver',severity:'high',status:'open'}] },
    { id:'archive',  name:'Archivar',  icon:'box-archive',      x:736, y:180, type:'phase',
      desc:'Marca la issue como implemented + actualiza CHANGELOG Unreleased.',
      tools:['domain_openspec_status','domain_issue_set_status'],
      ops:[{type:'read',label:'openspec status'},{type:'write',label:'CHANGELOG'}], output:'archived=true',
      spec:'Verificar estado openspec pre-cierre y archivar la issue.',
      tasks:[{id:'T-021',label:'Chequear openspec status',status:'pending',assignee:'agente'},{id:'T-022',label:'Marcar issue implemented',status:'pending',assignee:'agente'}], issues:[] },
    { id:'onboard',  name:'Onboard',   icon:'rocket',           x:824, y:180, type:'phase',
      desc:'Materializa el conocimiento del cambio (patrones, gotchas) si aplica.',
      tools:['domain_knowledge_save'],
      ops:[{type:'save',label:'knowledge_doc'}], output:'skipped=true | doc_created=true',
      spec:'Documentar conceptos no obvios del cambio.',
      tasks:[{id:'T-023',label:'Documentar gotchas/convenciones',status:'pending',assignee:'agente'}],
      issues:[] },
    { id:'end',      name:'Fin',       icon:'flag-checkered',   x:912, y:180, type:'end',
      desc:'Pipeline completado.',
      tools:[], ops:[], output:'status: success',
      spec:'Completado.', tasks:[], issues:[] },
    { id:'solo',     name:'Server-side', icon:'server',         x:96,  y:290, type:'phase',
      desc:'Ejecución inline server-side vía LLM provider directo, sin cliente IDE colaborador.',
      tools:['domain_orchestrate'],
      ops:[{type:'run',label:'LLM inline'}], output:'flow_run completo (server)',
      spec:'Modo solo: el server ejecuta el pipeline internamente sin desglose de fases cliente.',
      tasks:[], issues:[] },
  ];
  var NODE_MAP = {};
  NODES.forEach(function(n){ NODE_MAP[n.id] = n; });

  /* ================================================================== *
   *  EDGES & WORKFLOWS
   * ================================================================== */
  var EDGES = [
    { from:'start',to:'explore'},{from:'explore',to:'spec',label:'Si'},{from:'explore',to:'propose',label:'No'},
    { from:'spec',to:'design'},{from:'propose',to:'design'},{from:'design',to:'tasks'},{from:'tasks',to:'apply'},
    { from:'apply',to:'verify'},{from:'verify',to:'judge'},{from:'judge',to:'review'},
    { from:'review',to:'archive'},{from:'archive',to:'onboard'},{from:'onboard',to:'end'},
    /* modo solo: rama server-side inline */
    { from:'start',to:'solo',shortcut:true},{from:'solo',to:'end',shortcut:true},
    /* atajos de modos reducidos (lite / express) */
    { from:'explore',to:'apply',shortcut:true},{from:'start',to:'apply',shortcut:true},
  ];
  /* Modos REALES del orquestador (services/domain-mcp/internal/service/orchestrator).
     full/async/detect = 11 fases; lite = explore+apply+verify; express = apply+verify;
     solo = ejecución server-side inline. hybrid/manual NO son modos: son exec_modes
     (controlan dónde pausa el flujo), por eso no aparecen como tabs. */
  var WORKFLOWS = [
    { slug:'full',   name:'Full',    nodes:['start','explore','spec','propose','design','tasks','apply','verify','judge','review','archive','onboard','end'] },
    { slug:'lite',   name:'Lite',    nodes:['start','explore','apply','verify','end'] },
    { slug:'express',name:'Express', nodes:['start','apply','verify','end'] },
    { slug:'async',  name:'Async',   nodes:['start','explore','spec','propose','design','tasks','apply','verify','judge','review','archive','onboard','end'] },
    { slug:'detect', name:'Detect',  nodes:['start','explore','spec','propose','design','tasks','apply','verify','judge','review','archive','onboard','end'] },
    { slug:'solo',   name:'Solo',    nodes:['start','solo','end'] },
  ];

  /* ================================================================== *
   *  HELPERS
   * ================================================================== */
  var SVG_NS = 'http://www.w3.org/2000/svg';
  function el(tag, cls, attrs) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (attrs) for (var k in attrs) n.setAttribute(k, attrs[k]);
    return n;
  }
  function svg(tag, attrs) {
    var n = document.createElementNS(SVG_NS, tag);
    if (attrs) for (var k in attrs) n.setAttribute(k, attrs[k]);
    return n;
  }
  function esc(s) { return s ? String(s).replace(/[&<>"]/g,function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]; }) : ''; }

  var R = 18, NODE_W = 72, NODE_H = 52;
  var TICK_MS = 2200;

  /* ================================================================== *
   *  SUB-GRAPH (neuronas) generator
   * ================================================================== */
  var SG = 14; // sub-node radius

  function buildSubGraph(tools, ops, output) {
    var sn = [], se = [];
    var T_X = 24, O_X = 130, OUT_X = 236;
    var Y_GAP = 38;
    var maxN = Math.max(tools ? tools.length : 0, ops ? ops.length : 0, output ? 1 : 0);
    var totalH = Math.max(maxN * Y_GAP, 60);

    function centerY(items, i) {
      if (!items || items.length === 0) return totalH / 2;
      var colH = (items.length - 1) * Y_GAP;
      var startY = (totalH - colH) / 2;
      return startY + i * Y_GAP;
    }

    (tools || []).forEach(function(t, i){
      sn.push({ id:'t'+i, label:t, type:'tool', x:T_X, y:centerY(tools, i), r:SG });
    });
    (ops || []).forEach(function(o, i){
      var id = 'o'+i;
      sn.push({ id:id, label:o.label, type:'op', x:O_X, y:centerY(ops, i), r:SG });
      (tools || []).forEach(function(_, ti){ se.push({ from:'t'+ti, to:id }); });
    });
    if (output) {
      sn.push({ id:'out', label:output, type:'output', x:OUT_X, y:totalH/2, r:SG });
      (ops || []).forEach(function(_, oi){ se.push({ from:'o'+oi, to:'out' }); });
    }
    return { sn: sn, se: se, h: totalH + 16 };
  }

  function renderSubGraphSVG(container, data) {
    var w = 280;
    var svgEl = svg('svg', { class:'sdf-subg', viewBox:'0 0 '+w+' '+data.h, width:w, height:data.h });
    // edges
    data.se.forEach(function(edge){
      var a = data.sn.filter(function(s){ return s.id === edge.from; })[0];
      var b = data.sn.filter(function(s){ return s.id === edge.to; })[0];
      if (!a || !b) return;
      var dx = b.x - a.x, dy = b.y - a.y;
      var dist = Math.sqrt(dx*dx + dy*dy) || 1;
      var x1 = a.x + dx/dist * a.r, y1 = a.y + dy/dist * a.r;
      var x2 = b.x - dx/dist * b.r, y2 = b.y - dy/dist * b.r;
      var path = svg('path', { d:'M'+x1+','+y1+' L'+x2+','+y2, class:'sdf-sub-edge' });
      svgEl.appendChild(path);
    });
    // nodes
    data.sn.forEach(function(n){
      var cls = 'sdf-sub-n sdf-sub-n--' + n.type;
      var circle = svg('circle', { cx:n.x, cy:n.y, r:n.r, class:cls });
      svgEl.appendChild(circle);
      var label = svg('text', { x:n.x, y:n.y + n.r + 10, class:'sdf-sub-lbl', 'text-anchor':'middle' });
      label.textContent = n.label.length > 18 ? n.label.slice(0, 16) + '..' : n.label;
      svgEl.appendChild(label);
    });
    // column headers
    var headers = [ { x:T_X, label:'Tools' }, { x:O_X, label:'Ops' }, { x:OUT_X, label:'Output' } ];
    // calculate actual column X positions from the generated data
    var TX = 24, OX = 130, OUTX = 236;
    [ { x:TX, label:'Tools' }, { x:OX, label:'Ops' }, { x:OUTX, label:'Output' } ].forEach(function(h){
      var txt = svg('text', { x:h.x, y:10, class:'sdf-sub-col', 'text-anchor':'middle' });
      txt.textContent = h.label;
      svgEl.appendChild(txt);
    });
    container.appendChild(svgEl);
  }

  var T_X = 24, O_X = 130, OUT_X = 236;

  /* ================================================================== *
   *  DETAIL CONTENT
   * ================================================================== */
  function detailContentHTML(node, slug) {
    var key = 'sdd-spec-' + slug + '-' + node.id;
    var saved; try { saved = localStorage.getItem(key); } catch(e) {}
    var specText = saved || node.spec || '';

    var toolsHtml = (node.tools && node.tools.length)
      ? node.tools.map(function(t){ return '<span class="sdf-chip sdf-chip--tool">'+esc(t)+'</span>'; }).join('')
      : '<span class="sdf-chip" style="opacity:0.3">—</span>';
    var opsHtml = (node.ops && node.ops.length)
      ? node.ops.map(function(o){ return '<span class="sdf-chip sdf-chip--op">'+esc(o)+'</span>'; }).join('')
      : '<span class="sdf-chip" style="opacity:0.3">—</span>';
    var outputHtml = node.output
      ? '<span class="sdf-chip sdf-chip--output">'+esc(node.output)+'</span>'
      : '<span class="sdf-chip" style="opacity:0.3">—</span>';

    var tasksCount = (node.tasks && node.tasks.length) || 0;
    var issuesCount = (node.issues && node.issues.length) || 0;

    var tasksHtml = tasksCount
      ? node.tasks.map(function(t){ return '<li><span class="sdf-dot sdf-dot--'+(t.status==='done'?'ok':'pend')+'"></span>'+esc(t.label)+'<span class="meta">'+esc(t.assignee||'')+'</span></li>'; }).join('')
      : '<li class="empty">Sin tareas</li>';
    var issuesHtml = issuesCount
      ? node.issues.map(function(i){ return '<li><span class="sdf-dot sdf-dot--'+i.severity+'"></span>'+esc(i.label)+'<span class="meta">'+i.status+'</span></li>'; }).join('')
      : '<li class="empty">Sin issues</li>';

    return (
      '<div class="sdf-dtl-body">' +
        /* sub-graph (tools → ops → output) */
        '<div class="sdf-dtl-subg" data-role="subgraph"></div>' +
        /* info cards: tools, ops, output */
        '<div class="sdf-dtl-cards">' +
          '<div class="sdf-dtl-card"><div class="sdf-dtl-card-label"><i class="fas fa-wrench"></i> Tools</div><div class="sdf-dtl-card-items">'+toolsHtml+'</div></div>' +
          '<div class="sdf-dtl-card"><div class="sdf-dtl-card-label"><i class="fas fa-arrow-right-arrow-left"></i> Ops</div><div class="sdf-dtl-card-items">'+opsHtml+'</div></div>' +
          '<div class="sdf-dtl-card"><div class="sdf-dtl-card-label"><i class="fas fa-file-export"></i> Output</div><div class="sdf-dtl-card-items">'+outputHtml+'</div></div>' +
        '</div>' +
        /* spec (collapsible) */
        '<details class="sdf-dtl-spec" data-spec-key="'+key+'">' +
          '<summary><i class="fas fa-file-lines"></i> Spec</summary>' +
          '<div style="display:flex;gap:4px;margin:4px 0">' +
            '<button class="sdf-dtl-edit-btn" data-action="edit-spec">✎ Editar</button>' +
          '</div>' +
          '<p class="sdf-dtl-spec-txt" data-role="spec-text">'+esc(specText)+'</p>' +
          '<textarea class="sdf-dtl-spec-ed" data-role="spec-editor" style="display:none">'+esc(specText)+'</textarea>' +
          '<div class="sdf-dtl-acts" data-role="spec-actions" style="display:none">' +
            '<button class="sdf-btn-s" data-action="save-spec">Guardar</button>' +
            '<button class="sdf-btn-c" data-action="cancel-spec">Cancelar</button>' +
          '</div>' +
        '</details>' +
        /* tasks + issues side by side */
        '<div class="sdf-dtl-items">' +
          '<div class="sdf-dtl-items-col"><div class="sdf-dtl-items-hd"><i class="fas fa-list-check"></i> Tasks <span class="sdf-dtl-items-count">'+tasksCount+'</span></div><ul class="sdf-dtl-list">'+tasksHtml+'</ul></div>' +
          '<div class="sdf-dtl-items-col"><div class="sdf-dtl-items-hd"><i class="fas fa-bug"></i> Issues <span class="sdf-dtl-items-count">'+issuesCount+'</span></div><ul class="sdf-dtl-list">'+issuesHtml+'</ul></div>' +
        '</div>' +
      '</div>'
    );
  }

  /* ================================================================== *
   *  MAIN
   * ================================================================== */
  function init() {
    var host = document.querySelector('[data-sdd-flow]:not(.sdf-modal-body)');
    if (!host) return;
    host.innerHTML = '';

    /* ---- tabs ---- */
    var tabBar = el('div', 'sdf-tabs');
    tabBar.innerHTML = '<span class="sdf-tabs-label">Workflow:</span>';
    var tabMap = {};
    WORKFLOWS.forEach(function(wf, i){
      var btn = el('button', 'sdf-tab' + (i === 0 ? ' on' : ''));
      btn.innerHTML = '<span class="sdf-tab-dot"></span> ' + wf.name;
      tabMap[wf.slug] = btn;
      btn.addEventListener('click', function(){ switchWf(wf.slug); });
      tabBar.appendChild(btn);
    });
    host.appendChild(tabBar);

    /* ---- status ---- */
    var statusEl = el('div', 'sdf-status');
    statusEl.innerHTML =
      '<span class="sdf-status-flow"><i class="fas fa-diagram-project"></i> SDD Pipeline</span>' +
      '<span class="sdf-status-phase"><i class="fas fa-circle-notch fa-spin"></i> —</span>' +
      '<span class="sdf-status-bar"><i style="width:0%"></i></span>' +
      '<span class="sdf-status-meta">0/0 fases</span>';
    host.appendChild(statusEl);
    var phaseEl = statusEl.querySelector('.sdf-status-phase');
    var barEl   = statusEl.querySelector('.sdf-status-bar i');
    var metaEl  = statusEl.querySelector('.sdf-status-meta');

    /* ---- graph ---- */
    var graph = el('div', 'sdf-graph');
    var edgesSvg = svg('svg', { class:'sdf-edges' });
    graph.appendChild(edgesSvg);
    host.appendChild(graph);

    var maxX = 0, maxY = 0;
    NODES.forEach(function(n){
      if (n.x + NODE_W > maxX) maxX = n.x + NODE_W;
      if (n.y + NODE_H > maxY) maxY = n.y + NODE_H;
    });
    var GRAPH_W = maxX + 20;
    var GRAPH_H = maxY + 20;
    graph.style.width = GRAPH_W + 'px';
    graph.style.height = GRAPH_H + 'px';

    /* ---- node elements ---- */
    NODES.forEach(function(n){
      var div = el('div', 'sdf-node');
      div.dataset.id = n.id;
      div.style.left = n.x + 'px';
      div.style.top  = n.y + 'px';
      if (n.type === 'start') div.classList.add('is-start');
      if (n.type === 'end')   div.classList.add('is-end');
      div.innerHTML =
        '<span class="sdf-node-ic"><i class="fas fa-' + n.icon + '"></i></span>' +
        '<span class="sdf-node-txt"><b>' + n.name + '</b></span>';
      div.addEventListener('click', function(e){ onNodeClick(e, n, div); });
      graph.appendChild(div);
    });

    /* ---- detail overlay ---- */
    var detail = el('div', 'sdf-dtl');
    detail.innerHTML =
      '<div class="sdf-dtl-head">' +
        '<button class="sdf-dtl-back" data-action="close-dtl"><i class="fas fa-arrow-left"></i></button>' +
        '<span class="sdf-dtl-ic"><i class="fas fa-circle"></i></span>' +
        '<span class="sdf-dtl-name"></span>' +
        '<span class="sdf-dtl-state"></span>' +
        '<button class="sdf-dtl-x" data-action="close-dtl"><i class="fas fa-xmark"></i></button>' +
      '</div>' +
      '<div class="sdf-dtl-container"></div>';
    host.appendChild(detail);

    var detailIc = detail.querySelector('.sdf-dtl-ic i');
    var detailName = detail.querySelector('.sdf-dtl-name');
    var detailState = detail.querySelector('.sdf-dtl-state');
    var detailContainer = detail.querySelector('.sdf-dtl-container');
    var detailOpen = false;
    var detailNodeId = null;

    /* ---- spec editing delegation ---- */
    detail.addEventListener('click', function(e) {
      var act = e.target.closest('[data-action]');
      if (!act) return;
      switch (act.dataset.action) {
        case 'close-dtl': closeDetail(); break;
        case 'edit-spec': {
          var grp = detailContainer.querySelector('.sdf-dtl-spec');
          if (!grp) break;
          grp.querySelector('[data-role="spec-text"]').style.display = 'none';
          grp.querySelector('[data-role="spec-editor"]').style.display = '';
          grp.querySelector('[data-role="spec-actions"]').style.display = 'flex';
          act.textContent = 'Editando...';
          act.disabled = true;
          break;
        }
        case 'save-spec': {
          var n = detailNodeId ? NODE_MAP[detailNodeId] : null;
          if (!n) break;
          var key = 'sdd-spec-' + curSlug + '-' + detailNodeId;
          var ta = detailContainer.querySelector('[data-role="spec-editor"]');
          var txt = ta.value;
          try { localStorage.setItem(key, txt); } catch(e) {}
          detailContainer.querySelector('[data-role="spec-text"]').textContent = txt;
          detailContainer.querySelector('[data-role="spec-text"]').style.display = '';
          ta.style.display = 'none';
          detailContainer.querySelector('[data-role="spec-actions"]').style.display = 'none';
          var eb = detailContainer.querySelector('[data-action="edit-spec"]');
          eb.textContent = 'Editar'; eb.disabled = false;
          showToast('Spec guardado para ' + n.name);
          break;
        }
        case 'cancel-spec': {
          var n = detailNodeId ? NODE_MAP[detailNodeId] : null;
          if (!n) break;
          var key = 'sdd-spec-' + curSlug + '-' + detailNodeId;
          var saved; try { saved = localStorage.getItem(key); } catch(e) {}
          var ta = detailContainer.querySelector('[data-role="spec-editor"]');
          ta.value = saved || n.spec || '';
          detailContainer.querySelector('[data-role="spec-text"]').textContent = ta.value;
          detailContainer.querySelector('[data-role="spec-text"]').style.display = '';
          ta.style.display = 'none';
          detailContainer.querySelector('[data-role="spec-actions"]').style.display = 'none';
          var eb = detailContainer.querySelector('[data-action="edit-spec"]');
          eb.textContent = 'Editar'; eb.disabled = false;
          break;
        }
      }
    });

    function openDetail(node, nodeEl) {
      if (detailOpen && detailNodeId === node.id) { closeDetail(); return; }

      var state = 'pending';
      if (nodeEl.classList.contains('is-done')) state = 'done';
      else if (nodeEl.classList.contains('is-active')) state = 'active';

      detailIc.className = 'fas fa-' + node.icon;
      detailName.textContent = node.name;
      detailState.textContent = state;
      detailState.className = 'sdf-dtl-state sdf-dtl-state--' + state;
      detailContainer.innerHTML = detailContentHTML(node, curSlug);

      // render sub-graph
      var subgContainer = detailContainer.querySelector('[data-role="subgraph"]');
      if (subgContainer) {
        var subData = buildSubGraph(node.tools, node.ops, node.output);
        renderSubGraphSVG(subgContainer, subData);
      }

      detailNodeId = node.id;

      if (!detailOpen) {
        graph.classList.add('sdf-graph--min');
        void detail.offsetWidth;
        detail.classList.add('open');
        detailOpen = true;
      }
    }

    function closeDetail() {
      if (!detailOpen) return;
      detail.classList.remove('open');
      setTimeout(function() {
        graph.classList.remove('sdf-graph--min');
        detailNodeId = null;
        detailOpen = false;
      }, 350);
    }

    function onNodeClick(e, node, el) {
      openDetail(node, el);
    }

    function showToast(msg) {
      var t = document.getElementById('toast');
      if (!t) return;
      t.querySelector('#toastMessage').textContent = msg;
      t.classList.add('show');
      if (window.toastTimer) clearTimeout(window.toastTimer);
      window.toastTimer = setTimeout(function(){ t.classList.remove('show'); }, 2500);
    }

    /* ---- state ---- */
    var curSlug = WORKFLOWS[0].slug;
    var tick = 0;
    var tickerId = null;

    function scrollToNode(nodeId) {
      var nEl = graph.querySelector('.sdf-node[data-id="' + nodeId + '"]');
      if (!nEl) return;
      var targetLeft = nEl.offsetLeft - host.clientWidth/2 + nEl.offsetWidth/2;
      var targetTop  = nEl.offsetTop  - host.clientHeight/2 + nEl.offsetHeight/2;
      targetLeft = Math.max(0, Math.min(targetLeft, host.scrollWidth - host.clientWidth));
      targetTop  = Math.max(0, Math.min(targetTop,  host.scrollHeight - host.clientHeight));
      host.scrollTo({ left: targetLeft, top: targetTop, behavior: 'smooth' });
    }

    /* ---- draw edges ---- */
    function drawAllEdges(activeNodes, tickVal) {
      edgesSvg.innerHTML = '';
      edgesSvg.setAttribute('viewBox', '0 0 ' + GRAPH_W + ' ' + GRAPH_H);
      edgesSvg.setAttribute('width', GRAPH_W);
      edgesSvg.setAttribute('height', GRAPH_H);
      EDGES.forEach(function(edge){
        var a = NODE_MAP[edge.from], b = NODE_MAP[edge.to];
        if (!a || !b) return;
        var cx1 = a.x + NODE_W/2, cy1 = a.y + R;
        var cx2 = b.x + NODE_W/2, cy2 = b.y + R;
        var dx = cx2 - cx1, dy = cy2 - cy1;
        var dist = Math.sqrt(dx*dx + dy*dy) || 1;
        var x1 = cx1 + dx/dist * R, y1 = cy1 + dy/dist * R;
        var x2 = cx2 - dx/dist * R, y2 = cy2 - dy/dist * R;
        var cdx = Math.abs(x2-x1)*0.4, cdy = (y2-y1)*0.5;
        var d = 'M'+x1+','+y1+' C'+(x1+cdx)+','+(y1+cdy)+' '+(x2-cdx)+','+(y2-cdy)+' '+x2+','+y2;
        var base = svg('path', { class:'sdf-edge' + (edge.shortcut?' sdf-edge--shortcut':''), d: d });
        edgesSvg.appendChild(base);
        var isActive = false;
        if (activeNodes && tickVal !== undefined && tickVal > 0) {
          for (var i = 0; i < activeNodes.length - 1 && i < tickVal; i++) {
            if (activeNodes[i] === edge.from && activeNodes[i+1] === edge.to) { isActive = true; break; }
          }
        }
        var flow = svg('path', { class:'sdf-edge-flow', d: d });
        if (isActive) flow.classList.add('on');
        edgesSvg.appendChild(flow);
        if (edge.label) {
          var txt = svg('text', { class:'sdf-edge-label', x:(x1+x2)/2, y:(y1+y2)/2-10 });
          txt.textContent = edge.label;
          edgesSvg.appendChild(txt);
        }
      });
    }

    /* ---- apply workflow ---- */
    function applyWf(slug, t) {
      curSlug = slug;
      var wf = null;
      for (var i = 0; i < WORKFLOWS.length; i++) {
        if (WORKFLOWS[i].slug === slug) { wf = WORKFLOWS[i]; break; }
      }
      var activeIds = wf ? wf.nodes : [];
      tick = (typeof t === 'number') ? t : 0;
      if (tick >= activeIds.length) tick = 0;

      // update node states but KEEP detail open if it was open
      NODES.forEach(function(n){
        var idx = activeIds.indexOf(n.id);
        var nEl = graph.querySelector('.sdf-node[data-id="' + n.id + '"]');
        if (!nEl) return;
        if (idx >= 0) {
          nEl.style.display = '';
          nEl.classList.remove('is-done','is-active','is-pending','is-skipped');
          if (idx < tick) nEl.classList.add('is-done');
          else if (idx === tick) nEl.classList.add('is-active');
          else nEl.classList.add('is-pending');
        } else {
          nEl.style.display = 'none';
        }
      });

      drawAllEdges(activeIds, tick);

      // update detail state badge if detail is open for a node
      if (detailOpen && detailNodeId) {
        var nEl = graph.querySelector('.sdf-node[data-id="' + detailNodeId + '"]');
        if (nEl) {
          var st = 'pending';
          if (nEl.classList.contains('is-done')) st = 'done';
          else if (nEl.classList.contains('is-active')) st = 'active';
          detailState.textContent = st;
          detailState.className = 'sdf-dtl-state sdf-dtl-state--' + st;
        }
      }

      var targetId = activeIds[tick] || activeIds[activeIds.length-1];
      if (targetId && !detailOpen) scrollToNode(targetId);

      var total = activeIds.length;
      barEl.style.width = (total > 0 ? Math.round(tick / total * 100) : 0) + '%';
      var done = tick >= total;
      var curName = (!done && total > 0 && NODE_MAP[activeIds[tick]]) ? NODE_MAP[activeIds[tick]].name : 'Completado';
      phaseEl.innerHTML = '<i class="fas fa-' + (done ? 'circle-check' : 'circle-notch fa-spin') + '"></i> ' + curName;
      metaEl.textContent = Math.min(tick, total) + '/' + total + ' fases';
    }

    function switchWf(slug) {
      WORKFLOWS.forEach(function(wf){
        tabMap[wf.slug].classList.toggle('on', wf.slug === slug);
      });
      applyWf(slug, 0);
    }

    /* ---- ticker ---- */
    function advance() {
      var wf = null;
      for (var i = 0; i < WORKFLOWS.length; i++) {
        if (WORKFLOWS[i].slug === curSlug) { wf = WORKFLOWS[i]; break; }
      }
      var ids = wf ? wf.nodes : [];
      if (ids.length === 0) return;
      tick = (tick + 1) % (ids.length + 1);

      NODES.forEach(function(n){
        var idx = ids.indexOf(n.id);
        var nEl = graph.querySelector('.sdf-node[data-id="' + n.id + '"]');
        if (!nEl) return;
        if (idx >= 0) {
          nEl.style.display = '';
          nEl.classList.remove('is-done','is-active','is-pending','is-skipped');
          if (idx < tick) nEl.classList.add('is-done');
          else if (idx === tick) nEl.classList.add('is-active');
          else nEl.classList.add('is-pending');
        }
      });

      drawAllEdges(ids, tick);

      // update detail state if open
      if (detailOpen && detailNodeId) {
        var nEl = graph.querySelector('.sdf-node[data-id="' + detailNodeId + '"]');
        if (nEl) {
          var st = 'pending';
          if (nEl.classList.contains('is-done')) st = 'done';
          else if (nEl.classList.contains('is-active')) st = 'active';
          detailState.textContent = st;
          detailState.className = 'sdf-dtl-state sdf-dtl-state--' + st;
        }
      }

      var total = ids.length;
      barEl.style.width = (total > 0 ? Math.round(tick / total * 100) : 0) + '%';
      var done = tick >= total;
      var curName = (!done && total > 0 && NODE_MAP[ids[tick]]) ? NODE_MAP[ids[tick]].name : 'Completado';
      phaseEl.innerHTML = '<i class="fas fa-' + (done ? 'circle-check' : 'circle-notch fa-spin') + '"></i> ' + curName;
      metaEl.textContent = Math.min(tick, total) + '/' + total + ' fases';
    }

    function startTicker() {
      if (tickerId) clearInterval(tickerId);
      tickerId = setInterval(advance, TICK_MS);
    }

    /* ---- kickoff ---- */
    applyWf(WORKFLOWS[0].slug, 0);
    requestAnimationFrame(function() {
      host.scrollLeft = Math.max(0, Math.floor((GRAPH_W - host.clientWidth) / 2));
    });
    startTicker();
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
