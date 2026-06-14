FROM nginx:1.27-alpine
LABEL org.opencontainers.image.source="https://github.com/nunezlagos/domain"
LABEL org.opencontainers.image.description="Domain frontend — Angular dashboard SPA (or placeholder)"

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY web/ /usr/share/nginx/html/

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["wget", "-q", "-O", "/dev/null", "http://localhost/"]
