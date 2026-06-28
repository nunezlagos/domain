let focusActive = false;
const mosaic = document.querySelector('.dash-mosaic');
const focusCloseFab = document.getElementById('focusCloseFab');

function viewportCenterRect() {
  const cx = window.innerWidth / 2;
  const cy = window.innerHeight / 2;
  return { left: cx - 140, top: cy - 90, width: 280, height: 180, right: cx + 140, bottom: cy + 90 };
}

function getFocusTitle(card) {
  const t = card.querySelector('.mi-title');
  if (t) return t.textContent.trim();
  const pw = card.querySelector('.pw-value');
  if (pw) return 'KPIs';
  return 'Detalle';
}

function captureRects(items) {
  const map = new Map();
  items.forEach(item => map.set(item, item.getBoundingClientRect()));
  return map;
}

function assignFocusClasses(card, items, heroIdx, total) {
  items.forEach(c => FOCUS_CLASSES.forEach(cls => c.classList.remove(cls)));
  card.classList.add('hero');
  for (let i = 1; i <= 3; i++) {
    items[(heroIdx - i + total) % total].classList.add(`pos-left-${i}`);
    items[(heroIdx + i) % total].classList.add(`pos-right-${i}`);
  }
  items[(heroIdx - 4 + total) % total].classList.add('pos-bottom-1');
  items[(heroIdx + 4) % total].classList.add('pos-bottom-2');
}

function clearFocusClasses(items) {
  items.forEach(c => FOCUS_CLASSES.forEach(cls => c.classList.remove(cls)));
}

function distanceFromHero(i, heroIdx, total) {
  const rawDist = Math.abs(i - heroIdx);
  return Math.min(rawDist, total - rawDist);
}

function computeCenterDelta(rect, spawnRect) {
  const dx = spawnRect.left - rect.left + (spawnRect.width - rect.width) / 2;
  const dy = spawnRect.top - rect.top + (spawnRect.height - rect.height) / 2;
  return { dx, dy };
}

function setHero(card) {
  if (activeSegment) return;
  const items = [...mosaic.querySelectorAll('.mosaic-item')];
  const heroIdx = items.indexOf(card);
  const total = items.length;
  const maxDist = Math.floor(total / 2);

  assignFocusClasses(card, items, heroIdx, total);
  void mosaic.offsetHeight;

  // una sola animación FLIP: las cards nacen del centro y van a su lugar
  const spawnRect = viewportCenterRect();
  animateSpawnFromCenter(items, heroIdx, maxDist, spawnRect);

  const totalAnim = (maxDist * 70) + 750;
  setTimeout(resizeAllCharts, totalAnim);
  showToast(getFocusTitle(card), 'success');
}

function animateSpawnFromCenter(items, heroIdx, maxDist, spawnRect) {
  items.forEach((item, i) => {
    // cancelar una entrada previa: evita que clicks rápidos acumulen
    // animaciones en delay (fill:both) que dejan la card en opacity 0.
    if (item._spawnAnim) item._spawnAnim.cancel();
    item.style.transform = '';
    item.style.opacity = '';

    const last = item.getBoundingClientRect();
    const isHero = item.classList.contains('hero');
    const targetOpacity = isHero ? 1 : 0.55;
    const dx = spawnRect.left - last.left + (spawnRect.width - last.width) / 2;
    const dy = spawnRect.top - last.top + (spawnRect.height - last.height) / 2;
    const sx = spawnRect.width / last.width;
    const sy = spawnRect.height / last.height;
    const dist = distanceFromHero(i, heroIdx, items.length);
    const delay = (maxDist - dist) * 70;

    item.style.transformOrigin = 'center center';
    const anim = item.animate([
      { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})`, opacity: 0, offset: 0 },
      { transform: 'translate(0, 0) scale(1)', opacity: targetOpacity, offset: 1 }
    ], {
      duration: 750, delay,
      easing: 'cubic-bezier(0.22, 1, 0.36, 1)',
      fill: 'both'
    });
    item._spawnAnim = anim;
    anim.onfinish = () => {
      // limpiar transform Y opacity inline → el CSS (.hero / :not(.hero)) manda
      item.style.transform = '';
      item.style.opacity = '';
      anim.cancel();
      if (item._spawnAnim === anim) item._spawnAnim = null;
    };
  });
}

function enterFocus(card) {
  if (focusActive || activeSegment) return;
  focusActive = true;

  mosaic.classList.add('focus');
  focusCloseFab.classList.add('active');

  // setHero hace el reorder + la animación de entrada (spawn desde el centro)
  setHero(card);
}

function animateExitPhase1(item, cdx, cdy, delay) {
  item.style.transformOrigin = 'center center';
  return item.animate([
    { transform: 'translate(0, 0) scale(1)', opacity: 1, offset: 0, easing: 'cubic-bezier(0.55, 0, 1, 1)' },
    { transform: `translate(${cdx}px, ${cdy}px) scale(0.4)`, opacity: 0, offset: 1 }
  ], { duration: 300, delay, fill: 'forwards' });
}

function animateExitPhase2(item, cdx, cdy, delay) {
  setTimeout(() => {
    item.animate([
      { transform: `translate(${cdx}px, ${cdy}px) scale(0.4)`, opacity: 0, offset: 0, easing: 'cubic-bezier(0, 0, 0.2, 1)' },
      { transform: 'translate(0, 0) scale(1)', opacity: 1, offset: 1 }
    ], { duration: 700, easing: 'cubic-bezier(0.22, 1, 0.36, 1)', fill: 'forwards',
      complete: () => { item.style.transform = ''; } });
  }, delay + 300);
}

function exitFocus() {
  if (!focusActive) return;
  focusActive = false;
  focusCloseFab.classList.remove('active');

  const items = [...mosaic.querySelectorAll('.mosaic-item')];
  const firstRects = captureRects(items);

  const heroCard = items.find(item => item.classList.contains('hero'));
  const heroIdx = heroCard ? items.indexOf(heroCard) : 0;
  const total = items.length;
  const maxDist = Math.floor(total / 2);

  mosaic.classList.remove('focus');
  clearFocusClasses(items);
  void mosaic.offsetHeight;

  const spawnRect = viewportCenterRect();
  items.forEach((item, i) => {
    const first = firstRects.get(item);
    const dist = distanceFromHero(i, heroIdx, total);
    const delay = (maxDist - dist) * 65;
    const { dx, dy } = computeCenterDelta(first, spawnRect);

    animateExitPhase1(item, dx, dy, delay);
    animateExitPhase2(item, dx, dy, delay);
  });

  const totalAnim = (maxDist * 65) + 300 + 700;
  setTimeout(resizeAllCharts, totalAnim);
}
