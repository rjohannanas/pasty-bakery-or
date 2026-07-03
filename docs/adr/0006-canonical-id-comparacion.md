# ADR 0006 — `canonical_id` para comparar entidades entre escenarios

**Estado:** Aceptado

## Contexto

Con escenarios instanciados (0003), "Empanada" en el escenario A y en el B son
filas distintas sin vínculo. El what-if necesita comparar el **mismo** producto
entre planes ("¿cuánto cambió la producción óptima de Empanada si subo la harina?").
Comparar por nombre es frágil (renombres, typos).

## Decisión

Cada entidad de dominio (Product/Ingredient/Machine/OperationalResource) lleva
`canonical_id` (nullable, FK a la entidad "origen"). Al forkear un escenario, cada
copia hereda el `canonical_id` de su origen. La identidad cruzada es entonces
`canonical_id`, no el nombre.

- Comparar entre escenarios = `GROUP BY canonical_id`.
- Una entidad **nueva** creada dentro de un escenario tiene `canonical_id = NULL`
  hasta promoverse (no existe en otros escenarios → no comparable, que es correcto).
- `OptimizationResult` denormaliza `canonical_product_id` para analítica cruzada
  sin joins frágiles.

## Consecuencias

- **+** Comparación e identidad estables ante renombres.
- **+** Analítica cruzada por SQL directo.
- **−** El linaje por puntero es sintético; hay que setearlo bien al clonar/forkear
  (apuntar a la raíz canónica, no al padre inmediato, para evitar caminar cadenas).

## Decisión abierta relacionada

¿`canonical_id` apunta siempre a la **raíz** o al **padre inmediato**? Se resuelve
apuntando a la raíz (comparación en un salto). A confirmar al implementar el clone.
