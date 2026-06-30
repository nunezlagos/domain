# HU-47.2: Django + gunicorn + WhiteNoise para static files.
#
# WhiteNoise sirve archivos estáticos directo desde gunicorn (sin nginx).
# collectstatic al startup copia templates/static → staticfiles/.
#
# Multi-stage: deps cache layer separado del código.
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

# collectstatic + gunicorn. WhiteNoise sirve /static/ desde staticfiles/.
CMD ["sh", "-c", "python manage.py collectstatic --noinput && gunicorn --bind 0.0.0.0:80 --workers 2 --threads 4 --access-logfile - config.wsgi:application"]