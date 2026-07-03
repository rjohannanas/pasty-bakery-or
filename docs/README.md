# Documentación de diseño — lingo-backend

Esta carpeta es la **fuente de verdad** del sistema. El código se *deriva* de
estos documentos, no al revés. Cuando el código y estos docs difieren, el doc
manda y el código se corrige.

## Principio

- **Los invariantes viven en el modelo, no dispersos en el código.** Cada regla
  ("una cantidad puede ser 0", "borrar un producto cascadea su receta") se
  define una vez acá y se *deriva* a: validación de API, constraints de DB y
  tipos del contrato. Nunca se re-implementa a mano en front y back por separado
  (eso es lo que causó los bugs de interfaz).
- **Estados ilegales irrepresentables.** Tipos, NOT NULL, CHECK, enums y FKs con
  comportamiento explícito hacen que un dato malo no se pueda ni guardar.
- **Una fuente de verdad por regla.** Front y back derivan del mismo contrato;
  no adivinan los supuestos del otro.

## Los tres "schemas" (no confundir)

| Schema | Vive en | Naturaleza |
|---|---|---|
| **Lógico / conceptual** | este `docs/` (diccionario + ERD) | diseño humano, autoritativo |
| **Contrato de API** | `docs/openapi.yaml` (**contract-first**) | autoritativo; back y front **derivan** de él |
| **Físico** | `internal/models/`, migraciones | ejecutable, derivado del lógico + contrato |

> La salida swaggo vieja (`docs/swagger.*`, code-first del singleton anterior) ya
> fue **eliminada**. `openapi.yaml` es la única fuente del contrato y se sirve en
> `GET /openapi.yaml`.

## Índice

| # | Documento | Qué es |
|---|---|---|
| 00 | [purpose.md](00-purpose.md) | Negocio, propósito de la app, usuarios, job-to-be-done |
| 01 | [glossary.md](01-glossary.md) | Lenguaje ubicuo + mapeo término ↔ modelo ↔ variable LINGO |
| 02 | [data-dictionary.md](02-data-dictionary.md) | Entidades, atributos (dominio/invariantes), relaciones (ciclo de vida) |
| 03 | [invariants.md](03-invariants.md) | Reglas de negocio transversales (multi-entidad) |
| 04 | [optimization-process.md](04-optimization-process.md) | Ciclo del job: submit → cola → solver → resultados |
| — | [openapi.yaml](openapi.yaml) | **Contrato de API autoritativo** (OpenAPI 3.1, contract-first) |
| — | [erd.html](erd.html) | Diagrama visual del esquema (3 capas + ciclo de vida) |
| — | [adr/](adr/) | Registros de decisión de arquitectura (por qué de cada elección) |

## Estado

Modelo **escenarios instanciados + fork + identidad archivable** (ver
`02-data-dictionary.md`) ya implementado en el código. Los invariantes del doc 03
se derivan a la DB: CHECK de dominio/nombre y FK compuesta same-scenario (M2) en
`internal/db/postgres.go`.
