# 01 — Glosario (lenguaje ubicuo)

Un solo vocabulario para negocio, modelo y solver. Front y back **deben** usar
estos términos. Si un término no está acá, no existe.

## Términos de negocio

| Término | Definición |
|---|---|
| **Producto** | Algo que la panadería produce y vende (Pan Francés, Empanada). Tiene precio, demanda y tamaños de lote |
| **Insumo / Ingrediente** | Materia prima consumida por las recetas (Harina, Levadura). Tiene costo unitario y cantidad en stock |
| **Máquina** | Equipo con capacidad horaria limitada (Horno, Amasadora) |
| **Recurso operativo** | Recurso consumible no-insumo con tope y costo (Horas-Hombre, Electricidad, Gas) |
| **Receta** | Cuánto de cada insumo / minutos de cada máquina / consumo de cada recurso operativo necesita un producto. Son las matrices Q, T, CM |
| **Stock / Inventario** | Cantidad disponible de cada insumo |
| **Capacidad** | Horas disponibles de cada máquina + disponibilidad de cada recurso operativo |
| **Escenario** | Un set completo y editable de todos los datos de entrada (productos, recetas, precios, costos, stock, capacidad, parámetros). Un "plan" sobre el que se optimiza. Es la unidad clonable del what-if |
| **Optimización / Corrida** | Una ejecución del solver sobre un escenario. Produce un plan de producción |
| **Plan de producción** | El resultado: cuánto producir de cada producto, y la ganancia esperada |

## Mapeo término ↔ modelo ↔ variable LINGO

La fuente de la verdad de los nombres LINGO es el modelo matemático. El modelo de
datos **debe** mantener este mapeo (documentado también en los comentarios de
`internal/models/`).

### Datos de entrada (parámetros)

| Negocio | Atributo del modelo | LINGO | Descripción |
|---|---|---|---|
| Precio de venta | `Product.sale_price` | **P(I)** | Precio de venta del producto I |
| Demanda | `Product.demand` | **D(I)** | Demanda / tope de venta del producto I |
| Lote mínimo | `Product.min_batch` | **LI(I)** | Producción mínima si se produce I |
| Lote máximo | `Product.max_batch` | **LS(I)** | Producción máxima del producto I |
| Costo unitario insumo | `Ingredient.unit_cost` | **CU(J)** | Costo por unidad del insumo J |
| Stock disponible | `Ingredient.stock_available` | **IN(J)** | Cantidad disponible del insumo J |
| Capacidad máquina | `Machine.capacity_minutes` | **CAP(K)** | Minutos disponibles máquina K (misma unidad que T) |
| Disponibilidad rec. op. | `OperationalResource.available` | **DISP(R)** | Tope del recurso operativo R |
| Costo rec. op. | `OperationalResource.cost_per_unit` | **CR(R)** | Costo por unidad del recurso operativo R |
| Receta insumo | `ProductIngredient.quantity` | **Q(I,J)** | Insumo J por unidad de producto I |
| Receta máquina | `ProductMachine.minutes_per_unit` | **T(I,K)** | Minutos de máquina K por unidad de I |
| Receta rec. op. | `ProductOperationalResource.consumption_per_batch` | **CM(I,R)** | Consumo de recurso R por lote de I |
| Producción máx. global | `Scenario.max_production` | **M** | Cota superior de producción total |
| Variedad mínima | `Scenario.min_variety` | **PRO** | Mínimo de productos distintos a producir |

### Resultados (variables de decisión)

| Negocio | Atributo del modelo | LINGO | Descripción |
|---|---|---|---|
| Cantidad a producir | `OptimizationResult.quantity_to_produce` | **X(I)** | Unidades a producir del producto I |
| Lote activo | `OptimizationResult.batch_active` | **Y(I)** | 1 si se produce I (binaria) |
| Bandera de variedad | `OptimizationResult.variety_flag` | **W(I)** | Cuenta para la restricción de variedad |
| Ganancia esperada | `OptimizationResult.expected_profit` | — | Aporte de I a la ganancia |
| Ganancia total | `Optimization.total_profit` | objetivo | Valor de la función objetivo |

## Índices LINGO

| Índice | Recorre |
|---|---|
| I | Productos |
| J | Insumos |
| K | Máquinas |
| R | Recursos operativos |
