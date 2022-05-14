package main

import (
	"fmt"
	"github.com/recomengine/engine"
	"github.com/recomengine/postgres"
	"time"
)

func main() {
	dsn := "user=postgres password=hg56LK76h0 dbname=store sslmode=disable binary_parameters=yes"
	dbConn, err := postgres.New(dsn, dsn, 5)
	if err != nil {
		fmt.Println("connections failed")

		return
	}
	masterDb := dbConn.MasterX()
	query := `select user_id,
			   order_id,
			   product_id
		
		from order_items
				 inner join orders o on order_items.order_id = o.id
		group by order_id, o.user_id, order_items.product_id, o.created_at
		order by o.user_id, order_id, o.created_at`
	type UserOrderProduct struct {
		UserID    string `db:"user_id"`
		OrderID   string `db:"order_id"`
		ProductID string `db:"product_id"`
	}
	var result []UserOrderProduct
	if err = masterDb.Select(&result, query); err != nil {
		fmt.Println("select failed")
		return
	}

	var userOrders []engine.UserOrders
	curUserID := ""
	curOrderID := ""
	var orderProducts []engine.Product
	var orders []engine.Order
	for _, res := range result {
		if curUserID != res.UserID {
			if curUserID != "" {

				userOrders = append(userOrders, engine.UserOrders{
					UserID: curUserID,
					Orders: orders,
				})
				orders = []engine.Order{}
			}
			curUserID = res.UserID
		}
		if curOrderID != res.OrderID {
			if curOrderID != "" {

				orders = append(orders, engine.Order{
					Products:  orderProducts,
					CreatedAt: time.Now(),
				})
				orderProducts = []engine.Product{}

			}
			curOrderID = res.OrderID

		}
		orderProducts = append(orderProducts, engine.Product{
			ID: res.ProductID,
		})
	}

	type Product struct {
		ID   string `db:"id"`
		Name string `db:"name_en"`
	}
	var products []Product
	query2 := `
select distinct id, name_en from products
`
	if err = masterDb.Select(&products, query2); err != nil {
		fmt.Println("failed select 2")
		return
	}

	var engineProducts []engine.Product
	productMap := make(map[string]string)
	for _, product := range products {
		engineProducts = append(engineProducts, engine.Product{
			ID: product.ID,
		})
		productMap[product.ID] = product.Name
	}

	var test []engine.UserOrders
	for i := range userOrders {
		if len(userOrders[i].Orders) > 1 {
			test = append(test, engine.UserOrders{
				UserID: userOrders[i].UserID,
				Orders: userOrders[i].Orders[len(userOrders[i].Orders)-1:],
			})
			userOrders[i].Orders = userOrders[i].Orders[:len(userOrders[i].Orders)-1]

		}
	}

	myEngine := engine.InitEngine(engineProducts, userOrders, 64)
	if err = myEngine.ComputeModel(); err != nil {
		fmt.Println("failed computed model")
		return
	}

	sucessCases := 0
	allCases := 0
	for _, t := range test {
		var productIDs []string
		if len(t.Orders[0].Products) > 1 {
			fmt.Println("CART:")
			for i := 0; i < len(t.Orders[0].Products)-1; i++ {
				productIDs = append(productIDs, t.Orders[0].Products[i].ID)
				fmt.Println(productMap[t.Orders[0].Products[i].ID])
			}
			fmt.Println("CHECKED_PRODUCT: ", productMap[t.Orders[0].Products[len(t.Orders[0].Products)-1].ID])

			resultIDs := myEngine.GetRecomProducts(t.UserID, productIDs)
			fmt.Println("RECOM:")
			for _, res := range resultIDs {
				fmt.Println(productMap[res])
			}

			success := false
			for _, resultID := range resultIDs {
				if resultID == t.Orders[0].Products[len(t.Orders[0].Products)-1].ID {
					success = true
					sucessCases++
				}

			}
			fmt.Println("SUCCESS=", success)
			fmt.Println("__________")
			allCases++
		}

	}
	fmt.Println("FINAL sucess cases ", sucessCases, " of ", allCases)

}
