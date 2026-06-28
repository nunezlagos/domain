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

let cardsLockTimeout = null;
// feedback visual del cooldown anti-spam: marca las cards como no clickeables
// (cursor not-allowed + sin hover) durante el lock; se limpia al terminar.
function lockCards(ms = 2000) {
  const m = document.querySelector('.dash-mosaic');
  if (!m) return;
  clearTimeout(cardsLockTimeout);
  m.classList.add('cards-locked');
  cardsLockTimeout = setTimeout(() => m.classList.remove('cards-locked'), ms);
}
