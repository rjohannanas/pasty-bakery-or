# lingo-backend

API Go (Gin + GORM/Postgres + Redis) para optimización de producción de una panadería. Un job asíncrono arma un modelo matemático, corre el solver LINGO, y guarda resultados. Frontend separado: `pasty-bakery-front`, corre en otra máquina (`pasty-front`, 192.168.1.104), ver su propio `CLAUDE.md`.

**Docs autoritativos (contract-first):** `docs/` es la fuente de verdad; el código deriva de ahí, no al revés. Ver `docs/02-data-dictionary.md` (entidades), `docs/03-invariants.md` (reglas), `docs/openapi.yaml` (contrato de API, servido en `GET /openapi.yaml`) y `docs/adr/` (decisiones). Cuando código y doc difieran, manda el doc.

## Arquitectura

- `cmd/api/main.go` — entrypoint, registra todas las rutas Gin.
- `cmd/admin/main.go` — CLI aparte, solo inspecciona/cancela jobs en Redis (`admin list|queue|cancel`). No toca Postgres, no crea datos maestros.
- `internal/handlers/` — un archivo por recurso (scenarios, products, ingredients, machines, operational_resources, optimize, logs, ws).
- `internal/models/models.go` — todos los modelos GORM. Los nombres coinciden con las variables del modelo LINGO (P, D, LI, LS, CU, IN, CAP, DISP, CR, Q, T, CM, X, Y, W — ver comentarios en el archivo).
- `internal/db/postgres.go` — `Connect` corre AutoMigrate y **deriva los invariantes de `docs/03` a la DB**: `ensureDomainChecks` (CHECK de ≥0, nombre no vacío, `max_batch ≥ min_batch`) y `ensureRecipeCompositeFK` (FK compuesta `(scenario_id, *_id)` que garantiza que una celda de receta y su entidad son del mismo escenario — invariante M2).
- `internal/solver/lingo.go` — `BuildModel` arma el `.lng`, `RunLINGO` ejecuta el binario LINGO (`LINGO_PATH` en `.env`), `ParseOutput` parsea resultados. **`CAP` (`capacity_minutes`) y `T` (`minutes_per_unit`) están ambos en minutos — sin conversión.**
- `internal/worker/worker.go` — loop infinito que consume jobs de Redis, corre el solver, guarda `OptimizationResult` por producto, notifica por WS.
- `internal/queue/redis.go` — cola simple: `lingo:jobs` (lista), `lingo:status:<jobID>`, `lingo:retries:<jobID>`.
- `internal/ws/hub.go` — WS hub con ping/pong y read pump. Broadcast global (sin filtrar por cliente), asumido OK porque solo hay 1-2 usuarios.
- `internal/middleware/apikey.go` — gate simple por header `X-API-Key` (HTTP) / query `api_key` (WS).

## Modelo de negocio: escenarios instanciados + fork

El negocio es una sola panadería (dueño + 1 trabajador, sin roles), pero el modelo de datos gira en torno al **what-if**: un `Scenario` es un plan completo y editable (unidad clonable). Cada entidad de dominio (`Product`, `Ingredient`, `Machine`, `OperationalResource`) **pertenece a un escenario** y guarda sus propios parámetros — no hay tablas globales ni singleton. Ver ADR 0003 (supersede al viejo singleton Stock/Resource de ADR 0002).

- **Ciclo de vida** (`Scenario.Status`): `draft` (editable) → `frozen` (ya optimizado, inmutable) → `archived` (oculto, conserva historia). No hay vuelta atrás desde `frozen`: para editar un frozen se **clona/forkea** (`POST /scenarios/:id/clone`, `parent_id` = origen).
- **Identidad cruzada:** al clonar, cada entidad hereda `canonical_id` (raíz) para comparar "el mismo producto" entre escenarios.
- **Archivar, no borrar:** una identidad con historia se archiva (ADR 0004). Borrar un `draft` sin corridas sí lo elimina; borrar uno con corridas lo archiva y desvincula las corridas (`scenario_id → SET NULL`, la corrida queda autocontenida).
- **Resultado autocontenido:** `OptimizationResult` denormaliza `product_name` para que un plan histórico se lea aunque el producto se archive.

## Qué necesita `POST /scenarios/:id/optimize` para no ser degenerado

Congela el escenario (`draft → frozen`) y encola la corrida. `BuildModel` no falla si faltan datos (arma un modelo con ceros), por eso el handler exige antes:

1. Al menos 1 `Product` con receta (`ProductIngredient`) en el escenario — si no, responde **422** `"No hay productos configurados para optimizar"` (invariante M4).
2. Para un modelo útil: los productos con sus matrices Q/T/CM (`ProductIngredient`/`ProductMachine`/`ProductOperationalResource`), los `Ingredient` con `stock_available` (IN), las `Machine` con `capacity_minutes` (CAP) y los `OperationalResource` con `available`/`cost_per_unit` (DISP/CR).
3. El escenario lleva `max_production` (M) y `min_variety` (PRO); el body de optimize puede overridearlos.

No hay corridas duplicadas simultáneas por escenario (el handler lo rechaza con 409).

## Auth

`APP_API_KEY` en `.env` — si está vacía, la API queda abierta (con warning en log). Es un filtro anti-scraping casual, **no un secreto fuerte**: el front la expone en su bundle JS público (`NEXT_PUBLIC_API_KEY`). No hay usuarios/roles/login — decisión explícita (ADR 0001), negocio de 2 personas de confianza.

## Deploy

- Corre supervisado por **systemd** (`lingo-api.service`, instalado en `/etc/systemd/system/`, `enabled` → arranca en boot, `Restart=always`/`RestartSec=5`, `WorkingDirectory` = este repo, `User=dulcinea`). Es el único jefe del proceso: **NO** lo arranques a mano con `./lingo-api &` — dos supervisores pelean el puerto :8080 y el que pierde entra en crash-loop llenando el log. Operar siempre por systemd:
  - Reiniciar (tras recompilar): `go build -o lingo-api ./cmd/api && sudo systemctl restart lingo-api`.
  - Estado / logs: `systemctl status lingo-api` · `journalctl -u lingo-api -f` (los logs van a **journald**, no a `logs/backend.log` — ese archivo quedó del viejo modelo manual, está congelado).
- Expuesto a internet vía Cloudflare Tunnel (`cloudflared.service`, sí instalado como servicio de sistema). Config real en `/etc/cloudflared/config.yml` (NO en `~/.cloudflared/config.yml`, que existe pero no lo usa el servicio — cuidado con editar el archivo equivocado). `cloudflared-config.yml` en este repo es solo copia de referencia/documentación.
- Postgres + Redis vía `docker-compose.yml`, puertos atados a `127.0.0.1` (no exponer a la LAN sin querer). Credenciales via `.env` (`POSTGRES_USER/PASSWORD/DB`), no hardcodeadas en el yml.
- Password de Postgres sigue siendo la original de dev (`secret` en `.env`, ya no en git) — pendiente rotarla cuando haya tiempo (requiere recrear el volumen o `ALTER USER` desde `psql`, no es solo cambiar el `.env`).

## Convenciones del código

- Handlers usan structs anónimos `input struct{...}` con punteros (`*float64`, `*string`) para el bind del body, no bindean directo contra `models.X`.
- **Invariante A2 (crítico):** en un float, **NO** usar `binding:"required"` — rechaza el `0` legítimo (fue la causa del bug de auto-ligado). La obligatoriedad se expresa con `NOT NULL`, no con "distinto de cero". Los inputs numéricos son punteros: `nil` = ausente, `0` = valor válido.
- Toda mutación de dominio pasa por `requireDraft` (guard de escenario `frozen`/`archived` → 409). Editar un frozen se resuelve forkeando, no destrabando.
- Los invariantes se enforzan en la capa más baja posible (tipo > CHECK/FK en DB > código); nunca se duplica la misma regla en front y back. Ver `docs/03-invariants.md`.
- Patrón upsert de celda de receta (Q/T/CM): `First` por `(product_id, sub_id)`, si `gorm.ErrRecordNotFound` se crea, si no se actualiza y `Save`.
- Nombres de rutas anidadas siguen `/scenarios/:scenario_id/<recurso>/:sub_id` (ej. `/scenarios/:scenario_id/products/:product_id/machines/:machine_id`), no uses otro patrón para nuevos sub-recursos.
