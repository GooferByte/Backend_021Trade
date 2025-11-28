package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application level configuration loaded from environment variables.
type Config struct {
	Port             string
	DBURL            string
	UseInMemoryStore bool
	PriceTTL         time.Duration
	Environment      string
}

// Load reads configuration from environment variables. A .env file is loaded
// if present to simplify local development. We look in bin/.env so the file
// can live alongside a built binary, and fall back to .env in the project
// root for compatibility.
func Load() Config {
	loadDotEnv()

	cfg := Config{
		Port:        getString("PORT", "8080"),
		DBURL:       getString("DATABASE_URL", ""),
		PriceTTL:    getDurationMinutes("PRICE_TTL_MINUTES", 60),
		Environment: getString("ENVIRONMENT", "local"),
	}

	cfg.UseInMemoryStore = cfg.DBURL == ""
	return cfg
}

func loadDotEnv() {
	candidates := []string{
		filepath.Join("bin", ".env"),
		".env",
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append([]string{
			filepath.Join(exeDir, ".env"),
			filepath.Join(exeDir, "bin", ".env"),
		}, candidates...)
	}

	for _, path := range candidates {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

func getString(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getDurationMinutes(key string, fallback int) time.Duration {
	if val := os.Getenv(key); val != "" {
		mins, err := strconv.Atoi(val)
		if err != nil {
			log.Printf("invalid value for %s, using fallback: %v", key, err)
			return time.Duration(fallback) * time.Minute
		}
		return time.Duration(mins) * time.Minute
	}
	return time.Duration(fallback) * time.Minute
}
