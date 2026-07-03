# Architecture Decision Records

Cada ADR captura **una** decisión: contexto, qué se decidió y las consecuencias
(incluidas las malas). Son inmutables: una decisión no se edita, se **supersede**
con un ADR nuevo que la referencia.

| # | Decisión | Estado |
|---|---|---|
| [0001](0001-auth-api-key.md) | Auth por una sola API key, sin usuarios ni roles | Aceptado |
| [0002](0002-singleton-stock-resource.md) | Stock y Resource singleton | **Superseded por 0003** |
| [0003](0003-escenarios-instanciados-fork.md) | Escenarios instanciados + fork como unidad de what-if | Aceptado |
| [0004](0004-archivar-no-borrar.md) | La identidad se archiva, nunca se hard-delete | Aceptado |
| [0005](0005-cero-es-valido.md) | `0` es válido: nunca `binding:"required"` en floats | Aceptado |
| [0006](0006-canonical-id-comparacion.md) | `canonical_id` para comparar entidades entre escenarios | Aceptado |
