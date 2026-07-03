# 04 — Proceso de optimización

Cómo fluye una corrida y cómo evoluciona un escenario. Este doc describe el
**proceso** (la dinámica); el diccionario describe la **estructura** (los datos).

## Ciclo de vida del escenario

```
        crear / clonar / fork
                 │
                 ▼
            ┌─────────┐   optimizar    ┌──────────┐
            │  draft  │ ─────────────▶ │  frozen  │
            │ editable│                │ inmutable│
            └─────────┘                └──────────┘
                 ▲                           │
                 │  editar un frozen         │  archivar
                 │  = fork a nuevo draft     ▼
                 └───────────────────    ┌──────────┐
                                         │ archived │
                                         └──────────┘
```

- **draft** — editable. Se cargan/ajustan productos, recetas, precios, stock, params.
- **frozen** — se alcanza al optimizar. Inmutable: garantiza que la corrida es
  reproducible (el escenario congelado **es** la foto de los inputs). Cualquier
  intento de edición devuelve 409 y sugiere forkear.
- **archived** — oculto de listados normales; conserva su id e historia.

**Fork:** editar un `frozen` crea un `draft` nuevo con `parent_id` = el frozen y
copia de todas sus filas (cada `canonical_id` se hereda). Es el "modificar el plan
que tenía antes".

## Flujo de una corrida

```
POST /optimize {scenario_id}
   │  1. valida escenario (existe, tiene ≥1 producto con receta → si no, 422)
   │  2. escenario draft → frozen  (invariante M6)
   │  3. crea Optimization(status=pending), encola job en Redis
   ▼
Redis  lingo:jobs (lista)          lingo:status:<jobID>     lingo:retries:<jobID>
   │
   ▼
Worker (loop infinito)
   │  4. BLPOP job → status=processing → broadcast WS
   │  5. BuildModel(escenario frozen) → arma el .lng
   │  6. RunLINGO(binario externo) → output crudo  (+ guarda LingoLog)
   │  7. ParseOutput → variables X/Y/W por producto
   │  8. Transacción: guarda OptimizationResult por producto
   │  9. status=done, total_profit → broadcast WS
   ▼
Front (WebSocket)  recibe {"job_id":"...","status":"..."} en cada transición
   │  GET /results/:id  →  plan de producción
```

## Estados del job (WebSocket)

El worker emite un broadcast global en cada transición. Mensaje:

```json
{ "job_id": "uuid", "status": "processing|done|error|cancelled" }
```

| status | Significa |
|---|---|
| `pending` | encolado, sin tomar |
| `processing` | worker corriendo el solver |
| `done` | resultados guardados; `GET /results/:id` disponible |
| `error` | falló build/solver/parse; ver `LingoLog` |
| `cancelled` | cancelado vía admin CLI |

Broadcast es global (sin filtrar por cliente): asumido OK porque hay 1-2 usuarios.

## Cola (Redis)

| Clave | Tipo | Rol |
|---|---|---|
| `lingo:jobs` | lista | cola FIFO de jobs pendientes |
| `lingo:status:<jobID>` | string | último estado conocido |
| `lingo:retries:<jobID>` | string | contador de reintentos |

**Recuperación de huérfanos:** al arrancar, el worker busca jobs que quedaron en
`processing` (server caído a mitad) y los reencola o marca según reintentos.

## Errores y degradación

- Sin productos con receta → **error explícito** (invariante M4), nunca un modelo
  de coeficientes en cero que "corre" pero no significa nada.
- Falla del binario LINGO → `status=error`, se guarda el `LingoLog` con el modelo
  generado y el output para diagnóstico.
- El `LingoLog` se guarda **pase lo que pase** (éxito o error) para auditar.

## Reproducibilidad

Como el escenario se congela antes de correr y es inmutable, `GET /results/:id`
siempre puede mostrar **con qué datos** se calculó ese plan: se leen del escenario
`frozen` asociado. No hace falta el `input_snapshot` JSON del modelo singleton
(se elimina en la migración).
