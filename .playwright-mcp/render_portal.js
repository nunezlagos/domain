async (page) => {
  await page.setContent(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Domain Admin — Portal</title>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css" crossorigin="anonymous">
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.7/dist/chart.umd.min.js" crossorigin="anonymous"></script>
<style>
  :root {
    /* Django Admin palette (heritage) */
    --dj-primary: #79aec8;
    --dj-secondary: #417690;
    --dj-accent: #f5dd5d;
    --dj-link: #447e9b;
    --dj-link-hover: #036;
    --dj-button-hover: #609ab6;
    --dj-link-selected: #5b80b2;
    --dj-darkened: #f8f8f8;
    --dj-selected-bg: #e8f4fd;
    /* derived for dark mode */
    --dj-text: #e9e7f3;
    --dj-text-muted: rgba(233,231,243,0.45);
    --dj-border: rgba(255,255,255,0.08);
    --dj-bg: #0a0612;
    --dj-glass: rgba(255,255,255,0.05);
    --dj-glass-strong: rgba(255,255,255,0.08);
    --dj-accent-alpha: rgba(121,174,200,0.15);
    /* chart cycle */
    --dj-c1: #79aec8; --dj-c2: #f5dd5d; --dj-c3: #417690;
    --dj-c4: #447e9b; --dj-c5: #609ab6; --dj-c6: #5b80b2;
    --dj-c7: #86b9d0; --dj-c8: #306080;
  }
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  html, body { height: 100%; overflow: hidden; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Inter", system-ui, sans-serif;
    background: var(--dj-bg);
    color: var(--dj-text);
    -webkit-font-smoothing: antialiased;
    line-height: 1.5;
    overflow: hidden;
    user-select: none;
  }
  button { font: inherit; color: inherit; cursor: pointer; border: none; background: none; }
  a { color: inherit; text-decoration: none; }

  /* Background */
  .bg {
    position: fixed; inset: 0; z-index: 0; overflow: hidden;
    background: var(--dj-bg);
  }
  .bg::before, .bg::after, .bg-orb {
    content: ""; position: absolute; border-radius: 50%;
    filter: blur(140px); opacity: 0.55;
    animation: float 24s ease-in-out infinite;
  }
  .bg::before {
    width: 700px; height: 700px;
    background: linear-gradient(135deg, var(--dj-secondary), var(--dj-primary));
    top: -250px; left: -150px;
  }
  .bg::after {
    width: 600px; height: 600px;
    background: linear-gradient(135deg, #306080, var(--dj-secondary));
    bottom: -250px; right: -150px;
    animation-delay: -12s;
  }
  .bg-orb {
    width: 500px; height: 500px;
    background: linear-gradient(135deg, var(--dj-button-hover), var(--dj-link));
    top: 30%; left: 40%;
    animation-delay: -6s;
  }
  @keyframes float {
    0%, 100% { transform: translate(0,0) scale(1); }
    50% { transform: translate(100px, 80px) scale(1.1); }
  }

  .glass {
    background: var(--dj-glass);
    backdrop-filter: blur(24px) saturate(180%);
    -webkit-backdrop-filter: blur(24px) saturate(180%);
    border: 1px solid var(--dj-border);
    border-radius: 20px;
    box-shadow:
      0 8px 32px rgba(0, 0, 0, 0.4),
      inset 0 1px 0 rgba(255, 255, 255, 0.06);
  }

  /* Portal layout - dashboard */
  .portal {
    position: relative; z-index: 1;
    height: 100vh; width: 100vw;
    display: flex; flex-direction: column;
    overflow: hidden;
  }

  /* Header */
  .portal-header {
    position: relative;
    display: flex; align-items: center; gap: 16px;
    padding: 12px 24px;
    background: rgba(255, 255, 255, 0.02);
    border-bottom: 1px solid var(--dj-border);
    flex-shrink: 0;
    z-index: 10;
    animation: fadeIn 0.6s 0.1s forwards;
    opacity: 0;
  }
  .header-logo {
    display: flex; align-items: center; gap: 10px;
    flex-shrink: 0;
  }
  .header-logo svg { width: 36px; height: 36px; }
  .header-brand {
    font-size: 16px; font-weight: 700;
    background: linear-gradient(135deg, var(--dj-c1) 0%, var(--dj-c4) 100%);
    -webkit-background-clip: text; background-clip: text;
    color: transparent;
    letter-spacing: -0.02em;
  }
  .header-nav {
    display: flex; gap: 2px;
    margin-left: auto;
  }
  .header-nav-item {
    display: flex; align-items: center; gap: 5px;
    padding: 5px 9px;
    border-radius: 8px;
    font-size: 11px; font-weight: 500;
    color: var(--dj-text-muted);
    transition: all 0.2s;
    white-space: nowrap;
  }
  .header-nav-item:hover {
    color: #fff;
    background: var(--dj-glass);
  }
  .header-nav-item i { font-size: 12px; }

  /* User button */
  .header-user-btn {
    width: 30px; height: 30px; border-radius: 8px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--dj-border);
    display: flex; align-items: center; justify-content: center;
    font-size: 12px;
    color: var(--dj-text-muted);
    transition: all 0.2s;
    margin-left: 16px;
    padding: 0;
    flex-shrink: 0;
  }
  .header-user-btn:hover {
    background: var(--dj-glass-strong);
    color: #fff;
  }

  /* Profile card (dropdown from user btn) */
  .profile-card {
    position: absolute; top: calc(100% + 6px); right: 24px;
    z-index: 50;
    width: 220px;
    background: rgba(20, 16, 32, 0.96);
    backdrop-filter: blur(24px);
    border: 1px solid var(--dj-border);
    border-radius: 14px;
    box-shadow: 0 12px 48px rgba(0, 0, 0, 0.5);
    padding: 18px;
    text-align: center;
    opacity: 0;
    transform: translateY(-6px);
    pointer-events: none;
    transition: opacity 0.2s, transform 0.2s;
  }
  .profile-card.open {
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }
  .profile-avatar {
    width: 44px; height: 44px; border-radius: 12px;
    background: linear-gradient(135deg, var(--dj-accent-alpha), rgba(96,165,250,0.1));
    border: 1px solid var(--dj-border);
    display: flex; align-items: center; justify-content: center;
    font-size: 14px; font-weight: 700;
    margin: 0 auto 10px;
    color: var(--dj-c1);
  }
  .profile-name { font-size: 14px; font-weight: 600; }
  .profile-email { font-size: 11px; color: var(--dj-text-muted); margin-top: 2px; }
  .profile-role {
    display: inline-block;
    margin-top: 8px;
    padding: 2px 10px;
    border-radius: 4px;
    font-size: 10px; font-weight: 600;
    background: var(--dj-accent-alpha);
    color: var(--dj-c1);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .profile-meta {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--dj-border);
    display: flex; flex-direction: column; gap: 4px;
    font-size: 11px;
    color: rgba(233,231,243,0.35);
  }
  .profile-meta span { display: flex; align-items: center; gap: 6px; justify-content: center; }
  .profile-logout {
    margin-top: 14px;
    width: 100%; padding: 8px;
    border-radius: 8px;
    font-size: 12px; font-weight: 500;
    color: var(--dj-text-muted);
    display: flex; align-items: center; justify-content: center; gap: 6px;
    background: rgba(255,255,255,0.03);
    border: 1px solid var(--dj-border);
    transition: all 0.2s;
  }
  .profile-logout:hover {
    background: rgba(239,68,68,0.15);
    border-color: rgba(239,68,68,0.3);
    color: #fca5a5;
  }

  /* Dashboard body */
  .dashboard {
    flex: 1;
    display: flex; flex-direction: column;
    padding: 10px 16px;
    gap: 8px;
    animation: fadeIn 0.6s 0.2s forwards;
    opacity: 0;
    overflow: hidden;
    min-height: 0;
  }

  .dash-filters {
    flex-shrink: 0;
    display: flex; gap: 6px;
    align-items: center;
  }
  .dash-filter {
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 10px; font-weight: 500;
    color: var(--dj-text-muted);
    background: transparent;
    border: 1px solid var(--dj-border);
    transition: all 0.15s;
  }
  .dash-filter:hover {
    color: var(--dj-text);
    border-color: var(--dj-glass-strong);
  }
  .dash-filter.active {
    color: #fff;
    background: var(--dj-glass);
    border-color: var(--dj-glass-strong);
  }

  .dash-mosaic {
    flex: 1;
    display: grid;
    grid-template-columns: 3fr 1fr 1fr;
    grid-template-rows: 1fr 1fr;
    gap: 10px;
    min-height: 0;
  }
  .mosaic-item {
    padding: 10px;
    display: flex; flex-direction: column;
    min-height: 0;
    position: relative;
    overflow: hidden;
  }
  .mosaic-item::before {
    content: ''; position: absolute; top: 0; left: 8px; right: 8px;
    height: 2px; border-radius: 0 0 2px 2px;
  }
  .mosaic-item:nth-child(1) { border-radius: 16px; }
  .mosaic-item:nth-child(1)::before { background: linear-gradient(90deg, var(--dj-c1), var(--dj-c4)); }
  .mosaic-item:nth-child(2) { border-radius: 16px; }
  .mosaic-item:nth-child(2)::before { background: var(--dj-c5); }
  .mosaic-item:nth-child(3) {
    border-radius: 16px;
    transform: rotate(-1.2deg);
    background: linear-gradient(135deg, var(--dj-accent-alpha), rgba(96,165,250,0.05));
    border: 1px solid var(--dj-border);
    box-shadow: 0 4px 24px rgba(65,118,144,0.15);
    z-index: 2;
  }
  .mosaic-item:nth-child(4) { border-radius: 16px; }
  .mosaic-item:nth-child(4)::before { background: var(--dj-c2); }
  .mosaic-item:nth-child(5) { border-radius: 16px; }
  .mosaic-item:nth-child(5)::before { background: var(--dj-c3); }
  .mosaic-item:nth-child(6) { border-radius: 16px; }
  .mosaic-item:nth-child(6)::before { background: var(--dj-c6); }

  .mosaic-item .mi-title {
    flex-shrink: 0;
    font-size: 10px; font-weight: 600;
    color: var(--dj-text-muted);
    margin-bottom: 6px;
    display: flex; align-items: center; gap: 5px;
  }
  .mosaic-item .mi-canvas-wrap {
    flex: 1;
    position: relative;
    min-height: 0;
  }
  .mosaic-item canvas {
    position: absolute; inset: 0;
    width: 100% !important; height: 100% !important;
  }

  .dash-badge {
    display: inline-block; padding: 1px 7px; border-radius: 4px;
    font-size: 10px; font-weight: 600;
  }

  /* Phone-style KPI widget (mosaic-item:nth-child(3)) */
  .phone-widget {
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    gap: 4px;
    padding: 8px;
    height: 100%;
    position: relative;
  }
  .phone-widget::before {
    content: ''; position: absolute; top: 5px;
    width: 32px; height: 3px; border-radius: 2px;
    background: var(--dj-border);
  }
  .phone-widget .pw-row {
    display: flex; align-items: center; gap: 6px;
    width: 100%;
    padding: 4px 6px;
    border-radius: 6px;
    background: rgba(255,255,255,0.03);
  }
  .phone-widget .pw-icon {
    width: 20px; height: 20px; border-radius: 5px;
    display: flex; align-items: center; justify-content: center;
    font-size: 9px;
    flex-shrink: 0;
  }
  .phone-widget .pw-info { flex: 1; min-width: 0; }
  .phone-widget .pw-label { font-size: 8px; color: var(--dj-text-muted); text-transform: uppercase; letter-spacing: 0.04em; }
  .phone-widget .pw-value { font-size: 13px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .phone-widget .pw-trend { font-size: 9px; }

  /* Split panel (30/70) for first mosaic item */
  .ms-left {
    width: 30%; flex-shrink: 0;
    display: flex; flex-direction: column;
    padding: 10px;
    border-right: 1px solid var(--dj-border);
    background: rgba(255,255,255,0.02);
  }
  .ms-left-title {
    font-size: 10px; font-weight: 600;
    color: var(--dj-text-muted);
    margin-bottom: 8px;
    display: flex; align-items: center; gap: 5px;
  }
  .ms-left-list { flex: 1; display: flex; flex-direction: column; gap: 4px; }
  .msl-row {
    display: flex; align-items: flex-start; gap: 6px;
    padding: 4px 6px;
    border-radius: 6px;
    background: rgba(255,255,255,0.03);
  }
  .msl-dot { width: 5px; height: 5px; border-radius: 50%; margin-top: 4px; flex-shrink: 0; }
  .msl-info { flex: 1; min-width: 0; }
  .msl-name { font-size: 11px; font-weight: 500; }
  .msl-meta { display: flex; gap: 6px; font-size: 9px; color: var(--dj-text-muted); }
  .msl-footer {
    font-size: 9px; color: rgba(233,231,243,0.25);
    text-align: center;
    padding-top: 6px;
    border-top: 1px solid var(--dj-border);
    margin-top: 4px;
  }
  .ms-right {
    flex: 1;
    display: flex; flex-direction: column;
    padding: 10px;
    min-width: 0;
  }
  .ms-right .mi-title { margin-bottom: 6px; }

  /* Footer */
  .portal-footer {
    padding: 10px 24px;
    background: rgba(255, 255, 255, 0.02);
    border-top: 1px solid var(--dj-border);
    display: flex; align-items: center; justify-content: space-between;
    font-size: 11px;
    color: rgba(233, 231, 243, 0.35);
    flex-shrink: 0;
    animation: fadeIn 0.6s 0.3s forwards;
    opacity: 0;
  }
  .portal-footer .status-dot {
    display: inline-block;
    width: 6px; height: 6px; border-radius: 50%;
    background: #4ade80;
    margin-right: 6px;
    box-shadow: 0 0 6px rgba(74, 222, 128, 0.5);
  }

  @keyframes fadeIn { to { opacity: 1; } }

  /* ============================================================
     VIEW (slide-in modal)
     ============================================================ */
  .view-backdrop {
    position: fixed; inset: 0; z-index: 100;
    background: rgba(10, 6, 18, 0.6);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    opacity: 0; pointer-events: none;
    transition: opacity 0.4s;
  }
  .view-backdrop.active { opacity: 1; pointer-events: auto; }

  .view {
    position: fixed;
    top: 50%; left: 50%;
    transform: translate(-50%, -50%) scale(0.7);
    z-index: 101;
    width: 94vw; max-width: 1280px;
    height: 90vh; max-height: 820px;
    background: rgba(20, 16, 32, 0.92);
    backdrop-filter: blur(32px) saturate(180%);
    -webkit-backdrop-filter: blur(32px) saturate(180%);
    border: 1px solid var(--dj-border);
    border-radius: 24px;
    box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6);
    display: flex; flex-direction: column;
    overflow: hidden;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.45s, transform 0.55s cubic-bezier(0.34, 1.4, 0.64, 1);
  }
  .view.active {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1);
    pointer-events: auto;
  }

  .view-header {
    padding: 9px 14px;
    border-bottom: 1px solid var(--dj-border);
    display: flex; align-items: center; gap: 10px;
    background: linear-gradient(135deg, rgba(65,118,144,0.08), rgba(121,174,200,0.08));
    flex-shrink: 0;
  }
  .view-back {
    width: 28px; height: 28px; border-radius: 7px;
    background: var(--dj-glass);
    border: 1px solid var(--dj-border);
    display: flex; align-items: center; justify-content: center;
    transition: all 0.2s;
  }
  .view-back:hover { background: var(--dj-glass-strong); transform: translateX(-2px); }
  .view-back-icon { width: 16px; height: 16px; color: #fff; }
  .view-header-icon {
    width: 32px; height: 32px; border-radius: 8px;
    background: linear-gradient(135deg, rgba(65,118,144,0.15), rgba(121,174,200,0.15));
    border: 1px solid var(--dj-border);
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
  }
  .view-header-icon svg { width: 22px; height: 22px; }
  .view-header-text { flex: 1; min-width: 0; }
  .view-title { font-size: 18px; font-weight: 700; }
  .view-subtitle { font-size: 12px; color: var(--dj-text-muted); }
  .view-header-actions { display: flex; gap: 8px; }
  .view-close {
    width: 24px; height: 24px; border-radius: 6px;
    background: transparent;
    color: var(--dj-text-muted);
    display: flex; align-items: center; justify-content: center;
    transition: all 0.2s;
  }
  .view-close:hover { background: rgba(255, 100, 100, 0.15); color: #f87171; }

  .view-body { flex: 1; overflow-y: auto; padding: 24px 28px; }
  .view-body::-webkit-scrollbar { width: 8px; }
  .view-body::-webkit-scrollbar-track { background: transparent; }
  .view-body::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 4px; }

  .view-toolbar {
    display: flex; gap: 8px; margin-bottom: 20px;
    align-items: center;
  }
  .view-search { flex: 1; position: relative; }
  .view-search input {
    width: 100%;
    padding: 10px 12px 10px 36px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid var(--dj-border);
    border-radius: 10px;
    color: var(--dj-text);
    font-family: inherit; font-size: 13px;
  }
  .view-search input:focus {
    outline: none;
    border-color: var(--dj-c1);
    background: rgba(255, 255, 255, 0.06);
    box-shadow: 0 0 0 3px var(--dj-accent-alpha);
  }
  .view-search input::placeholder { color: var(--dj-text-muted); }
  .view-search svg {
    position: absolute; left: 12px; top: 50%;
    transform: translateY(-50%);
    color: var(--dj-text-muted);
  }
  .view-btn {
    padding: 9px 14px; border-radius: 10px;
    background: var(--dj-glass);
    border: 1px solid var(--dj-border);
    color: var(--dj-text);
    font-size: 13px; font-weight: 500;
    display: inline-flex; align-items: center; gap: 6px;
    transition: all 0.2s;
    white-space: nowrap;
  }
  .view-btn:hover { background: var(--dj-glass-strong); }
  .view-btn.primary {
    background: linear-gradient(135deg, var(--dj-secondary), var(--dj-link));
    border-color: transparent;
    color: #fff;
    box-shadow: 0 4px 12px rgba(65,118,144,0.3);
  }

  .view-stats {
    display: grid; grid-template-columns: repeat(4, 1fr);
    gap: 12px; margin-bottom: 20px;
  }
  .view-stat { padding: 16px 18px; }
  .view-stat-label {
    font-size: 11px; color: var(--dj-text-muted);
    text-transform: uppercase; letter-spacing: 0.08em;
    margin-bottom: 6px;
  }
  .view-stat-value {
    font-size: 24px; font-weight: 700;
    font-variant-numeric: tabular-nums;
  }

  .view-table {
    width: 100%;
    border-collapse: separate;
    border-spacing: 0;
    font-size: 13px;
  }
  .view-table th {
    text-align: left; padding: 11px 14px 7px;
    font-size: 10px; text-transform: uppercase; letter-spacing: 0.08em;
    color: transparent; font-weight: 600;
    background: transparent;
    border-bottom: none;
    position: relative;
    cursor: default;
  }
  .view-table th::after {
    content: attr(data-tooltip);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%) translateY(4px);
    padding: 6px 10px;
    background: rgba(20, 16, 32, 0.95);
    backdrop-filter: blur(20px);
    border: 1px solid var(--dj-border);
    border-radius: 8px;
    color: var(--dj-text);
    font-size: 11px;
    text-transform: none;
    letter-spacing: 0;
    white-space: nowrap;
    pointer-events: none;
    opacity: 0;
    transform: translateX(-50%) translateY(8px);
    transition: opacity 0.15s, transform 0.15s;
    z-index: 10;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }
  .view-table th::before {
    content: attr(data-tooltip);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 4px solid transparent;
    border-top-color: rgba(20, 16, 32, 0.95);
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
    z-index: 10;
  }
  .view-table th:hover {
    color: var(--dj-text);
  }
  .view-table th:hover::after,
  .view-table th:hover::before {
    opacity: 1;
  }
  .view-table th:hover::after {
    transform: translateX(-50%) translateY(-4px);
  }
  .view-table td {
    padding: 12px 14px;
    border-bottom: 1px solid var(--dj-border);
  }
  .view-table tbody tr:hover { background: rgba(255, 255, 255, 0.04); }
  .view-table-actions { display: flex; gap: 6px; }
  .view-table-btn {
    width: 28px; height: 28px; border-radius: 6px;
    background: var(--dj-glass);
    color: var(--dj-text-muted);
    display: flex; align-items: center; justify-content: center;
    transition: all 0.15s;
  }
  .view-table-btn:hover { background: var(--dj-glass-strong); }
  .view-table-btn.danger:hover { background: rgba(239, 68, 68, 0.2); color: #fca5a5; }

  .badge {
    display: inline-block;
    padding: 3px 8px; border-radius: 4px;
    font-size: 11px; font-weight: 600;
  }
  .badge.active { background: rgba(74, 222, 128, 0.15); color: #4ade80; }
  .badge.inactive { background: var(--dj-glass); color: var(--dj-text-muted); }
  .badge.mcp { background: rgba(96, 165, 250, 0.15); color: #93c5fd; }
  .badge.prompt { background: var(--dj-accent-alpha); color: var(--dj-c1); }
  .badge.code { background: rgba(34, 197, 94, 0.15); color: #4ade80; }

  .view-empty {
    padding: 60px 24px;
    text-align: center;
    color: var(--dj-text-muted);
  }
  .view-empty h3 { font-size: 18px; font-weight: 600; margin-bottom: 6px; color: var(--dj-text); }
  .view-empty p { font-size: 13px; margin-bottom: 16px; }

  /* Modal */
  .modal-backdrop {
    position: fixed; inset: 0; z-index: 200;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(4px);
    opacity: 0; pointer-events: none;
    transition: opacity 0.3s;
  }
  .modal-backdrop.active { opacity: 1; pointer-events: auto; }
  .modal {
    position: fixed; top: 50%; left: 50%;
    transform: translate(-50%, -50%) scale(0.9);
    z-index: 201;
    width: 90vw; max-width: 520px;
    background: rgba(20, 16, 32, 0.95);
    backdrop-filter: blur(32px);
    border: 1px solid var(--dj-border);
    border-radius: 20px;
    box-shadow: 0 24px 80px rgba(0, 0, 0, 0.5);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.3s, transform 0.3s;
  }
  .modal.active {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1);
    pointer-events: auto;
  }
  .modal-header {
    padding: 20px 24px;
    border-bottom: 1px solid var(--dj-border);
    display: flex; align-items: center; justify-content: space-between;
  }
  .modal-header h2 { font-size: 18px; font-weight: 700; }
  .modal-close {
    width: 32px; height: 32px; border-radius: 8px;
    background: transparent; color: var(--dj-text-muted);
    display: flex; align-items: center; justify-content: center;
  }
  .modal-close:hover { background: rgba(255, 100, 100, 0.15); color: #f87171; }
  .modal-form { padding: 20px 24px; }
  .form-row { margin-bottom: 16px; }
  .form-row label {
    display: block; font-size: 11px; font-weight: 600;
    color: var(--dj-text);
    text-transform: uppercase; letter-spacing: 0.05em;
    margin-bottom: 6px;
  }
  .form-row label .required { color: #f87171; margin-left: 2px; }
  .form-row input, .form-row textarea, .form-row select {
    width: 100%; padding: 10px 12px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid var(--dj-border);
    border-radius: 8px;
    color: var(--dj-text);
    font-family: inherit; font-size: 14px;
  }
  .form-row input:focus, .form-row textarea:focus, .form-row select:focus {
    outline: none;
    border-color: var(--dj-c1);
    background: rgba(255, 255, 255, 0.06);
    box-shadow: 0 0 0 3px var(--dj-accent-alpha);
  }
  .form-row textarea { resize: vertical; min-height: 70px; font-family: inherit; }
  .form-row .error-msg { color: #f87171; font-size: 11px; margin-top: 4px; display: none; }
  .form-row.has-error input { border-color: rgba(239, 68, 68, 0.5); }
  .form-row.has-error .error-msg { display: block; }
  .form-row select option { background: #1c162c; }
  .form-actions {
    display: flex; gap: 10px; justify-content: flex-end;
    margin-top: 24px; padding-top: 16px;
    border-top: 1px solid var(--dj-border);
  }

  /* Toast */
  .toast {
    position: fixed; bottom: 24px; right: 24px; z-index: 300;
    padding: 12px 20px;
    background: rgba(20, 16, 32, 0.95);
    backdrop-filter: blur(20px);
    border: 1px solid var(--dj-border);
    border-radius: 12px;
    color: #fff;
    font-size: 13px; font-weight: 500;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    transform: translateX(120%);
    transition: transform 0.3s;
    display: flex; align-items: center; gap: 10px;
  }
  .toast.active { transform: translateX(0); }
  .toast.success { border-color: rgba(34, 197, 94, 0.3); }
  .toast.error { border-color: rgba(239, 68, 68, 0.3); }

  ::selection { background: var(--dj-accent-alpha); color: #fff; }

  @media (max-width: 700px) {
    .dash-mosaic { grid-template-columns: 1fr 1fr; grid-template-rows: auto; }
    .dash-mosaic .mosaic-item:nth-child(3) { transform: none; grid-column: 1 / -1; }
    .view { width: 96vw; height: 92vh; }
    .view-stats { grid-template-columns: repeat(2, 1fr); }
  }
</style>
</head>
<body>

<div class="bg"></div>

<div class="portal" id="portal">
  <header class="portal-header">
    <div class="header-logo">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" stroke="url(#brandGrad)" stroke-width="1.8" stroke-linecap="round">
        <defs>
          <linearGradient id="brandGrad" x1="0" y1="0" x2="48" y2="48" gradientUnits="userSpaceOnUse">
            <stop offset="0" stop-color="#79aec8"/>
            <stop offset="0.5" stop-color="#447e9b"/>
            <stop offset="1" stop-color="#417690"/>
          </linearGradient>
        </defs>
        <line x1="24" y1="6" x2="24" y2="42" />
        <line x1="6" y1="24" x2="42" y2="24" />
        <circle cx="24" cy="6" r="4.5" />
        <circle cx="42" cy="24" r="4.5" />
        <circle cx="24" cy="42" r="4.5" />
        <circle cx="6" cy="24" r="4.5" />
        <circle cx="24" cy="24" r="6" />
      </svg>
      <span class="header-brand">Domain Admin</span>
    </div>
    <nav class="header-nav" id="headerNav">
    </nav>
    <button class="header-user-btn" id="userBtn" title="Usuario" aria-label="Usuario">
      <i class="fas fa-user"></i>
    </button>
    <div class="profile-card" id="profileCard">
      <div class="profile-avatar">AD</div>
      <div class="profile-name">Admin</div>
      <div class="profile-email">admin@domain.app</div>
      <div class="profile-role">Administrator</div>
      <div class="profile-meta">
        <span><i class="fas fa-clock"></i> Hoy 09:42</span>
        <span><i class="fas fa-globe"></i> 13.140.183.236</span>
      </div>
      <button class="profile-logout"><i class="fas fa-sign-out-alt"></i> Cerrar sesion</button>
    </div>
  </header>

  <div class="dashboard" id="dashboard">
    <div class="dash-filters" id="dashFilters">
      <button class="dash-filter active">All</button>
      <button class="dash-filter">Active</button>
      <button class="dash-filter">Inactive</button>
    </div>

    <div class="dash-mosaic">
      <div class="mosaic-item glass" style="padding:0;overflow:hidden;display:flex;">
        <div class="ms-left">
          <div class="ms-left-title"><i class="fas fa-sync-alt"></i> Workflows</div>
          <div class="ms-left-list">
            <div class="msl-row">
              <span class="msl-dot" style="background:#4ade80"></span>
              <div class="msl-info"><div class="msl-name">SDD Pipeline</div><div class="msl-meta"><span>10 phases</span><span>2h</span></div></div>
            </div>
            <div class="msl-row">
              <span class="msl-dot" style="background:var(--dj-c2)"></span>
              <div class="msl-info"><div class="msl-name">Issue Intake</div><div class="msl-meta"><span>5 phases</span><span>1m</span></div></div>
            </div>
          </div>
          <div class="msl-footer">2 active · 59 runs</div>
        </div>
        <div class="ms-right">
          <div class="mi-title"><i class="fas fa-chart-line"></i> Token Usage (7d)</div>
          <div class="mi-canvas-wrap"><canvas id="chartTokens"></canvas></div>
        </div>
      </div>
      <div class="mosaic-item glass">
        <div class="mi-title"><i class="fas fa-chart-pie"></i> Requests</div>
        <div class="mi-canvas-wrap"><canvas id="chartRequests"></canvas></div>
      </div>
      <div class="mosaic-item">
        <div class="phone-widget" id="phoneWidget">
          <div class="pw-row">
            <div class="pw-icon" style="background:rgba(121,174,200,0.15);color:#79aec8;"><i class="fas fa-robot"></i></div>
            <div class="pw-info"><div class="pw-label">Agentes</div><div class="pw-value">3</div></div>
            <div class="pw-trend" style="color:#4ade80;">+12%</div>
          </div>
          <div class="pw-row">
            <div class="pw-icon" style="background:rgba(245,221,93,0.15);color:#f5dd5d;"><i class="fas fa-bolt"></i></div>
            <div class="pw-info"><div class="pw-label">Skills</div><div class="pw-value">2.7k</div></div>
            <div class="pw-trend" style="color:#4ade80;">98%</div>
          </div>
          <div class="pw-row">
            <div class="pw-icon" style="background:rgba(68,126,155,0.15);color:#447e9b;"><i class="fas fa-database"></i></div>
            <div class="pw-info"><div class="pw-label">Total Requests</div><div class="pw-value">5.4k</div></div>
            <div class="pw-trend" style="color:var(--dj-c2);">+8%</div>
          </div>
        </div>
      </div>
      <div class="mosaic-item glass">
        <div class="mi-title"><i class="fas fa-chart-bar"></i> Frecuencia 24h</div>
        <div class="mi-canvas-wrap"><canvas id="chartFrequency"></canvas></div>
      </div>
      <div class="mosaic-item glass">
        <div class="mi-title"><i class="fas fa-code-branch"></i> Flows Pipeline</div>
        <div class="mi-canvas-wrap"><canvas id="chartFlows"></canvas></div>
      </div>
      <div class="mosaic-item glass">
        <div class="mi-title"><i class="fas fa-project-diagram"></i> Relaciones</div>
        <div class="mi-canvas-wrap"><canvas id="chartHealth"></canvas></div>
      </div>
    </div>

  </div>

  <footer class="portal-footer">
    <span><span class="status-dot"></span>All systems healthy</span>
    <span>Domain Admin v1.3 · MiniMax-M3 · MCP</span>
  </footer>
</div>

<!-- View modal -->
<div class="view-backdrop" id="viewBackdrop"></div>
<div class="view" id="view" role="dialog" aria-modal="true" aria-hidden="true">
  <header class="view-header">
    <button class="view-back" id="viewBack" aria-label="Volver">
      <svg class="view-back-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 18l-6-6 6-6"/></svg>
    </button>
    <div class="view-header-icon" id="viewHeaderIcon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="url(#brandGrad)" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  <defs>
    <linearGradient id="brandGrad" x1="0" y1="0" x2="24" y2="24" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#79aec8"/>
      <stop offset="0.5" stop-color="#447e9b"/>
      <stop offset="1" stop-color="#417690"/>
    </linearGradient>
  </defs>
  <line x1="14.25" y1="7.90" x2="16.68" y2="12.10" />
  <line x1="16.68" y1="11.90" x2="14.25" y2="16.10" />
  <line x1="14.43" y1="16.00" x2="9.57" y2="16.00" />
  <line x1="9.75" y1="16.10" x2="7.32" y2="11.90" />
  <line x1="7.32" y1="12.10" x2="9.75" y2="7.90" />
  <line x1="9.57" y1="8.00" x2="14.43" y2="8.00" />
  <circle cx="12.00" cy="4.00" r="4.5" />
  <circle cx="18.93" cy="8.00" r="4.5" />
  <circle cx="18.93" cy="16.00" r="4.5" />
  <circle cx="12.00" cy="20.00" r="4.5" />
  <circle cx="5.07" cy="16.00" r="4.5" />
  <circle cx="5.07" cy="8.00" r="4.5" />
  <circle cx="12" cy="12" r="3.5" />
</svg></div>
    <div class="view-header-text">
      <div class="view-title" id="viewTitle">Skills</div>
      <div class="view-subtitle" id="viewSubtitle">Mantenedor</div>
    </div>
    <div class="view-header-actions">
      <button class="view-close" id="viewClose" aria-label="Cerrar">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"/></svg>
      </button>
    </div>
  </header>
  <div class="view-body" id="viewBody"></div>
</div>

<!-- Modal -->
<div class="modal-backdrop" id="modalBackdrop"></div>
<div class="modal" id="modal" role="dialog" aria-modal="true" aria-hidden="true">
  <header class="modal-header">
    <h2 id="modalTitle">Nuevo</h2>
    <button class="modal-close" id="modalClose" aria-label="Cerrar">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"/></svg>
    </button>
  </header>
  <form class="modal-form" id="modalForm">
    <div class="form-row">
      <label>Nombre <span class="required">*</span></label>
      <input type="text" name="name" required placeholder="ej: Mi agente" />
      <div class="error-msg">El nombre es obligatorio</div>
    </div>
    <div class="form-row">
      <label>Slug <span class="required">*</span></label>
      <input type="text" name="slug" required placeholder="ej: mi-agente" pattern="[a-z0-9-]+" />
      <div class="error-msg">Solo minusculas, numeros y guiones</div>
    </div>
    <div class="form-row">
      <label>Descripcion</label>
      <textarea name="description" rows="3" placeholder="Opcional"></textarea>
    </div>
    <div class="form-row">
      <label>Estado</label>
      <select name="status">
        <option value="active">Activo</option>
        <option value="inactive">Inactivo</option>
      </select>
    </div>
    <div class="form-actions">
      <button type="button" class="view-btn" id="modalCancel">Cancelar</button>
      <button type="submit" class="view-btn primary">Guardar</button>
    </div>
  </form>
</div>

<!-- Toast -->
<div class="toast" id="toast">
  <span id="toastIcon">✓</span>
  <span id="toastMessage">Mensaje</span>
</div>

<script>
/* ============================================================
   SEGMENTS
   ============================================================ */
const SEGMENTS = [
  { id: 'agents',   label: 'Agents',   icon: 'robot',      color: '#79aec8' },
  { id: 'skills',   label: 'Skills',   icon: 'bolt',       color: '#f5dd5d' },
  { id: 'flows',    label: 'Flows',    icon: 'repeat',     color: '#417690' },
  { id: 'prompts',  label: 'Prompts',  icon: 'file-lines', color: '#447e9b' },
  { id: 'projects', label: 'Proyectos',icon: 'folder',     color: '#609ab6' },
  { id: 'users',    label: 'Usuarios', icon: 'users',      color: '#5b80b2' },
  { id: 'policies', label: 'Politicas',icon: 'shield',     color: '#86b9d0' },
  { id: 'crons',    label: 'Crons',    icon: 'clock',      color: '#306080' },
];

/* ============================================================
   HEADER NAV (dynamic from SEGMENTS)
   ============================================================ */
const headerNav = document.getElementById('headerNav');
SEGMENTS.forEach(seg => {
  const btn = document.createElement('button');
  btn.className = 'header-nav-item';
  btn.innerHTML = \`<i class="fas fa-\${seg.icon}"></i>\${seg.label}\`;
  btn.addEventListener('click', () => openSegment(seg));
  headerNav.appendChild(btn);
});

/* Dashboard filter pills */
document.querySelectorAll('.dash-filter').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.dash-filter').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
  });
});

/* User button -> profile modal */
document.getElementById('userBtn').addEventListener('click', openProfile);

/* ============================================================
   DASHBOARD CHARTS (Chart.js)
   ============================================================ */
let charts = {};

/* ============================================================
   MOCK DATA (must be before initCharts)
   ============================================================ */
const MOCK_DATA = {
  agents: [
    { name: 'Bot de Soporte',     slug: 'soporte-bot',  provider: 'minimax',   model: 'MiniMax-M3',  status: 'active',   calls: 847 },
    { name: 'Code Reviewer',      slug: 'code-reviewer', provider: 'anthropic', model: 'claude-sonnet-4-5', status: 'active', calls: 312 },
    { name: 'SDD Generator',      slug: 'sdd-generator', provider: 'openai',   model: 'gpt-4o',     status: 'inactive', calls: 0 },
  ],
  skills: [
    { name: 'Send Email',         slug: 'send-email',    type: 'mcp', desc: 'Envia emails transaccionales',     calls: 1247, success: 98 },
    { name: 'Query Database',     slug: 'query-db',      type: 'code', desc: 'Queries SQL de solo lectura',     calls: 892,  success: 99 },
    { name: 'Web Search',         slug: 'web-search',    type: 'mcp', desc: 'Busqueda web via Brave API',       calls: 654,  success: 68 },
  ],
  flows: [
    { name: 'SDD Pipeline v1',  slug: 'sdd-pipeline-v1', phases: 10, status: 'active',   runs: 12 },
    { name: 'Issue Intake',     slug: 'issue-intake',   phases: 5,  status: 'active',   runs: 47 },
  ],
  prompts: [
    { name: 'Code Review',        slug: 'code-review',     model: 'claude-sonnet-4-5', status: 'active',   uses: 312 },
    { name: 'SDD Phase: Spec',    slug: 'sdd-spec',        model: 'claude-sonnet-4-5', status: 'active',   uses: 89 },
  ],
  projects: [
    { name: 'test-kanban',        slug: 'test-kanban',   status: 'active',   skills: 8, agents: 1, flows: 1 },
  ],
  users: [
    { name: 'admin@admin.com',    email: 'admin@admin.com', role: 'admin',   status: 'active' },
    { name: 'operator@saargo.com', email: 'operator@saargo.com', role: 'operator', status: 'active' },
  ],
  policies: [
    { name: 'No PII in logs',     scope: 'platform', kind: 'security_rule',  status: 'active' },
    { name: 'No PII in commits',  scope: 'platform', kind: 'security_rule',  status: 'active' },
  ],
  crons: [
    { name: 'Backup diario',      schedule: '0 2 * * *',     status: 'active',   last_run: 'hace 2h' },
    { name: 'Health check',       schedule: '*/5 * * * *',   status: 'active',   last_run: 'hace 1m' },
  ],
};

const SEGMENT_SUBTITLES = {
  agents: 'Agentes LLM del sistema',
  skills: 'Habilidades reutilizables',
  flows: 'Pipelines de ejecucion',
  prompts: 'Templates de prompts',
  projects: 'Proyectos multi-tenant',
  users: 'Usuarios y roles',
  policies: 'Platform + project + skill',
  crons: 'Trabajos programados',
};

function initCharts() {
  const ctx1 = document.getElementById('chartTokens').getContext('2d');
  const days = ['Lun','Mar','Mie','Jue','Vie','Sab','Dom'];
  charts.tokens = new Chart(ctx1, {
    type: 'line',
    data: {
      labels: days,
      datasets: [
        { label: 'Input Tokens', data: [12400,13800,11200,15600,14200,9800,16500],
          borderColor: '#79aec8', backgroundColor: 'rgba(121,174,200,0.1)',
          fill: true, tension: 0, stepped: 'before', pointRadius: 5, pointHoverRadius: 7, pointBorderColor: '#79aec8', pointBackgroundColor: '#0a0612', pointBorderWidth: 2.5, borderWidth: 2.5 },
        { label: 'Output Tokens', data: [8900,9200,7800,10500,9800,7200,11200],
          borderColor: '#447e9b', backgroundColor: 'rgba(68,126,155,0.1)',
          fill: true, tension: 0, stepped: 'before', pointRadius: 5, pointHoverRadius: 7, pointBorderColor: '#447e9b', pointBackgroundColor: '#0a0612', pointBorderWidth: 2.5, borderWidth: 2.5 }
      ]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { labels: { color: 'rgba(233,231,243,0.45)', font: { size: 10 }, boxWidth: 10, padding: 10, usePointStyle: true, pointStyle: 'circle' } } },
      scales: {
        x: { ticks: { color: 'rgba(233,231,243,0.25)', font: { size: 9 } }, grid: { color: 'rgba(255,255,255,0.02)' } },
        y: { ticks: { color: 'rgba(233,231,243,0.25)', font: { size: 9 } }, grid: { color: 'rgba(255,255,255,0.02)' }, beginAtZero: true }
      }
    }
  });

  // Requests - computed from MOCK_DATA
  const reqLabels = ['Agents','Skills','Flows','Prompts','Crons'];
  const reqData = [
    MOCK_DATA.agents.reduce((s,d) => s + (d.calls||0), 0),
    MOCK_DATA.skills.reduce((s,d) => s + (d.calls||0), 0),
    MOCK_DATA.flows.reduce((s,d) => s + (d.runs||0), 0),
    MOCK_DATA.prompts.reduce((s,d) => s + (d.uses||0), 0),
    MOCK_DATA.crons.length * 100, // simulated
  ];
  const ctx2 = document.getElementById('chartRequests').getContext('2d');
  const totalReqs = reqData.reduce((a, b) => a + b, 0);
  charts.requests = new Chart(ctx2, {
    type: 'doughnut',
    data: {
      labels: reqLabels,
      datasets: [{
        data: reqData,
        backgroundColor: ['rgba(121,174,200,0.7)','rgba(245,221,93,0.7)','rgba(65,118,144,0.7)','rgba(68,126,155,0.7)','rgba(48,96,128,0.7)'],
        borderColor: ['#79aec8','#f5dd5d','#417690','#447e9b','#306080'],
        borderWidth: 1.5
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      cutout: '72%',
      plugins: {
        legend: { position: 'right', labels: { color: 'rgba(233,231,243,0.4)', font: { size: 10 }, boxWidth: 10, padding: 8, usePointStyle: true, pointStyle: 'circle' } }
      }
    },
    plugins: [{
      id: 'centerText',
      beforeDraw: function(chart) {
        const {width, height, ctx, chartArea: {top, left, right, bottom}} = chart;
        const cx = (left + right) / 2, cy = (top + bottom) / 2;
        ctx.save();
        ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
        ctx.font = '600 18px system-ui, sans-serif';
        ctx.fillStyle = 'rgba(233,231,243,0.8)';
        ctx.fillText(totalReqs.toLocaleString(), cx, cy - 6);
        ctx.font = '9px system-ui, sans-serif';
        ctx.fillStyle = 'rgba(233,231,243,0.25)';
        ctx.fillText('total', cx, cy + 14);
        ctx.restore();
      }
    }]
  });

  const ctx3 = document.getElementById('chartFrequency').getContext('2d');
  const hours = Array.from({length: 24}, (_, i) => String(i).padStart(2,'0') + 'h');
  charts.frequency = new Chart(ctx3, {
    type: 'bar',
    data: {
      labels: hours,
      datasets: [{
        label: 'Requests',
        data: [12,8,5,3,2,1,4,18,42,56,48,38,45,52,58,44,62,78,85,72,54,38,28,18],
        backgroundColor: 'rgba(121,174,200,0.4)',
        borderColor: '#79aec8',
        borderWidth: 1.5,
        borderRadius: { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 },
        borderSkipped: false
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: 'rgba(233,231,243,0.25)', font: { size: 8 }, maxTicksLimit: 12 }, grid: { display: false } },
        y: { ticks: { color: 'rgba(233,231,243,0.25)', font: { size: 9 } }, grid: { color: 'rgba(255,255,255,0.02)' }, beginAtZero: true }
      }
    }
  });

  // Flows Pipeline - horizontal bar (from MOCK_DATA)
  const flowLabels = MOCK_DATA.flows.map(d => d.name);
  const flowRuns = MOCK_DATA.flows.map(d => d.runs);
  const flowColors = ['rgba(65,118,144,0.6)', 'rgba(245,221,93,0.6)'];
  const flowBorders = ['#417690', '#f5dd5d'];
  const ctx4 = document.getElementById('chartFlows').getContext('2d');
  charts.flows = new Chart(ctx4, {
    type: 'bar',
    data: {
      labels: flowLabels,
      datasets: [
        { label: 'Runs', data: flowRuns, backgroundColor: flowColors,
          borderColor: flowBorders, borderWidth: 1.5, borderRadius: { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 }, borderSkipped: false }
      ]
    },
    options: {
      indexAxis: 'y', responsive: true, maintainAspectRatio: false,
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: 'rgba(233,231,243,0.3)', font: { size: 10 } }, grid: { color: 'rgba(255,255,255,0.02)' }, beginAtZero: true },
        y: { ticks: { color: 'rgba(233,231,243,0.5)', font: { size: 11 } }, grid: { display: false } }
      }
    }
  });

  // Network graph - relaciones agentes-skills-flows con aristas
  const canvas5 = document.getElementById('chartHealth');
  function drawNetworkGraph() {
    const rect = canvas5.parentElement.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    canvas5.width = rect.width * dpr;
    canvas5.height = rect.height * dpr;
    canvas5.style.width = rect.width + 'px';
    canvas5.style.height = rect.height + 'px';
    const ctx = canvas5.getContext('2d');
    ctx.scale(dpr, dpr);
    const W = rect.width, H = rect.height;
    if (W < 10) { requestAnimationFrame(drawNetworkGraph); return; }

    const agents = MOCK_DATA.agents.filter(a => a.status === 'active');
    const skills = MOCK_DATA.skills;
    const flows = MOCK_DATA.flows.filter(f => f.status === 'active');

    const nodes = [], nodeMap = {};
    function add(id, label, type, color, x, y) {
      const n = { id, label, type, color, x, y, vx: 0, vy: 0 };
      nodes.push(n); nodeMap[id] = n;
    }

    // Position by type in columns
    const colX = { flow: W * 0.15, agent: W * 0.5, skill: W * 0.85 };
    const spacing = H / (Math.max(agents.length, skills.length, flows.length) + 1);
    agents.forEach((a, i) => add(a.slug, a.name.split(' ')[0], 'agent', '#79aec8', colX.agent, spacing * (i + 1)));
    skills.forEach((s, i) => add(s.slug, s.name.split(' ')[0], 'skill', '#f5dd5d', colX.skill, spacing * (i + 1)));
    flows.forEach((f, i) => add(f.slug, f.name.split(' ')[0], 'flow', '#417690', colX.flow, spacing * (i + 1.5)));

    const edges = [
      ['soporte-bot', 'web-search'], ['code-reviewer', 'query-db'], ['sdd-generator', 'send-email'],
      ['sdd-pipeline-v1', 'code-reviewer'], ['sdd-pipeline-v1', 'sdd-generator'], ['issue-intake', 'soporte-bot'],
    ];

    // Force-directed: repel all, attract along edges
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

    ctx.clearRect(0, 0, W, H);

    // Orthogonal edges (manhattan-style) with glow
    for (const [f, t] of edges) {
      const a = nodeMap[f], b = nodeMap[t];
      if (!a || !b) continue;
      const mx = (a.x + b.x) / 2;
      ctx.beginPath(); ctx.moveTo(a.x, a.y);
      ctx.lineTo(mx, a.y); ctx.lineTo(mx, b.y); ctx.lineTo(b.x, b.y);
      ctx.strokeStyle = a.color + '20'; ctx.lineWidth = 2; ctx.stroke();
      ctx.beginPath(); ctx.moveTo(a.x, a.y);
      ctx.lineTo(mx, a.y); ctx.lineTo(mx, b.y); ctx.lineTo(b.x, b.y);
      ctx.strokeStyle = 'rgba(255,255,255,0.04)'; ctx.lineWidth = 1; ctx.setLineDash([3, 4]); ctx.stroke(); ctx.setLineDash([]);
    }

    // Hexagon node helper
    function hexagon(x, y, r) {
      ctx.beginPath();
      for (let i = 0; i < 6; i++) {
        const a = Math.PI / 3 * i - Math.PI / 6, px = x + r * Math.cos(a), py = y + r * Math.sin(a);
        i === 0 ? ctx.moveTo(px, py) : ctx.lineTo(px, py);
      }
      ctx.closePath();
    }

    // Node glow (hexagonal)
    for (const n of nodes) {
      const g = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, 16);
      g.addColorStop(0, n.color + '60'); g.addColorStop(1, n.color + '00');
      ctx.fillStyle = g; hexagon(n.x, n.y, 16); ctx.fill();
    }

    // Hexagon nodes
    for (const n of nodes) {
      hexagon(n.x, n.y, 6);
      ctx.fillStyle = n.color; ctx.fill();
      ctx.strokeStyle = n.color + '80'; ctx.lineWidth = 1.5; ctx.stroke();
    }

    // Labels
    ctx.fillStyle = 'rgba(233,231,243,0.45)'; ctx.font = '9px system-ui, sans-serif';
    ctx.textAlign = 'center'; ctx.textBaseline = 'top';
    for (const n of nodes) ctx.fillText(n.label, n.x, n.y + 9);
  }
  drawNetworkGraph();
  window.addEventListener('resize', drawNetworkGraph);
}
initCharts();

/* ============================================================
   VIEW
   ============================================================ */
const view = document.getElementById('view');
const viewBackdrop = document.getElementById('viewBackdrop');
const viewTitle = document.getElementById('viewTitle');
const viewSubtitle = document.getElementById('viewSubtitle');
const viewBody = document.getElementById('viewBody');
const viewHeaderIcon = document.getElementById('viewHeaderIcon');
const viewBack = document.getElementById('viewBack');
const viewClose = document.getElementById('viewClose');

let activeSegment = null;
let searchFilter = '';

function renderView(seg) {
  viewHeaderIcon.style.color = seg.color;
  viewHeaderIcon.querySelector("svg")?.setAttribute("style", "color: " + seg.color);
  viewTitle.textContent = seg.label;
  viewSubtitle.textContent = SEGMENT_SUBTITLES[seg.id] || '';

  const data = (MOCK_DATA[seg.id] || []).filter(d => {
    if (!searchFilter) return true;
    const q = searchFilter.toLowerCase();
    return Object.values(d).some(v => String(v).toLowerCase().includes(q));
  });

  const total = data.length;
  const active = data.filter(d => d.status === 'active').length;
  const totalUse = data.reduce((s, d) => s + (d.calls || d.uses || d.runs || 0), 0);

  let html = \`
    <div class="view-toolbar">
      <div class="view-search">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
        </svg>
        <input type="text" id="viewSearchInput" placeholder="Buscar en \${seg.label}..." value="\${searchFilter}" />
      </div>
      <button class="view-btn" id="refresh-btn">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
        Refrescar
      </button>
      <button class="view-btn primary" id="addNew">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 5v14M5 12h14"/></svg>
        Nuevo
      </button>
    </div>

    <div class="view-stats">
      <div class="view-stat glass">
        <div class="view-stat-label">Total</div>
        <div class="view-stat-value">\${total}</div>
      </div>
      <div class="view-stat glass">
        <div class="view-stat-label">Activos</div>
        <div class="view-stat-value" style="color: #4ade80">\${active}</div>  
      </div>
      <div class="view-stat glass">
        <div class="view-stat-label">Inactivos</div>
        <div class="view-stat-value" style="color: var(--dj-text-muted)">\${total - active}</div>
      </div>
      <div class="view-stat glass">
        <div class="view-stat-label">Uso 7d</div>
        <div class="view-stat-value">\${totalUse.toLocaleString()}</div>
      </div>
    </div>
  \`;

  if (data.length > 0) {
    html += '<table class="view-table"><thead><tr>';
    if (seg.id === 'agents') html += '<th data-tooltip="Nombre del agente">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Provider LLM (anthropic, openai, etc)">Provider</th><th data-tooltip="Modelo del provider (ej: claude-sonnet-4-5)">Modelo</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cantidad de invocaciones">Calls</th><th></th>';
    else if (seg.id === 'skills') html += '<th data-tooltip="Nombre de la skill">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Tipo (prompt/code/api/mcp_tool)">Tipo</th><th data-tooltip="Descripcion breve">Descripcion</th><th data-tooltip="Cantidad de invocaciones">Calls</th><th data-tooltip="Porcentaje de ejecuciones exitosas">Success</th><th></th>';
    else if (seg.id === 'flows') html += '<th data-tooltip="Nombre del flow">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Cantidad de fases del flow">Fases</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cantidad de ejecuciones">Runs</th><th></th>';
    else if (seg.id === 'prompts') html += '<th data-tooltip="Nombre del prompt">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Modelo LLM target">Modelo</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cantidad de veces usado">Usos</th><th></th>';
    else if (seg.id === 'projects') html += '<th data-tooltip="Nombre del proyecto">Nombre</th><th data-tooltip="Slug unico (lowercase, guion)">Slug</th><th data-tooltip="Estado activo/archivado">Estado</th><th data-tooltip="Cantidad de skills asociadas">Skills</th><th data-tooltip="Cantidad de agents">Agents</th><th data-tooltip="Cantidad de flows">Flows</th><th></th>';
    else if (seg.id === 'users') html += '<th data-tooltip="Email del usuario">Email</th><th data-tooltip="Rol (admin, operator, etc)">Rol</th><th data-tooltip="Estado activo/inactivo">Estado</th><th></th>';
    else if (seg.id === 'policies') html += '<th data-tooltip="Nombre de la policy">Nombre</th><th data-tooltip="Scope (platform/project/skill)">Scope</th><th data-tooltip="Tipo (security_rule, architecture, etc)">Kind</th><th data-tooltip="Estado activo/inactivo">Estado</th><th></th>';
    else if (seg.id === 'crons') html += '<th data-tooltip="Nombre del cron">Nombre</th><th data-tooltip="Expresion cron (5 fields)">Schedule</th><th data-tooltip="Estado activo/inactivo">Estado</th><th data-tooltip="Cuando se ejecuto por ultima vez">Ultimo run</th><th></th>';
    html += '</tr></thead><tbody>';

    data.forEach(row => {
      html += '<tr>';
      if (seg.id === 'agents') {
        html += \`<td><b>\${row.name}</b></td><td><code>\${row.slug}</code></td><td>\${row.provider}</td><td><code>\${row.model}</code></td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td><td>\${row.calls}</td>\`;
      } else if (seg.id === 'skills') {
        html += \`<td><b>\${row.name}</b></td><td><code>\${row.slug}</code></td><td><span class="badge \${row.type}">\${row.type.toUpperCase()}</span></td><td>\${row.desc}</td><td>\${row.calls}</td><td>\${row.success}%</td>\`;
      } else if (seg.id === 'flows') {
        html += \`<td><b>\${row.name}</b></td><td><code>\${row.slug}</code></td><td>\${row.phases}</td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td><td>\${row.runs}</td>\`;
      } else if (seg.id === 'prompts') {
        html += \`<td><b>\${row.name}</b></td><td><code>\${row.slug}</code></td><td><code>\${row.model}</code></td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td><td>\${row.uses}</td>\`;
      } else if (seg.id === 'projects') {
        html += \`<td><b>\${row.name}</b></td><td><code>\${row.slug}</code></td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td><td>\${row.skills}</td><td>\${row.agents}</td><td>\${row.flows}</td>\`;
      } else if (seg.id === 'users') {
        html += \`<td><b>\${row.name}</b></td><td><span class="badge mcp">\${row.role}</span></td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td>\`;
      } else if (seg.id === 'policies') {
        html += \`<td><b>\${row.name}</b></td><td><span class="badge prompt">\${row.scope}</span></td><td><code>\${row.kind}</code></td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td>\`;
      } else if (seg.id === 'crons') {
        html += \`<td><b>\${row.name}</b></td><td><code>\${row.schedule}</code></td><td><span class="badge \${row.status === 'active' ? 'active' : 'inactive'}">\${row.status}</span></td><td>\${row.last_run}</td>\`;
      }
      html += \`<td><div class="view-table-actions">
        <button class="view-table-btn" data-action="view" data-id="\${row.slug || row.email || row.name}" title="Ver">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
        </button>
        <button class="view-table-btn" data-action="edit" data-id="\${row.slug || row.email || row.name}" title="Editar">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </button>
        <button class="view-table-btn danger" data-action="delete" data-id="\${row.slug || row.email || row.name}" title="Eliminar">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6"/></svg>
        </button>
      </div></td></tr>\`;
    });
    html += '</tbody></table>';
  } else {
    html += \`<div class="view-empty">
      <div style="font-size: 48px; margin-bottom: 16px; opacity: 0.5; font-family: 'Font Awesome 6 Free'; font-weight: 900;">\${seg.icon}</div>
      <h3>Sin items</h3>
      <p>No hay items que coincidan con "\${searchFilter}"</p>
      <button class="view-btn primary" id="addNewEmpty">+ Crear el primero</button>
    </div>\`;
  }

  viewBody.innerHTML = html;

  // Wire up
  const searchInput = document.getElementById('viewSearchInput');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      searchFilter = e.target.value;
      renderView(seg);
      setTimeout(() => {
        const newInput = document.getElementById('viewSearchInput');
        if (newInput) {
          newInput.focus();
          newInput.setSelectionRange(newInput.value.length, newInput.value.length);
        }
      }, 0);
    });
  }

  document.getElementById('addNew')?.addEventListener('click', () => openModal(seg, 'create'));
  document.getElementById('addNewEmpty')?.addEventListener('click', () => openModal(seg, 'create'));
  document.getElementById('refresh-btn')?.addEventListener('click', () => showToast('Lista actualizada', 'success'));

  viewBody.querySelectorAll('[data-action]').forEach(btn => {
    btn.addEventListener('click', () => {
      const action = btn.dataset.action;
      const id = btn.dataset.id;
      if (action === 'view') showToast('Viendo ' + id, 'success');
      else if (action === 'edit') openModal(seg, 'edit', MOCK_DATA[seg.id].find(d => (d.slug || d.email || d.name) === id));
      else if (action === 'delete') {
        if (confirm('¿Eliminar ' + id + '?')) {
          MOCK_DATA[seg.id] = MOCK_DATA[seg.id].filter(d => (d.slug || d.email || d.name) !== id);
          renderView(seg);
          showToast('Item eliminado', 'success');
        }
      }
    });
  });
}

function openSegment(seg) {
  if (activeSegment) return;
  activeSegment = seg;
  searchFilter = '';
  renderView(seg);

  // Save which element had focus (so we can restore it on close)
  view._previouslyFocused = document.activeElement;

  // CRITICAL: change aria-hidden BEFORE making the view visible.
  // Otherwise Chrome detects "focused element's ancestor has aria-hidden=true"
  // and blocks the focus with the warning we saw.
  view.setAttribute('aria-hidden', 'false');

  // Show the view

  setTimeout(() => viewBackdrop.classList.add('active'), 100);
  setTimeout(() => {
    view.classList.add('active');
    // Focus the back button (always present, always safe to focus).
    // We focus inside the view AFTER it's no longer aria-hidden,
    // so Chrome doesn't complain about focusing a hidden element.
    if (viewBack && viewBack.focus) viewBack.focus();
  }, 350);
  setTimeout(() => {
    // Now move focus to the search input if it exists
    const search = document.getElementById('viewSearchInput');
    if (search && search.focus) search.focus();
  }, 700);
}

function closeView() {
  if (!activeSegment) return;
  // Move focus OUT of the view BEFORE we hide it
  if (view._previouslyFocused && view._previouslyFocused.focus) {
    try { view._previouslyFocused.focus(); } catch (e) {}
  }
  view.classList.remove('active');
  view.setAttribute('aria-hidden', 'true');
  viewBackdrop.classList.remove('active');
  // no ring to animate anymore
  setTimeout(() => {
    activeSegment = null;
    searchFilter = '';
  }, 600);
}

viewBack.addEventListener('click', closeView);
viewClose.addEventListener('click', closeView);
viewBackdrop.addEventListener('click', closeView);

/* ============================================================
   MODAL
   ============================================================ */
const modal = document.getElementById('modal');
const modalBackdrop = document.getElementById('modalBackdrop');
const modalClose = document.getElementById('modalClose');
const modalCancel = document.getElementById('modalCancel');
const modalForm = document.getElementById('modalForm');
const modalTitle = document.getElementById('modalTitle');

let modalMode = 'create';
let editingItem = null;

function openModal(seg, mode, item) {
  modalMode = mode;
  editingItem = item;
  modalTitle.textContent = (mode === 'edit' ? 'Editar' : 'Nuevo') + ' ' + seg.label.slice(0, -1);

  // Save the element that triggered the modal (for focus restoration)
  modal._previouslyFocused = document.activeElement;

  // CRITICAL: change aria-hidden BEFORE adding the .active class.
  // Otherwise the focused element (the "+ Nuevo" button) has an
  // ancestor with aria-hidden=true and Chrome blocks the focus.
  modal.setAttribute('aria-hidden', 'false');

  // Show the modal
  modal.classList.add('active');
  modalBackdrop.classList.add('active');

  // Pre-fill values for edit mode
  if (mode === 'edit' && item) {
    modalForm.name.value = item.name || '';
    modalForm.slug.value = item.slug || '';
    modalForm.description.value = item.desc || item.description || '';
    modalForm.status.value = item.status || 'active';
  } else {
    modalForm.reset();
  }

  // Focus the first field AFTER the modal is no longer hidden
  setTimeout(() => {
    if (modalForm.name && modalForm.name.focus) modalForm.name.focus();
  }, 150);
}

function closeModal() {
  // Move focus OUT of the modal BEFORE hiding it
  if (modal._previouslyFocused && modal._previouslyFocused.focus) {
    try { modal._previouslyFocused.focus(); } catch (e) {}
  }

  modal.classList.remove('active');
  modalBackdrop.classList.remove('active');
  modal.setAttribute('aria-hidden', 'true');
  modalForm.reset();
  document.querySelectorAll('.form-row.has-error').forEach(r => r.classList.remove('has-error'));
}

modalClose.addEventListener('click', closeModal);
modalCancel.addEventListener('click', closeModal);
modalBackdrop.addEventListener('click', closeModal);

/* ============================================================
   PROFILE
   ============================================================ */
const profileCard = document.getElementById('profileCard');

function openProfile() {
  profileCard.classList.toggle('open');
}

document.addEventListener('click', (e) => {
  if (profileCard.classList.contains('open') &&
      !profileCard.contains(e.target) &&
      e.target.id !== 'userBtn' &&
      !e.target.closest('#userBtn')) {
    profileCard.classList.remove('open');
  }
});

document.querySelector('.profile-logout')?.addEventListener('click', () => {
  profileCard.classList.remove('open');
  showToast('Sesion cerrada', 'success');
});

modalForm.addEventListener('submit', (e) => {
  e.preventDefault();
  let valid = true;
  const name = modalForm.name.value.trim();
  const slug = modalForm.slug.value.trim();
  const desc = modalForm.description.value.trim();
  const status = modalForm.status.value;

  if (!name) { modalForm.name.closest('.form-row').classList.add('has-error'); valid = false; }
  else { modalForm.name.closest('.form-row').classList.remove('has-error'); }
  if (!slug || !/^[a-z0-9-]+$/.test(slug)) { modalForm.slug.closest('.form-row').classList.add('has-error'); valid = false; }
  else { modalForm.slug.closest('.form-row').classList.remove('has-error'); }

  if (!valid) { showToast('Corrige los errores', 'error'); return; }

  const seg = activeSegment;
  const newItem = { name, slug, desc, status, type: 'code', calls: 0, success: 100 };

  if (modalMode === 'edit' && editingItem) {
    const idx = MOCK_DATA[seg.id].findIndex(d => d === editingItem);
    if (idx >= 0) MOCK_DATA[seg.id][idx] = { ...editingItem, ...newItem };
    showToast(seg.label.slice(0, -1) + ' actualizado', 'success');
  } else {
    MOCK_DATA[seg.id].unshift(newItem);
    showToast(seg.label.slice(0, -1) + ' creado', 'success');
  }

  closeModal();
  renderView(seg);
});

modalForm.querySelectorAll('input, textarea').forEach(input => {
  input.addEventListener('input', () => input.closest('.form-row')?.classList.remove('has-error'));
});

/* ============================================================
   TOAST
   ============================================================ */
const toast = document.getElementById('toast');
const toastIcon = document.getElementById('toastIcon');
const toastMessage = document.getElementById('toastMessage');
let toastTimeout = null;

function showToast(message, type = 'success') {
  clearTimeout(toastTimeout);
  toast.className = 'toast ' + type;
  toastIcon.textContent = type === 'success' ? '✓' : type === 'error' ? '✕' : 'ℹ';
  toastMessage.textContent = message;
  setTimeout(() => toast.classList.add('active'), 10);
  toastTimeout = setTimeout(() => toast.classList.remove('active'), 2500);
}

/* ============================================================
   KEYBOARD
   ============================================================ */
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    if (modal.classList.contains('active')) closeModal();
    else if (activeSegment) closeView();
  }
  if ((e.key === 'r' || e.key === 'R') && !activeSegment && !modal.classList.contains('active')) {
    showToast('Dashboard refrescado', 'success');
  }
});


</script>

</body>
</html>`);
  await page.waitForTimeout(2000);
  return "loaded";
}
