# ADR 0004 — La identidad se archiva, nunca se hard-delete

**Estado:** Aceptado

## Contexto

El bug "No se puede eliminar (puede estar en uso)" venía de proteger la historia
candando filas vivas con `RESTRICT`: un producto referenciado por una optimización
no se podía borrar. Esto cripta la edición normal del catálogo.

## Decisión

Las entidades que son **anclas de identidad** (productos, insumos, máquinas,
recursos operativos con historia) **no se hard-deletean**: se marcan
`status=archived`. Archivado = oculto de escenarios nuevos, pero su id vive para
siempre. La historia se protege por dos mecanismos, **no** por candados:

1. **Denormalización** del `product_name` (y `canonical_product_id`) en
   `OptimizationResult` → el resultado es autocontenido.
2. **Archivado** de la identidad → `canonical_id` nunca queda colgado; la analítica
   cruzada (`GROUP BY canonical_id`) no se rompe.

Las filas puramente locales de un `draft` (una receta, un producto de un borrador
sin corridas) **sí** se hard-deletean (CASCADE) — no son identidad con historia.

## Consecuencias

- **+** La clase de bug "no se puede eliminar" desaparece: borrar procede o archiva.
- **+** Modelado correcto: como un plan de cuentas o un SKU, una identidad con
  historia no debe destruirse.
- **−** El catálogo acumula filas archivadas → se filtran por `status` en las vistas
  normales.

## Regla derivada

Ningún FK hacia una identidad usa `ON DELETE RESTRICT`. Se usa `SET NULL`
(preservando denormalización) o el archivado a nivel aplicación.
