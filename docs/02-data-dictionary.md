# 02 — Diccionario de datos

Autoritativo. Define **entidades**, **atributos** (con su dominio e invariantes)
y **relaciones** (con su regla de ciclo de vida). El código físico
(`internal/models/`, migraciones) se deriva de acá.

## Modelo objetivo (right-sized)

**Escenarios instanciados + fork + identidad archivable.** Cada entidad de dominio
pertenece a un escenario y guarda sus propios parámetros. El escenario es la
unidad clonable del what-if. La identidad se **archiva, nunca se borra**.

### Las tres capas

```
ESCENARIO (contenedor / un "plan")
  Scenario

DOMINIO (instanciado por escenario — cada fila lleva scenario_id)
  Product · Ingredient · Machine · OperationalResource
  ProductIngredient · ProductMachine · ProductOperationalResource   (recetas Q/T/CM)

CORRIDA (ejecución sobre un escenario congelado)
  Optimization → OptimizationResult
  LingoLog
```

## Convenciones de dominio

- **Todo atributo numérico es `≥ 0` y `0 es un valor VÁLIDO`** (no "faltante").
  Esta regla es explícita porque su ausencia causó bugs: nunca usar
  `binding:"required"` en un float — un lote de 0, una celda de receta en 0 o un
  insumo sin stock son legítimos. "Requerido" se expresa con `NOT NULL`, no con
  rechazo del cero.
- **Nombres:** no vacíos. Únicos dentro de su escenario.
- **IDs:** surrogate `uint` autoincremental. `0` = ausente (inválido como FK).

---

## Scenario

Un plan completo y editable. Unidad de clonación del what-if.

| Atributo | Tipo | Dominio / invariante | Nota |
|---|---|---|---|
| id | uint | PK | |
| name | string | no vacío, único entre no-archivados | |
| notes | string | opcional | |
| status | enum | `draft` \| `frozen` \| `archived` | `draft`=editable; `frozen`=ya optimizado, inmutable; `archived`=oculto |
| is_base | bool | default false | el plan "canónico" del día a día |
| parent_id | *uint | FK→Scenario, nullable | de qué escenario se forkeó |
| max_production | float | ≥ 0, default 200 | M |
| min_variety | int | ≥ 0, default 7 | PRO |
| created_at / updated_at | time | | |

**Ciclo de vida:**
- `draft` → editable libremente.
- Al optimizarse pasa a `frozen` → **inmutable** (ninguna escritura a él ni a sus
  hijos). Editar un `frozen` **forkea** un nuevo `draft` (`parent_id` = el frozen).
- No se borra un escenario con historia: se `archived`.

---

## Entidades de dominio (instanciadas)

Las cuatro comparten el patrón: `scenario_id` (dueño), `canonical_id` (identidad
cruzada para comparar entre escenarios), nombre único por escenario, y sus
parámetros. `canonical_id` apunta a la fila "origen" al forkear; permite
`GROUP BY canonical_id` para comparar el mismo producto entre planes.

### Product

| Atributo | Tipo | Dominio / invariante | LINGO |
|---|---|---|---|
| id | uint | PK | |
| scenario_id | uint | FK→Scenario, NOT NULL | |
| canonical_id | *uint | FK→Product, nullable | identidad cruzada |
| name | string | no vacío, único por (scenario_id) | |
| sale_price | float | ≥ 0 | P |
| demand | float | ≥ 0 | D |
| min_batch | float | ≥ 0 | LI |
| max_batch | float | ≥ 0, **≥ min_batch** | LS |

### Ingredient

| Atributo | Tipo | Dominio / invariante | LINGO |
|---|---|---|---|
| id | uint | PK | |
| scenario_id | uint | FK→Scenario, NOT NULL | |
| canonical_id | *uint | FK→Ingredient, nullable | |
| name | string | no vacío, único por (scenario_id) | |
| unit | string | no vacío (kg, l, u) | |
| unit_cost | float | ≥ 0 | CU |
| stock_available | float | ≥ 0 | IN |

### Machine

| Atributo | Tipo | Dominio / invariante | LINGO |
|---|---|---|---|
| id | uint | PK | |
| scenario_id | uint | FK→Scenario, NOT NULL | |
| canonical_id | *uint | FK→Machine, nullable | |
| name | string | no vacío, único por (scenario_id) | |
| capacity_minutes | float | ≥ 0 | CAP (minutos; misma unidad que T) |

### OperationalResource

| Atributo | Tipo | Dominio / invariante | LINGO |
|---|---|---|---|
| id | uint | PK | |
| scenario_id | uint | FK→Scenario, NOT NULL | |
| canonical_id | *uint | FK→OperationalResource, nullable | |
| name | string | no vacío, único por (scenario_id) | |
| available | float | ≥ 0 | DISP |
| cost_per_unit | float | ≥ 0 | CR |

---

## Recetas (matrices Q / T / CM)

Join producto↔recurso **dentro del mismo escenario**. Llevan `scenario_id`
explícito para poder garantizar por **FK compuesta** que ambos lados son del mismo
escenario (`(scenario_id, product_id)` y `(scenario_id, X_id)` como claves
referenciadas). Sin eso la integridad "mismo escenario" quedaría solo en código.

### ProductIngredient — Q(I,J)

| Atributo | Tipo | Dominio | LINGO |
|---|---|---|---|
| id | uint | PK | |
| scenario_id | uint | NOT NULL | |
| product_id | uint | FK, único con ingredient_id | |
| ingredient_id | uint | FK, único con product_id | |
| quantity | float | ≥ 0 | Q |

### ProductMachine — T(I,K)

| product_id, machine_id | uint | únicos juntos | |
| minutes_per_unit | float | ≥ 0 | T |

### ProductOperationalResource — CM(I,R)

| product_id, operational_resource_id | uint | únicos juntos | |
| consumption_per_batch | float | ≥ 0 | CM |

---

## Corrida

### Optimization

| Atributo | Tipo | Dominio / invariante | LINGO |
|---|---|---|---|
| id | uint | PK | |
| scenario_id | *uint | FK→Scenario, nullable | la corrida sobrevive si se archiva el escenario |
| job_id | string | no vacío, único | id de la cola |
| status | enum | `pending`\|`processing`\|`done`\|`error`\|`cancelled` | |
| max_production | float | ≥ 0 | M efectivo de la corrida |
| min_variety | int | ≥ 0 | PRO efectivo |
| total_profit | float | | objetivo |
| created_at / started_at / finished_at | time | started/finished nullable | |

> El escenario congelado **es** la foto de los inputs — no hace falta un
> `input_snapshot` JSON. (En el modelo actual singleton sí existe; se elimina al
> migrar a escenarios.)

### OptimizationResult

Fila por producto. **Autocontenido**: guarda el nombre denormalizado para que el
resultado histórico se lea aunque el producto se archive.

| Atributo | Tipo | Dominio | LINGO |
|---|---|---|---|
| id | uint | PK | |
| optimization_id | uint | FK→Optimization, NOT NULL | |
| product_id | *uint | FK→Product, nullable | link blando |
| canonical_product_id | *uint | para analítica cruzada | |
| product_name | string | denormalizado | |
| quantity_to_produce | float | ≥ 0 | X |
| batch_active | float | ≥0 (entero, nº de lotes) | Y (@GIN) |
| variety_flag | float | 0 ó 1 | W (@BIN) |
| expected_profit | float | | |

### LingoLog

Sin cambios respecto al modelo actual: `job_id`, `optimization_id`, `level`,
`message`, `model_generated`, `lingo_output`, `duration_ms`, `created_at`.

---

## Reglas de ciclo de vida (borrado / archivado)

| Acción | Comportamiento | Por qué |
|---|---|---|
| Borrar **Product/Ingredient/Machine/OpResource** en un `draft` | CASCADE sus celdas de receta | Son locales al escenario; su receta no tiene sentido sin ellos. **No hay bloqueo cruzado** — solo toca ese draft |
| Borrar **Scenario** `draft` sin corridas | CASCADE todos sus hijos | Draft descartable |
| **Scenario** con corridas | **Archivar, no borrar** (`status=archived`) | Es historia. Las corridas quedan legibles |
| **Optimization** al archivarse su escenario | `scenario_id` → SET NULL | La corrida es autocontenida (nombre denormalizado en resultados) |
| Borrar **Product** que aparece en resultados | `OptimizationResult.product_id` → SET NULL (conserva `product_name` y `canonical_product_id`) | Historia intacta |
| Identidad canónica con historia | **Nunca hard-delete. Archivar.** | Ancla de identidad permanente; `canonical_id` nunca queda colgado |

**Consecuencia clave:** la clase de bug "no se puede eliminar (puede estar en
uso)" **desaparece**. Nada usa `RESTRICT` para proteger historia — la historia se
protege archivando identidades y denormalizando resultados.

---

## Invariantes multi-entidad (resumen; detalle en `03-invariants.md`)

1. `Product.max_batch ≥ Product.min_batch`.
2. Celda de receta: `product` e `ingredient/machine/opres` del **mismo** escenario
   (FK compuesta).
3. Un escenario `frozen` es inmutable: ninguna escritura a él ni a sus hijos.
4. Optimizar exige ≥ 1 producto con receta; si no, error explícito, no un modelo
   degenerado.
5. Todo numérico `≥ 0`; `0` es válido.

## Decisiones abiertas

- **`canonical_id`**: confirmado incluirlo desde v1 (habilita comparación entre
  escenarios). Ver ADR pendiente.
- **`is_base`**: ¿un único escenario base marcado, o el concepto de "base" es solo
  el escenario activo? A definir en ADR.
