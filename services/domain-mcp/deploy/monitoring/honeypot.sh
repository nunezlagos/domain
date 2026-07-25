#!/bin/sh
# DOMAINSERV-130: honeypot de puertos señuelo.
#
# Escucha en puertos que NINGUN cliente legitimo tocaria y emite una linea por
# conexion con la IP de origen. Alloy la recoge de stdout -> Loki -> CrowdSec la
# parsea (crowdsec-parser-honeypot.yaml) y banea la IP 24h.
#
# No hay falsos positivos posibles: no existe razon legitima para conectarse a
# ninguno de estos puertos en este host.
#
# Nunca responde nada util ni ejecuta lo que recibe: el peer va a /dev/null. Es
# honeypot de BAJA interaccion a proposito — la variante que ejecuta payloads es
# DOMAINSERV-131 y NO va en este host, que corre bases de clientes.
#
# Por que `socat -d -d` y no `SYSTEM:` con printf: se probo el segundo y NO
# funciona. socat conecta stdin/stdout del comando hijo al socket, asi que un
# `>&2` dentro del SYSTEM no llega al stderr del proceso principal y no se emite
# nada. `-d -d` en cambio logea al stderr del socat, que es lo que el container
# publica. Verificado con una conexion real antes de dejarlo asi.
#
# La linea que produce es:
#   <fecha> socat[pid] N accepting connection from AF=2 1.2.3.4:54321 on AF=2 ...

set -eu

# Los "famosos": los sondean bots masivos. Mucho volumen, señal mas ruidosa.
#   23    telnet          445   SMB            1433  MSSQL
#   3389  RDP             6379  Redis          9200  Elasticsearch
#   11211 memcached       27017 MongoDB
#   2375  API de Docker SIN TLS — quien la busca quiere el daemon entero
PUERTOS_CONOCIDOS="23 445 1433 3389 6379 9200 11211 27017 2375"

# Los inesperados: casi nadie los sondea salvo un barrido exhaustivo, asi que dan
# señal de MAYOR CALIDAD — quien los toca no es un bot oportunista de paso.
#   4444  handler por default de Metasploit
#   31337 "eleet", clasico de backdoors
#   5555  ADB de Android
#   1099  Java RMI
#   50050 servidor de Cobalt Strike
#
# Nota honesta: la eleccion de puerto NO esconde nada de un barrido completo
# (nmap -p- toca los 65535). Lo que cambia es la CALIDAD de la señal.
PUERTOS_RAROS="4444 31337 5555 1099 50050"

# TCP4-LISTEN, no TCP-LISTEN: con el segundo la IP sale en notacion IPv6-mapped
#   AF=10 [0000:0000:0000:0000:0000:ffff:7f00:0001]
# y el parser tendria que desarmar hex. Con TCP4 sale plana:
#   AF=2 127.0.0.1
# Ambas variantes se probaron con conexiones reales antes de elegir.
#
# Sin `| grep` a proposito: filtrar el chatter de socat por pipe parecia prolijo
# pero lo ROMPE — el pipe buffea por bloques y las lineas no salen hasta llenarlo.
# Se probo y no emitia nada. El ruido (listening/forked/EOF) viaja a Loki y lo
# descarta el grok del parser, que solo matchea "accepting connection".
for p in $PUERTOS_CONOCIDOS $PUERTOS_RAROS; do
  socat -d -d "TCP4-LISTEN:${p},fork,reuseaddr" /dev/null 2>&1 &
done

echo "honeypot escuchando en: ${PUERTOS_CONOCIDOS} ${PUERTOS_RAROS}"

# si un socat muere, que el container no quede a medias en silencio
wait
