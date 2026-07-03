# 03 — Invariantes y reglas de negocio

Reglas que **siempre** deben valer. Cada una declara **dónde se enforcea** y **qué
pasa si se viola**. Regla de oro: enforzar en la capa más baja posible (tipo > DB
constraint > código), y **nunca** duplicar la misma regla en front y back — se
define una vez y se deriva.

## Jerarquía de enforcement

1. **Tipo / NOT NULL** — el estado ilegal no se puede ni representar.
2. **DB constraint** (CHECK, UNIQUE, FK con acción) — el motor lo rechaza.
3. **Código** (handler / servicio) — solo para reglas que la DB no puede expresar.

El front puede validar **por UX** (feedback rápido), pero nunca es la fuente de
verdad: aunque el front no valide, el back + DB deben rechazar el estado ilegal.

---

## Invariantes de atributo (dominio)

| # | Regla | Enforce | Si se viola |
|---|---|---|---|
| A1 | Todo numérico `≥ 0` | CHECK `col >= 0` + tipo | DB rechaza (400) |
| A2 | `0` es un valor **válido**, no "faltante" | **Ausencia de `binding:"required"`** en floats; `NOT NULL` expresa obligatoriedad | — (era la causa del bug de auto-ligado) |
| A3 | Nombres no vacíos | CHECK `length(trim(name)) > 0` | DB/handler rechaza (400) |
| A4 | Nombre único por escenario | UNIQUE `(scenario_id, name)` | DB rechaza (409) |
| A5 | `variety_flag` ∈ {0,1} (W, `@BIN`); `batch_active` entero ≥0 (Y, `@GIN`, nº de lotes) | CHECK `>= 0` | DB rechaza |

> **A2 es la regla más importante de este documento.** Su ausencia causó el 400
> del auto-ligado. `binding:"required"` en un `float64` rechaza el cero de Go.
> Obligatoriedad = `NOT NULL`, no = "distinto de cero".

---

## Invariantes multi-entidad

| # | Regla | Enforce | Si se viola |
|---|---|---|---|
| M1 | `Product.max_batch ≥ Product.min_batch` | CHECK `max_batch >= min_batch` | DB rechaza (400) |
| M2 | Celda de receta: producto e insumo/máquina/opres del **mismo escenario** | **FK compuesta** con `scenario_id` en la clave referenciada | DB rechaza |
| M3 | Escenario `frozen` es **inmutable** (ni él ni sus hijos aceptan escritura) | Código: guard en toda mutación que verifica `status != frozen`; editar un frozen **forkea** | Handler rechaza (409) |
| M4 | Optimizar exige ≥ 1 producto con receta | Código en el builder del modelo | Error explícito `"no hay productos configurados"`, no un modelo degenerado |
| M5 | Identidad con historia **se archiva, nunca se borra** | FK sin `RESTRICT`; borrado de identidad = `status=archived` | — |
| M6 | Una corrida referencia un escenario `frozen` (nunca uno `draft` mutando) | Código: al encolar, el escenario pasa a `frozen` primero | — |

---

## Invariantes de ciclo de vida

| # | Regla | Enforce |
|---|---|---|
| L1 | `Scenario.status`: `draft → frozen → archived`. No hay vuelta atrás desde `frozen` (se forkea) | Código (máquina de estados) |
| L2 | Borrar entidad de dominio en un `draft` cascadea su receta; **nunca** bloquea | FK `ON DELETE CASCADE` hacia la entidad |
| L3 | Archivar escenario → sus optimizaciones `scenario_id → SET NULL` (la corrida sobrevive, autocontenida) | FK `ON DELETE SET NULL` |
| L4 | Borrar producto referenciado en resultados → `product_id → SET NULL`; se conservan `product_name` y `canonical_product_id` | FK `ON DELETE SET NULL` + denormalización |
| L5 | Resultado es **autocontenido**: legible sin joins al catálogo vivo | Denormalización de `product_name` en `OptimizationResult` |

---

## Errores → códigos HTTP (contrato)

Para que el front reaccione consistente, cada violación mapea a un código y un
`error` legible. El front **muestra el `error` del backend**, no un genérico.

| Situación | Código | `error` ejemplo |
|---|---|---|
| Dominio inválido (negativo, nombre vacío, max<min) | 400 | `"max_batch debe ser ≥ min_batch"` |
| Nombre duplicado en escenario | 409 | `"Ya existe un producto con ese nombre en el escenario"` |
| Escritura a escenario `frozen` | 409 | `"El escenario está congelado; forkealo para editar"` |
| Optimizar sin productos con receta | 422 | `"No hay productos configurados para optimizar"` |
| Recurso no encontrado | 404 | `"Producto no encontrado"` |
| Sin/mala API key | 401 | `"API key inválida o ausente"` |

> Nota: el borrado **ya no produce 409 "en uso"** — con archivado + cascade de
> receta la operación siempre procede o archiva. Ver `02-data-dictionary.md`.
