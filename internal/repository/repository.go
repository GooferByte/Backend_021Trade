package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/GooferByte/Backend_021Trade/internal/models"
)

var (
	// ErrDuplicateReward indicates an idempotent reward already exists.
	ErrDuplicateReward = fmt.Errorf("duplicate reward")
)

// RewardRepository abstracts persistence for rewards and ledger lines.
type RewardRepository interface {
	CreateReward(ctx context.Context, reward models.RewardEvent) error
	FindByIdempotencyKey(ctx context.Context, userID, key string) (*models.RewardEvent, error)
	ListRewardsByUserAndDate(ctx context.Context, userID string, day time.Time) ([]models.RewardEvent, error)
	ListRewardsBeforeDate(ctx context.Context, userID string, before time.Time) ([]models.RewardEvent, error)
	ListAllRewards(ctx context.Context, userID string) ([]models.RewardEvent, error)
	UpsertLedgerEntries(ctx context.Context, entries []models.LedgerEntry) error
}
