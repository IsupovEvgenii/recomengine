package engine

import "time"

type (
	UserOrders struct {
		UserID string
		Orders []Order
	}

	Order struct {
		Products  []Product
		CreatedAt time.Time
	}

	Product struct {
		ID string
	}
)
