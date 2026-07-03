# ADR 0005 — `0` es un valor válido: nunca `binding:"required"` en floats

**Estado:** Aceptado

## Contexto

El auto-ligado de ingredientes/máquinas nuevos al escenario fallaba con **400**.
Causa: los handlers usaban `binding:"required"` en campos `float64`. En Go, el
validador de Gin trata el zero-value (`0.0`) como "faltante" y lo rechaza. Dar de
alta un insumo con cantidad 0 (nuevo, sin stock aún), una celda de receta en 0
(no usa ese insumo) o un lote de 0 son casos **legítimos**, pero se rechazaban.

Ese 400, dentro de un `Promise.all` del front, abortaba toda la carga de la
pantalla de Configuración → todo se veía vacío aunque los datos existían.

## Decisión

En este dominio **todo atributo numérico puede ser `0`**. Por lo tanto:

- **Prohibido** `binding:"required"` en campos `float64`.
- La obligatoriedad se expresa con `NOT NULL` (columna) + puntero `*float64` cuando
  hay que distinguir "campo omitido" de "cero explícito" en un update parcial.
- Solo IDs (`uint`) y nombres (`string`) usan `required`.

## Consecuencias

- **+** El auto-ligado y las celdas de matriz en 0 funcionan.
- **+** Regla explícita y única (ver invariante A2), imposible de reintroducir por
  descuido si se deriva la validación del diccionario.
- **−** Un body vacío bindea a `0` en vez de dar error; aceptable para upserts de
  cantidad (0 es un default sano).

## Referencia

Invariante A2 en [`../03-invariants.md`](../03-invariants.md).
