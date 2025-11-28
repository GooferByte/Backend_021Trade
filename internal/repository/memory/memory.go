package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/GooferByte/Backend_021Trade/internal/models"
	"github.com/GooferByte/Backend_021Trade/internal/repository"
)

type InMemoryRepo struct {
	mu            sync.RWMutex
	rewardsByUser map[string][]models.RewardEvent
	idemIndex     map[string]string
	ledger        []models.LedgerEntry
}

func New() *InMemoryRepo {
	return &InMemoryRepo{
		rewardsByUser: make(map[string][]models.RewardEvent),
		idemIndex:     make(map[string]string),
		ledger:        []models.LedgerEntry{},
	}
}

func (r *InMemoryRepo) CreateReward(ctx context.Context, reward models.RewardEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if reward.IdempotencyKey != "" {
		key := r.key(reward.UserID, reward.IdempotencyKey)
		if _, ok := r.idemIndex[key]; ok {
			return repository.ErrDuplicateReward
		}
		r.idemIndex[key] = reward.ID
	}

	r.rewardsByUser[reward.UserID] = append(r.rewardsByUser[reward.UserID], reward)
	return nil
}

func (r *InMemoryRepo) FindByIdempotencyKey(ctx context.Context, userID, key string) (*models.RewardEvent, error) {
	if key == "" {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.idemIndex[r.key(userID, key)]; ok {
		for _, evt := range r.rewardsByUser[userID] {
			if evt.ID == id {
				copy := evt
				return &copy, nil
			}
		}
	}
	return nil, nil
}

func (r *InMemoryRepo) ListRewardsByUserAndDate(ctx context.Context, userID string, day time.Time) ([]models.RewardEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	start := startOfDay(day)
	end := start.Add(24 * time.Hour)
	events := []models.RewardEvent{}
	for _, evt := range r.rewardsByUser[userID] {
		if !evt.RewardedAt.Before(start) && evt.RewardedAt.Before(end) {
			events = append(events, evt)
		}
	}
	slices.SortFunc(events, func(a, b models.RewardEvent) int {
		if a.RewardedAt.Before(b.RewardedAt) {
			return -1
		}
		if a.RewardedAt.After(b.RewardedAt) {
			return 1
		}
		return 0
	})
	return events, nil
}

func (r *InMemoryRepo) ListRewardsBeforeDate(ctx context.Context, userID string, before time.Time) ([]models.RewardEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cutoff := startOfDay(before)
	events := []models.RewardEvent{}
	for _, evt := range r.rewardsByUser[userID] {
		if evt.RewardedAt.Before(cutoff) {
			events = append(events, evt)
		}
	}
	slices.SortFunc(events, func(a, b models.RewardEvent) int {
		if a.RewardedAt.Before(b.RewardedAt) {
			return -1
		}
		if a.RewardedAt.After(b.RewardedAt) {
			return 1
		}
		return 0
	})
	return events, nil
}

func (r *InMemoryRepo) ListAllRewards(ctx context.Context, userID string) ([]models.RewardEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events := append([]models.RewardEvent(nil), r.rewardsByUser[userID]...)
	slices.SortFunc(events, func(a, b models.RewardEvent) int {
		if a.RewardedAt.Before(b.RewardedAt) {
			return -1
		}
		if a.RewardedAt.After(b.RewardedAt) {
			return 1
		}
		return 0
	})
	return events, nil
}

func (r *InMemoryRepo) UpsertLedgerEntries(ctx context.Context, entries []models.LedgerEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ledger = append(r.ledger, entries...)
	return nil
}

func (r *InMemoryRepo) key(userID, idem string) string {
	return userID + "::" + idem
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
