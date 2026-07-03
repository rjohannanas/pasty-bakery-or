# ADR 0003 — Escenarios instanciados + fork como unidad de what-if

**Estado:** Aceptado. Supersede [0002](0002-singleton-stock-resource.md).

## Contexto

El dueño necesita **análisis what-if**: tomar un plan, variar algo (quitar un
producto, subir el costo de la harina, cambiar un precio) y ver cómo cambia el
plan óptimo, comparando contra el anterior. La config singleton mutable no lo
permite: sobrescribe y pierde.

Se evaluaron tres modelos:
1. **Dividir** identidad vs parámetros (catálogo + tablas de parámetros por escenario).
2. **Instanciar**: cada escenario dueño de copias completas de sus entidades.
3. **Overlay + materializar**: escenarios como diffs sobre un canónico, congelados al correr.

## Decisión

**Instanciar (opción 2), con fork.** Cada entidad de dominio lleva `scenario_id` y
guarda sus parámetros. Clonar un escenario copia sus filas con nuevo `scenario_id`.
Un escenario se **congela** (`frozen`) al optimizarse; editar un congelado
**forkea** un nuevo `draft`.

## Por qué instanciar y no las otras

- **vs dividir:** instanciar está más cerca del modelo actual (las entidades ya
  tienen sus params), colapsa el andamiaje singleton (`stock_available` es columna
  del Ingredient; `hours_available` del Machine → desaparecen Stock/Resource/
  StockIngredient/ResourceMachine), y da aislamiento total entre escenarios.
- **vs overlay+materializar:** el overlay es el modelo "más robusto" (propaga
  cambios del canónico, sin drift) pero introduce materialización crítica, overlay
  tri-estado, borradores no-reproducibles y merge duplicado UI/solver. Es
  infraestructura grado ERP, **sobredimensionada** para una panadería de 2 personas.

## Consecuencias

- **+** Aislamiento por construcción; el problema "no se puede borrar" desaparece
  (entidades locales al escenario).
- **+** Delta chico sobre el código actual; entregable en días.
- **−** **Drift:** un cambio real (ej. sube la harina de verdad) no propaga; hay que
  tocarlo en cada escenario. Se acepta: el dueño casi no lo va a disparar, y hay un
  escenario base como referencia operativa.
- **−** Identidad cruzada no intrínseca → se resuelve con `canonical_id` (ver 0006).
- **−** Clonar copia varias tablas; se referencian miembros de receta por id estable
  para evitar remapeo frágil.

## Referencia

Modelo completo en [`../02-data-dictionary.md`](../02-data-dictionary.md).
