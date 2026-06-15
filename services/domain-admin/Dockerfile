# REQ-74: dockerizar admin. Multi-stage:
#   1) Node build → genera dist/template
#   2) Nginx serve estático + entrypoint que genera /assets/env.js
#      desde env vars al boot (un .env global, N VPS).

FROM node:22-alpine AS builder
WORKDIR /src

# El context de build incluye el dir template/ con package.json + src/.
COPY template/package.json template/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund

COPY template/. ./
# CoreUI template tiene "prebuild": "ng test --watch=false" — skip en CI.
RUN npm pkg delete scripts.prebuild
RUN npx ng build --configuration production


FROM nginx:1.27-alpine AS runtime
RUN rm -f /etc/nginx/conf.d/default.conf

# nginx config: SPA fallback + /api proxy hacia backend (opcional;
# en VPS suele estar Caddy delante haciendo el routing).
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Output del build Angular. CoreUI 5.x lo deja en dist/<project>/browser.
# El project name del package.json es coreui-free-angular-admin-template.
COPY --from=builder /src/dist/coreui-free-angular-admin-template/browser /usr/share/nginx/html

# REQ-73/74 runtime env injection. Genera /usr/share/nginx/html/assets/env.js
# en cada arranque desde env vars del container.
COPY docker-entrypoint.sh /docker-entrypoint.d/40-env-inject.sh
RUN chmod +x /docker-entrypoint.d/40-env-inject.sh

EXPOSE 80
