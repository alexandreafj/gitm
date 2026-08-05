

<div align="center">

# gitm

**Gestor de Git Multi-Repositorio**

Ejecuta operaciones de git en docenas de repositorios en paralelo — checkout, pull, commit, stash, reset, track — desde un solo comando.

[![CI](https://github.com/alexandreafj/gitm/actions/workflows/ci.yml/badge.svg)](https://github.com/alexandreafj/gitm/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/alexandreafj/gitm?sort=semver&cacheSeconds=300)](https://github.com/alexandreafj/gitm/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/alexandreafj/gitm)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexandreafj/gitm)](https://goreportcard.com/report/github.com/alexandreafj/gitm)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#installation)
[![License](https://img.shields.io/github/license/alexandreafj/gitm)](LICENSE)

</div>

---

## Aspectos destacados

- **Paralelismo por defecto** — 10 operaciones de git concurrentes, con salida en tiempo real.
- **Seguro** — nunca fuerza el reinicio de tu trabajo; los repos sucios se omiten, no se sobrescriben.
- **Vistas previas de prueba (dry-run)** — inspecciona operaciones peligrosas de branch/delete/reset/sync/checkout/discard antes de que cambien algo.
- **Panel de ramas** — `gitm branches feature/JIRA-123` muestra, por repositorio, dónde existe una rama de características, su estado de seguimiento/adelante-atrás, y si ya se fusionó en la rama predeterminada.
- **Contextos** — conjuntos de trabajo de repos aislados (`gitm context use company-a`); cada comando opera solo en el contexto activo.
- **TUI interactiva** — selección múltiple de repos y archivos con bubbletea.
- **Auto-actualización (instalaciones manuales)** — `gitm upgrade` descarga binarios firmados desde GitHub Releases en instalaciones manuales de macOS/Linux.
- **Cero configuración** — un único archivo SQLite en `~/.gitm/gitm.db`, sin demonios.

---

## Tabla de contenidos

- [¿Por qué gitm?](#why-gitm)
- [Instalación](#installation)
- [Inicio rápido](#quick-start)
- [Referencia de comandos](#commands-reference)
  - [repo add](#gitm-repo-add)
  - [repo list](#gitm-repo-list)
  - [repo remove](#gitm-repo-remove)
  - [repo rename](#gitm-repo-rename)
  - [group](#gitm-group)
  - [context](#gitm-context)
  - [checkout](#gitm-checkout)
  - [branch create](#gitm-branch-create)
  - [branch rename](#gitm-branch-rename)
  - [branch delete](#gitm-branch-delete)
  - [status](#gitm-status)
  - [branches](#gitm-branches)
  - [update](#gitm-update)
  - [sync](#gitm-sync)
  - [discard](#gitm-discard)
  - [commit](#gitm-commit)
  - [push](#gitm-push)
  - [stash](#gitm-stash)
  - [reset](#gitm-reset)
  - [track](#gitm-track)
  - [untrack](#gitm-untrack)
  - [doctor](#gitm-doctor)
  - [upgrade](#gitm-upgrade)
- [¿Cómo funciona?](#how-it-works)
- [Almacenamiento de datos](#data-storage)
- [Pruebas](#testing)
  - [Ejecutar pruebas](#running-tests)
  - [Estadísticas de pruebas](#test-stats)
- [Desarrollo](#development)

---

## ¿Por qué gitm?

Al trabajar con muchos repositorios, las operaciones diarias de git se vuelven repetitivas:

| Sin gitm | Con gitm |
|---|---|
| `cd repo1 && git checkout main && git pull` × 23 repos | `gitm checkout master` |
| Revisar una rama de características en repos específicos de forma interactiva | `gitm checkout` |
| Revisar una rama en todos los repos a la vez | `gitm checkout feature/JIRA-12345` |
| Navegar manualmente con `cd` a 6 repos para crear una rama de características | `gitm branch create feature/JIRA-123` |
| Renombrar manualmente una rama en cada repo + actualizar el remoto | `gitm branch rename old-name new-name` |
| Olvidar qué repos están sucios o detrás de origin | `gitm status` |
| Verificar qué repos tienen una rama de características y cómo se compara cada una con la predeterminada | `gitm branches feature/JIRA-123` |
| Navegar manualmente con `cd` a cada repo para hacer stage + commit + push | `gitm commit` |
| Guardar cambios (stash) en repos específicos antes de cambiar de rama | `gitm stash` |
| Reaplicar el trabajo guardado (stash) después de volver a la rama | `gitm stash pop` |
| Deshacer commits locales y limpiar el historial antes de hacer push | `gitm reset` |
| Reescribir el historial push de forma segura en múltiples repos | `gitm reset --hard --commits 2` y luego aprobar force-push |

---

## Instalación

### Homebrew Cask (macOS)

```bash
brew tap alexandreafj/gitm
brew install --cask gitm
```

### Scoop (Windows)

```powershell
scoop bucket add gitm https://github.com/alexandreafj/scoop-gitm
scoop install gitm
```

### Script de shell (macOS / Linux)

Detecta automáticamente tu sistema operativo y arquitectura, descarga el binario, verifica la suma de comprobación SHA-256 y lo instala en `/usr/local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/alexandreafj/gitm/master/install.sh | sh
```

Para instalar una versión específica o en un directorio personalizado:

```bash
curl -fsSL https://raw.githubusercontent.com/alexandreafj/gitm/master/install.sh | VERSION=v1.0.12 INSTALL_DIR="$HOME/.local/bin" sh
```

### Auto-actualización

Para instalaciones manuales en macOS/Linux, gitm puede actualizarse a sí mismo con verificación de firma:

```bash
gitm upgrade
```

Si se instaló mediante un administrador de paquetes, usa en su lugar el flujo de actualización del administrador:

```bash
brew upgrade --cask gitm   # Homebrew (macOS)
scoop update gitm          # Scoop (Windows)
```

### Descargar binario precompilado

Los binarios precompilados para todas las plataformas principales también están disponibles en la página de [GitHub Releases](https://github.com/alexandreafj/gitm/releases).

| Plataforma | Binario |
|---|---|
| macOS (Apple Silicon) | `gitm-macos-arm64` |
| macOS (Intel) | `gitm-macos-x86_64` |
| Linux (x86_64) | `gitm-linux-amd64` |
| Linux (ARM64) | `gitm-linux-arm64` |
| Windows (x86_64) | `gitm-windows-amd64.exe` |

```bash
# Ejemplo: macOS Apple Silicon
curl -L https://github.com/alexandreafj/gitm/releases/latest/download/gitm-macos-arm64 -o gitm
chmod +x gitm
sudo mv gitm /usr/local/bin/
```

#### Verificar instalación

```bash
gitm --help
```

### Prerrequisitos (compilar desde el código fuente)

- [Go 1.26+](https://golang.org/dl/)
- `git` disponible en tu `PATH`

### Desde el código fuente

```bash
# Clonar el repositorio
git clone https://github.com/alexandreafj/gitm.git
cd gitm

# Compilar e instalar en GOPATH/bin
make install

# Verificar instalación
gitm --help
```

### Verificación

En instalaciones manuales de macOS/Linux, `gitm upgrade` verifica la firma en `checksums.txt` contra el flujo de lanzamiento de este repositorio antes de instalar cualquier binario nuevo:

- El flujo de lanzamiento firma `checksums.txt` con [cosign](https://github.com/sigstore/cosign) en modo sin claves (OIDC vinculado a `release.yml` en un push con etiqueta). La firma, el certificado y la prueba del registro de transparencia Rekor se empaquetan en `checksums.txt.bundle` y se suben con cada lanzamiento.
- `gitm upgrade` requiere el binario de la plataforma, `checksums.txt`, `checksums.txt.bundle` y un verificador disponible. Verifica las sumas de comprobación firmadas contra la raíz de confianza pública de Sigstore, luego verifica la suma de comprobación del binario.
- Un activo de verificación faltante o cualquier fallo de firma/suma de comprobación aborta la actualización antes de que se modifique el ejecutable instalado. No existe un respaldo sin firmar.

Para verificar un binario descargado manualmente fuera de `gitm upgrade`:

```bash
# Descargar el binario, las sumas de comprobación y el paquete para la versión elegida
curl -L -O https://github.com/alexandreafj/gitm/releases/download/<tag>/gitm-macos-arm64
curl -L -O https://github.com/alexandreafj/gitm/releases/download/<tag>/checksums.txt
curl -L -O https://github.com/alexandreafj/gitm/releases/download/<tag>/checksums.txt.bundle

# Verificar la firma en checksums.txt (requiere tener cosign instalado)
cosign verify-blob --bundle checksums.txt.bundle \
  --certificate-identity-regexp '^https://github\.com/alexandreafj/gitm/\.github/workflows/release\.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Luego verificar el binario contra el archivo de sumas de comprobación (ahora confiable)
sha256sum -c checksums.txt --ignore-missing
```

### Solo compilar (sin instalar)

```bash
make build
# El binario estará en ./bin/gitm
./bin/gitm --help
```

### Agregar a PATH (si usas ./bin/ directamente)

```bash
export PATH="$PATH:/path/to/gitm/bin"
# Agrega lo anterior a tu ~/.bashrc o ~/.zshrc para hacerlo permanente
```

---

## Inicio rápido

```bash
# 1. Registra tus repositorios
gitm repo add /path/to/api-gateway
gitm repo add /path/to/auth-service /path/to/frontend /path/to/payment-svc

# Agregar con un alias personalizado (útil cuando dos repos comparten el mismo nombre de directorio)
gitm repo add /path/to/www-api/v1 --alias www-api-v1
gitm repo add /path/to/docs-api/v1 --alias docs-api-v1

# O registrar el directorio actual
cd /path/to/my-repo && gitm repo add .

# ¿Tienes una carpeta llena de repos? Regístralos todos a la vez
gitm repo add /path/to/projects --auto-detect

# 2. Ver todos los repos registrados
gitm repo list

# 3. Sincronizar todo a la rama predeterminada (main/master) y hacer pull
gitm checkout master

# 4. Iniciar una nueva tarea: crear una rama en los repos seleccionados de forma interactiva
gitm branch create feature/JIRA-456

# 5. Verificar en qué estado están todos tus repos
gitm status

# 6. Hacer pull de lo último en la rama en la que se encuentre cada repo actualmente
gitm update
```

---

## Referencia de comandos

### `gitm repo add`

Registra uno o más repositorios de git locales en gitm.

```
gitm repo add <path> [path...]
gitm repo add <path> --group <name>
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `<path>` | Ruta absoluta o relativa a un repositorio de git. Usa `.` para el directorio actual. |

**Banderas:**

| Bandera | Predeterminado | Descripción |
|---|---|---|
| `--alias` | _(nombre del directorio)_ | Nombre de visualización personalizado para el repositorio. Debe ser único en todos los repos registrados. Útil cuando dos repos comparten el mismo nombre de directorio (ej. dos repos llamados `v1`). No se puede combinar con `--auto-detect`. |
| `--auto-detect` | false | Escanea los subdirectorios inmediatos de la ruta dada y registra cada repositorio de git encontrado. Omite directorios planos y ocultos (nombres que comienzan con `.`). No se puede combinar con `--alias`. |
| `--depth` | 1 | Cuántos niveles de directorio escanear al usar `--auto-detect`. Usa `--depth 2` cuando los repos están anidados dentro de carpetas de agrupación (ej. `project/v1`). Requiere `--auto-detect`. |
| `--group`, `-g` | _(ninguno)_ | También agrega el repositorio a un grupo personalizado existente. Se puede repetir o separar por comas. El repo se agrega automáticamente al grupo integrado `all`. |

**Ejemplos:**

```bash
# Agregar un solo repo
gitm repo add /home/user/work/api-gateway

# Agregar un repo y colocarlo en un grupo existente
gitm repo add /home/user/work/api-gateway --group backend

# Agregar múltiples repos a la vez
gitm repo add /home/user/work/api-gateway /home/user/work/auth-service /home/user/work/frontend

# Agregar el directorio actual
gitm repo add .

# Agregar dos repos que comparten el mismo nombre de directorio usando alias
gitm repo add /home/user/work/www-api/v1 --alias www-api-v1
gitm repo add /home/user/work/docs-api/v1 --alias docs-api-v1

# Escanear una carpeta padre y registrar cada repo git encontrado dentro
gitm repo add /home/user/work --auto-detect

# Escanear dos niveles de profundidad para encontrar repos en subcarpetas (ej. api-group/v1, api-group/v2)
gitm repo add /home/user/work --auto-detect --depth 2
```

**Ejemplo de salida con `--auto-detect`:**

```
$ gitm repo add /home/user/work --auto-detect

Se encontraron 4 repositorio(s) git en /home/user/work

  ✓ agregado api-gateway (rama predeterminada: main)
  ✓ agregado auth-service (rama predeterminada: master)
  ✓ agregado frontend (rama predeterminada: main)
  ✓ agregado payment-svc (rama predeterminada: main)

4 repositorio(s) registrados. Ejecuta `gitm repo list` para verlos todos.
```

Si algunos repos ya están registrados, se reportan como omitidos (⚠) y no causan un error:

```
$ gitm repo add /home/user/work --auto-detect

Se encontraron 4 repositorio(s) git en /home/user/work

  ✓ agregado auth-service (rama predeterminada: master)
  ⚠ /home/user/work/api-gateway: ya registrado como "api-gateway"
  ⚠ /home/user/work/frontend: ya registrado como "frontend"
  ✓ agregado payment-svc (rama predeterminada: main)

2 repositorio(s) registrados. Ejecuta `gitm repo list` para verlos todos.
```

**Comportamiento:**

- Valida que la ruta exista y sea un repositorio de git (comprobación con `git rev-parse`).
- Auto-detecta la rama predeterminada (`main` o `master`) inspeccionando `origin/HEAD`.
- El alias (nombre de visualización) predetermina al nombre base del directorio. Usa `--alias` para anularlo — esto es obligatorio cuando dos repos comparten el mismo nombre de directorio.
- Si el alias ya está en uso por otra ruta, imprime un error claro con un comando `--alias` sugerido.
- Almacena el alias, la ruta y la rama predeterminada en `~/.gitm/gitm.db`.
- Cada repositorio se agrega automáticamente al grupo integrado `all`. Usa `--group` solo para membresías personalizadas adicionales; el grupo debe existir ya.
- Con `--auto-detect`: escanea subdirectorios hasta `--depth` niveles de profundidad (predeterminado 1). Cuando se encuentra un repo git, sus hijos no se escanean. Los directorios ocultos (`.git`, `.cache`, etc.) siempre se omiten en cada nivel.

---

### `gitm repo list`

Lista los repositorios registrados en el contexto activo. Usa `--all` para listar cada repositorio en todos los contextos.

gitm realiza una búsqueda de red ligera de `origin`'s simbólico `HEAD` para
cada repositorio antes de mostrar la tabla; esto no es un fetch completo y no
modifica el worktree ni los metadatos de Git. Un cambio descubierto con éxito
actualiza la caché de rama predeterminada de SQLite de GitM. Si no se puede consultar un remoto, gitm
imprime una advertencia nombrando los repositorios afectados y continúa con sus
valores en caché. Las operaciones de rama predeterminada usan la misma búsqueda en vivo; sus vistas previas
de dry-run nunca persisten un valor actualizado.

```
gitm repo list [--all]
```

**Banderas:**

| Bandera | Descripción |
|---|---|
| `--all` | Lista repositorios de todos los contextos, no solo del activo. |

**Ejemplo de salida:**

```
Contexto: default

#     ALIAS                     RAMA PREDETERMINADA  RUTA
1     api-gateway               main            /home/user/work/api-gateway
2     auth-service              master          /home/user/work/auth-service
3     docs-api-v1               master          /home/user/work/docs-api/v1
4     frontend                  main            /home/user/work/frontend
5     www-api-v1                main            /home/user/work/www-api/v1

5 repositorio(s) registrados.
```

---

### `gitm repo remove`

Deregistra un repositorio por alias. Esto solo lo elimina de la base de datos de gitm — no **elimina** el repositorio del disco.

```
gitm repo remove <alias>
gitm repo rm <alias>       # alias
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `<alias>` | El alias del repositorio tal como se muestra en `gitm repo list`. |

**Ejemplos:**

```bash
gitm repo remove api-gateway
gitm repo rm www-api-v1
```

---

### `gitm repo rename`

Renombra el alias de un repositorio registrado sin eliminarlo y volverlo a agregar. Útil para corregir nombres ambiguos en repos ya registrados.

```
gitm repo rename <old-alias> <new-alias>
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `<old-alias>` | El alias actual tal como se muestra en `gitm repo list`. |
| `<new-alias>` | El nuevo alias a usar. No debe estar ya en uso. |

**Ejemplos:**

```bash
# Corregir un nombre ambiguo después del registro
gitm repo rename v1 www-api-v1
gitm repo rename v2 www-api-v2
```

---

### `gitm group`

Gestiona grupos de repositorios opcionales. Los usuarios existentes no necesitan hacer nada después de actualizar: la migración de base de datos crea un grupo integrado `all` y rellena automáticamente cada repositorio registrado en él.

El grupo `all` es de gestión del sistema. Aparece en `gitm group list` y `gitm group show all`, pero no puede crearse, renombrarse, eliminarse o editarse manualmente.

```
gitm group list
gitm group show <name>
gitm group create <name>
gitm group rename <old-name> <new-name>
gitm group delete <name>
gitm group add <name> <repo-alias...>
gitm group remove <name> <repo-alias...>
```

**Subcomandos:**

| Comando | Descripción |
|---|---|
| `gitm group list` | Lista grupos, incluyendo `all`, con conteos de repositorios. |
| `gitm group show <name>` | Muestra repositorios en un grupo. |
| `gitm group create <name>` | Crea un grupo personalizado. |
| `gitm group rename <old> <new>` | Renombra un grupo personalizado. |
| `gitm group delete <name>` | Elimina un grupo personalizado y sus membresías. Los repositorios no se eliminan. |
| `gitm group add <name> <repo-alias...>` | Agrega repositorios registrados a un grupo personalizado. |
| `gitm group remove <name> <repo-alias...>` | Elimina repositorios registrados de un grupo personalizado. |

**Comportamiento:**

- Los nombres de grupo no pueden estar vacíos, contener espacios ni comas.
- Los alias de repositorio deben existir antes de poder agregarse a un grupo.
- El grupo `all` siempre contiene cada repositorio registrado y se actualiza automáticamente por `repo add` y `repo remove`.
- `--group all` en comandos conscientes de repos es equivalente a no usar filtro de grupo.
- `--repo` y `--group` pueden combinarse; gitm usa la intersección y preserva el orden explícito de `--repo`.

**Ejemplos:**

```bash
# Crear un grupo y agregar repos a él
gitm group create backend
gitm group add backend api-gateway auth-service

# Mostrar grupos y contenidos de grupos
gitm group list
gitm group show backend
gitm group show all

# Renombrar o eliminar un grupo personalizado
gitm group rename backend services
gitm group delete services
```

**Ejemplo de salida:**

```
$ gitm group list

GRUPO                     REPOS       TIPO
all                       5           integrado
backend                   2           personalizado
frontend                  1           personalizado
```

---

### `gitm context`

Gestiona contextos de repositorios — conjuntos de trabajo aislados de repositorios. Cada repositorio pertenece a **exactamente un** contexto (1:N), y todos los comandos multi-repo (`checkout`, `branch`, `status`, `commit`, `push`, `stash`, `reset`, `sync`, `update`, `discard`, `track`, `untrack`, `doctor`, `branches`) operan **solo** en los repositorios del contexto activo actualmente.

El contexto integrado `default` siempre existe: las nuevas instalaciones comienzan en él, y después de actualizar, todos los repositorios registrados previamente se migran automáticamente a él. No puede renombrarse ni eliminarse.

Cambiar de contexto se persiste en la base de datos, por lo que la elección sobrevive entre invocaciones — configúralo una vez y cada comando `gitm` siguiente permanece en el alcance de ese contexto hasta que cambies nuevamente.

```
gitm context list
gitm context show [name]
gitm context current
gitm context create <name>
gitm context use <name>
gitm context rename <old-name> <new-name>
gitm context delete <name>
gitm context add <name> <repo-alias...>
gitm context remove <name> <repo-alias...>
```

**Subcomandos:**

| Comando | Descripción |
|---|---|
| `gitm context list` | Lista contextos con conteos de repositorios. El activo se marca con `*`. |
| `gitm context show [name]` | Muestra repositorios en un contexto. Sin un nombre, muestra el contexto activo. |
| `gitm context current` | Imprime el nombre del contexto activo (amigable para scripts). |
| `gitm context create <name>` | Crea un contexto personalizado. |
| `gitm context use <name>` | Cambia el contexto activo (alias: `switch`). Persiste entre invocaciones. |
| `gitm context rename <old> <new>` | Renombra un contexto personalizado. |
| `gitm context delete <name>` | Elimina un contexto personalizado. Sus repositorios vuelven a `default`; nada se deregistra. |
| `gitm context add <name> <repo-alias...>` | Mueve repositorios a un contexto (alias: `assign`). Los elimina de su contexto anterior. |
| `gitm context remove <name> <repo-alias...>` | Mueve repositorios fuera de un contexto de vuelta a `default`. |

**Comportamiento:**

- Los nombres de contexto no pueden estar vacíos, contener espacios ni comas.
- Un repositorio pertenece a exactamente un contexto; `context add` lo mueve, nunca lo copia.
- `gitm repo add` registra nuevos repositorios en el contexto **activo**.
- Nombrar un repo explícitamente (`--repo`) que esté fuera del contexto activo es un error, con una sugerencia para cambiarlo o moverlo — los comandos nunca pueden alcanzar fuera del contexto activo.
- `--group` filtra intersecciones con el contexto activo: solo los miembros del grupo que también están en el contexto activo se usan.
- El contexto activo no puede eliminarse; cámbiate primero. El contexto `default` nunca puede eliminarse ni renombrarse.
- `gitm context` también responde al alias `gitm ctx`.

**Ejemplos:**

```bash
# Un contexto por cliente, cada uno con sus propios repos
gitm context create company-a
gitm context add company-a api-gateway auth-service

# Cambiar: de ahora en adelante cada comando solo toca repos de company-a
gitm context use company-a
gitm checkout feature/JIRA-123     # solo repos de company-a cambian de rama
gitm branch create feature/JIRA-456
gitm status

# Inspeccionar contextos
gitm context list
gitm context show company-a
gitm context current

# Volver al contexto predeterminado
gitm context use default
```

**Ejemplo de salida:**

```
$ gitm context list

   CONTEXTO                   REPOS       TIPO
   default                   3           integrado
*  company-a                 2           personalizado
   company-b                 4           personalizado

$ gitm context current
company-a
```

---

### `gitm checkout`

Cambia repositorios a una rama y hace pull. Tres modos de operación. Se ejecuta en **paralelo**.

```
gitm checkout [branch] [--repo alias1,alias2] [--group name] [--dry-run]
```

**Modos:**

| Invocación | Comportamiento |
|---|---|
| `gitm checkout` _(sin args)_ | Interactivo: selección múltiple de repos, luego escribe un nombre de rama |
| `gitm checkout master` o `gitm checkout main` | Alias equivalentes: detecta y cambia **todos** los repos a su rama predeterminada + pull |
| `gitm checkout <branch-name>` | Hace checkout de `<branch-name>` en **todos** los repos; omite con advertencia donde no exista |

**Banderas:**

| Bandera | Abreviatura | Descripción |
|---|---|---|
| `--repo` | `-r` | Limita el checkout a alias de repositorios específicos (separados por comas) |
| `--group` | `-g` | Limita el checkout a repositorios en un grupo. Se combina con `--repo` como una intersección. |
| `--dry-run` | — | Vista previa de comandos de checkout/fetch/pull sin cambiar ramas ni buscar ramas solo remotas. |

**Comportamiento (todos los modos):**

- Los repositorios con cambios **rastreados** sin commit se omiten (los archivos no rastreados como `AGENTS.md` se ignoran de forma segura).
- La existencia de la rama se verifica primero localmente, luego en el remoto — se omite con una advertencia si ninguno la tiene.
- Después del checkout, ejecuta `git pull --ff-only`. Si la rama no tiene upstream (una rama solo local), el pull se omite con una nota — el checkout aún tiene éxito.
- En modo de rama predeterminada, gitm resuelve el simbólico `HEAD` en vivo de cada remoto antes del checkout. Un cambio predeterminado actualiza la caché; si no se puede resolver, gitm advierte y usa la rama en caché.
- Con `--dry-run`, gitm imprime los comandos planificados y omisiones conocidas, pero no busca, hace checkout, pull ni muta repositorios. Los conflictos de checkout que Git solo detecta durante el checkout se muestran como notas de riesgo.
- Una ejecución de prueba de rama predeterminada usa el predeterminado en vivo para su vista previa pero no persiste un valor actualizado.
- Transmite resultados en vivo con un resumen final.

**Ejemplo — rama predeterminada:**

```
$ gitm checkout master

Haciendo checkout de la rama predeterminada y pull para 4 repositorios…

[api-gateway        ] ✓ en main — ya está actualizado
[auth-service       ] ✓ en master — 3 archivos cambiados, 47 inserciones(+)
[frontend           ] ⚠ OMITIDO: cambios sin commit (2 archivo(s)): M src/App.tsx, M package.json
[payment-svc        ] ✓ en main — ya está actualizado

Hecho: 3 exitosos, 1 omitido
```

**Ejemplo — repos específicos solo:**

```
$ gitm checkout master --repo=api-gateway,auth-service

Haciendo checkout de la rama predeterminada y pull para 2 repositorios…

[api-gateway        ] ✓ en main — ya está actualizado
[auth-service       ] ✓ en master — 3 archivos cambiados, 47 inserciones(+)

Hecho: 2 exitosos, 0 omitidos
```

**Ejemplo — solo grupo:**

```
$ gitm checkout master --group backend

Haciendo checkout de la rama predeterminada y pull para 2 repositorios…
```

**Ejemplo — rama específica:**

```
$ gitm checkout feature/JIRA-12345

Haciendo checkout de la rama "feature/JIRA-12345" en 4 repositorios…

[api-gateway        ] ✓ en feature/JIRA-12345 — ya está actualizado
[auth-service       ] ✓ en feature/JIRA-12345 — hecho pull
[frontend           ] ⚠ OMITIDO: rama "feature/JIRA-12345" no encontrada (local o remoto)
[payment-svc        ] ⚠ OMITIDO: cambios sin commit (1 archivo(s))

Hecho: 2 exitosos, 2 omitidos
```

**Ejemplo — vista previa de prueba (dry-run):**

```
$ gitm checkout feature/JIRA-12345 --dry-run

EJECUCIÓN DE PRUEBA: no se hicieron cambios

Vista previa de checkout de la rama "feature/JIRA-12345" para 4 repositorio(s)

[api-gateway] /home/user/work/api-gateway
  Ejecutaría:
    - git checkout feature/JIRA-12345
    - git pull --ff-only

No se hicieron cambios.
```

**Ejemplo — interactivo:**

```
$ gitm checkout

Selecciona repositorios para hacer checkout
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

▶ [✓] api-gateway   /home/user/work/api-gateway
  [✓] auth-service   /home/user/work/auth-service
  [ ] frontend       /home/user/work/frontend

2/3 seleccionados

Rama para hacer checkout
Escribe el nombre de la rama  •  enter para confirmar  •  esc para cancelar

feature/JIRA-12345

Haciendo checkout de la rama "feature/JIRA-12345" en 2 repositorios…
```

---

### `gitm branch create`

Crea una nueva rama en los repositorios seleccionados y la empuja a origin con seguimiento upstream, para que los pulls y pushes posteriores (`gitm update`, `gitm checkout`, `gitm push`) funcionen sin configuración adicional. Una interfaz interactiva de selección múltiple te permite elegir en qué repositorios aplicar la operación. Usa `--repo` para omitir la interfaz por completo y apuntar a repositorios específicos por alias.

```
gitm branch create <branch-name> [flags]
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `<branch-name>` | El nombre de la nueva rama a crear. |

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--all` | `-a` | false | Omitir la interfaz de selección y aplicar a todos los repositorios registrados. |
| `--from` | `-f` | _(rama predeterminada del repo)_ | Rama base desde la cual crear en lugar de la rama predeterminada del repo. |
| `--no-remote` | — | false | Crear la rama solo localmente. Omitir empujarla a origin y configurar el seguimiento upstream. |
| `--repo` | `-r` | _(ninguno)_ | Lista separada por comas de alias de repositorios a apuntar. Omite la interfaz de selección interactiva. Tiene prioridad sobre `--all`. |
| `--group` | `-g` | _(todos los repos)_ | Limitar candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Interfaz interactiva:**

Cuando ejecutas `gitm branch create feature/JIRA-123`, verás:

```
Selecciona repositorios para la nueva rama: feature/JIRA-123
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [ ] api-gateway     /home/user/work/api-gateway
▶ [✓] auth-service    /home/user/work/auth-service
  [✓] frontend        /home/user/work/frontend
  [ ] payment-svc     /home/user/work/payment-svc

2/4 seleccionados
```

**Atajos de teclado:**

| Tecla | Acción |
|---|---|
| `↑` / `k` | Mover cursor arriba |
| `↓` / `j` | Mover cursor abajo |
| `Space` | Alternar selección |
| `a` | Seleccionar / deseleccionar todo |
| `Enter` | Confirmar selección y continuar |
| `q` / `Esc` | Cancelar |

**Comportamiento:**

1. A menos que se proporcione `--from`, realiza una búsqueda de red ligera del
   simbólico `HEAD` de `origin` de cada repositorio seleccionado (no un fetch completo). Un
   cambio predeterminado actualiza la caché SQLite de GitM; una búsqueda fallida emite una advertencia y
   usa el predeterminado en caché. Proporcionar `--from` omite esta búsqueda.
2. Para cada repo seleccionado (en paralelo):
   - Verifica cambios sin commit — omite si está sucio.
   - Hace checkout de la rama base. Si tiene un upstream, hace pull de lo último y aborta la creación de rama en ese repo si la actualización falla. Si no tiene upstream, usa explícitamente el estado base local.
   - Crea y hace checkout de la nueva rama (`git checkout -b <branch-name>`).
   - Si la rama ya existe, la hace checkout en lugar de fallar.
   - Empuja la rama a origin y configura el seguimiento upstream (`git push --set-upstream origin <branch-name>`), para que `gitm update` / `gitm checkout` nunca fallen con "sin información de seguimiento" después. Se omite con `--no-remote` o cuando el repositorio no tiene el remoto `origin`; las ramas que ya rastrean un remoto se dejan sin tocar.
3. Transmite resultados en vivo. Después de que cada repositorio seleccionado finalice, sale con código distinto de cero si algún repositorio reportó un error operativo.

**Ejemplos:**

```bash
# Selección interactiva
gitm branch create feature/JIRA-456

# Crear en todos los repos sin preguntar
gitm branch create feature/JIRA-456 --all

# Crear solo localmente — omitir el push a origin
gitm branch create feature/JIRA-456 --no-remote

# Crear en cada repo de un grupo
gitm branch create feature/JIRA-456 --all --group backend

# Crear solo en repos específicos por alias (sin prompt)
gitm branch create feature/AA-1 --repo api-gateway,auth-service --group backend

# Crear desde una rama base específica
gitm branch create hotfix/critical-bug --from develop

# Apuntar a repos específicos y usar una rama base personalizada
gitm branch create feature/AA-1 --repo api-gateway --from develop
```

---

### `gitm branch rename`

Renombra una rama en los repositorios seleccionados — tanto local como en el remoto. Se ejecuta en **paralelo**.

```
gitm branch rename <old-name> <new-name> [flags]
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `<old-name>` | El nombre actual de la rama. |
| `<new-name>` | El nuevo nombre de la rama. |

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--all` | `-a` | false | Aplicar a todos los repositorios que tienen la rama antigua. |
| `--no-remote` | — | false | Renombrar solo localmente. Omitir eliminar la rama remota antigua y empujar la nueva. |
| `--repo` | `-r` | _(ninguno)_ | Lista separada por comas de alias de repositorios a apuntar. Omite la interfaz de selección interactiva. Tiene prioridad sobre `--all`. |
| `--group` | `-g` | _(todos los repos)_ | Limitar candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Comportamiento:**

1. Filtra los repositorios registrados a solo aquellos que tienen una rama local llamada `<old-name>`.
2. Abre la interfaz interactiva de selección múltiple mostrando solo los repositorios coincidentes.
3. Para cada repo seleccionado (en paralelo):
   - `git branch -m <old-name> <new-name>` — renombra localmente.
   - `git push --set-upstream origin <new-name>` — empuja la nueva rama y configura el seguimiento.
   - `git push origin --delete <old-name>` — solo después de publicar la nueva rama, elimina la rama remota antigua (si existe). Un fallo en el push de la nueva rama por lo tanto preserva el nombre remoto antiguo.
4. Transmite resultados en vivo, luego sale con código distinto de cero si algún repositorio falló.

**Ejemplos:**

```bash
# Interactivo: renombrar feature/JIRA-123 a feature/JIRA-456 en repos seleccionados
gitm branch rename feature/JIRA-123 feature/JIRA-456

# Aplicar a todos los repos que tienen la rama
gitm branch rename feature/JIRA-123 feature/JIRA-456 --all

# Renombrar solo en repos específicos por alias (sin prompt)
gitm branch rename feature/JIRA-123 feature/JIRA-456 --repo api-gateway,auth-service --group backend

# Renombrado solo local (omitir remoto)
gitm branch rename old-name new-name --no-remote
```

**Ejemplo de salida:**

```
Renombrando "feature/JIRA-123" → "feature/JIRA-456" en 2 repositorio(s)…

[auth-service        ] ✓ renombrado feature/JIRA-123 → feature/JIRA-456 (local + remoto)
[frontend            ] ✓ renombrado feature/JIRA-123 → feature/JIRA-456 (local + remoto)

Hecho: 2 exitosos
```

---

### `gitm branch delete`

Elimina una rama en los repositorios seleccionados — tanto local como en el remoto en un solo paso. Se ejecuta en **paralelo**.

```
gitm branch delete <branch-name> [flags]
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `<branch-name>` | La rama a eliminar. |

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--all` | `-a` | false | Aplicar a todos los repositorios que tienen la rama. |
| `--force` | `-f` | false | Forzar eliminación de ramas con commits sin fusionar (`git branch -D` en lugar de `-d`). |
| `--no-remote` | — | false | Eliminar solo la rama local. Omitir eliminar la rama en origin. |
| `--dry-run` | — | false | Vista previa de comandos de checkout y eliminación sin cambiar nada ni pedir confirmación. |
| `--repo` | `-r` | _(ninguno)_ | Lista separada por comas de alias de repositorios a apuntar. Omite la interfaz de selección interactiva. Tiene prioridad sobre `--all`. |
| `--group` | `-g` | _(todos los repos)_ | Limitar candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Comportamiento:**

1. Filtra los repositorios registrados a solo aquellos que tienen la rama (localmente, o en origin a menos que `--no-remote` esté configurado).
2. Selecciona repositorios:
   - Interactivo: abre la interfaz de selección múltiple mostrando solo los repositorios coincidentes.
   - `--all` / `--repo`: omite la interfaz y pide una única confirmación `y/N` listando los repositorios objetivo.
3. Realiza una búsqueda de red ligera del simbólico `HEAD` de `origin` de cada repositorio seleccionado (no un fetch completo) antes de aplicar la protección de rama predeterminada.
   Fuera de dry-run, un cambio predeterminado actualiza la caché SQLite de GitM; una búsqueda fallida emite una advertencia y usa el predeterminado en caché.
4. Con `--dry-run`, imprime los comandos de checkout, eliminación local y eliminación remota que se ejecutarían, incluyendo omisiones conocidas para ramas predeterminadas y sin fusionar, luego sale sin confirmación ni cambios en el worktree, metadatos de Git o caché SQLite. La búsqueda en vivo aún suministra la rama predeterminada de la vista previa.
5. Para cada repo seleccionado (en paralelo):
   - Si el objetivo está actualmente con checkout, `git checkout <default-branch>` cambia a la rama predeterminada detectada de ese repositorio sin hacer pull.
   - `git branch -d <branch-name>` — elimina localmente (`-D` cuando `--force`).
   - `git push origin --delete <branch-name>` — elimina la rama remota si existe (omitida con `--no-remote`).
6. Transmite resultados en vivo, luego sale con código distinto de cero si algún repositorio falló.

**Seguridad:**

- La eliminación local usa `git branch -d`, que se niega a eliminar ramas con commits sin fusionar. Pasa `--force` para eliminarlas de todos modos.
- La rama predeterminada detectada en vivo del repositorio (`main`/`master`) nunca se elimina — se omite.
- Una rama que está actualmente con checkout cambia automáticamente a la rama predeterminada detectada antes de la eliminación.
- El checkout automático no hace pull. Si el checkout falla, la rama se deja sin tocar y ese repositorio reporta un error.

**Ejemplos:**

```bash
# Interactivo: eliminar feature/JIRA-123 en repos seleccionados
gitm branch delete feature/JIRA-123

# Aplicar a todos los repos que tienen la rama
gitm branch delete feature/JIRA-123 --all

# Eliminar solo en repos específicos por alias (pide confirmación)
gitm branch delete feature/JIRA-123 --repo api-gateway,auth-service --group backend

# Forzar eliminación de una rama con commits sin fusionar
gitm branch delete feature/JIRA-123 --force

# Eliminar solo la rama local, mantenerla en origin
gitm branch delete feature/JIRA-123 --no-remote

# Vista previa de checkout y eliminación automática sin cambiar nada
gitm branch delete feature/JIRA-123 --all --dry-run
```

**Ejemplo de salida:**

```
La rama "feature/JIRA-123" se eliminará en 2 repositorio(s):
  - auth-service
  - frontend
¿Eliminar rama "feature/JIRA-123"? [y/N] y

Eliminando "feature/JIRA-123" en 2 repositorio(s)…

[auth-service        ] ✓ cambiado a main — eliminado feature/JIRA-123 (local + remoto)
[frontend            ] ✓ eliminado feature/JIRA-123 (local + remoto)

Hecho: 2 exitosos
```

---

### `gitm status`

Muestra un resumen de todos los repositorios registrados: rama actual, estado sucio, y commits adelante/atrás de origin. Se ejecuta en **paralelo** sin llamadas de red por defecto.

```
gitm status [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--fetch` | — | false | Ejecuta `git fetch` en todos los repos primero para números remotos actualizados (más lento, requiere red). |
| `--repo` | `-r` | _(todos los repos)_ | Limita la salida a alias de repositorios específicos (separados por comas). |
| `--group` | `-g` | _(todos los repos)_ | Limita la salida a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Ejemplo de salida (modo rápido, sin red):**

```
Recopilando estado para 11 repositorios…

REPO                    RAMA                      SUCIO         REMOTO
────────────────────────────────────────────────────────────────────────────────
api-gateway             feature/PROJ-101          2 modificados   actualizado
auth-service            feature/PROJ-202          1 modificado    actualizado
billing                 master                    limpio         14 atrás
data-pipeline           feature/PROJ-101          1 modificado    actualizado
frontend                master                    1 modificado    actualizado
notifications           feature/PROJ-303          limpio         actualizado
payments                feature/PROJ-101          2 modificados   actualizado
reporting               master                    2 modificados   actualizado
search                  master                    12 modificados  4 atrás
user-service            feature/PROJ-303          1 modificado    actualizado
worker                  master                    1 modificado    actualizado
```

**Descripciones de columnas:**

| Columna | Descripción |
|---|---|
| `REPO` | Nombre del repositorio |
| `RAMA` | Rama con checkout actualmente |
| `SUCIO` | `limpio` si no hay cambios sin commit; de lo contrario muestra el número de archivos modificados |
| `REMOTO` | Commits adelante/atrás de `origin`. Basado en el último estado remoto conocido (sin llamada de red). Usa `--fetch` para números actuales. |

**Ejemplos:**

```bash
# Rápido: instantáneo, usa información de seguimiento remoto en caché
gitm status

# Preciso: hace fetch de origin primero, luego muestra el estado (toma unos segundos)
gitm status --fetch

# Mostrar estado solo para repos específicos
gitm status -r api-gateway
gitm status -r api-gateway,auth-service --group backend --fetch

# Mostrar estado para un grupo
gitm status --group backend
```

> **Nota de rendimiento:** Por defecto, `gitm status` es casi instantáneo porque no hace fetch de origin. Los números de adelante/atrás reflejan el último estado conocido de las ramas remotas. Usa `--fetch` si necesitas precisión al segundo desde el remoto.

---

### `gitm branches`

Un panel de ramas en todos los repositorios registrados. Responde, de un vistazo: en qué rama está cada repo, si una rama de características dada existe en cada repo, si está rastreada/empujada, cuán adelante/atrás de origin está, y si ya se fusionó en la rama predeterminada. Se ejecuta en **paralelo** y evita un fetch completo por defecto. Diseñado específicamente para trabajo multi-repo de características.

```
gitm branches [target-branch] [flags]
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `[target-branch]` | _(opcional)_ Enfoca el panel en una rama específica (ej. `feature/JIRA-123`). Sin ella, cada fila describe la rama en la que se encuentra actualmente el repositorio. |

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--fetch` | — | false | Ejecuta un `git fetch` completo en cada repo primero para que la existencia de ramas remotas y números de adelante/atrás estén actualizados (más lento y más trabajo de red que la búsqueda simbólica HEAD predeterminada). |
| `--repo` | `-r` | _(todos los repos)_ | Limita la salida a alias de repositorios específicos (separados por comas). |
| `--group` | `-g` | _(todos los repos)_ | Limita la salida a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Dos modos:**

| Invocación | Sujeto de cada fila |
|---|---|
| `gitm branches` | La rama en la que cada repo se encuentra **actualmente**. |
| `gitm branches <target>` | La **rama objetivo** en cada repo (con una columna `CURRENT` que muestra dónde está cada repo realmente). |

**Comportamiento:**

- Sin `--fetch`, la existencia de ramas remotas y adelante/atrás proviene de los refs de `origin` último fetch. gitm aún realiza una búsqueda de red ligera de cada simbólico `HEAD` de `origin` (no un fetch completo) para que los cálculos sensibles al predeterminado usen el predeterminado actual. Un cambio predeterminado actualiza la caché SQLite de GitM; una búsqueda fallida emite una advertencia y usa la caché. Pasa `--fetch` para actualizar todos los refs remotos primero.
- En modo objetivo, la columna `TARGET` reporta existencia: `local+remote`, `solo local`, `solo remoto`, o `falta`. Un `●` verde marca repos con checkout actualmente **en** el objetivo.
- `UPSTREAM` y `ADELANTE/ATRÁS` describen la rama sujeta; requieren que la rama exista **localmente**, por lo que muestran `—` para un objetivo `solo remoto` o `falta`.
- `FUSIONADO` reporta si la rama sujeta se fusionó en la rama predeterminada del repositorio (`fusionado` / `no fusionado`). Muestra `(predeterminada)` cuando el sujeto **es** la rama predeterminada, y `—` cuando no se puede determinar (ej. un objetivo faltante).

**Descripciones de columnas:**

| Columna | Descripción |
|---|---|
| `REPO` | Alias del repositorio. |
| `TARGET` _(modo objetivo)_ | Si la rama objetivo existe localmente y/o en origin; `●` marca el repo si tiene checkout en ella. |
| `RAMA` / `CURRENT` | La rama en la que se encuentra actualmente el repositorio. |
| `UPSTREAM` | La ref de seguimiento upstream de la rama sujeta, o `ninguna`. |
| `ADELANTE/ATRÁS` | Commits que la rama sujeta está adelante/atrás de su upstream. |
| `FUSIONADO` | Si la rama sujeta se fusionó en la rama predeterminada del repositorio. |

**Ejemplos:**

```bash
# Panel de la rama en la que cada repo se encuentra actualmente
gitm branches

# Rastrear una rama de características en cada repo
gitm branches feature/JIRA-123

# Actualizar estado remoto primero para existencia precisa + adelante/atrás
gitm branches feature/JIRA-123 --fetch

# Limitar a repos específicos o un grupo
gitm branches feature/JIRA-123 -r api-gateway,auth-service
gitm branches -g backend
```

**Ejemplo de salida — sin argumento:**

```
$ gitm branches

Recopilando info de ramas para 3 repositorios…

REPO                    RAMA                        UPSTREAM                ADELANTE/ATRÁS    FUSIONADO
────────────────────────────────────────────────────────────────────────────────────────────
api-gateway             feature/JIRA-123            origin/feature/JIRA-1…  ↑2                no fusionado
auth-service            master                      origin/master           actualizado       (predeterminada)
frontend                develop                     ninguna                 —               no fusionado
```

**Ejemplo de salida — rama objetivo:**

```
$ gitm branches feature/JIRA-123

Recopilando info de ramas para "feature/JIRA-123" en 3 repositorios…

REPO                    TARGET              ACTUAL                    UPSTREAM                ADELANTE/ATRÁS    FUSIONADO
────────────────────────────────────────────────────────────────────────────────────────────────────────────
api-gateway             ● local+remote      feature/JIRA-123          origin/feature/JIRA-1…  actualizado       no fusionado
auth-service              solo remoto       master                    —                       —               no fusionado
frontend                  falta             master                    —                       —               —
```

---

### `gitm discard`

Selecciona interactivamente qué repositorios y **qué archivos** descartar cambios sin commit en. Solo los repositorios que realmente tienen cambios se muestran en la lista de selección — si ninguno de tus repos tiene cambios sin commit, el comando sale inmediatamente con un mensaje.

```
gitm discard [flags]
```

> **ADVERTENCIA:** Esta operación es irreversible. Los cambios descartados no pueden recuperarse.

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(todos los repos)_ | Limita a alias de repositorios específicos (separados por comas), omite la selección interactiva de repos. |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |
| `--dry-run` | — | false | Ejecuta el mismo flujo de selección de repo/archivo, luego previsualiza comandos de reset/checkout/clean sin descartar archivos. |

**Qué hace por archivo seleccionado (basado en estado):**

| Estado del archivo | Comando(s) de Git | Efecto |
|---|---|---|
| Modificado rastreado (` M`, `M `, `MM`) | `git reset HEAD -- <file>` + `git checkout -- <file>` | Revirtir a la última versión comprometida |
| Archivo nuevo en stage (`A `) | `git reset HEAD -- <file>` + `git clean -fd -- <file>` | Quitar de stage y eliminar el archivo |
| Archivo/directorio no rastreado (`??`) | `git clean -fd -- <file>` | Elimina el archivo o directorio |

**Comportamiento:**

1. Escanea todos los repositorios registrados (o aquellos especificados con `--repo` / `--group`) en busca de cambios sin commit.
2. Si **ninguno** está sucio, imprime `Nada que descartar — todos los repositorios están limpios.` y sale.
3. Si algunos están sucios, muestra un resumen de cuántos archivos tiene modificados cada uno, luego abre la selección múltiple interactiva mostrando **solo los repos sucios** (omitido cuando se usa `--repo`).
4. Para cada repo seleccionado, abre un **selector de archivos** donde eliges exactamente qué archivos descartar. **Ningún archivo está preseleccionado** — debes elegir explícitamente cada archivo que quieras eliminar.
5. Con `--dry-run`, imprime los archivos seleccionados y los comandos `git reset`, `git checkout`, y `git clean` que se ejecutarían, luego sale sin cambiar archivos.
6. De lo contrario, descarta solo los archivos seleccionados en cada repo.
7. Imprime un resumen por repo, luego sale con código distinto de cero si algún repositorio falló.

**Ejemplo de flujo:**

```
3 repositorios con cambios sin commit:

  api-gateway            2 archivo(s) modificados
  auth-service           5 archivo(s) modificados
  frontend               12 archivo(s) modificados

ADVERTENCIA: Selecciona repositorios para descartar cambios en (irreversible)
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [ ] api-gateway        /home/user/work/api-gateway
▶ [✓] auth-service       /home/user/work/auth-service
  [ ] frontend           /home/user/work/frontend

1/3 seleccionados
```

Después de seleccionar un repo, aparece el **selector de archivos**:

```
━━━ auth-service ━━━
Selecciona archivos para descartar en auth-service (irreversible)
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

▶ [ ] M  src/handler.go
  [✓] M  src/config.go
  [ ] ?? tmp/debug.log
  [✓] A  src/new_service.go

2/4 seleccionados
```

Después de confirmar:

```
  ✓ Descartados 2 archivo(s):
       src/config.go
       src/new_service.go

Resumen
───────────────────────
  ✓  auth-service (2 archivo(s) descartados)

1 descartado  0 omitidos  0 fallidos
```

**Ejemplo con banderas --repo:**

```
$ gitm discard --repo api-gateway
$ gitm discard -r api-gateway,auth-service --group backend
$ gitm discard --group backend
$ gitm discard --repo api-gateway --dry-run
```

**Ejemplo cuando no hay nada que descartar:**

```
$ gitm discard
Nada que descartar — todos los repositorios están limpios.
```

---

### `gitm update`

Hace pull de los últimos cambios en la **rama actual** de cada repositorio en paralelo. A diferencia de `checkout master`, esto normalmente mantiene cada repositorio en su rama actual; la recuperación de upstream faltante descrita a continuación cambia a la rama predeterminada detectada.

```
gitm update [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(todos los repos)_ | Limita la actualización a alias de repositorios específicos (separados por comas). |
| `--group` | `-g` | _(todos los repos)_ | Limita la actualización a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Comportamiento:**

1. Si se especifica `--repo` o `--group`, solo se actualizan los repos coincidentes. De lo contrario, se actualizan todos los repos registrados.
2. Para cada repositorio (en paralelo):
   - Verifica cambios sin commit — omite si está sucio.
   - Ejecuta `git pull --ff-only` en la rama actual.
   - Si la rama no tiene upstream (nunca push), el repo se omite con una nota — aún no hay nada que hacer pull.
   - Si la rama remota ya no existe (ej. eliminada después de fusionar un PR), realiza una búsqueda de red ligera del simbólico `HEAD` de `origin` (no un fetch completo), actualiza la caché SQLite de rama predeterminada de GitM si cambió, luego cambia a esa predeterminada y hace pull. Si la búsqueda falla, advierte y usa la predeterminada en caché. Los repositorios cuyo pull ordinario tiene éxito no realizan esta búsqueda.
3. Transmite resultados en vivo con un resumen.
4. Si un alias `--repo` no coincide con ningún repositorio registrado, el comando sale con un error antes de hacer pull de algo.

**Caso de uso:** Has estado trabajando en una rama de características por un tiempo y quieres hacer pull de los últimos cambios que tus compañeros empujaron a la misma rama en múltiples repos.

**Ejemplos:**

```bash
# Actualizar todos los repos registrados
gitm update

# Actualizar un solo repo por alias
gitm update --repo=api-gateway

# Actualizar múltiples repos específicos
gitm update --repo=api-gateway,auth-service

# Actualizar repos en un grupo
gitm update --group backend

# Forma corta
gitm update -r api-gateway,auth-service -g backend
```

**Ejemplo de salida:**

```
Haciendo pull de la rama actual para 4 repositorios…

[api-gateway        ] ✓ en main — ya está actualizado
[auth-service       ] ✓ en feature/JIRA-456 — hecho pull
[frontend           ] ⚠ OMITIDO: cambios sin commit — haz stash o commit primero
[payment-svc        ] ✓ en main — ya está actualizado

Hecho: 3 exitosos, 1 omitido
```

---

### `gitm sync`

Fusiona una rama en la rama en la que cada repositorio se encuentra **actualmente** — en paralelo. Por defecto esa rama es la **rama predeterminada** de cada repo (`main`/`master`, auto-detectada por repo); pasa un argumento opcional `[branch]` para fusionar una rama diferente en su lugar. Esto reemplaza la rutina manual, por repo, de hacer pull de lo último de `master`/`main` y fusionarlo en tu rama de trabajo con `git merge master` a mano.

```
gitm sync [branch] [flags]
```

**Argumentos:**

| Argumento | Descripción |
|---|---|
| `[branch]` | _(opcional)_ Rama a fusionar en la rama actual de cada repo. Omitela para usar la rama predeterminada de cada repo (`main`/`master`). Los repos donde la rama no existe localmente o en `origin` se omiten. |

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(prompt)_ | Sincroniza solo los alias de repositorios listados (separados por comas). Omite el selector interactivo. |
| `--all` | `-a` | `false` | Sincroniza cada repositorio registrado sin preguntar. |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos de sync a repositorios en un grupo. Se combina con `--repo` como una intersección. |
| `--dry-run` | — | `false` | Vista previa de comandos de fetch/merge y omisiones conocidas sin hacer fetch ni merge. |

**Modos de selección:**

| Invocación | Comportamiento |
|---|---|
| `gitm sync` | Interactivo — selecciona repositorios vía la TUI. |
| `gitm sync <branch>` | Fusiona `<branch>` (en lugar de la rama predeterminada) en la rama actual de cada repo. |
| `gitm sync --repo a,b` | Sincroniza solo los repos `a` y `b` (sin prompt). |
| `gitm sync <branch> --repo a,b` | Fusiona `<branch>` en los repos `a` y `b` (sin prompt). |
| `gitm sync --all` | Sincroniza cada repositorio registrado (sin prompt). |
| `gitm sync --group backend` | Interactivo — selecciona de repositorios en `backend`. |
| `gitm sync --all --dry-run` | Vista previa de sync para cada repositorio registrado sin hacer fetch ni merge. |

**Comportamiento (por repositorio, en paralelo):**

1. Determina la rama objetivo: cuando `[branch]` se omite, resuelve el `origin/HEAD` predeterminado en vivo del repositorio (`main` o `master`) y actualiza el valor en caché; si esa búsqueda falla, advierte y usa la caché. Un `[branch]` suministrado se usa para cada repo sin actualizar predeterminados.
2. **Omite** repos con cambios rastreados sin commit (haz stash o commit primero). Los archivos no rastreados no bloquean el sync.
3. **Omite** repos ya en la rama objetivo (usa `gitm update` para hacer pull en su lugar).
4. Hace fetch de la última rama objetivo desde `origin`, luego fusiona `origin/<branch>` en la rama actual (cae en la rama local cuando no hay remoto). Los repos donde la rama falta tanto local como en `origin` se **omiten**.
5. Con `--dry-run`, imprime los comandos `git fetch` y `git merge --no-edit` planificados, pero no hace fetch, merge ni persiste una rama predeterminada actualizada. Los conflictos de merge no pueden predecirse sin ejecutar `git merge`, por lo que se muestran como notas de riesgo.
6. **Los conflictos de merge se dejan en su lugar** — el repo se reporta y se mantiene en su estado de merge para que puedas resolver los conflictos y commit. Un conflicto no se trata como un fallo; el comando aún sale 0.
7. Transmite resultados en vivo con un resumen, seguido por una lista de cualquier repo dejado con conflictos.

**Caso de uso:** Tu rama de características se ha quedado atrás de `master`/`main` (o una rama de integración de larga vida como `master-raw`) en varios repos y quieres fusionar los últimos cambios en cada uno en un solo paso.

**Ejemplos:**

```bash
# Seleccionar interactivamente repos para sincronizar con su rama predeterminada
gitm sync

# Sincronizar cada repo con su rama predeterminada
gitm sync --all

# Sincronizar cada repo en un grupo
gitm sync --all --group backend

# Fusionar una rama específica en lugar de la rama predeterminada
gitm sync master-raw

# Fusionar una rama específica en repos específicos (opcionalmente limitado a un grupo)
gitm sync master-raw --repo=api-gateway,auth-service --group backend

# Sincronizar repos específicos por alias
gitm sync --repo=api-gateway,auth-service --group backend

# Vista previa de todas las operaciones de sync sin cambiar refs ni archivos
gitm sync --all --dry-run

# Forma corta
gitm sync -r api-gateway
```

**Ejemplo de salida:**

```
Fusionando rama predeterminada en la rama actual de 3 repositorio(s)…

[api-gateway        ] ✓ fusionado main en feature/JIRA-456 — fast-forward
[auth-service       ] ⚠ OMITIDO: actualmente en "main" — nada que fusionar (usa `gitm update` para hacer pull)
[frontend           ] ⚠ OMITIDO: conflicto de merge — 2 archivo(s) para resolver manualmente

Hecho: 1 exitoso, 2 omitidos

1 repositorio(s) tiene conflictos de merge pendientes para resolver:
  - frontend (/Users/me/code/frontend)
      conflicto: src/app.tsx
      conflicto: package.json

Resuelve los conflictos en cada repo, luego `git add` + `git commit` (o `git merge --abort`).
```

---

### `gitm commit`

Selecciona interactivamente archivos y hace commit en repositorios sucios. Te guía por cada repositorio seleccionado **secuencialmente** — selecciona archivos, escribe un mensaje, y hace push. Usa `--repo` para omitir la interfaz de selección y apuntar a repositorios específicos por alias.

```
gitm commit [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--no-push` | — | false | Commit pero omite `git push` después de cada commit. |
| `--repo` | `-r` | _(ninguno)_ | Lista separada por comas de alias de repositorios a apuntar. Omite la interfaz interactiva de selección múltiple. Los repos no sucios en la lista se omiten silenciosamente. |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Qué hace:**

1. **Escanea** los repositorios registrados (todos, o solo aquellos en `--repo` / `--group`) en busca de cambios sin commit.
2. **Actualiza predeterminados para repos sucios solo** con una búsqueda de red ligera de cada simbólico `HEAD` de `origin` (no un fetch completo). Los cambios actualizan la caché SQLite de GitM; los fallos emiten una advertencia y usan predeterminados en caché.
3. **Filtra** a repos sucios solo — los repos en su rama predeterminada detectada se muestran pero marcados como `⛔ rama protegida` y no pueden seleccionarse.
4. **Interfaz de selección múltiple** — selecciona qué repos quieres commit. _(Omitido cuando se proporciona `--repo` — todos los coincidentes sucios y sin protección proceden automáticamente.)_
5. Para cada repo seleccionado, **secuencialmente**:
   - **Selector de archivos** — muestra todos los archivos sucios con prefijos de estado codificados por color (amarillo `M`, verde `A`, rojo `D`, naranja `U` para conflictos, `??` tenue). Nada está preseleccionado.
   - **Entrada de mensaje de commit** — entrada de texto de una sola línea; rechaza mensajes vacíos.
   - `git add -- <archivos seleccionados>`
   - `git commit -m "<mensaje>"` (durante un merge, commit el índice completo para completar el merge)
   - `git push --set-upstream origin <branch>` (omitido con `--no-push`). Si el push se rechaza porque la rama remota avanzó (no fast-forward), gitm hace rebase automáticamente del nuevo commit sobre origin y reintentar — ver [`gitm push`](#gitm-push). Un conflicto de rebase se deja en su lugar y se reporta para que lo resuelvas y ejecutes `gitm push`.
   - Resultado en vivo impreso por repo.
6. **Resumen final** — `N commit, N omitidos, N fallidos`. Los fallos operativos producen un estado de salida distinto de cero después de procesar todos los repositorios seleccionados.

**Resolución de conflictos de merge:**

Si un repositorio está en medio de un merge (ej. después de que `git merge` produjo conflictos), `gitm commit` lo maneja automáticamente. Resuelve los archivos en conflicto, selecciónalos en el selector de archivos, y commit — gitm detecta el estado de merge y usa un commit de índice completo (sin pathspec) como git requiere.

**Protección contra rebase / cherry-pick / revert:**

Los repos con un rebase, cherry-pick o revert en progreso se **omiten** con un mensaje sugiriendo usar `git <operación> --continue` en su lugar.

**Ejemplo de flujo:**

```
Escaneando repositorios en busca de cambios sin commit…

Selecciona repositorios para hacer commit
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [ ] api-gateway       /home/user/work/api-gateway
▶ [ ] auth-service      /home/user/work/auth-service
  [ ] main-repo         /home/user/work/main-repo   ⛔ rama protegida

0/2 seleccionados
```

Después de seleccionar repos, para cada uno:

```
━━━ auth-service ━━━

Selecciona archivos para hacer stage en auth-service
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [✓] M  src/auth/login.go
  [ ] ?? scratch.txt

1/2 seleccionados

Mensaje de commit para auth-service
Escribe tu mensaje de commit  •  enter para confirmar  •  esc para cancelar

fix: corrección de verificación de expiración de token

  ✓ 1 archivo(s) en stage
  ✓ Commit: [feature/JIRA-456 3a4b5c6] fix: corrección de verificación de expiración de token
  ✓ Push
```

Resumen final:

```
Resumen
───────────────────────
  ✓  auth-service (commit + push)
  ~  api-gateway

1 commit  0 omitidos  0 fallidos
```

**Comportamiento de rama protegida:**

Los repos que están actualmente en su rama predeterminada detectada en vivo (ej. `main` o `master`) se muestran en gris en la lista de selección con una etiqueta `⛔ rama protegida` y **no pueden alternarse**. Esto previene commits directos accidentales a la rama predeterminada. Si la búsqueda remota falló, la advertencia aclara que la protección está usando el predeterminado en caché.

**Ejemplos:**

```bash
# Commit + push interactivo
gitm commit

# Commit interactivo, omitir push
gitm commit --no-push

# Commit solo en repos específicos por alias (sin prompt de selección)
gitm commit --repo api-gateway,auth-service --group backend

# Commit en repos específicos y omitir push
gitm commit --repo api-gateway --group backend --no-push
```

---

### `gitm push`

Empuja la rama actual de uno o más repositorios a origin **en paralelo**, recuperándose automáticamente cuando el remoto ha divergido. A diferencia de `gitm commit`, esto solo empuja lo que ya está comprometido — útil cuando existe un commit pero nunca se hizo push (después de `gitm commit --no-push`, o después de que un push se rechazó).

```
gitm push [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(ninguno)_ | Lista separada por comas de alias de repositorios para push. Omite la interfaz interactiva de selección múltiple. |
| `--all` | `-a` | false | Push cada repositorio registrado sin preguntar. |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Qué hace:**

1. **Selecciona** repositorios — selección múltiple interactiva por defecto, o todas las coincidencias cuando se da `--repo` / `--all`.
2. Para cada repo seleccionado, en paralelo:
   - Omite el repo cuando ya rastrea un remoto y no tiene nada nuevo para push (`nada que empujar — <branch> está actualizado`).
   - Ejecuta `git push --set-upstream origin <branch>`.
   - **Recuperación de remoto divergido:** si el push se rechaza porque la rama remota avanzó (un no-fast-forward `! [rejected] … (fetch first)`), gitm hace fetch de origin y ejecuta `git pull --rebase --autostash origin <branch>` para reproducir tus commits locales encima del remoto, luego reintenta el push una vez. El historial permanece lineal y nunca tienes que `cd` en el repo para hacer pull a mano.
   - Los fallos de autenticación, red, hook y configuración remota se devuelven directamente; nunca desencadenan un rebase.
3. **Resumen** — `N exitosos, N omitidos, N fallidos`.

**Conflictos de rebase:**

Si el rebase de recuperación choca con conflictos, el repositorio se **deja en su estado de rebase** (no abortado) y se reporta. Resuelve los conflictos, ejecuta `git rebase --continue`, luego vuelve a ejecutar `gitm push`.

**Ramas protegidas:** a diferencia de `gitm commit`, `gitm push` **no** protege la rama predeterminada — hacer push de un `main`/`master` que tiene commits locales es una operación válida.

**Ejemplo de salida:**

```
Haciendo push de la rama actual de 2 repositorio(s)…

[auth-service        ] ✓ el remoto había divergido — hecho rebase sobre origin/feature/JIRA-456 y push
[api-gateway         ] ⚠ OMITIDO: nada que empujar — main está actualizado

Hecho: 1 exitoso, 1 omitido
```

Cuando se deja atrás un conflicto de rebase:

```
1 repositorio(s) tiene conflictos de rebase pendientes para resolver:
  - auth-service (/home/user/work/auth-service)
      conflicto: src/auth/login.go

Resuelve los conflictos en cada repo, luego `git rebase --continue` y `gitm push` (o `git rebase --abort`).
```

**Ejemplos:**

```bash
# Push interactivo
gitm push

# Push cada repositorio registrado
gitm push --all

# Push en repos específicos por alias (sin prompt de selección)
gitm push --repo api-gateway,auth-service

# Push solo repos en un grupo
gitm push --group backend
```

---

### `gitm stash`

Gestiona stashes de git en repositorios seleccionados. Por defecto, una TUI interactiva de selección múltiple te permite elegir en qué repos operar. Usa `--repo` / `-r` para omitir la interfaz y apuntar a repos específicos por alias.

```
gitm stash [flags]
gitm stash apply [flags]
gitm stash pop [flags]
gitm stash list [flags]
```

**Banderas (todos los subcomandos):**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(todos los repos)_ | Limita a alias de repositorios específicos (separados por comas), omite la selección interactiva. |
| `--group` | `-g` | _(todos los repos)_ | Limita a repositorios en un grupo. Se combina con `--repo` como una intersección. |

#### `gitm stash` _(push)_

Escanea repos en busca de cambios sin commit (incluyendo archivos no rastreados), muestra solo repos sucios en la selección múltiple, luego ejecuta `git stash push --include-untracked` con un mensaje auto-generado en cada repo seleccionado en paralelo.

Las operaciones de stash push/apply/pop finalizan todos los repositorios seleccionados y salen con código distinto de cero cuando algún repositorio reporta un fallo operativo.

```
$ gitm stash

Escaneando repositorios en busca de cambios sin commit…

Selecciona repositorios para hacer stash
↑/↓ o j/k  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

▶ [✓] repo1          /home/user/work/repo1
  [ ] repo2          /home/user/work/repo2

1/2 seleccionados

Haciendo stash de cambios en 1 repositorio(s)…

[repo1                 ] ✓ stash (gitm stash en feature/JIRA-456)

Hecho: 1 exitoso
```

#### `gitm stash apply`

Escanea repos en busca de entradas de stash, muestra solo repos con stash en la selección múltiple, luego ejecuta `git stash apply` (mantiene el stash) en cada repo seleccionado en paralelo.

#### `gitm stash pop`

Igual que `apply`, pero ejecuta `git stash pop` — aplica y elimina la entrada de stash.

#### `gitm stash list`

Imprime una tabla de repos que tienen entradas de stash, con el conteo y el mensaje de stash superior.

```
$ gitm stash list

REPO          STASHES  STASH SUPERIOR
────────────────────────────────────────────────────
repo1          1        En feature/JIRA-456: gitm stash en feature/JIRA-456
repo2          2        En master: gitm stash en master

2 repositorio(s) con entradas de stash.
```

**Ejemplos:**

```bash
# Interactivo — seleccionar repos vía TUI
gitm stash

# Stash repos sucios de un grupo
gitm stash --group backend

# Stash en repos específicos por alias (sin prompt)
gitm stash -r api-gateway,auth-service --group backend

# Aplicar stash a un repo específico
gitm stash apply -r api-gateway --group backend

# Pop stash de repos específicos
gitm stash pop --repo=api-gateway,auth-service

# Listar entradas de stash para un repo específico
gitm stash list -r api-gateway --group backend
```

---

### `gitm reset`

Deshace los últimos N commits en repositorios seleccionados en tres modos seguros: soft, mixed (predeterminado), o hard. Perfecto para deshacer commits locales antes de push.

```
gitm reset [flags]
```

**Modos:**

| Modo | Efecto | Caso de uso |
|---|---|---|
| `--soft` | Mueve HEAD hacia atrás; **mantiene cambios en stage** y listos para volver a commit | Squash, corregir, u organizar commits antes de push |
| _(predeterminado, mixed)_ | Mueve HEAD hacia atrás; **quita de stage los cambios pero los mantiene en el working tree** | Deshacer commits y volver a hacer stage selectivamente |
| `--hard` | Mueve HEAD hacia atrás Y **descarta todos los cambios** irreversible | Descartar completamente commits y cambios no deseados |

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--commits` | — | 1 | Número de commits a deshacer (reset hacia atrás N commits) |
| `--soft` | — | false | Mantener cambios en stage después del reset |
| `--hard` | — | false | Descartar todos los cambios (irreversible) |
| `--dry-run` | — | false | Vista previa de operaciones de reset y force-push seleccionadas sin mover HEAD ni reescribir historial remoto. |
| `--repo` | `-r` | _(todos los repos)_ | Limita a alias de repositorios específicos (separados por comas), omite la selección interactiva. |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

> **⚠️ ADVERTENCIA:** `--hard` es irreversible. Los cambios descartados no pueden recuperarse. Solo úsalo cuando estés seguro.

**Comprobación previa:**

Antes de aplicar el reset:
1. Muestra una tabla resumen de:
   - Cada repositorio y los commits que se desharán (por hash + mensaje)
   - Qué commits ya están empujados a origin (⚠️ bandera roja)
2. Abre la interfaz interactiva de selección múltiple para elegir qué repos resetear
3. Con `--dry-run`, imprime el comando exacto `git reset --<modo> HEAD~N` para cada repo seleccionado y si aparecería un prompt de force-push, luego sale sin cambios.
4. Si algunos commits ya están empujados, se te pedirá una vez: **"¿Force-push para limpiar el historial remoto? [y/N]"**
   - Se usa `--force-with-lease` (la forma más segura) para reescribir el historial de forma segura
   - Solo se ofrece si eres dueño de esas ramas (cuidado: ¡las ramas compartidas romperán los clones de tus compañeros!)

**Comportamiento:**

1. Para cada repo seleccionado (en paralelo):
   - Mueve HEAD hacia atrás N commits
   - Aplica el modo de reset elegido (soft/mixed/hard)
   - Reporta qué commits se deshicieron

2. Si algunos commits deshechos ya estaban empujados:
   - Se te advierte con una caja de precaución roja
   - Se te pide una vez para todos los repos: aprobar u omitir el force-push
   - Si se aprueba: se usa `git push --force-with-lease` para reescribir el historial remoto

Los lotes de reset y force-push aprobados salen con código distinto de cero después de sus resúmenes cuando algún repositorio falla.

**Ejemplos:**

```bash
# Deshacer último commit, mantener cambios en stage (más seguro)
gitm reset --soft

# Deshacer último commit, quitar de stage cambios, mantener en working tree (predeterminado)
gitm reset

# Deshacer últimos 3 commits, quitar de stage cambios
gitm reset --commits 3

# Deshacer últimos 2 commits, descartar todos los cambios (IRREVERSIBLE)
gitm reset --hard --commits 2

# Vista previa de las implicaciones de hard reset y force-push primero
gitm reset --hard --commits 2 --dry-run

# Reset en repos específicos por alias (sin prompt de selección)
gitm reset -r api-gateway
gitm reset --soft -r api-gateway,auth-service --group backend

# Reset solo repos en un grupo
gitm reset --group backend
```

**Ejemplo de flujo:**

```
$ gitm reset --commits 2

Modo:  mixed  —  HEAD se mueve hacia atrás; los cambios se quitan de stage pero se mantienen en el working tree
Alcance:  últimos 2 commit(s) por repositorio

  api-gateway          [2 commit(s) ya empujados — el remoto necesitará force-push]
    ↩ a1b2c3d feat: agregar autenticación
    ↩ e4f5g6h fix: corrección de validación de token

Selecciona repositorios para resetear
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [✓] api-gateway        /home/user/work/api-gateway

1/1 seleccionado

Aplicando reset mixed (HEAD~2) a 1 repositorio(s)…

[api-gateway        ] ✓ reset mixed — deshizo 2 commit(s):
       ↩ a1b2c3d feat: agregar autenticación
       ↩ e4f5g6h fix: corrección de validación de token
       los cambios están sin stage pero presentes en el working tree

Hecho: 1 exitoso

┌──────────────────────────────────────────────────────┐
│  PRECAUCIÓN: Reescritura de historial remoto         │
│                                                      │
│  1 del/os repo(s) reseteado(s) tenía commit(s)       │
│  empujados ya deshechos. Force-push reescribirá el   │
│  historial de la rama remota. Cualquier persona que   │
│  ya haya hecho pull de esos commits necesitará       │
│  hard-reset su rama local. Solo haz esto en ramas     │
│  que poseas y nadie más esté usando.                  │
└──────────────────────────────────────────────────────┘

Los siguientes 1 repo(s) serán force-push:
  api-gateway        /home/user/work/api-gateway

¿Force-push para limpiar el historial remoto? [y/N] y

Haciendo force-push a 1 repositorio(s)…

[api-gateway        ] ✓ force-push rama master a origin

Hecho: 1 exitoso
```

---

### `gitm track`

Comienza a rastrear archivos no rastreados en múltiples repositorios. Solo los repositorios con archivos no rastreados se muestran en la lista de selección.

```
gitm track [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(todos los repos)_ | Limita a alias de repositorios específicos (separados por comas). |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Qué hace:**

1. Escanea todos los repositorios registrados en busca de archivos no rastreados.
2. Si **ninguno** tiene archivos no rastreados, imprime un mensaje y sale.
3. Muestra un resumen de cuántos archivos no rastreados tiene cada repo.
4. Abre la selección múltiple interactiva para elegir en qué repos rastrear archivos.
5. Para cada repo seleccionado, abre el selector de archivos mostrando solo archivos no rastreados.
6. Ejecuta `git add` en los archivos seleccionados.

El comando imprime el resumen completo, luego sale con código distinto de cero si el rastreo falló en algún repositorio seleccionado.

**Ejemplo de flujo:**

```
$ gitm track

2 repositorios con archivos no rastreados:

  api-gateway            3 archivo(s) no rastreados
  auth-service           1 archivo(s) no rastreados

Selecciona repositorios para rastrear archivos en
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

▶ [✓] api-gateway       /home/user/work/api-gateway
  [ ] auth-service       /home/user/work/auth-service

1/2 seleccionados

Selecciona archivos para rastrear en api-gateway
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [✓] ?? src/new-handler.go
  [✓] ?? src/new-handler_test.go
  [ ] ?? scratch.txt

2/3 seleccionados

[api-gateway        ] ✓ 2 archivo(s) rastreados

Hecho: 1 exitoso
```

**Ejemplos:**

```bash
# Interactivo — seleccionar repos y archivos
gitm track

# Rastrear archivos solo en repos específicos por alias
gitm track --repo api-gateway,auth-service --group backend

# Rastrear archivos en repos de un grupo
gitm track --group backend
```

---

### `gitm untrack`

Deja de rastrear archivos en múltiples repositorios. Los archivos se eliminan del índice de git pero **permanecen en el disco**. Esto es útil para archivos comprometidos por accidente como `.env`, logs, o artefactos de build.

```
gitm untrack [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(todos los repos)_ | Limita a alias de repositorios específicos (separados por comas). |
| `--group` | `-g` | _(todos los repos)_ | Limita candidatos a repositorios en un grupo. Se combina con `--repo` como una intersección. |
| `--path` | `-p` | _(todos los archivos)_ | Filtra archivos por patrón glob o prefijo de ruta (ej. `"*.env"`, `"public/"`). |

**Qué hace:**

1. Abre la selección múltiple interactiva para elegir en qué repos dejar de rastrear archivos.
2. Para cada repo seleccionado, muestra archivos rastreados en el selector de archivos (filtrados por `--path` si se proporciona).
3. Ejecuta `git rm --cached` en los archivos seleccionados — elimina solo del índice de git.
4. Los archivos permanecen en el disco sin tocar.

El comando imprime el resumen completo, luego sale con código distinto de cero si dejar de rastrear falló en algún repositorio seleccionado.

> **Consejo:** Después de dejar de rastrear un archivo, agrégalo a `.gitignore` para evitar que se rastree de nuevo.

**Ejemplo de flujo:**

```
$ gitm untrack

Selecciona repositorios para dejar de rastrear archivos en
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

▶ [✓] api-gateway       /home/user/work/api-gateway
  [ ] auth-service       /home/user/work/auth-service

1/1 seleccionado

Selecciona archivos para dejar de rastrear en api-gateway (los archivos permanecen en el disco)
↑/↓ o j/k para mover  •  espacio para alternar  •  a para seleccionar todo  •  enter para confirmar  •  q/esc para cancelar

  [ ] T  go.mod
  [ ] T  go.sum
  [✓] T  .env
  [✓] T  debug.log

2/15 seleccionados

[api-gateway        ] ✓ 2 archivo(s) dejaron de rastrearse

Hecho: 1 exitoso
```

**Ejemplos:**

```bash
# Interactivo — seleccionar repos y archivos
gitm untrack

# Dejar de rastrear archivos solo en repos específicos por alias
gitm untrack --repo api-gateway --group backend

# Filtrar para mostrar solo archivos .env
gitm untrack --path "*.env"

# Filtrar para mostrar solo archivos bajo public/
gitm untrack --path "public/"

# Combinar filtros de repo y ruta
gitm untrack --repo api-gateway --group backend --path "*.log"
```

---

### `gitm doctor`

Ejecuta diagnósticos de solo lectura de worktree en repositorios registrados y reporta problemas comunes de salud antes de que interrumpan un flujo de trabajo.

```
gitm doctor [flags]
```

**Banderas:**

| Bandera | Abrev. | Predeterminado | Descripción |
|---|---|---|---|
| `--repo` | `-r` | _(todos los repos)_ | Limita a alias de repositorios específicos (separados por comas). |
| `--group` | `-g` | _(todos los repos)_ | Limita diagnósticos a repositorios en un grupo. Se combina con `--repo` como una intersección. |

**Qué verifica:**

1. La ruta registrada aún existe y es un directorio.
2. La ruta sigue siendo la raíz de un repositorio de git.
3. La rama actual puede leerse y no está desconectada (detached).
4. La rama predeterminada detectada en vivo existe localmente.
5. Un remoto `origin` está configurado.
6. La rama actual tiene un upstream.
7. El working tree tiene cambios sin commit.
8. Una operación de merge, rebase, cherry-pick, revert o bisect está en progreso.

**Notas de comportamiento:**

- `OK` significa que el repositorio pasó todos los diagnósticos.
- `WARN` significa que el repositorio es usable pero puede necesitar atención, como un working tree sucio o upstream faltante.
- `ERROR` significa que el repositorio registrado está roto o no puede inspeccionarse, como una ruta faltante o directorio no-git.
- El comando sale con código distinto de cero solo cuando uno o más repositorios tienen estado `ERROR`.
- Antes de verificar, doctor realiza una búsqueda de red ligera de cada
  simbólico `HEAD` de `origin` (no un fetch completo). Un cambio actualiza la caché
  de rama predeterminada SQLite de GitM; un fallo emite una advertencia y usa el valor en caché.
- El repositorio permanece de solo lectura: doctor no hace fetch de refs remotos, pull,
  push, checkout, modifica archivos ni cambia metadatos de Git. Solo la caché SQLite
  de GitM puede actualizarse.

**Ejemplo de salida:**

```
$ gitm doctor

Verificando 3 repositorio(s) registrado(s)…

REPO                    ESTADO   DETALLES
──────────────────────────────────────────────────────────────────────────────────────────
api-gateway             OK       saludable
auth-service            WARN     working tree tiene cambios sin commit; rama actual no tiene upstream
old-worker              ERROR    ruta no accesible: stat /home/user/work/old-worker: no such file or directory
```

**Ejemplos:**

```bash
# Verificar cada repositorio registrado
gitm doctor

# Verificar solo repos específicos por alias
gitm doctor --repo api-gateway,auth-service --group backend

# Verificar un grupo
gitm doctor --group backend
```

---

### `gitm upgrade`

Auto-actualiza gitm al último lanzamiento desde GitHub para instalaciones manuales de macOS/Linux. Descarga el binario correcto para tu plataforma, verifica sus sumas de comprobación firmadas y reemplaza el binario actual — sin necesidad de descarga manual.

```
gitm upgrade
```

Si instalaste `gitm` con un administrador de paquetes, usa:

```bash
brew upgrade --cask gitm   # Homebrew (macOS)
scoop update gitm          # Scoop (Windows)
```

**Qué hace:**

1. Consulta la [API de GitHub Releases](https://github.com/alexandreafj/gitm/releases) para la última versión.
2. Compara contra la versión instalada actualmente (`gitm --version`).
3. Detecta tu SO y arquitectura para descargar el binario correcto.
4. Requiere y descarga el binario, `checksums.txt`, y `checksums.txt.bundle`.
5. Verifica el paquete de firma de Sigstore, luego verifica la suma de comprobación SHA-256 del binario. Activos de confianza faltantes o fallos de verificación abortan sin cambiar el ejecutable instalado.
6. Reemplaza átomamente el binario actual (hace backup del antiguo, intercambia el nuevo).
7. Establece permisos ejecutables en Linux/macOS (`chmod 755`).

Las descargas toleran conexiones lentas o proxied: el handshake TLS se da hasta 30s (el predeterminado de Go de 10s es demasiado agresivo para el CDN de activos de GitHub detrás de algunas redes), y los fallos transitorios — timeouts de handshake TLS, conexiones caídas y respuestas 5xx/429 — se reintentan hasta 3 veces con backoff exponencial antes de rendirse.

**Plataformas soportadas:**

| Plataforma | Binario |
|---|---|
| macOS (Apple Silicon) | `gitm-macos-arm64` |
| macOS (Intel) | `gitm-macos-x86_64` |
| Linux (x86_64) | `gitm-linux-amd64` |
| Linux (ARM64) | `gitm-linux-arm64` |
| Windows (x86_64) | `gitm-windows-amd64.exe` |

**Ejemplo — actualización disponible:**

```
$ gitm upgrade

Buscando actualizaciones... encontrado v1.1.0
Descargando gitm-macos-arm64... hecho
Verificando firma... ok
Verificando suma de comprobación... ok
Gitm actualizado: v1.0.6 → v1.1.0
```

**Ejemplo — ya actualizado:**

```
$ gitm upgrade

Buscando actualizaciones... ya actualizado (v1.1.0)
```

**Comprobación de versión:**

```bash
# Ver tu versión actual
gitm --version
```

> **Nota:** Este comando no requiere acceso a base de datos — funciona incluso si `~/.gitm/gitm.db` aún no existe.
>
> **Nota:** `gitm upgrade` está deshabilitado para instalaciones gestionadas por paquetes (Homebrew/Scoop) y deshabilitado en Windows.

---

## ¿Cómo funciona?

### Ejecución en paralelo

Cada operación multi-repo usa un grupo de trabajo concurrente (`golang.org/x/sync/errgroup`) con un límite de concurrencia predeterminado de **10 operaciones de git en paralelo**. Los resultados se transmiten a la terminal a medida que cada operación se completa, por lo que no esperas por un repositorio lento para ver los resultados de los demás.

### Optimizaciones de rendimiento

**`gitm status` está optimizado para velocidad:**
- Por defecto, **no hace fetch de origin**, lo que lo hace casi instantáneo (~2 segundos para 11 repos) porque solo lee el estado local de git y usa información de seguimiento remoto en caché.
- Usa la bandera `--fetch` si necesitas números precisos de adelante/atrás al segundo desde el remoto (requiere llamadas de red).

**¿Por qué importa esto:** Cuando verificas el estado de 20+ repos varias veces al día, quieres que sea rápido. El estado remoto en caché es lo suficientemente preciso para la mayoría de los flujos de trabajo diarios — solo necesitas `--fetch` cuando te preparas para fusionar o hacer push.

### Detección de rama predeterminada

`gitm` detecta la rama predeterminada autoritativa con
`git ls-remote --symref origin HEAD`. Esta es una búsqueda de red ligera del
simbólico `HEAD`, no un fetch completo; tiene un límite de 10 segundos y no puede
abrir un prompt de autenticación interactivo de Git.

Cuando se agrega un repositorio, un remoto no disponible cae en el estado local:

1. `git symbolic-ref refs/remotes/origin/HEAD`.
2. Una rama local llamada `main`.
3. Una rama local llamada `master`.
4. La rama actual `HEAD`.

El resultado se almacena en la base de datos SQLite de GitM. Los comandos sensibles al predeterminado
lo actualizan automáticamente: checkout predeterminado, sync implícito, creación de rama
sin `--from`, protección de eliminación de rama, protección de commit de repositorio sucio,
el panel de ramas, doctor, y lista de repos. `gitm update` solo lo hace para repositorios cuyo upstream desapareció y necesitan la
recuperación predeterminada. Las búsquedas se ejecutan concurrentemente; los valores cambiados se persisten secuencialmente.
Si una búsqueda falla, gitm emite una advertencia nombrando los repositorios afectados y
usa sus valores en caché. Las operaciones de dry-run usan el resultado en vivo en memoria pero
no lo persisten. Estas búsquedas pueden cambiar la caché SQLite de GitM, pero nunca hacen fetch
de refs remotos ni mutan el worktree o metadatos de Git de un repositorio.

### Omitir, nunca forzar

`gitm` **nunca** fuerza el reinicio o stash de tu trabajo. Si un repositorio tiene cambios sin commit cuando se intenta un checkout o pull, se **omite** y reporta en el resumen. Tu trabajo siempre está seguro.

---

## Almacenamiento de datos

gitm almacena la configuración de repositorios en una base de datos SQLite en:

```
~/.gitm/gitm.db
```

La base de datos se crea automáticamente en la primera ejecución. Cuando gitm abre la base de datos después de una actualización, aplica automáticamente las migraciones faltantes; los usuarios no ejecutan comandos de migración manualmente. El soporte de grupos crea el grupo integrado `all` y rellena cada repositorio existente en él. El soporte de contextos crea el contexto integrado `default` y mueve cada repositorio existente a él.

Contiene estas tablas:

```sql
CREATE TABLE repositories (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    name           TEXT     NOT NULL,               -- nombre de directorio auto-detectado
    alias          TEXT     NOT NULL UNIQUE,        -- nombre de visualización (controlado por usuario)
    path           TEXT     NOT NULL UNIQUE,        -- ruta absoluta
    default_branch TEXT     NOT NULL,               -- auto-detectado: main o master
    context_id     INTEGER REFERENCES contexts(id), -- el único contexto al que pertenece el repo
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE groups (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL UNIQUE,             -- incluye integrado "all"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE group_repositories (
    group_id      INTEGER NOT NULL,
    repository_id INTEGER NOT NULL,
    PRIMARY KEY (group_id, repository_id),
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE TABLE contexts (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL UNIQUE,             -- incluye integrado "default"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,                          -- ej. active_context_id
    value TEXT NOT NULL
);
```

Para hacer backup o migrar tu configuración:

```bash
# Backup
cp ~/.gitm/gitm.db ~/gitm-backup.db

# Mover a una nueva máquina: copia el binario y el archivo .db
scp ~/.gitm/gitm.db newmachine:~/.gitm/gitm.db
```

---

## Pruebas

### Ejecutar pruebas

```bash
# Ejecutar todas las pruebas con detección de race
make test

# Ejecutar pruebas verbose
go test ./... -v -race -timeout 180s

# Ejecutar pruebas de un paquete específico
go test ./internal/cli/... -v -race

# Ejecutar una sola prueba por nombre
go test ./internal/cli/... -v -race -run TestResetSoft
```

### Estadísticas de pruebas

| Métrica | Conteo |
|---|---|
| Archivos de prueba | 55 |
| Funciones de prueba | 510 |
| Lenguaje | Go |

---

## Desarrollo

### Estructura del proyecto

```
cli-git-commands/
├── cmd/
│   └── gitm/
│       └── main.go              # Punto de entrada
├── internal/
│   ├── cli/
│   │   ├── root.go              # Comando cobra raíz
│   │   ├── dry_run.go           # Ayudas compartidas de salida de vista previa dry-run
│   │   ├── default_branch.go     # Búsqueda predeterminada en vivo y conciliación de caché SQLite
│   │   ├── repo.go              # repo add/list/remove/rename
│   │   ├── group.go             # group list/show/create/rename/delete/add/remove
│   │   ├── context.go           # context list/show/current/create/use/rename/delete/add/remove
│   │   ├── checkout.go          # checkout master
│   │   ├── branch.go            # branch create/rename/delete
│   │   ├── status.go            # status
│   │   ├── branches.go          # branches (panel multi-repo de ramas)
│   │   ├── update.go            # update
│   │   ├── sync.go              # sync (fusionar rama predeterminada en rama actual)
│   │   ├── discard.go           # discard
│   │   ├── commit.go            # commit
│   │   ├── push.go              # push (con recuperación de rebase de remoto divergido)
│   │   ├── stash.go             # stash / stash apply / stash pop / stash list
│   │   ├── reset.go             # reset --soft / --hard con soporte de force-push
│   │   ├── track.go             # comenzar a rastrear archivos no rastreados
│   │   ├── untrack.go           # dejar de rastrear archivos (git rm --cached)
│   │   ├── doctor.go            # diagnósticos de salud de repositorio
│   │   └── upgrade.go           # auto-actualización desde lanzamientos de GitHub
│   ├── config/
│   │   └── config.go            # Configuración de app y directorio de datos
│   ├── db/
│   │   ├── db.go                # Conexión SQLite y migraciones
│   │   ├── group.go             # CRUD de grupos de repositorios y membresías
│   │   ├── context.go           # CRUD de contextos de repositorios y estado de contexto activo
│   │   └── repository.go        # CRUD de repositorios
│   ├── git/
│   │   └── git.go               # Operaciones de Git
│   ├── runner/
│   │   └── parallel.go          # Motor de ejecución paralela
│   └── tui/
│       ├── multiselect.go       # Interfaz TUI de selección múltiple bubbletea (con soporte de items deshabilitados)
│       ├── fileselect.go        # Interfaz selector de archivos (estado porcelain, codificado por color)
│       └── textinput.go         # Entrada de mensaje de commit de una sola línea
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

### Objetivos de build

```bash
make build    # Compilar a ./bin/gitm
make install  # Instalar en GOPATH/bin
make test     # Ejecutar todas las pruebas con detector de race
make lint     # Ejecutar go vet + staticcheck
make clean    # Eliminar ./bin/
make tidy     # Ordenar go.mod
make help     # Mostrar todos los objetivos
```

### Agregar un nuevo comando

1. Crea una rama de características: `git checkout -b feat/<nombre-comando>`.
2. Crea `internal/cli/<comando>.go`.
3. Define una función `func <comando>Cmd() *cobra.Command`.
4. Regístrala en `internal/cli/root.go` agregando `root.AddCommand(<comando>Cmd())`.
5. Si el comando no necesita acceso a DB, agrega su nombre a la lista de omitidos en `PersistentPreRunE`.
6. Crea `internal/cli/<comando>_test.go` con pruebas unitarias (repos git reales, sin mocks).
7. Actualiza este `README.md`: agrega a Tabla de Contenidos, Referencia de Comandos, y Estructura del Proyecto.
8. Ejecuta `make test && make lint` antes de commit.

### Dependencias

| Paquete | Versión | Propósito |
|---|---|---|
| `github.com/spf13/cobra` | v1.x | Marco CLI |
| `modernc.org/sqlite` | v1.x | SQLite puro Go (sin CGO) |
| `github.com/charmbracelet/bubbletea` | v1.x | Marco TUI |
| `github.com/charmbracelet/bubbles` | v1.x | Componentes TUI |
| `github.com/charmbracelet/lipgloss` | v1.x | Estilizado de terminal |
| `github.com/fatih/color` | v1.x | Salida coloreada |
| `golang.org/x/sync` | última | `errgroup` para ops paralelas |

---

## Contribuir

Ver [`AGENTS.md`](./AGENTS.md) para el flujo de desarrollo, estándares de código y la (corta) lista de dependencias aprobadas. Brevemente: rama de características desde `master`, git real en pruebas (sin mocks), `make lint && make test` antes de push.

## Licencia

[MIT](./LICENSE) — ver el archivo LICENSE para detalles.
