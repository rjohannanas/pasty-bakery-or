# ADR 0002 — Stock y Resource singleton

**Estado:** ~~Aceptado~~ **Superseded por [0003](0003-escenarios-instanciados-fork.md)**

## Contexto

El negocio maneja un solo inventario diario y un solo pool de recursos. El modelo
inicial permitía múltiples Stock/Resource, pero en la práctica solo se usaba uno.

## Decisión (original)

`Stock` y `Resource` como singleton: `GET /stocks/default` y `GET /resources/default`
devuelven la única fila, creándola si no existe. Al dar de alta un ingrediente o
máquina global, el front debía además ligarlo al singleton.

## Por qué se supersede

El singleton es **una sola configuración mutable**. No permite el what-if (variar
y comparar planes) sin sobrescribir y perder el anterior. Además, proteger la
historia candando filas vivas (`RESTRICT`) rompía la edición/borrado normal.

El modelo de escenarios (0003) reemplaza el singleton: la "configuración actual"
pasa a ser un escenario marcado, y cada what-if es otro escenario.

## Consecuencia de la migración

La config singleton actual se migra a un `Scenario{is_base:true}`. El paso de
auto-ligado ingrediente/máquina→singleton desaparece (todo vive dentro del
escenario).
