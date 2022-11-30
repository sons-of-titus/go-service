package models

import (
	"github.com/google/uuid"
	"time"
)

type OrderStatus string

const (
	OrderStatusNew       OrderStatus = "New"
	OrderStatusCompleted OrderStatus = "Completed"
	OrderStatusRejected  OrderStatus = "Rejected"
)

const timeFormat = "2006-01-02 15:04:05.000"

type Order struct {
	ID        string      `json:"id,omitempty"`
	Item      Item        `json:"item"`
	Total     *float64    `json:"total,omitempty"`
	Status    OrderStatus `json:"status,omitempty"`
	Error     string      `json:"error,omitempty"`
	CreatedAt string      `json:"createdAt,omitempty"`
}

type Item struct {
	ProductID string `json:"productId"`
	Amount    int    `json:"amount"`
}

func NewOrder(item Item) Order {
	return Order{
		ID:        uuid.New().String(),
		Status:    OrderStatusNew,
		CreatedAt: time.Now().Format(timeFormat),
		Item:      item,
	}
}

func (o *Order) Complete() {
	o.Status = OrderStatusCompleted
}
