package engine

import (
	"fmt"
	"gonum.org/v1/gonum/mat"
	"math"
	"sort"
)

type Engine struct {
	products          []Product
	userOrders        []UserOrders
	dParam            int64
	U1                mat.Matrix
	I1                mat.Matrix
	I2                mat.Matrix
	mapVectorProducts map[string]int
	mapProductVector  map[int]string
	mapVectorUsers    map[string]int
}

func InitEngine(
	products []Product,
	userOrders []UserOrders,
	dParam int64,
) *Engine {
	return &Engine{
		products:   products,
		userOrders: userOrders,
		dParam:     dParam,
	}
}

func (e *Engine) GetRecomProducts(userID string, productIDs []string) []string {
	var vecUser []float64
	if _, ok := e.mapVectorUsers[userID]; ok {
		mat.Row(vecUser, e.mapVectorUsers[userID], e.U1)
	}

	vecProducts := make([][]float64, 0, len(productIDs))
	for _, productID := range productIDs {
		var cur []float64
		vecProducts = append(vecProducts, mat.Row(cur, e.mapVectorProducts[productID], e.I2))
	}

	var actual []float64
	E := mat.NewVecDense(int(e.dParam), actual)
	if len(vecUser) > 0 {
		E.ScaleVec(0.5, mat.NewVecDense(len(vecUser), vecUser))
	}
	var actual2 []float64
	E2 := mat.NewVecDense(int(e.dParam), actual2)
	if len(vecProducts) > 0 {
		for _, vecProduct := range vecProducts {
			E2.AddVec(E2, mat.NewVecDense(len(vecProduct), vecProduct))
		}
		E2.ScaleVec(1.0/float64(len(vecProducts)), E2)
	}

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

	sort.Slice(resultishe, func(i, j int) bool {
		return resultishe[i].Score > resultishe[j].Score
	})

	var super []string
	if len(resultishe) > 30 {
		for i := 0; i < 30; i++ {
			super = append(super, resultishe[i].ProductID)
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
		e.mapVectorProducts[user.UserID] = i
	}

	// user-item
	var ui [][]float64
	max := 0.0
	for _, u := range e.userOrders {
		cur := make([]float64, len(e.products))
		for _, o := range u.Orders {
			for _, i := range o.Products {
				cur[e.mapVectorProducts[i.ID]]++
				if cur[e.mapVectorProducts[i.ID]] > max {
					max = cur[e.mapVectorProducts[i.ID]]
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
						ii[e.mapVectorProducts[o.Products[i].ID]][e.mapVectorProducts[o.Products[j].ID]]++
						if ii[e.mapVectorProducts[o.Products[i].ID]][e.mapVectorProducts[o.Products[j].ID]] > max {
							max = ii[e.mapVectorProducts[o.Products[i].ID]][e.mapVectorProducts[o.Products[j].ID]]
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
