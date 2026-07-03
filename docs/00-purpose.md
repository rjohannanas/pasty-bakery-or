# 00 — Propósito

## Qué es

Sistema de **optimización de producción** para una panadería. Dado el inventario
de insumos, la capacidad de máquinas y recursos operativos, las recetas y los
precios/demandas de cada producto, calcula **cuánto producir de cada producto**
para **maximizar la ganancia** sin exceder ningún recurso. El cálculo lo resuelve
un modelo matemático corrido en el solver **LINGO**.

## El negocio

- Una sola panadería. Dueño + 1 trabajador. **Dos personas de confianza, sin
  roles ni jerarquía.**
- No hay múltiples sucursales, ni multi-usuario real, ni necesidad de permisos.
- Volumen chico: ~20 productos, ~12 insumos, ~4 máquinas, ~3 recursos operativos.

Esto justifica decisiones que en un producto grande serían inaceptables (auth
por una sola API key, sin login/roles) y **descarta** sobre-ingeniería (no hace
falta infraestructura de planificación multi-usuario grado ERP).

## Job-to-be-done

1. **Plan del día:** "con lo que tengo hoy, ¿cuánto hago de cada cosa para ganar
   más?" → una optimización sobre la configuración actual.
2. **What-if (análisis de escenarios):** "¿y si saco Panetón?", "¿y si sube la
   harina?", "¿y si cambio el precio del Bizcocho?" → clonar el plan, tocar unos
   valores, correr de nuevo, **comparar** contra el plan anterior.

El (2) es la razón principal por la que el modelo de datos necesita **escenarios**
y no una sola configuración mutable: el dueño quiere *variar y comparar*, no
*sobrescribir y perder* el plan anterior.

## Qué NO es

- No es un ERP ni un sistema de inventario transaccional. No factura, no vende, no
  controla stock en tiempo real.
- No es multi-tenant. No es multi-panadería.
- El resultado es **sugerencia de producción**, no una orden ejecutada.

## Actores

| Actor | Qué hace |
|---|---|
| Dueño / trabajador | Carga catálogo y cantidades, arma escenarios, corre optimizaciones, lee el plan sugerido |
| Front (`pasty-bakery-front`) | UI web. Otra máquina, otro repo. Consume la API por HTTP + WebSocket |
| Worker (este repo) | Consume jobs de la cola, corre LINGO, guarda resultados, notifica por WS |
| LINGO | Binario externo que resuelve el modelo matemático |
