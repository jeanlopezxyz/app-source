# app-source · catalogo

Código y CI de **catalogo**, la aplicación de ejemplo del curso **Argo CD de cero a producción**. Un catálogo de productos con tres piezas: un frontend estático, una API en Go y una base de datos PostgreSQL gestionada por [CloudNativePG](https://cloudnative-pg.io/).

## Relación con el curso

El curso usa dos repositorios, separados a propósito:

| Repositorio | Contenido | Quién lo cambia |
|---|---|---|
| [`app-source`](https://github.com/jeanlopezxyz/app-source) (este) | Código, `Containerfile` y CI que publica las imágenes | Los desarrolladores, con commits y tags |
| [`app-gitops`](https://github.com/jeanlopezxyz/app-gitops) | Manifiestos que sincroniza Argo CD | El equipo de plataforma y, desde la sesión 8, el CI |

El CI de este repo **no despliega**: publica imágenes con versión. Argo CD despliega lo que dice `app-gitops`. En la sesión 8 el CI da un paso más y actualiza la imagen en `app-gitops` con un commit ([Automation from CI Pipelines](https://argo-cd.readthedocs.io/en/stable/user-guide/ci_automation/)).

## Piezas

| Pieza | Imagen | Detalle |
|---|---|---|
| `api/` | `ghcr.io/jeanlopezxyz/catalogo-api:<versión>` | Go (biblioteca estándar y [pgx](https://github.com/jackc/pgx)). HTTP en el puerto 8080. Imagen final distroless, usuario 65532 |
| `frontend/` | `ghcr.io/jeanlopezxyz/catalogo-frontend:<versión>` | HTML, CSS y JS servidos por `nginxinc/nginx-unprivileged` en el puerto 8080, usuario 101 |
| PostgreSQL | CloudNativePG | No se construye aquí: el operador y el `Cluster` viven en `app-gitops` |

### API

| Ruta | Uso |
|---|---|
| `GET /healthz` | Liveness: el proceso responde. No mira la base de datos |
| `GET /readyz` | Readiness: `200` si la base de datos responde a `SELECT 1`, `503` si no |
| `GET /api/products` | Productos en JSON |
| `GET /api/version` | `{"version","commit","color"}`. El color depende de la versión |
| `GET /metrics` | Formato Prometheus: `catalogo_http_requests_total` (contador), `catalogo_http_request_duration_seconds` (histograma) y `catalogo_build_info` |

Subcomandos: `serve` (por defecto), `migrate` (crea o actualiza el esquema e inserta los productos de ejemplo; es idempotente y usa un advisory lock, pensado para el hook `PreSync` de la sesión 4) y `version`.

Configuración por variables de entorno:

| Variable | Por defecto | Detalle |
|---|---|---|
| `PORT` | `8080` | Puerto HTTP |
| `DB_HOST` | `localhost` | Con CloudNativePG, el servicio `<cluster>-rw` |
| `DB_PORT` | `5432` | |
| `DB_NAME` | `catalogo` | |
| `DB_USER` | `catalogo` | |
| `DB_SSLMODE` | `prefer` | Modo TLS de libpq |
| `DB_PASSWORD_FILE` | `/etc/catalogo/db/password` | Archivo con la contraseña |

**La contraseña se lee del archivo en cada conexión nueva**, nunca de una variable de entorno. Si el Secret se monta como volumen (sin `subPath`), el kubelet actualiza el archivo al rotarlo y las conexiones nuevas usan la contraseña nueva sin reiniciar el pod ([Secrets como archivos](https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets-as-files-from-a-pod)). Las conexiones abiertas siguen vivas, porque PostgreSQL solo valida la contraseña al conectar. Es el cierre del laboratorio 3.

### Frontend

Muestra los productos, la versión de la API y la suya, y pinta la cabecera con el color que devuelve `/api/version`. Además consulta `/api/version` cada 2 segundos y dibuja un punto por respuesta: durante el canary de la sesión 8 se ven los dos colores mezclados. Llama a `/api` en el mismo host, así que necesita un Ingress que envíe `/api` a la API y `/` al frontend.

## Versiones

Las publica el CI al empujar un tag `vX.Y.Z`; la imagen se etiqueta `X.Y.Z` (sin `v`).

| Versión | Color | Cambios |
|---|---|---|
| `1.0.0` | Azul `#2563eb` | Primera versión |
| `1.1.0` | Verde `#16a34a` | Columna `stock` en los productos. La migración la añade como columna opcional, así la API `1.0.0` sigue funcionando mientras conviven las dos versiones durante el canary |

## Construir y probar en local

Requisitos: Go 1.27 y Docker o Podman.

```bash
# Tests
cd api && go vet ./... && go test ./...

# Imágenes
docker build -f api/Containerfile --build-arg VERSION=1.0.0 -t catalogo-api:1.0.0 api
docker build -f frontend/Containerfile --build-arg VERSION=1.0.0 -t catalogo-frontend:1.0.0 frontend
```

Prueba rápida con un PostgreSQL de usar y tirar (la contraseña se genera en el momento y solo vive en un archivo temporal):

```bash
dir=$(mktemp -d) && chmod 755 "$dir" && openssl rand -hex 16 > "$dir/password" && chmod 644 "$dir/password"
docker network create catalogo
docker run -d --rm --name catalogo-db --network catalogo \
  -e POSTGRES_USER=catalogo -e POSTGRES_DB=catalogo \
  -e POSTGRES_PASSWORD_FILE=/run/pw/password -v "$dir:/run/pw:ro" postgres:18-alpine
docker run --rm --network catalogo -e DB_HOST=catalogo-db -v "$dir:/etc/catalogo/db:ro" catalogo-api:1.0.0 migrate
docker run -d --rm --name catalogo-api --network catalogo -p 8080:8080 \
  -e DB_HOST=catalogo-db -v "$dir:/etc/catalogo/db:ro" catalogo-api:1.0.0
curl -s localhost:8080/api/products
docker rm -f catalogo-api catalogo-db && docker network rm catalogo && rm -rf "$dir"
```

## CI

[`.github/workflows/ci.yaml`](.github/workflows/ci.yaml):

- **Push a `main` y pull requests:** `go vet` y `go test -race`.
- **Tag `vX.Y.Z`:** tests y, si pasan, imágenes `linux/amd64` y `linux/arm64` publicadas en GHCR con `GITHUB_TOKEN`. Las imágenes llevan `org.opencontainers.image.source`, que enlaza el paquete con este repo.
- **`update-gitops` (sesión 8, desactivado):** actualiza la imagen en `apps/catalogo/overlays/dev` de `app-gitops` con `kustomize edit set image` y un commit. Se activa con la variable de repositorio `UPDATE_GITOPS=true` y el secret `GITOPS_DEPLOY_KEY`: la clave privada de una deploy key con escritura solo en `app-gitops`. Variables opcionales: `GITOPS_REPO` (`<dueño>/<repo>`, por defecto `<owner>/app-gitops`) y `BASE_IMAGE` (el prefijo de las imágenes en la base de `app-gitops`, por defecto `ghcr.io/jeanlopezxyz`).

Publicar una versión:

```bash
git tag v1.0.0 && git push origin v1.0.0
```

## Licencia

[Apache-2.0](LICENSE)
