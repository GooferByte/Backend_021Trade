package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// RewardEvent represents a reward granted to a user in stock units.
type RewardEvent struct {
	ID              string          `json:"id"`
	UserID          string          `json:"userId"`
	Symbol          string          `json:"symbol"`
	Quantity        decimal.Decimal `json:"quantity"`
	RewardedAt      time.Time       `json:"rewardedAt"`
	IdempotencyKey  string          `json:"idempotencyKey,omitempty"`
	Fees            FeeBreakdown    `json:"fees"`
	TotalINRCost    decimal.Decimal `json:"totalInrCost"`
	PricedAt        time.Time       `json:"pricedAt"`
	UnitPriceINR    decimal.Decimal `json:"unitPriceInr"`
	CreatedLedger   bool            `json:"-"`
	CorporateAction string          `json:"corporateAction,omitempty"`
}

// FeeBreakdown captures all charges the company incurs while buying the stock.
type FeeBreakdown struct {
	Brokerage decimal.Decimal `json:"brokerage"`
	STT       decimal.Decimal `json:"stt"`
	GST       decimal.Decimal `json:"gst"`
	Other     decimal.Decimal `json:"other"`
}

// Total returns the aggregate of all fees.
func (f FeeBreakdown) Total() decimal.Decimal {
	return f.Brokerage.Add(f.STT).Add(f.GST).Add(f.Other)
}

// LedgerEntry implements a simple double-entry ledger line.
type LedgerEntry struct {
	ID        string          `json:"id"`
	EventID   string          `json:"eventId"`
	UserID    string          `json:"userId"`
	Account   string          `json:"account"`
	Symbol    string          `json:"symbol"`
	Units     decimal.Decimal `json:"units"`
	AmountINR decimal.Decimal `json:"amountInr"`
	EntryType string          `json:"entryType"` // debit or credit
	CreatedAt time.Time       `json:"createdAt"`
}

// PortfolioPosition represents holdings per symbol with the latest valuation.
type PortfolioPosition struct {
	Symbol   string          `json:"symbol"`
	Quantity decimal.Decimal `json:"quantity"`
	Price    decimal.Decimal `json:"price"`
	ValueINR decimal.Decimal `json:"valueInr"`
}

// PriceQuote models the latest or historical price.
type PriceQuote struct {
	Symbol    string
	Price     decimal.Decimal
	Timestamp time.Time
}
