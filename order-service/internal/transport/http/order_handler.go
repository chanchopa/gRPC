package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"order-service/internal/usecase"
)

type OrderHandler struct {
	uc *usecase.OrderUseCase
}

func NewOrderHandler(uc *usecase.OrderUseCase) *OrderHandler {
	return &OrderHandler{uc: uc}
}

func (h *OrderHandler) RegisterRoutes(r *gin.Engine) {
	r.POST("/orders", h.CreateOrder)
	r.GET("/orders/:id", h.GetOrder)
	r.PATCH("/orders/:id/cancel", h.CancelOrder)
}

type createOrderRequest struct {
	CustomerID    string `json:"customer_id" binding:"required"`
	CustomerEmail string `json:"customer_email" binding:"required,email"`
	ItemName      string `json:"item_name" binding:"required"`
	Amount        int64  `json:"amount" binding:"required"`
}

type orderResponse struct {
	ID            string `json:"id"`
	CustomerID    string `json:"customer_id"`
	CustomerEmail string `json:"customer_email"`
	ItemName      string `json:"item_name"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")

	input := usecase.CreateOrderInput{
		CustomerID:     req.CustomerID,
		CustomerEmail:  req.CustomerEmail,
		ItemName:       req.ItemName,
		Amount:         req.Amount,
		IdempotencyKey: idempotencyKey,
	}

	order, err := h.uc.CreateOrder(c.Request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "payment service unavailable") {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment service unavailable"})
			return
		}
		if strings.Contains(err.Error(), "validation error") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, orderResponse{
		ID:            order.ID,
		CustomerID:    order.CustomerID,
		CustomerEmail: order.CustomerEmail,
		ItemName:      order.ItemName,
		Amount:        order.Amount,
		Status:        order.Status,
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	order, err := h.uc.GetOrder(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orderResponse{
		ID:            order.ID,
		CustomerID:    order.CustomerID,
		CustomerEmail: order.CustomerEmail,
		ItemName:      order.ItemName,
		Amount:        order.Amount,
		Status:        order.Status,
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	order, err := h.uc.CancelOrder(c.Request.Context(), id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, orderResponse{
		ID:            order.ID,
		CustomerID:    order.CustomerID,
		CustomerEmail: order.CustomerEmail,
		ItemName:      order.ItemName,
		Amount:        order.Amount,
		Status:        order.Status,
		CreatedAt:     order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
