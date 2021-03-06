package engine

import (
	"fmt"
	"gonum.org/v1/gonum/mat"
	"math"
	"sort"
)

type Engine struct {
	products             []Product
	userOrders           []UserOrders
	mapProductCategories map[string]string
	dParam               int64
	U1                   mat.Matrix
	I1                   mat.Matrix
	I2                   mat.Matrix
	mapVectorProducts    map[string]int
	mapProductVector     map[int]string
	mapVectorUsers       map[string]int
}

func InitEngine(
	products []Product,
	userOrders []UserOrders,
	dParam int64,
	minUserOrders int64,
	minPopularScoreProduct int64,
) *Engine {
	popularScoreProducts := make(map[string]int64)
	for _, userOrder := range userOrders {
		for _, order := range userOrder.Orders {
			for _, product := range order.Products {
				popularScoreProducts[product.ID]++
			}
		}
	}
	var newProducts []Product
	mapProductCategories := make(map[string]string)
	for _, product := range products {
		if score, ok := popularScoreProducts[product.ID]; ok && score >= minPopularScoreProduct {
			newProducts = append(newProducts, product)
			mapProductCategories[product.ID] = product.CategoryID
		}
	}

	var newUserOrders []UserOrders
	for _, userOrder := range userOrders {
		if len(userOrder.Orders) >= int(minUserOrders) {
			newUserOrders = append(newUserOrders, userOrder)
		}
	}

	return &Engine{
		products:             newProducts,
		userOrders:           newUserOrders,
		dParam:               dParam,
		mapProductCategories: mapProductCategories,
	}
}

func (e *Engine) GetRecomProducts(userID string, productIDs []string) []string {
	var vecUser []float64
	if _, ok := e.mapVectorUsers[userID]; ok {
		vecUser = mat.Row(vecUser, e.mapVectorUsers[userID], e.U1)
	}
	//fmt.Println(e.U1)

	vecProducts := make([][]float64, 0, len(productIDs))
	for _, productID := range productIDs {
		var cur []float64
		if _, ok := e.mapVectorProducts[productID]; ok {
			vecProducts = append(vecProducts, mat.Row(cur, e.mapVectorProducts[productID], e.I2))
		}
	}

	var actual []float64
	E := mat.NewVecDense(int(e.dParam), actual)
	//fmt.Println(vecUser)
	if len(vecUser) > 0 {
		E.ScaleVec(0.1, mat.NewVecDense(len(vecUser), vecUser))
	}
	var actual2 []float64
	E2 := mat.NewVecDense(int(e.dParam), actual2)
	if len(vecProducts) > 0 {
		for _, vecProduct := range vecProducts {
			E2.AddVec(E2, mat.NewVecDense(len(vecProduct), vecProduct))
		}
		E2.ScaleVec(1.0/float64(len(vecProducts)), E2)
	}
	E2.ScaleVec(0.5, E2)

	E.AddVec(E, E2)

	_, c := e.I1.Dims()
	var resVec []float64
	scores := mat.NewVecDense(c, resVec)
	scores.MulVec(e.I1.T(), E)

	type ProductScore struct {
		ProductID string
		Score     float64
	}
	var resultishe []ProductScore
	resVec = mat.Col(resVec, 0, scores)
	for i, res := range resVec {
		resultishe = append(resultishe, ProductScore{
			ProductID: e.mapProductVector[i],
			Score:     res,
		})

	}

	var newResultishe []ProductScore
	for _, re := range resultishe {
		isExist := false
		for _, productID := range productIDs {
			if productID == re.ProductID {
				isExist = true
			}
		}
		if !isExist {
			newResultishe = append(newResultishe, re)
		}

	}

	sort.Slice(newResultishe, func(i, j int) bool {
		return newResultishe[i].Score > newResultishe[j].Score
	})

	//var newNewResultishe []ProductScore
	//for _, re := range newResultishe {
	//	isSimCategory := false
	//	for _, productID := range productIDs {
	//		if e.mapProductCategories[re.ProductID] == e.mapProductCategories[productID] {
	//			isSimCategory = true
	//		}
	//	}
	//
	//	if !isSimCategory {
	//		newNewResultishe = append(newNewResultishe, re)
	//	}
	//
	//}

	var super []string
	lenArray := 0
	i := 0
	//j := 0
	for lenArray < 20 {
		if len(super) == 0 {
			super = append(super, newResultishe[i].ProductID)
			lenArray++
		}
		//isExistJ := false
		//for _, sup := range super {
		//	if newNewResultishe[j].ProductID == sup {
		//		j++
		//		isExistJ = true
		//		break
		//	}
		//}
		isExistI := false
		for _, sup := range super {
			if newResultishe[i].ProductID == sup {
				i++
				isExistI = true
				break
			}
		}

		//if !isExistJ {
		//	super = append(super, newNewResultishe[j].ProductID)
		//	lenArray++
		//}
		if !isExistI {
			super = append(super, newResultishe[i].ProductID)
			lenArray++
		}
	}

	return super
}

func (e *Engine) ComputeModel() error {
	e.mapProductVector = make(map[int]string)
	e.mapVectorProducts = make(map[string]int)
	for i, product := range e.products {
		e.mapVectorProducts[product.ID] = i
		e.mapProductVector[i] = product.ID
	}
	e.mapVectorUsers = make(map[string]int)
	for i, user := range e.userOrders {
		e.mapVectorUsers[user.UserID] = i
	}

	// user-item
	var ui [][]float64
	max := 0.0
	for _, u := range e.userOrders {
		cur := make([]float64, len(e.products))
		for _, o := range u.Orders {
			for _, i := range o.Products {
				if _, ok := e.mapVectorProducts[i.ID]; ok {
					cur[e.mapVectorProducts[i.ID]] += float64(i.Count)
					if cur[e.mapVectorProducts[i.ID]] > max {
						max = cur[e.mapVectorProducts[i.ID]]
					}
				}
			}
		}
		ui = append(ui, cur)
	}
	// normalize
	for i := 0; i < len(ui); i++ {
		for j := 0; j < len(ui[i]); j++ {
			ui[i][j] /= max
		}
	}

	// item-item
	var ii [][]float64
	for i := 0; i < len(e.products); i++ {
		cur := make([]float64, len(e.products))
		ii = append(ii, cur)
	}
	max = 0.0
	for _, u := range e.userOrders {
		for _, o := range u.Orders {
			for i := 0; i < len(o.Products)-1; i++ {
				if i+1 < len(o.Products) {
					for j := i + 1; j < len(o.Products); j++ {
						if _, ok := e.mapVectorProducts[o.Products[i].ID]; ok {
							if _, ok := e.mapVectorProducts[o.Products[j].ID]; ok {
								ii[e.mapVectorProducts[o.Products[i].ID]][e.mapVectorProducts[o.Products[j].ID]]++
								if ii[e.mapVectorProducts[o.Products[i].ID]][e.mapVectorProducts[o.Products[j].ID]] > max {
									max = ii[e.mapVectorProducts[o.Products[i].ID]][e.mapVectorProducts[o.Products[j].ID]]
								}
							}

						}

					}
				}
			}
		}
	}
	// normalize
	for i := 0; i < len(ii); i++ {
		for j := 0; j < len(ii[i]); j++ {
			ii[i][j] /= max
		}
	}
	var uiArray []float64
	for _, u := range ui {
		uiArray = append(uiArray, u...)
	}
	var iiArray []float64
	for _, i := range ii {
		iiArray = append(iiArray, i...)
	}
	UI := mat.NewDense(len(ui), len(e.products), uiArray)
	II := mat.NewDense(len(e.products), len(e.products), iiArray)
	svd1 := mat.SVD{}
	if ok := svd1.Factorize(UI, mat.SVDThin); !ok {
		fmt.Println("can not factorize")
	}
	_s, _u, _vh := extractSVD(&svd1)
	e.I1 = _vh.Slice(0, int(e.dParam), 0, len(e.products))
	cur1 := _u.Slice(0, len(e.userOrders), 0, int(e.dParam))

	cur2 := mat.NewDiagDense(int(e.dParam), _s[:e.dParam])
	r, c := cur1.Dims()
	actual := make([]float64, len(e.userOrders)*int(e.dParam))
	U1 := mat.NewDense(r, c, actual)
	U1.Mul(cur1, cur2)
	e.U1 = U1
	var svd mat.SVD
	svd.Factorize(e.I1, mat.SVDThin)
	svdS, svdU, svdV := extractSVD(&svd)

	cutoff := getCutoff(svdS)
	for i := range svdS {
		if svdS[i] > cutoff {
			svdS[i] = 1.0 / svdS[i]
		} else {
			svdS[i] = 0.0
		}
	}
	svdUt := svdU.T()
	utn, utm := svdUt.Dims()
	b := newArray(svdS, utn, utm)
	b.MulElem(b, svdUt)
	var pinvI1 mat.Dense
	pinvI1.Mul(svdV, b)

	var I2 mat.Dense
	I2.Mul(II, &pinvI1)
	e.I2 = &I2

	return nil
}

func MPInverse(a *mat.Dense) mat.Dense {
	var svd mat.SVD
	svd.Factorize(a, mat.SVDThin)
	svdS, svdU, svdV := extractSVD(&svd)

	cutoff := getCutoff(svdS)
	for i := range svdS {
		if svdS[i] > cutoff {
			svdS[i] = 1.0 / svdS[i]
		} else {
			svdS[i] = 0.0
		}
	}
	svdUt := svdU.T()
	utn, utm := svdUt.Dims()
	b := newArray(svdS, utn, utm)
	b.MulElem(b, svdUt)
	var ib mat.Dense
	ib.Mul(svdV, b)
	return ib
}

func newArray(svdS []float64, utn, utm int) *mat.Dense {
	S := make([]float64, utn*utm)
	k := 0
	for i := 0; i < utn; i++ {
		for j := 0; j < utm; j++ {
			S[k] = svdS[i]
			k++
		}
	}
	return mat.NewDense(utn, utm, S)
}

func getCutoff(svdS []float64) float64 {
	v1 := svdS[0]
	for _, v := range svdS {
		v1 = math.Max(v, v1)
	}
	return 1e-15 * v1
}

func extractSVD(svd *mat.SVD) (s []float64, u, v *mat.Dense) {
	u = &mat.Dense{}
	svd.UTo(u)
	v = &mat.Dense{}
	svd.VTo(v)
	return svd.Values(nil), u, v
}
