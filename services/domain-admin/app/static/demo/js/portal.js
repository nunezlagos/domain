function initPortal() {
  buildHeaderNav();
  try { initCharts(); } catch (e) { console.warn('chart init error:', e); }
  observeMosaicResize();
  bindEvents();
}

function buildHeaderNav() {
  const headerNav = document.getElementById('headerNav');
  SEGMENTS.forEach(seg => {
    const btn = document.createElement('button');
    btn.className = 'header-nav-item';
    btn.innerHTML = `<i class="fas fa-${seg.icon}"></i>${seg.label}`;
    btn.addEventListener('click', () => openSegment(seg));
    headerNav.appendChild(btn);
  });
}

function observeMosaicResize() {
  const observer = new ResizeObserver(() => resizeAllCharts());
  observer.observe(mosaic);
}

function bindEvents() {
  bindFilterPills();
  bindUserButton();
  bindProfileOutsideClick();
  bindProfileLogout();
  bindMosaicClicks();
  bindEscapeKey();
  bindMosaicSpaceClick();
  bindViewCloseClicks();
  bindModalCloseClicks();
  bindKeydown();
  bindFormSubmit();
  bindFormInputClear();
  bindMousemove();
}

function bindFilterPills() {
  document.querySelectorAll('.dash-filter').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.dash-filter').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
    });
  });
}

function bindUserButton() {
  document.getElementById('userBtn').addEventListener('click', openProfile);
}

function bindProfileOutsideClick() {
  document.addEventListener('click', (e) => {
    if (profileCard.classList.contains('open') &&
        !profileCard.contains(e.target) &&
        e.target.id !== 'userBtn' &&
        !e.target.closest('#userBtn')) {
      profileCard.classList.remove('open');
    }
  });
}

function bindProfileLogout() {
  document.querySelector('.profile-logout')?.addEventListener('click', () => {
    profileCard.classList.remove('open');
    showToast('Sesion cerrada', 'success');
  });
}

function bindMosaicClicks() {
  // throttle leading-edge: procesa 1 click y descarta los siguientes 2s.
  // Por timestamp (no setTimeout) → no se puede desincronizar con spam.
  const COOLDOWN_MS = 2000;
  let lastClickTs = -COOLDOWN_MS;
  document.addEventListener('click', (e) => {
    const item = e.target.closest('.mosaic-item');
    if (!item) return;
    const now = performance.now();
    if (now - lastClickTs < COOLDOWN_MS) return;
    lastClickTs = now;
    lockCards(COOLDOWN_MS);
    if (focusActive) {
      if (!item.classList.contains('hero')) setHero(item);
      return;
    }
    enterFocus(item);
  });
}

function bindEscapeKey() {
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && focusActive && !activeSegment) exitFocus();
  });
}

function bindMosaicSpaceClick() {
  mosaic.addEventListener('click', (e) => {
    if (focusActive && e.target === mosaic) exitFocus();
  });
}

function bindViewCloseClicks() {
  viewBack.addEventListener('click', closeView);
  viewClose.addEventListener('click', closeView);
  viewBackdrop.addEventListener('click', closeView);
}

function bindModalCloseClicks() {
  modalClose.addEventListener('click', closeModal);
  modalCancel.addEventListener('click', closeModal);
  modalBackdrop.addEventListener('click', closeModal);
}

function bindFormSubmit() {
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
}

function bindFormInputClear() {
  modalForm.querySelectorAll('input, textarea').forEach(input => {
    input.addEventListener('input', () => input.closest('.form-row')?.classList.remove('has-error'));
  });
}

function bindKeydown() {
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (modal.classList.contains('active')) closeModal();
      else if (activeSegment) closeView();
    }
    if ((e.key === 'r' || e.key === 'R') && !activeSegment && !modal.classList.contains('active') && !focusActive) {
      showToast('Dashboard refrescado', 'success');
    }
  });
}

function bindMousemove() {
  let mouseX = window.innerWidth / 2;
  let mouseY = window.innerHeight / 2;
  let rafMouse = null;

  function updateMouseIllumination() {
    rafMouse = null;
  }

  document.addEventListener('mousemove', (e) => {
    mouseX = e.clientX;
    mouseY = e.clientY;
    if (!rafMouse) rafMouse = requestAnimationFrame(updateMouseIllumination);
  }, { passive: true });
}

document.addEventListener('DOMContentLoaded', initPortal);
