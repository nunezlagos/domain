# issue-29.1-install-cwd-guard

**Origen:** `REQ-29-install-quick-fixes`
**Prioridad tentativa:** alta
**Tipo:** fix

## Historia de usuario

**Como** developer corriendo `domain install` desde un directorio equivocado
**Quiero** que el binario detecte que NO está en el root del repo y aborte limpio con un mensaje accionable
**Para** no terminar escribiendo en el `.env` de un proyecto ajeno ni creando `.bak` espurios en el cwd del usuario

## Criterios de aceptación

### Escenario 1: Install aborta limpio fuera del repo

```gherkin
Dado que el cwd NO contiene ni `.env.example` ni `docker-compose.yml`
Cuando corro `domain install` (sin flags)
Entonces el proceso termina con exit code != 0
Y se imprime por stderr un mensaje que dice: "no estás en el root del repo domain"
Y el mensaje sugiere: "corré `bash install.sh` o pasá `--src /path/al/repo`"
Y NO se creó ningún archivo `.env`, `.bak.*`, ni se modificaron archivos del cwd
Y NO se intentó levantar docker
```

### Escenario 2: Install continúa cuando está en el repo

```gherkin
Dado que el cwd contiene `.env.example` y `docker-compose.yml`
Cuando corro `domain install --non-interactive`
Entonces el guard pasa transparente y el install continúa su flujo normal
Y el guard no agrega pasos visibles al progress (sin step "validating cwd")
```

### Escenario 3: Override con `--src` apunta a otro path

```gherkin
Dado que paso `--src /path/al/repo` donde ese path SÍ contiene `.env.example` y `docker-compose.yml`
Cuando corro `domain install` desde `/tmp`
Entonces el guard valida `/path/al/repo` (no el cwd) y pasa
Y los archivos `.env` se escriben en `/path/al/repo`, no en `/tmp`
```

### Escenario 4: `--src` inválido aborta con mensaje claro

```gherkin
Dado que paso `--src /no/existe` o `--src /path/sin/env.example`
Cuando corro `domain install`
Entonces el proceso termina con exit code != 0
Y el mensaje dice: "--src <path> no es un root de repo domain válido (faltan .env.example y/o docker-compose.yml)"
```

### Escenario 5: Sabotaje — guard deshabilitado no toca nada

```gherkin
Dado que el cwd es un tempdir sin `.env.example`
Cuando corro el comando CON el guard desactivado (sabotaje)
Entonces el test e2e del Escenario 1 debe FALLAR (no debe pasar "aborta limpio")
Cuando restauro el guard
Entonces el test vuelve a pasar
```

### Escenario 6: Edge case — solo uno de los dos archivos presentes

```gherkin
Dado que el cwd contiene `.env.example` pero NO `docker-compose.yml`
Cuando corro `domain install`
Entonces el guard aborta (ambos deben estar presentes)
Y el mensaje lo aclara: "faltan .env.example y/o docker-compose.yml"
```

## Notas

- El guard es la PRIMERA verificación de `runInstall`, antes de backups, antes de `loadEnvCascade`, antes de cualquier side effect.
- El guard es testeable unitariamente: helper puro `IsProjectRoot(path string) (bool, []string, error)` que retorna la lista de archivos faltantes.
