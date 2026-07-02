package solver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"lingo-backend/internal/logger"
	"lingo-backend/internal/models"
)

// LingoResult contiene los resultados de las variables X, Y, W y el valor objetivo.
type LingoResult struct {
	X              map[uint]float64 // ProductID -> X(I)
	Y              map[uint]float64 // ProductID -> Y(I)
	W              map[uint]float64 // ProductID -> W(I)
	ObjectiveValue float64
}

// floatSliceToString formatea una lista de float64 para LINGO.
// Fragmenta en líneas de máximo 10 valores para evitar el límite de 800 caracteres por línea.
func floatSliceToString(slice []float64) string {
	const maxPerLine = 10
	var lines []string
	var current []string
	for i, val := range slice {
		current = append(current, fmt.Sprintf("%.6f", val))
		if (i+1)%maxPerLine == 0 || i == len(slice)-1 {
			lines = append(lines, strings.Join(current, " "))
			current = nil
		}
	}
	return strings.Join(lines, "\n    ")
}

// BuildModel genera el contenido del modelo LINGO (.lng) basado en la DB.
// BuildSnapshot arma una foto congelada (JSON) de los datos de entrada que usa
// BuildModel: parámetros, stock+ingredientes, resource+máquinas+op.resources y
// productos con sus matrices. Se guarda en Optimization.InputSnapshot para poder
// reproducir/comparar una corrida aunque después se editen los singleton
// Stock/Resource (que se mutan in-place). Recarga con los mismos Preload que
// BuildModel para reflejar exactamente lo que se optimizó.
func BuildSnapshot(db *gorm.DB, opt *models.Optimization) (json.RawMessage, error) {
	var stock models.Stock
	if err := db.Preload("Ingredients.Ingredient").First(&stock, opt.StockID).Error; err != nil {
		return nil, fmt.Errorf("snapshot: error cargando stock: %w", err)
	}

	var resource models.Resource
	if err := db.Preload("Machines.Machine").Preload("OperationalResources").First(&resource, opt.ResourceID).Error; err != nil {
		return nil, fmt.Errorf("snapshot: error cargando recursos: %w", err)
	}

	var products []models.Product
	if err := db.Preload("Ingredients.Ingredient").Preload("Machines.Machine").Preload("OperationalResources.OperationalResource").Order("id").Find(&products).Error; err != nil {
		return nil, fmt.Errorf("snapshot: error cargando productos: %w", err)
	}

	snap := map[string]interface{}{
		"captured_at": time.Now(),
		"params": map[string]interface{}{
			"max_production": opt.MaxProduction,
			"min_variety":    opt.MinVariety,
		},
		"stock":    stock,
		"resource": resource,
		"products": products,
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("snapshot: error serializando: %w", err)
	}
	return raw, nil
}

func BuildModel(db *gorm.DB, opt *models.Optimization) (string, []models.Product, error) {
	var stock models.Stock
	if err := db.Preload("Ingredients.Ingredient").First(&stock, opt.StockID).Error; err != nil {
		return "", nil, fmt.Errorf("error cargando stock: %w", err)
	}

	var resource models.Resource
	if err := db.Preload("Machines.Machine").Preload("OperationalResources").First(&resource, opt.ResourceID).Error; err != nil {
		return "", nil, fmt.Errorf("error cargando recursos: %w", err)
	}

	var products []models.Product
	if err := db.Preload("Ingredients.Ingredient").Preload("Machines.Machine").Preload("OperationalResources.OperationalResource").Order("id").Find(&products).Error; err != nil {
		return "", nil, fmt.Errorf("error cargando productos: %w", err)
	}

	if len(products) == 0 {
		return "", nil, fmt.Errorf("no hay productos configurados")
	}

	var ingredients []models.Ingredient
	if err := db.Order("id").Find(&ingredients).Error; err != nil {
		return "", nil, fmt.Errorf("error cargando ingredientes: %w", err)
	}

	var machines []models.Machine
	if err := db.Order("id").Find(&machines).Error; err != nil {
		return "", nil, fmt.Errorf("error cargando máquinas: %w", err)
	}

	opResources := resource.OperationalResources
	sort.Slice(opResources, func(i, j int) bool {
		return opResources[i].ID < opResources[j].ID
	})

	N := len(products)
	M := len(ingredients)
	K := len(machines)
	R := len(opResources)

	prodIDToIndex := make(map[uint]int)
	for i, p := range products {
		prodIDToIndex[p.ID] = i + 1
	}

	ingIDToIndex := make(map[uint]int)
	for j, ing := range ingredients {
		ingIDToIndex[ing.ID] = j + 1
	}

	machIDToIndex := make(map[uint]int)
	for k, m := range machines {
		machIDToIndex[m.ID] = k + 1
	}

	opResIDToIndex := make(map[uint]int)
	for r, opr := range opResources {
		opResIDToIndex[opr.ID] = r + 1
	}

	pValues := make([]float64, N)
	dValues := make([]float64, N)
	liValues := make([]float64, N)
	lsValues := make([]float64, N)

	for i, p := range products {
		pValues[i] = p.SalePrice
		dValues[i] = p.Demand
		liValues[i] = p.MinBatch
		lsValues[i] = p.MaxBatch
	}

	cuValues := make([]float64, M)
	for j, ing := range ingredients {
		cuValues[j] = ing.UnitCost
	}

	inValues := make([]float64, M)
	for _, si := range stock.Ingredients {
		if idx, ok := ingIDToIndex[si.IngredientID]; ok {
			inValues[idx-1] = si.QuantityAvailable
		}
	}

	capValues := make([]float64, K)
	for _, rm := range resource.Machines {
		if idx, ok := machIDToIndex[rm.MachineID]; ok {
			capValues[idx-1] = rm.HoursAvailable * 60.0
		}
	}

	dispValues := make([]float64, R)
	crValues := make([]float64, R)
	for r, opr := range opResources {
		dispValues[r] = opr.Available
		crValues[r] = opr.CostPerUnit
	}

	qMatrix := make([]float64, N*M)
	for i, p := range products {
		for _, pi := range p.Ingredients {
			if jIdx, ok := ingIDToIndex[pi.IngredientID]; ok {
				qMatrix[i*M+(jIdx-1)] = pi.Quantity
			}
		}
	}

	tMatrix := make([]float64, N*K)
	for i, p := range products {
		for _, pm := range p.Machines {
			if kIdx, ok := machIDToIndex[pm.MachineID]; ok {
				tMatrix[i*K+(kIdx-1)] = pm.MinutesPerUnit
			}
		}
	}

	cmMatrix := make([]float64, N*R)
	for i, p := range products {
		for _, por := range p.OperationalResources {
			if rIdx, ok := opResIDToIndex[por.OperationalResourceID]; ok {
				cmMatrix[i*R+(rIdx-1)] = por.ConsumptionPerBatch
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("MODEL:\n")
	sb.WriteString("SETS:\n")
	sb.WriteString(fmt.Sprintf("  PRODUCTOS/1..%d/: P, D, LI, LS, X, Y, W;\n", N))
	sb.WriteString(fmt.Sprintf("  INGREDIENTES/1..%d/: CU, IN;\n", M))
	sb.WriteString(fmt.Sprintf("  MAQUINAS/1..%d/: CAP;\n", K))
	if R > 0 {
		sb.WriteString(fmt.Sprintf("  RECURSOS/1..%d/: DISP, CR;\n", R))
	} else {
		sb.WriteString("  RECURSOS/1..1/: DISP, CR;\n")
	}
	sb.WriteString("  RUTAPI(PRODUCTOS, INGREDIENTES): Q;\n")
	sb.WriteString("  RUTAPM(PRODUCTOS, MAQUINAS): T;\n")
	sb.WriteString("  RUTAPR(PRODUCTOS, RECURSOS): CM;\n")
	sb.WriteString("ENDSETS\n\n")

	sb.WriteString("DATA:\n")
	sb.WriteString(fmt.Sprintf("  P = %s;\n", floatSliceToString(pValues)))
	sb.WriteString(fmt.Sprintf("  D = %s;\n", floatSliceToString(dValues)))
	sb.WriteString(fmt.Sprintf("  LI = %s;\n", floatSliceToString(liValues)))
	sb.WriteString(fmt.Sprintf("  LS = %s;\n", floatSliceToString(lsValues)))
	sb.WriteString(fmt.Sprintf("  CU = %s;\n", floatSliceToString(cuValues)))
	sb.WriteString(fmt.Sprintf("  IN = %s;\n", floatSliceToString(inValues)))
	sb.WriteString(fmt.Sprintf("  CAP = %s;\n", floatSliceToString(capValues)))
	if R > 0 {
		sb.WriteString(fmt.Sprintf("  DISP = %s;\n", floatSliceToString(dispValues)))
		sb.WriteString(fmt.Sprintf("  CR = %s;\n", floatSliceToString(crValues)))
	} else {
		sb.WriteString("  DISP = 0;\n")
		sb.WriteString("  CR = 0;\n")
	}
	sb.WriteString(fmt.Sprintf("  Q = %s;\n", floatSliceToString(qMatrix)))
	sb.WriteString(fmt.Sprintf("  T = %s;\n", floatSliceToString(tMatrix)))
	sb.WriteString(fmt.Sprintf("  CM = %s;\n", floatSliceToString(cmMatrix)))
	sb.WriteString(fmt.Sprintf("  M = %f;\n", opt.MaxProduction))
	sb.WriteString(fmt.Sprintf("  PRO = %d;\n", opt.MinVariety))
	sb.WriteString("ENDDATA\n\n")

	sb.WriteString("!FUNCION OBJETIVO;\n")
	sb.WriteString("MAX = @SUM(PRODUCTOS(I): P(I)*X(I)) -\n")
	sb.WriteString("  @SUM(PRODUCTOS(I): @SUM(INGREDIENTES(J): Q(I,J)*CU(J)*X(I))) -\n")
	if R > 0 {
		sb.WriteString("  @SUM(PRODUCTOS(I): @SUM(RECURSOS(R): CM(I,R)*CR(R)*Y(I)));\n\n")
	} else {
		sb.WriteString("  0;\n\n")
	}

	sb.WriteString("!LIMITE DE INVENTARIO;\n")
	sb.WriteString("@FOR(INGREDIENTES(J): @SUM(PRODUCTOS(I): X(I)*Q(I,J)) <= IN(J));\n\n")

	sb.WriteString("!META DE PRODUCCION;\n")
	sb.WriteString("@SUM(PRODUCTOS(I): X(I)) <= M;\n\n")

	sb.WriteString("!VARIEDAD DE PRODUCCION;\n")
	sb.WriteString("@SUM(PRODUCTOS(I): W(I)) >= PRO;\n\n")

	sb.WriteString("!DEMANDA DE LOS PRODUCTOS;\n")
	sb.WriteString("@FOR(PRODUCTOS(I): W(I) <= X(I));\n")
	sb.WriteString("@FOR(PRODUCTOS(I): X(I) <= D(I)*W(I));\n\n")

	sb.WriteString("!LIMITE INFERIOR DE LOTES;\n")
	sb.WriteString("@FOR(PRODUCTOS(I): X(I) >= LI(I)*Y(I));\n\n")

	sb.WriteString("!LIMITE SUPERIOR DE LOTES;\n")
	sb.WriteString("@FOR(PRODUCTOS(I): X(I) <= LS(I)*Y(I));\n\n")

	if R > 0 {
		sb.WriteString("!LIMITE DE DISPONIBILIDAD DE RECURSOS;\n")
		sb.WriteString("@FOR(RECURSOS(R): @SUM(PRODUCTOS(I): CM(I,R)*Y(I)) <= DISP(R));\n\n")
	}

	sb.WriteString("!CAPACIDAD DE LAS MAQUINAS;\n")
	sb.WriteString("@FOR(MAQUINAS(K): @SUM(PRODUCTOS(I): T(I,K)*Y(I)) <= CAP(K));\n\n")

	sb.WriteString("!VARIABLES ENTERAS Y BINARIAS;\n")
	sb.WriteString("@FOR(PRODUCTOS(I): @GIN(X(I)); @GIN(Y(I)));\n")
	sb.WriteString("@FOR(PRODUCTOS(I): @BIN(W(I)));\n")
	sb.WriteString("END\n")

	return sb.String(), products, nil
}

// RunLINGO ejecuta el solver LINGO usando el wrapper runlingo.
func RunLINGO(ctx context.Context, jobID, modelContent string) (string, error) {
	lingoPath := os.Getenv("LINGO_PATH")
	if lingoPath == "" {
		lingoPath = "/home/dulcinea/lingo20/runlingo"
	}

	scriptContent := modelContent
	if !strings.HasSuffix(scriptContent, "\n") {
		scriptContent += "\n"
	}
	scriptContent += "GO\nQUIT\n"

	// Escribimos el modelo a un archivo temporal
	tmpFile := fmt.Sprintf("/tmp/lingo_%s.lng", jobID)
	err := os.WriteFile(tmpFile, []byte(scriptContent), 0644)
	if err != nil {
		return "", fmt.Errorf("error creando archivo temporal de modelo: %w", err)
	}
	defer os.Remove(tmpFile)

	// En Linux pasamos el script como argumento directo
	cmd := exec.CommandContext(ctx, lingoPath, tmpFile)

	logger.L.Debug().Str("job_id", jobID).Msg("[LINGO] Ejecutando solver...")

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		return string(output), fmt.Errorf("error ejecutando lingo (duración %v): %w", duration, err)
	}

	return string(output), nil
}

// ParseOutput extrae los valores de las variables X, Y, W y el valor objetivo.
func ParseOutput(lingoOutput string, products []models.Product) (*LingoResult, error) {
	if strings.Contains(strings.ToUpper(lingoOutput), "INFEASIBLE") {
		return nil, fmt.Errorf("modelo infactible (no hay solución posible con estos recursos)")
	}
	if strings.Contains(strings.ToUpper(lingoOutput), "UNBOUNDED") {
		return nil, fmt.Errorf("modelo no acotado (ganancia infinita, revisa las restricciones)")
	}

	result := &LingoResult{
		X: make(map[uint]float64),
		Y: make(map[uint]float64),
		W: make(map[uint]float64),
	}

	objRe := regexp.MustCompile(`(?i)Objective\s+value:\s+([0-9\.\-\+eE]+)`)
	if match := objRe.FindStringSubmatch(lingoOutput); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			result.ObjectiveValue = val
		}
	}

	re := regexp.MustCompile(`(?i)\b(X|Y|W)\s*\(?\s*(\d+)\s*\)?\s+([0-9\.\-\+eE]+)`)
	matches := re.FindAllStringSubmatch(lingoOutput, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no se encontraron variables de decisión en el output de LINGO")
	}

	for _, match := range matches {
		varName := strings.ToUpper(match[1])
		idx, _ := strconv.Atoi(match[2])
		val, _ := strconv.ParseFloat(match[3], 64)

		if idx > 0 && idx <= len(products) {
			productID := products[idx-1].ID
			switch varName {
			case "X":
				result.X[productID] = val
			case "Y":
				result.Y[productID] = val
			case "W":
				result.W[productID] = val
			}
		}
	}

	return result, nil
}
