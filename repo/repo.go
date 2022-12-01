package repo

import (
	"context"
	"fmt"
	"github.com/sons-of-titus/go-service/db"
	"github.com/sons-of-titus/go-service/models"
	"github.com/sons-of-titus/go-service/stats"
	"math"
)

type repo struct {
	products  *db.ProductDB
	orders    *db.OrderDB
	incoming  chan models.Order
	stats     stats.StatsService
	done      chan struct{}
	processed chan models.Order
}

// Repo is the interface we expose to outside packages
type Repo interface {
	CreateOrder(item models.Item) (*models.Order, error)
	GetAllProducts() []models.Product
	GetOrder(id string) (models.Order, error)
	RequestReversal(id string) (*models.Order, error)
	Close()
	GetOrderStats(ctx context.Context) (models.Statistics, error)
}

// New creates a new Order repo with the correct database dependencies
func New() (Repo, error) {
	processed := make(chan models.Order, stats.WorkerCount)
	done := make(chan struct{})
	p, err := db.NewProducts()
	if err != nil {
		return nil, err
	}
	statsService := stats.New(processed, done)
	o := repo{
		products:  p,
		orders:    db.NewOrders(),
		stats:     statsService,
		incoming:  make(chan models.Order),
		done:      done,
		processed: processed,
	}
	//start the order processor
	go o.processOrders()
	return &o, nil
}

// GetAllProducts returns all products in the system
func (r *repo) GetAllProducts() []models.Product {
	return r.products.FindAll()
}

// GetOrder returns the given order if one exists
func (r *repo) GetOrder(id string) (models.Order, error) {
	return r.orders.Find(id)
}

// CreateOrder creates a new order for the given item
func (r *repo) CreateOrder(item models.Item) (*models.Order, error) {
	if err := r.validateItem(item); err != nil {
		return nil, err
	}
	order := models.NewOrder(item)
	//place the order on the incoming orders channel or fail if the app is closed
	select {
	case r.incoming <- order:
		r.orders.Upsert(order)
		return &order, nil
	case <-r.done:
		return nil, fmt.Errorf("order app is closed, try again later")
	}

}

// validateItem runs validations on a given order
func (r *repo) validateItem(item models.Item) error {
	if item.Amount < 1 {
		return fmt.Errorf("order amount must be at least 1:got %d", item.Amount)
	}
	if err := r.products.Exists(item.ProductID); err != nil {
		return fmt.Errorf("product %s does not exist", item.ProductID)
	}
	return nil
}

func (r *repo) processOrders() {
	fmt.Println("Order processing started!")
	for {
		select {
		case order := <-r.incoming:
			r.processOrder(&order)
			r.orders.Upsert(order)
			fmt.Printf("Processing order %s completed\n", order.ID)
			r.processed <- order
		case <-r.done:
			fmt.Println("Order processing stopped!")
			return
		}
	}
}

// processOrder is an internal method which completes or rejects an order
func (r *repo) processOrder(order *models.Order) {
	// ensure the order is still completed
	fetchedOrder, err := r.orders.Find(order.ID)
	if err != nil || fetchedOrder.Status != models.OrderStatusCompleted {
		fmt.Println("duplicate reversal on order ", order.ID)
	}
	item := order.Item
	if order.Status == models.OrderStatusReversalRequested {
		item.Amount = -item.Amount
	}
	product, err := r.products.Find(item.ProductID)
	if err != nil {
		order.Status = models.OrderStatusRejected
		order.Error = err.Error()
		return
	}
	if product.Stock < item.Amount {
		order.Status = models.OrderStatusRejected
		order.Error = fmt.Sprintf("not enough stock for product %s:got %d, want %d", item.ProductID, product.Stock, item.Amount)
		return
	}
	product.Stock = product.Stock - item.Amount
	r.products.Upsert(product)
	total := math.Round(float64(order.Item.Amount)*product.Price*100) / 100
	order.Total = &total
	order.Complete()
}

// RequestReversal fetches an existing order and updates it for reversal
func (r *repo) RequestReversal(id string) (*models.Order, error) {
	// tries to find the order first
	order, err := r.orders.Find(id)
	if err != nil {
		return nil, err
	}
	if order.Status != models.OrderStatusCompleted {
		return nil, fmt.Errorf("order status is %s, only completed orders can be reverted ", order.Status)
	}
	// set order status to reversalRequested
	order.Status = models.OrderStatusReversalRequested
	//place order on the incoming channel
	select {
	case r.incoming <- order:
		r.orders.Upsert(order)
		return &order, nil
	case <-r.done:
		return nil, fmt.Errorf("sorry, the order app is closed")
	}
}

// Close closes the orders app for incoming orders
func (r *repo) Close() {
	close(r.done)
}

// GetOrderStats returns the order statistics of the orders app
func (r *repo) GetOrderStats(ctx context.Context) (models.Statistics, error) {
	select {
	case s := <-r.stats.GetStats(ctx):
		return s, nil
	case <-ctx.Done():
		return models.Statistics{}, ctx.Err()
	}
}
