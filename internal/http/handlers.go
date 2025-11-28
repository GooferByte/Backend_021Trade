package http

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/GooferByte/Backend_021Trade/internal/models"
	"github.com/GooferByte/Backend_021Trade/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// Router wires all handlers.
func Router(rewardSvc *service.RewardService, logger *logrus.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logMiddleware(logger))

	r.POST("/reward", func(c *gin.Context) {
		handleCreateReward(c, rewardSvc)
	})
	r.GET("/today-stocks/:userId", func(c *gin.Context) {
		handleTodayStocks(c, rewardSvc)
	})
	r.GET("/historical-inr/:userId", func(c *gin.Context) {
		handleHistorical(c, rewardSvc)
	})
	r.GET("/stats/:userId", func(c *gin.Context) {
		handleStats(c, rewardSvc)
	})
	r.GET("/portfolio/:userId", func(c *gin.Context) {
		handlePortfolio(c, rewardSvc)
	})
	return r
}

type rewardRequest struct {
	UserID     string     `json:"userId" binding:"required"`
	Symbol     string     `json:"symbol" binding:"required"`
	Quantity   string     `json:"quantity" binding:"required"`
	RewardedAt *time.Time `json:"rewardedAt"`
	EventID    string     `json:"eventId"`
	Fees       feeRequest `json:"fees"`
	Adjustment bool       `json:"adjustment"`
}

type feeRequest struct {
	Brokerage string `json:"brokerage"`
	STT       string `json:"stt"`
	GST       string `json:"gst"`
	Other     string `json:"other"`
}

func handleCreateReward(c *gin.Context, svc *service.RewardService) {
	var req rewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	qty, err := decimal.NewFromString(req.Quantity)
	if err != nil || qty.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be a positive decimal string"})
		return
	}

	fees, err := parseFees(req.Fees)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	evt, err := svc.CreateReward(c.Request.Context(), service.CreateRewardInput{
		UserID:         req.UserID,
		Symbol:         req.Symbol,
		Quantity:       qty,
		RewardedAt:     derefTime(req.RewardedAt),
		IdempotencyKey: req.EventID,
		Fees:           fees,
		IsAdjustment:   req.Adjustment,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrValidation) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, service.ErrDuplicate) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"rewardId":     evt.ID,
		"userId":       evt.UserID,
		"symbol":       evt.Symbol,
		"quantity":     evt.Quantity.String(),
		"rewardedAt":   evt.RewardedAt,
		"totalInrCost": evt.TotalINRCost.StringFixed(4),
	})
}

func handleTodayStocks(c *gin.Context, svc *service.RewardService) {
	userID := c.Param("userId")
	rewards, err := svc.GetTodayRewards(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := []gin.H{}
	for _, r := range rewards {
		resp = append(resp, gin.H{
			"id":         r.ID,
			"symbol":     r.Symbol,
			"quantity":   r.Quantity.String(),
			"rewardedAt": r.RewardedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"rewards": resp})
}

func handleHistorical(c *gin.Context, svc *service.RewardService) {
	userID := c.Param("userId")
	values, err := svc.GetHistoricalINR(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := []gin.H{}
	for _, v := range values {
		resp = append(resp, gin.H{
			"date":     v.Date,
			"totalInr": v.TotalINR.StringFixed(2),
		})
	}
	c.JSON(http.StatusOK, gin.H{"days": resp})
}

func handleStats(c *gin.Context, svc *service.RewardService) {
	userID := c.Param("userId")
	stats, err := svc.GetStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totals := gin.H{}
	for symbol, qty := range stats.TotalSharesToday {
		totals[symbol] = qty.String()
	}
	c.JSON(http.StatusOK, gin.H{
		"totalSharesToday":  totals,
		"portfolioValueInr": stats.PortfolioValue.StringFixed(2),
	})
}

func handlePortfolio(c *gin.Context, svc *service.RewardService) {
	userID := c.Param("userId")
	positions, err := svc.GetPortfolio(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := []gin.H{}
	for _, p := range positions {
		resp = append(resp, gin.H{
			"symbol":   p.Symbol,
			"quantity": p.Quantity.String(),
			"price":    p.Price.StringFixed(2),
			"valueInr": p.ValueINR.StringFixed(2),
		})
	}
	c.JSON(http.StatusOK, gin.H{"positions": resp})
}

func parseFees(req feeRequest) (models.FeeBreakdown, error) {
	fields := map[string]string{
		"brokerage": req.Brokerage,
		"stt":       req.STT,
		"gst":       req.GST,
		"other":     req.Other,
	}
	res := models.FeeBreakdown{}
	for name, val := range fields {
		if val == "" {
			continue
		}
		num, err := decimal.NewFromString(val)
		if err != nil {
			return res, fmt.Errorf("%s must be a decimal string", name)
		}
		switch name {
		case "brokerage":
			res.Brokerage = num
		case "stt":
			res.STT = num
		case "gst":
			res.GST = num
		case "other":
			res.Other = num
		}
	}
	return res, nil
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func logMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.WithFields(logrus.Fields{
			"status":   c.Writer.Status(),
			"method":   c.Request.Method,
			"path":     c.Request.URL.Path,
			"latency":  time.Since(start).String(),
			"clientIP": c.ClientIP(),
		}).Info("request completed")
	}
}
