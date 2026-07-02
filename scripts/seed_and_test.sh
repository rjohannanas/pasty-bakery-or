#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════
# Seed script para LINGO Bakery Backend
# Carga los datos reales de MODELO_EXCEL.xlsx y lanza una optimización
# ═══════════════════════════════════════════════════════════════════════
set -e
API="http://localhost:8080/api"

echo "══════════════════════════════════════════════"
echo "  PASO 1: Crear 12 ingredientes"
echo "══════════════════════════════════════════════"

declare -a ING_IDS
ING_DATA=(
  '{"name":"Harina de Trigo","unit":"kg","unit_cost":3.2}'
  '{"name":"Harina Integral","unit":"kg","unit_cost":4.0}'
  '{"name":"Levadura","unit":"gramos","unit_cost":0.04}'
  '{"name":"Azúcar","unit":"kg","unit_cost":3.8}'
  '{"name":"Sal","unit":"gramos","unit_cost":0.01}'
  '{"name":"Mantequilla","unit":"kg","unit_cost":22.0}'
  '{"name":"Huevos","unit":"unidades","unit_cost":0.4}'
  '{"name":"Leche","unit":"litros","unit_cost":4.5}'
  '{"name":"Chocolate (Cacao)","unit":"kg","unit_cost":28.0}'
  '{"name":"Relleno Salado","unit":"kg","unit_cost":18.0}'
  '{"name":"Manjar Blanco","unit":"kg","unit_cost":12.0}'
  '{"name":"Polvo de Hornear","unit":"gramos","unit_cost":0.03}'
)

for data in "${ING_DATA[@]}"; do
  resp=$(curl -s -X POST "$API/ingredients" -H "Content-Type: application/json" -d "$data")
  id=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "?")
  ING_IDS+=("$id")
  echo "  ✅ Ingrediente $id"
done

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 2: Crear 4 máquinas"
echo "══════════════════════════════════════════════"

declare -a MACH_IDS
MACH_DATA=(
  '{"name":"Amasadora"}'
  '{"name":"Batidora"}'
  '{"name":"Horno"}'
  '{"name":"Cámara Fermentadora"}'
)

for data in "${MACH_DATA[@]}"; do
  resp=$(curl -s -X POST "$API/machines" -H "Content-Type: application/json" -d "$data")
  id=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "?")
  MACH_IDS+=("$id")
  echo "  ✅ Máquina $id"
done

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 3: Crear 20 productos"
echo "══════════════════════════════════════════════"

declare -a PROD_IDS
PROD_DATA=(
  '{"name":"Pan Francés","sale_price":0.3,"demand":400,"min_batch":40,"max_batch":60}'
  '{"name":"Pan Chabata","sale_price":0.4,"demand":300,"min_batch":30,"max_batch":50}'
  '{"name":"Pan Integral","sale_price":0.5,"demand":150,"min_batch":25,"max_batch":40}'
  '{"name":"Pan de Yema","sale_price":0.4,"demand":200,"min_batch":30,"max_batch":50}'
  '{"name":"Croissant","sale_price":1.5,"demand":100,"min_batch":15,"max_batch":25}'
  '{"name":"Empanada de Carne","sale_price":3.5,"demand":80,"min_batch":15,"max_batch":25}'
  '{"name":"Empanada de Pollo","sale_price":3.5,"demand":70,"min_batch":15,"max_batch":25}'
  '{"name":"Alfajor","sale_price":1.2,"demand":120,"min_batch":20,"max_batch":40}'
  '{"name":"Torta de Chocolate","sale_price":45.0,"demand":6,"min_batch":1,"max_batch":2}'
  '{"name":"Torta Tres Leches","sale_price":50.0,"demand":5,"min_batch":1,"max_batch":2}'
  '{"name":"Pie de Limón","sale_price":35.0,"demand":4,"min_batch":1,"max_batch":2}'
  '{"name":"Panetón","sale_price":15.0,"demand":20,"min_batch":5,"max_batch":10}'
  '{"name":"Keké de Vainilla","sale_price":12.0,"demand":15,"min_batch":4,"max_batch":8}'
  '{"name":"Keké de Naranja","sale_price":12.0,"demand":15,"min_batch":4,"max_batch":8}'
  '{"name":"Bizcocho","sale_price":2.5,"demand":40,"min_batch":10,"max_batch":20}'
  '{"name":"Pan de Molde Blanco","sale_price":7.0,"demand":30,"min_batch":5,"max_batch":10}'
  '{"name":"Pan de Molde Integral","sale_price":8.0,"demand":25,"min_batch":5,"max_batch":10}'
  '{"name":"Enrolado de Hot Dog","sale_price":2.5,"demand":50,"min_batch":15,"max_batch":25}'
  '{"name":"Cachito de Mantequilla","sale_price":1.0,"demand":80,"min_batch":20,"max_batch":30}'
  '{"name":"Brownie","sale_price":3.0,"demand":60,"min_batch":12,"max_batch":24}'
)

for data in "${PROD_DATA[@]}"; do
  resp=$(curl -s -X POST "$API/products" -H "Content-Type: application/json" -d "$data")
  id=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "?")
  PROD_IDS+=("$id")
  echo "  ✅ Producto $id"
done

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 4: Asignar ingredientes a productos (Matriz Q)"
echo "══════════════════════════════════════════════"

# Cada fila es un producto, cada columna es un ingrediente
# Q[producto][ingrediente] = cantidad por unidad
Q=(
  # HTrig  HInt   Lev    Azuc   Sal    Mant   Huev   Lech   Choc   Rell   Manj   Polv
  "0.03   0      0.5    0      0.5    0      0      0      0      0      0      0"      # Pan Francés
  "0.035  0      0.8    0      0.6    0      0      0      0      0      0      0"      # Pan Chabata
  "0.01   0.025  1      0      0.4    0      0      0      0      0      0      0"      # Pan Integral
  "0.03   0      0.6    0.005  0.3    0.005  0.2    0      0      0      0      0"      # Pan de Yema
  "0.04   0      1.5    0.01   0.2    0.025  0.1    0.01   0      0      0      0"      # Croissant
  "0.05   0      0      0.005  0.5    0.015  0.2    0      0      0.08   0      1"      # Empanada Carne
  "0.05   0      0      0.005  0.5    0.015  0.2    0      0      0.08   0      1"      # Empanada Pollo
  "0.02   0      0      0.015  0      0.01   0      0      0      0      0.03   0.5"    # Alfajor
  "0.35   0      0      0.3    1      0.15   6      0.25   0.25   0      0.2    5"      # Torta Chocolate
  "0.3    0      0      0.25   0.5    0      8      0.6    0      0      0      4"      # Torta Tres Leches
  "0.25   0      0      0.2    0.5    0.1    5      0.1    0      0      0      0"      # Pie de Limón
  "0.4    0      15     0.15   2      0.1    4      0.05   0      0      0      0"      # Panetón
  "0.25   0      0      0.15   0.5    0.08   3      0.1    0      0      0      8"      # Keké Vainilla
  "0.25   0      0      0.15   0.5    0.08   3      0.1    0      0      0      8"      # Keké Naranja
  "0.06   0      2      0.02   0.5    0.01   0.5    0.02   0      0      0      0"      # Bizcocho
  "0.4    0      8      0.03   5      0.02   0      0.05   0      0      0      0"      # Pan Molde Blanco
  "0.1    0.3    8      0.02   5      0.02   0      0.05   0      0      0      0"      # Pan Molde Integral
  "0.04   0      1      0.005  0.5    0.01   0.1    0      0      0.05   0      0"      # Enrolado Hot Dog
  "0.035  0      1      0.01   0.2    0.02   0.1    0.01   0      0      0      0"      # Cachito Mantequilla
  "0.05   0      0      0.08   0.2    0.04   1      0      0.06   0      0      1"      # Brownie
)

for i in "${!Q[@]}"; do
  p_idx=$((i))
  p_id="${PROD_IDS[$p_idx]}"
  read -ra vals <<< "${Q[$i]}"
  for j in "${!vals[@]}"; do
    qty="${vals[$j]}"
    if (( $(echo "$qty > 0" | bc -l) )); then
      ing_id="${ING_IDS[$j]}"
      curl -s -X POST "$API/products/$p_id/ingredients" \
        -H "Content-Type: application/json" \
        -d "{\"ingredient_id\":$ing_id,\"quantity\":$qty}" > /dev/null
    fi
  done
  echo "  ✅ Producto $p_id: ingredientes asignados"
done

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 5: Asignar máquinas a productos (Matriz T)"
echo "══════════════════════════════════════════════"

# T[producto][máquina] = minutos por unidad
T=(
  "15 0  20 45"   # Pan Francés
  "20 0  25 45"   # Pan Chabata
  "20 0  25 50"   # Pan Integral
  "15 0  20 40"   # Pan de Yema
  "25 0  25 60"   # Croissant
  "10 0  20 0"    # Empanada Carne
  "10 0  20 0"    # Empanada Pollo
  "15 0  15 0"    # Alfajor
  "0  25 45 0"    # Torta Chocolate
  "0  30 40 0"    # Torta Tres Leches
  "10 15 30 0"    # Pie de Limón
  "30 0  50 120"  # Panetón
  "0  15 40 0"    # Keké Vainilla
  "0  15 40 0"    # Keké Naranja
  "15 0  25 40"   # Bizcocho
  "25 0  35 60"   # Pan Molde Blanco
  "25 0  35 60"   # Pan Molde Integral
  "15 0  20 30"   # Enrolado Hot Dog
  "20 0  20 45"   # Cachito Mantequilla
  "0  15 25 0"    # Brownie
)

for i in "${!T[@]}"; do
  p_id="${PROD_IDS[$i]}"
  read -ra vals <<< "${T[$i]}"
  for j in "${!vals[@]}"; do
    mins="${vals[$j]}"
    if (( $(echo "$mins > 0" | bc -l) )); then
      m_id="${MACH_IDS[$j]}"
      curl -s -X POST "$API/products/$p_id/machines" \
        -H "Content-Type: application/json" \
        -d "{\"machine_id\":$m_id,\"minutes_per_unit\":$mins}" > /dev/null
    fi
  done
  echo "  ✅ Producto $p_id: máquinas asignadas"
done

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 6: Crear stock (inventario de ingredientes)"
echo "══════════════════════════════════════════════"

# IN(J) = Inventario disponible de cada ingrediente
INV=(80 15 500 25 1000 10 360 15 5 8 5 400)

STOCK_JSON='{"name":"Inventario del día","ingredients":['
for j in "${!INV[@]}"; do
  if [ $j -gt 0 ]; then STOCK_JSON+=","; fi
  STOCK_JSON+="{\"ingredient_id\":${ING_IDS[$j]},\"quantity_available\":${INV[$j]}}"
done
STOCK_JSON+=']}'

resp=$(curl -s -X POST "$API/stocks" -H "Content-Type: application/json" -d "$STOCK_JSON")
STOCK_ID=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  ✅ Stock creado: ID=$STOCK_ID"

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 7: Crear recurso (máquinas + recursos operativos)"
echo "══════════════════════════════════════════════"

RESOURCE_JSON=$(cat <<EOF
{
  "name": "Capacidad del día",
  "machines": [
    {"machine_id": ${MACH_IDS[0]}, "hours_available": 8.0},
    {"machine_id": ${MACH_IDS[1]}, "hours_available": 6.0},
    {"machine_id": ${MACH_IDS[2]}, "hours_available": 10.0},
    {"machine_id": ${MACH_IDS[3]}, "hours_available": 8.0}
  ],
  "operational_resources": [
    {"name": "Horas-Hombre (HH)", "available": 16, "cost_per_unit": 6.0},
    {"name": "Electricidad (kW)", "available": 150, "cost_per_unit": 0.8},
    {"name": "Gas Combustible (m3)", "available": 80, "cost_per_unit": 1.2}
  ]
}
EOF
)

resp=$(curl -s -X POST "$API/resources" -H "Content-Type: application/json" -d "$RESOURCE_JSON")
RESOURCE_ID=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  ✅ Recurso creado: ID=$RESOURCE_ID"

echo ""
echo "══════════════════════════════════════════════"
echo "  PASO 8: Asignar consumo de recursos operativos por producto (Matriz CM)"
echo "══════════════════════════════════════════════"

# Necesitamos los IDs de los OperationalResource recién creados
# Los obtenemos del GET del resource
OPRES_JSON=$(curl -s "$API/resources/$RESOURCE_ID")
OPRES_IDS=$(echo "$OPRES_JSON" | python3 -c "
import sys,json
data = json.load(sys.stdin)
ids = [str(r['id']) for r in data.get('operational_resources', [])]
print(' '.join(ids))
")
read -ra OPRES_ID_ARR <<< "$OPRES_IDS"
echo "  IDs de recursos operativos: ${OPRES_ID_ARR[*]}"

# CM[producto][recurso] = consumo por lote
CM=(
  "0.5  1.2  1.5"    # Pan Francés
  "0.6  1.5  1.8"    # Pan Chabata
  "0.6  1.5  1.8"    # Pan Integral
  "0.6  1.3  1.5"    # Pan de Yema
  "1.0  2.0  2.0"    # Croissant
  "0.8  1.0  1.5"    # Empanada Carne
  "0.8  1.0  1.5"    # Empanada Pollo
  "0.6  1.0  1.2"    # Alfajor
  "1.5  2.5  3.0"    # Torta Chocolate
  "1.5  2.5  2.8"    # Torta Tres Leches
  "1.0  1.5  2.0"    # Pie de Limón
  "1.2  3.0  3.5"    # Panetón
  "0.8  1.5  2.5"    # Keké Vainilla
  "0.8  1.5  2.5"    # Keké Naranja
  "0.6  1.2  1.8"    # Bizcocho
  "0.8  2.0  2.5"    # Pan Molde Blanco
  "0.8  2.0  2.5"    # Pan Molde Integral
  "0.8  1.2  1.5"    # Enrolado Hot Dog
  "0.8  1.5  1.8"    # Cachito Mantequilla
  "0.5  1.2  1.8"    # Brownie
)

# Para asignar CM usamos el nuevo endpoint REST
echo "  Asignando consumos de recursos operativos (Matriz CM)..."
for i in "${!CM[@]}"; do
  p_id="${PROD_IDS[$i]}"
  read -ra vals <<< "${CM[$i]}"
  for j in "${!vals[@]}"; do
    consumption="${vals[$j]}"
    opres_id="${OPRES_ID_ARR[$j]}"
    curl -s -X POST "$API/products/$p_id/operational-resources" \
      -H "Content-Type: application/json" \
      -d "{\"operational_resource_id\":$opres_id,\"consumption_per_batch\":$consumption}" > /dev/null
  done
  echo "  ✅ Producto $p_id: recursos operativos asignados"
done

echo ""
echo "══════════════════════════════════════════════════════════════════"
echo "  PASO 9: 🚀 Lanzar optimización (M=200, PRO=7)"
echo "══════════════════════════════════════════════════════════════════"

OPT_JSON="{\"stock_id\":$STOCK_ID,\"resource_id\":$RESOURCE_ID,\"max_production\":200,\"min_variety\":7}"
echo "  Request: $OPT_JSON"
resp=$(curl -s -X POST "$API/optimize" -H "Content-Type: application/json" -d "$OPT_JSON")
echo "  Response: $resp"

JOB_ID=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['job_id'])")
echo ""
echo "  Job ID: $JOB_ID"
echo "  Esperando resultado..."

for i in $(seq 1 30); do
  sleep 2
  status=$(curl -s "$API/optimize/$JOB_ID")
  st=$(echo "$status" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','?'))")
  echo "  [$i] Status: $st"
  if [ "$st" = "done" ] || [ "$st" = "error" ]; then
    break
  fi
done

echo ""
echo "══════════════════════════════════════════════════════════════════"
echo "  PASO 10: Consultar resultados"
echo "══════════════════════════════════════════════════════════════════"

# Obtener el ID de la optimización desde la API
OPT_ID=$(curl -s "$API/optimize/$JOB_ID" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))")

echo ""
echo "  Resultados de optimización (ID=$OPT_ID):"
curl -s "$API/results/$OPT_ID" | python3 -m json.tool

echo ""
echo "══════════════════════════════════════════════════════════════════"
echo "  Logs de LINGO:"
echo "══════════════════════════════════════════════════════════════════"
curl -s "$API/logs/lingo/$JOB_ID" | python3 -c "
import sys, json
logs = json.load(sys.stdin)
for log in logs:
    print(f\"  Level: {log['level']} | Duration: {log['duration_ms']}ms\")
    print(f\"  Message: {log['message']}\")
    if log.get('lingo_output'):
        out = log['lingo_output']
        # Solo las últimas 30 líneas
        lines = out.strip().split('\n')
        for line in lines[-30:]:
            print(f\"    {line}\")
"

echo ""
echo "✅ Seed y prueba completados."
