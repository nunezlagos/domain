# HU-45.1: Django default page placeholder.
# - Multi-stage: deps cache layer separado del código para mejor build cache.
# - python:3.12-slim corre como root por default (OK para placeholder; HU futura
#   puede pasar a nonroot con setcap o puerto alto + nginx).
# - gunicorn en :80 (dentro del container, no publicado al host).
FROM python:3.12-slim AS base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# Deps primero (cache de layer).
COPY requirements.txt /app/requirements.txt
RUN pip install -r /app/requirements.txt

# Código.
COPY app/ /app/

EXPOSE 80

# 2 workers + 4 threads es generoso para un placeholder. Tunear en HU futura.
CMD ["gunicorn", "--bind", "0.0.0.0:80", "--workers", "2", "--threads", "4", "--access-logfile", "-", "config.wsgi:application"]