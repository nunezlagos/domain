# Marca — Domain Admin

Sistema de identidad del portal. Todos los assets derivan de `logo.svg` (red interconectada,
trazo plomo `#2b2b2b`, líneas finas uniformes).

## Fuentes (vectoriales)

| Archivo | Uso |
|---|---|
| `logo.svg` | Logo oficial (plomo, fondo transparente). Vale como logo para **PDF/print** (es vectorial). |
| `logo-white.svg` | Variante para fondos oscuros. |
| `logo-animated.svg` | Logo con animación: pulsos de datos pasando entre nodos. Header / loaders. |

## Favicons / iconos (raster)

| Archivo | Tamaño | Uso |
|---|---|---|
| `favicon.ico` | 16/32/48 | Favicon clásico (todos los navegadores). |
| `favicon-16/32/48/96.png` | 16–96 | Favicons PNG modernos. |
| `apple-touch-icon.png` | 180 | iOS / iPadOS (pantalla de inicio). |
| `icon-192.png` / `icon-512.png` | 192 / 512 | PWA / Android (`purpose: any`). |
| `maskable-512.png` | 512 | PWA maskable (con padding seguro). |
| `logo-1024.png` | 1024 | Logo alta resolución (print/PDF rasterizado). |
| `og-image.png` | 1200×630 | Open Graph / Twitter (compartir en redes). |

## Metadata

`site.webmanifest` — manifiesto PWA (nombre, iconos, theme-color).
Los `<link>`/`<meta>` que consumen estos archivos están aplicados en `../portal.html`.

## Regenerar

Los raster se generan desde los SVG con `rsvg-convert` + ImageMagick (`magick`).
El og-image usa la fuente Noto Sans. Si cambia `logo.svg`, regenerar todos los PNG/ico/og.
