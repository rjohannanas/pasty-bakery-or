# 0007 — Modos configurables de resolución (LP / MILP)

- **Estado:** Aceptado
- **Fecha:** 2026-07-04
- **Autores:** Equipo LINGO / Backend

## Contexto

El modelo matemático LINGO original en la panadería está formulado como una Programación Lineal Entera Mixta (PLEM / MILP) donde:
- $X(I)$ y $Y(I)$ son variables enteras (`@GIN`).
- $W(I)$ es una variable binaria (`@BIN`) que activa o desactiva la producción de un producto e impone la meta de variedad mínima (`@SUM(W(I)) >= PRO`).

El cliente requirió la capacidad de evaluar escenarios bajo 4 combinaciones configurables mediante dos casillas de verificación en la interfaz:
1. **Variables Enteras (`use_integer_vars`)**: Si está activo, $X, Y \in \mathbb{Z}_{\ge 0}$ (`@GIN`). Si no, $X, Y \ge 0$ continuas.
2. **Variables Binarias (`use_binary_vars`)**: Si está activo, $W_i \in \{0,1\}$ (`@BIN`) y se exige la meta de variedad $\sum W_i \ge PRO$. Si no, $W_i = 1$ para todo $i$ y se desactiva la meta de variedad.

## Decisión

1. **Campos Opcionales en `Optimization`**: La corrida almacena `use_integer_vars` y `use_binary_vars` (ambos booleanos, `NOT NULL`, por defecto `true`).
2. **Compatibilidad Hacia Atrás**: Si la petición `POST /scenarios/{id}/optimize` omite estos campos, el backend asume `true` para ambos, manteniendo la PLEM completa por defecto.
3. **Condición de Variedad Mínima ($PRO$)**:
   - Cuando `use_binary_vars = true`: $W_i$ es binaria (`@BIN`), se incluye `@SUM(W) >= PRO` y $W_i \le X_i$. El usuario en UI puede ajustar `min_variety`.
   - Cuando `use_binary_vars = false`: $W_i = 1$ para todo $i$. Las restricciones de variedad y `@BIN` se excluyen del código LINGO. En UI, el input `min_variety` se deshabilita.

## Consecuencias

- El esquema de datos de resultados (`optimization_results`) no cambia, ya que los valores continuos o enteros caben en `float64`.
- Permite análisis de sensibilidad rápidos utilizando la relajación lineal (LP continua) cuando ambas casillas están desmarcadas.
