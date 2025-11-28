package pricing

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"time"

	"github.com/GooferByte/Backend_021Trade/internal/models"
	"github.com/shopspring/decimal"
)

// Service exposes price lookup behaviour.
type Service interface {
	GetLatestPrice(ctx context.Context, symbol string) (models.PriceQuote, error)
	GetHistoricalPrice(ctx context.Context, symbol string, day time.Time) (decimal.Decimal, error)
}

// RandomPriceService mocks a market data provider with deterministic pseudo-random quotes.
type RandomPriceService struct {
	mu      sync.Mutex
	cache   map[string]models.PriceQuote
	ttl     time.Duration
	nowFunc func() time.Time
}

func NewRandomPriceService(ttl time.Duration) *RandomPriceService {
	return &RandomPriceService{
		cache:   make(map[string]models.PriceQuote),
		ttl:     ttl,
		nowFunc: time.Now,
	}
}

func (s *RandomPriceService) GetLatestPrice(ctx context.Context, symbol string) (models.PriceQuote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFunc()
	if quote, ok := s.cache[symbol]; ok && now.Sub(quote.Timestamp) < s.ttl {
		return quote, nil
	}
	price := s.generatePrice(symbol, now)
	quote := models.PriceQuote{Symbol: symbol, Price: price, Timestamp: now}
	s.cache[symbol] = quote
	return quote, nil
}

func (s *RandomPriceService) GetHistoricalPrice(ctx context.Context, symbol string, day time.Time) (decimal.Decimal, error) {
	// Normalize to date only to keep values stable per day.
	anchor := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)
	return s.generatePrice(symbol, anchor), nil
}

func (s *RandomPriceService) generatePrice(symbol string, t time.Time) decimal.Decimal {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%s-%d-%d", symbol, t.YearDay(), t.Hour())))
	seed := int64(h.Sum64())
	r := rand.New(rand.NewSource(seed))
	// Price range between 80 and 2000 to mimic liquid stocks.
	price := 80 + r.Float64()*1920
	return decimal.NewFromFloat(price).Round(2)
}
