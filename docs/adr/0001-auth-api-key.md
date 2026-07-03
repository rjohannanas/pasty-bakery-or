# ADR 0001 — Auth por una sola API key, sin usuarios ni roles

**Estado:** Aceptado

## Contexto

El negocio son dos personas de confianza (dueño + trabajador), sin jerarquía. La
API se expone a internet vía Cloudflare Tunnel. El front es una SPA pública cuyo
bundle JS es visible.

## Decisión

Autenticación por un único secreto compartido (`APP_API_KEY`), enviado como header
`X-API-Key` en HTTP y query `api_key` en el handshake WebSocket. Sin usuarios,
sin login, sin roles. Si la key está vacía, la API queda abierta (con warning).

## Consecuencias

- **+** Cero fricción para dos usuarios; nada de gestión de cuentas.
- **−** No es un secreto fuerte: el front lo expone en su bundle (`NEXT_PUBLIC_API_KEY`).
  Es un filtro anti-scraping casual, no una barrera criptográfica.
- **−** No hay auditoría por usuario (no importa: dos personas).
- El WebSocket usa query param porque el navegador no permite headers custom en el
  handshake.

## Notas

Si algún día hay más usuarios o datos sensibles, esto se supersede con auth real
(sesiones/JWT + roles).
