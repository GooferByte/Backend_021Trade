package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/GooferByte/Backend_021Trade/internal/models"
	"github.com/GooferByte/Backend_021Trade/internal/pricing"
	"github.com/GooferByte/Backend_021Trade/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

var (
	ErrValidation = errors.New("validation_error")
	ErrDuplicate  = repository.ErrDuplicateReward
)

// RewardService coordinates reward creation and valuation logic.
type RewardService struct {
	repo      repository.RewardRepository
	priceSvc  pricing.Service
	now       func() time.Time
	logger    *logrus.Entry
	precision int32
}

// NewRewardService builds a RewardService with sane defaults.
func NewRewardService(repo repository.RewardRepository, priceSvc pricing.Service, logger *logrus.Logger) *RewardService {
	return &RewardService{
		repo:      repo,
		priceSvc:  priceSvc,
		now:       func() time.Time { return time.Now().UTC() },
		logger:    logger.WithField("component", "reward-service"),
		precision: 6,
	}
}

// CreateRewardInput is the DTO consumed by the service.
type CreateRewardInput struct {
	UserID         string
	Symbol         string
	Quantity       decimal.Decimal
	RewardedAt     time.Time
	IdempotencyKey string
	Fees           models.FeeBreakdown
	IsAdjustment   bool
}

// StatsResponse collates stats for /stats endpoint.
type StatsResponse struct {
	TotalSharesToday map[string]decimal.Decimal
	PortfolioValue   decimal.Decimal
}

// HistoricalDayValue captures historical INR valuation for a day.
type HistoricalDayValue struct {
	Date     string
	TotalINR decimal.Decimal
}

func (s *RewardService) CreateReward(ctx context.Context, input CreateRewardInput) (*models.RewardEvent, error) {
	if input.UserID == "" || input.Symbol == "" || input.Quantity.IsZero() {
		return nil, fmt.Errorf("%w: userId, symbol and non-zero quantity are required", ErrValidation)
	}
	if input.Quantity.Sign() < 0 && !input.IsAdjustment {
		return nil, fmt.Errorf("%w: negative quantities are only allowed for adjustments/refunds", ErrValidation)
	}
	rewardedAt := input.RewardedAt
	if rewardedAt.IsZero() {
		rewardedAt = s.now()
	}
	if existing, _ := s.repo.FindByIdempotencyKey(ctx, input.UserID, input.IdempotencyKey); existing != nil {
		return existing, ErrDuplicate
	}

	priceQuote, err := s.priceSvc.GetLatestPrice(ctx, input.Symbol)
	if err != nil {
		return nil, err
	}
	unitPrice := priceQuote.Price
	totalPrice := unitPrice.Mul(input.Quantity)
	fees := input.Fees.Total()
	totalCost := totalPrice.Add(fees)

	reward := models.RewardEvent{
		ID:              uuid.NewString(),
		UserID:          input.UserID,
		Symbol:          input.Symbol,
		Quantity:        input.Quantity,
		RewardedAt:      rewardedAt,
		IdempotencyKey:  input.IdempotencyKey,
		Fees:            input.Fees,
		TotalINRCost:    totalCost,
		PricedAt:        priceQuote.Timestamp,
		UnitPriceINR:    unitPrice,
		CorporateAction: "",
	}

	if err := s.repo.CreateReward(ctx, reward); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertLedgerEntries(ctx, s.buildLedgerEntries(reward)); err != nil {
		return nil, err
	}
	return &reward, nil
}

func (s *RewardService) buildLedgerEntries(reward models.RewardEvent) []models.LedgerEntry {
	now := s.now()
	priceComponent := reward.UnitPriceINR.Mul(reward.Quantity)
	feeTotal := reward.Fees.Total()
	total := reward.TotalINRCost
	absPrice := priceComponent.Abs()
	absTotal := total.Abs()

	inventoryType := "debit"
	if reward.Quantity.Sign() < 0 {
		inventoryType = "credit"
	}
	cashType := "credit"
	if total.Sign() < 0 {
		cashType = "debit"
	}
	return []models.LedgerEntry{
		{
			ID:        uuid.NewString(),
			EventID:   reward.ID,
			UserID:    reward.UserID,
			Account:   "stock_inventory",
			Symbol:    reward.Symbol,
			Units:     reward.Quantity,
			AmountINR: absPrice,
			EntryType: inventoryType,
			CreatedAt: now,
		},
		{
			ID:        uuid.NewString(),
			EventID:   reward.ID,
			UserID:    reward.UserID,
			Account:   "fees_expense",
			Symbol:    reward.Symbol,
			Units:     decimal.Zero,
			AmountINR: feeTotal.Abs(),
			EntryType: "debit",
			CreatedAt: now,
		},
		{
			ID:        uuid.NewString(),
			EventID:   reward.ID,
			UserID:    reward.UserID,
			Account:   "cash",
			Symbol:    reward.Symbol,
			Units:     decimal.Zero,
			AmountINR: absTotal,
			EntryType: cashType,
			CreatedAt: now,
		},
	}
}

func (s *RewardService) GetTodayRewards(ctx context.Context, userID string) ([]models.RewardEvent, error) {
	return s.repo.ListRewardsByUserAndDate(ctx, userID, s.now())
}

func (s *RewardService) GetHistoricalINR(ctx context.Context, userID string) ([]HistoricalDayValue, error) {
	now := s.now()
	rewards, err := s.repo.ListRewardsBeforeDate(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	byDate := map[string]map[string]decimal.Decimal{}
	for _, evt := range rewards {
		day := startOfDay(evt.RewardedAt).Format("2006-01-02")
		if _, ok := byDate[day]; !ok {
			byDate[day] = make(map[string]decimal.Decimal)
		}
		byDate[day][evt.Symbol] = byDate[day][evt.Symbol].Add(evt.Quantity)
	}

	result := []HistoricalDayValue{}
	for day, positions := range byDate {
		parsed, _ := time.Parse("2006-01-02", day)
		total := decimal.Zero
		for symbol, qty := range positions {
			price, err := s.priceSvc.GetHistoricalPrice(ctx, symbol, parsed)
			if err != nil {
				s.logger.WithError(err).WithFields(logrus.Fields{"symbol": symbol, "date": day}).Warn("failed to fetch historical price, using 0")
				continue
			}
			total = total.Add(price.Mul(qty))
		}
		result = append(result, HistoricalDayValue{Date: day, TotalINR: total})
	}
	s.sortHistorical(result)
	return result, nil
}

func (s *RewardService) GetStats(ctx context.Context, userID string) (*StatsResponse, error) {
	todayEvents, err := s.repo.ListRewardsByUserAndDate(ctx, userID, s.now())
	if err != nil {
		return nil, err
	}
	agg := make(map[string]decimal.Decimal)
	for _, evt := range todayEvents {
		agg[evt.Symbol] = agg[evt.Symbol].Add(evt.Quantity)
	}

	all, err := s.repo.ListAllRewards(ctx, userID)
	if err != nil {
		return nil, err
	}
	holdings := make(map[string]decimal.Decimal)
	for _, evt := range all {
		holdings[evt.Symbol] = holdings[evt.Symbol].Add(evt.Quantity)
	}
	portfolioValue := decimal.Zero
	for symbol, qty := range holdings {
		price, err := s.priceSvc.GetLatestPrice(ctx, symbol)
		if err != nil {
			s.logger.WithError(err).WithField("symbol", symbol).Warn("price lookup failed")
			continue
		}
		portfolioValue = portfolioValue.Add(price.Price.Mul(qty))
	}
	return &StatsResponse{TotalSharesToday: agg, PortfolioValue: portfolioValue}, nil
}

func (s *RewardService) GetPortfolio(ctx context.Context, userID string) ([]models.PortfolioPosition, error) {
	all, err := s.repo.ListAllRewards(ctx, userID)
	if err != nil {
		return nil, err
	}
	holdings := make(map[string]decimal.Decimal)
	for _, evt := range all {
		holdings[evt.Symbol] = holdings[evt.Symbol].Add(evt.Quantity)
	}
	positions := []models.PortfolioPosition{}
	for symbol, qty := range holdings {
		quote, err := s.priceSvc.GetLatestPrice(ctx, symbol)
		if err != nil {
			s.logger.WithError(err).WithField("symbol", symbol).Warn("price lookup failed")
			continue
		}
		value := quote.Price.Mul(qty)
		positions = append(positions, models.PortfolioPosition{
			Symbol:   symbol,
			Quantity: qty,
			Price:    quote.Price,
			ValueINR: value,
		})
	}
	return positions, nil
}

func (s *RewardService) sortHistorical(values []HistoricalDayValue) {
	sort.Slice(values, func(i, j int) bool {
		return values[i].Date < values[j].Date
	})
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
