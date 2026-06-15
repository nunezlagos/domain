// Runtime config (REQ-73): el container nginx ejecuta /docker-entrypoint.d
// que genera /assets/env.js leyendo env vars del .env global. Así un
// mismo binary corre en N VPS sin re-build.
//
// Fallback para `ng serve` local: lee /assets/env.json si existe;
// si no, defaults para localhost.

declare global {
  interface Window {
    __DOMAIN_ENV__?: {
      API_URL?: string;
    };
  }
}

export function apiBase(): string {
  if (typeof window !== 'undefined' && window.__DOMAIN_ENV__?.API_URL) {
    return window.__DOMAIN_ENV__.API_URL.replace(/\/+$/, '');
  }
  // ng serve local: el dev proxy puede apuntar a http://localhost:8000.
  return '';
}
