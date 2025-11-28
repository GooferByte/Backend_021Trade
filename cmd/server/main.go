package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/GooferByte/Backend_021Trade/internal/http"
	"github.com/GooferByte/Backend_021Trade/internal/logger"
	"github.com/GooferByte/Backend_021Trade/internal/pricing"
	"github.com/GooferByte/Backend_021Trade/internal/repository"
	"github.com/GooferByte/Backend_021Trade/internal/repository/memory"
	"github.com/GooferByte/Backend_021Trade/internal/repository/postgres"
	"github.com/GooferByte/Backend_021Trade/internal/service"
	"honnef.co/go/tools/config"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.Environment)
	priceSvc := pricing.NewRandomPriceService(cfg.PriceTTL)

	var repoImpl repository.RewardRepository
	if cfg.UseInMemoryStore {
		log.Warn("DATABASE_URL not set, using in-memory store. Data will reset on restart.")
		repoImpl = memory.New()
	} else {
		db, err := sql.Open("postgres", cfg.DBURL)
		if err != nil {
			log.WithError(err).Fatal("failed to connect to postgres")
		}
		if err := db.Ping(); err != nil {
			log.WithError(err).Fatal("postgres ping failed")
		}
		repoImpl = postgres.New(db)
		defer db.Close()
		log.Info("connected to postgres")
	}

	rewardSvc := service.NewRewardService(repoImpl, priceSvc, log)
	router := http.Router(rewardSvc, log)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Infof("Stocky incentive service listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.WithError(err).Error("server stopped")
		os.Exit(1)
	}
}
