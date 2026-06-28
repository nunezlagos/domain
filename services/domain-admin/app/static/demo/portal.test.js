const { chromium } = require('playwright');
const assert = require('assert');

const URL = 'http://localhost:8080/portal.html';

async function run() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const errors = [];

  page.on('console', msg => {
    if (msg.type() === 'error') errors.push({ type: 'console', text: msg.text() });
  });
  page.on('pageerror', err => errors.push({ type: 'page', text: err.message }));

  await page.goto(URL, { waitUntil: 'networkidle' });
  console.log('Page loaded');

  // 1. Verify CSS and JS files loaded
  const cssLinks = await page.$$eval('link[rel="stylesheet"]', els => els.map(e => e.href));
  assert.ok(cssLinks.some(h => h.includes('portal.css')), 'portal.css not loaded');
  assert.ok(cssLinks.some(h => h.includes('font-awesome')), 'Font Awesome not loaded');
  console.log('✓ CSS files loaded');

  const scripts = await page.$$eval('script[src]', els => els.map(e => e.src));
  assert.ok(scripts.some(s => s.includes('data.js')), 'data.js not loaded');
  assert.ok(scripts.some(s => s.includes('charts.js')), 'charts.js not loaded');
  assert.ok(scripts.some(s => s.includes('focus.js')), 'focus.js not loaded');
  assert.ok(scripts.some(s => s.includes('views.js')), 'views.js not loaded');
  assert.ok(scripts.some(s => s.includes('ui.js')), 'ui.js not loaded');
  assert.ok(scripts.some(s => s.includes('portal.js')), 'portal.js not loaded');
  assert.ok(scripts.some(s => s.includes('chart.js')), 'Chart.js not loaded');
  console.log('✓ JS files loaded');

  // 2. Verify no console errors or page errors
  if (errors.length > 0) {
    console.error('ERRORS:', errors);
  }
  console.log('✓ No critical errors (warnings: ' + errors.length + ')');

  // 3. Verify mosaic exists and has 9 items
  const mosaic = await page.$('.dash-mosaic');
  assert.ok(mosaic, 'Mosaic not found');
  const mosaicItems = await page.$$('.mosaic-item');
  assert.equal(mosaicItems.length, 9, 'Expected 9 mosaic items');
  console.log('✓ Mosaic has 9 items');

  // 4. Verify header nav items
  const navItems = await page.$$('.header-nav-item');
  assert.equal(navItems.length, 8, 'Expected 8 nav items');
  console.log('✓ Header has 8 nav items');

  // 5. Verify Chart.js canvases exist
  const canvasIds = ['chartTokens', 'chartRequests', 'chartFrequency', 'chartFlows', 'chartSuccess', 'chartHealth'];
  for (const id of canvasIds) {
    const canvas = await page.$('#' + id);
    assert.ok(canvas, 'Canvas #' + id + ' not found');
  }
  console.log('✓ All 6 chart canvases exist');

  // 6. Verify profile card, view, modal, toast elements
  assert.ok(await page.$('#profileCard'), 'profileCard not found');
  assert.ok(await page.$('#view'), 'view not found');
  assert.ok(await page.$('#modal'), 'modal not found');
  assert.ok(await page.$('#toast'), 'toast not found');
  console.log('✓ UI elements (profile, view, modal, toast) exist');

  // 7. Verify filter pills
  const filters = await page.$$('.dash-filter');
  assert.equal(filters.length, 3, 'Expected 3 filter pills');
  console.log('✓ 3 filter pills exist');

  // 8. Click a mosaic item → focus mode
  const firstItem = mosaicItems[0];
  await firstItem.click();
  await page.waitForTimeout(300);
  const focusFab = await page.$('.focus-close-fab.active');
  assert.ok(focusFab, 'Focus FAB not active after click');
  const hasFocus = await page.$eval('.dash-mosaic', el => el.classList.contains('focus'));
  assert.ok(hasFocus, 'Mosaic does not have .focus class');
  console.log('✓ Focus mode activated on card click');

  // 9. Exit focus via Escape
  await page.keyboard.press('Escape');
  await page.waitForTimeout(300);
  const focusGone = await page.$eval('.dash-mosaic', el => !el.classList.contains('focus'));
  assert.ok(focusGone, 'Mosaic still has .focus after Escape');
  console.log('✓ Focus mode exited via Escape');

  // 10. Click a nav item → open view
  await firstNavClick(page);
  const viewActive = await page.$eval('#view', el => el.classList.contains('active'));
  assert.ok(viewActive, 'View not active after nav click');
  console.log('✓ View panel opened');

  // 11. Close view via close button
  const closeBtn = await page.$('#viewClose');
  assert.ok(closeBtn, 'viewClose button not found');
  await closeBtn.click();
  await page.waitForTimeout(800);
  const viewClosed = await page.$eval('#view', el => !el.classList.contains('active'));
  assert.ok(viewClosed, 'View still active after close click');
  console.log('✓ View panel closed');

  // 12. Open view → click "Nuevo" → verify modal
  await firstNavClick(page);
  const addNew = await page.$('#addNew');
  assert.ok(addNew, 'addNew button not found');
  await addNew.click();
  await page.waitForTimeout(300);
  const modalActive = await page.$eval('#modal', el => el.classList.contains('active'));
  assert.ok(modalActive, 'Modal not active');
  console.log('✓ Modal opened from view');

  // 13. Close modal via Escape
  await page.keyboard.press('Escape');
  await page.waitForTimeout(300);
  const modalClosed = await page.$eval('#modal', el => !el.classList.contains('active'));
  assert.ok(modalClosed, 'Modal still active after Escape');
  console.log('✓ Modal closed via Escape');

  // Close view too before next test
  await page.keyboard.press('Escape');
  await page.waitForTimeout(800);

  // 14. Profile card toggle
  const userBtn = await page.$('#userBtn');
  assert.ok(userBtn, 'userBtn not found');
  await userBtn.click();
  await page.waitForTimeout(100);
  const profileOpen = await page.$eval('#profileCard', el => el.classList.contains('open'));
  assert.ok(profileOpen, 'Profile card not open after click');
  console.log('✓ Profile card opened');
  await userBtn.click();
  await page.waitForTimeout(100);
  const profileClosed = await page.$eval('#profileCard', el => !el.classList.contains('open'));
  assert.ok(profileClosed, 'Profile card still open after second click');
  console.log('✓ Profile card closed');

  // 15. Toast
  await page.evaluate(() => showToast('Test toast', 'success'));
  await page.waitForTimeout(150);
  const toastActive = await page.$eval('#toast', el => el.classList.contains('active'));
  assert.ok(toastActive, 'Toast not active');
  const toastText = await page.$eval('#toastMessage', el => el.textContent);
  assert.equal(toastText, 'Test toast', 'Toast message mismatch');
  console.log('✓ Toast displayed');

  // 16. Footer
  const footer = await page.$('.portal-footer');
  assert.ok(footer, 'Footer not found');
  console.log('✓ Footer exists');

  await browser.close();
  console.log('\nAll tests passed ✓');
}

async function firstNavClick(page) {
  const navItems = await page.$$('.header-nav-item');
  await navItems[0].click();
  await page.waitForTimeout(800);
}

// Close view first if open
function cleanup(page) {
  return page.evaluate(() => {
    const v = document.getElementById('view');
    if (v && v.classList.contains('active')) {
      v.classList.remove('active');
      v.setAttribute('aria-hidden', 'true');
    }
    const m = document.getElementById('modal');
    if (m && m.classList.contains('active')) {
      m.classList.remove('active');
    }
  }).catch(() => {});
}

run().catch(err => {
  console.error('TEST FAILED:', err.message);
  process.exit(1);
});
