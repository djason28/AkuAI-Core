package config

import (
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var (
	GeminiAPIKey       string
	GeminiModel        string
	GoogleAPIKey       string
	GoogleAPI_CX       string
	AppEnv             string
	PromptMode         string // baseline | engineered | both
	IsStaging          bool
	IsProduction       bool
	IsGeminiEnabled    bool
	IsGoogleAPIEnabled bool

	JWTSecret string
	Port      string

	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string

	RateLimitWindowSeconds int
	RateLimitCapacity      int
	UserConcurrencyLimit   int
	DuplicateWindowSeconds int
	ChatCacheTTLSeconds    int
)

func loadAppEnv() {
	AppEnv = os.Getenv("APP_ENV")

	if AppEnv == "production" {
		return
	}

	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func init() {
	loadAppEnv()

	GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
	GeminiModel = os.Getenv("GEMINI_MODEL")
	GoogleAPI_CX = os.Getenv("GOOGLE_API_CX")
	GoogleAPIKey = os.Getenv("GOOGLE_API_KEY")

	AppEnv = os.Getenv("APP_ENV")
	PromptMode = strings.ToLower(strings.TrimSpace(os.Getenv("PROMPT_MODE")))

	MySQLHost = os.Getenv("MYSQL_HOST")
	MySQLPort = os.Getenv("MYSQL_PORT")
	MySQLUser = os.Getenv("MYSQL_USER")
	MySQLPassword = os.Getenv("MYSQL_PASSWORD")
	MySQLDatabase = os.Getenv("MYSQL_DATABASE")

	if !slices.Contains([]string{"staging", "production"}, AppEnv) {
		log.Fatal("environment variable APP_ENV must be 'staging' or 'production'")
	}

	IsStaging = AppEnv == "staging"
	IsProduction = AppEnv == "production"

	IsGeminiEnabled = os.Getenv("IS_GEMINI_ENABLED") == "1"
	IsGoogleAPIEnabled = os.Getenv("IS_GOOGLEAPI_ENABLED") == "1"

	// default prompt mode
	if PromptMode == "" {
		PromptMode = "engineered"
	}
	if PromptMode != "baseline" && PromptMode != "engineered" && PromptMode != "both" {
		log.Printf("[config] WARN: invalid PROMPT_MODE=%s, defaulting to 'engineered'", PromptMode)
		PromptMode = "engineered"
	}

	if GeminiModel == "" {
		GeminiModel = "gemini-2.0-flash"
	}

	JWTSecret = os.Getenv("JWT_SECRET_KEY")
	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "5000"
	}

	RateLimitWindowSeconds = atoiOr(os.Getenv("RATE_LIMIT_WINDOW_SECONDS"), 10)
	RateLimitCapacity = atoiOr(os.Getenv("RATE_LIMIT_CAPACITY"), 5)
	UserConcurrencyLimit = atoiOr(os.Getenv("USER_CONCURRENCY_LIMIT"), 2)
	DuplicateWindowSeconds = atoiOr(os.Getenv("DUPLICATE_WINDOW_SECONDS"), 45)
	ChatCacheTTLSeconds = atoiOr(os.Getenv("CHAT_CACHE_TTL_SECONDS"), 600)

	if IsProduction && JWTSecret == "" {
		log.Fatal("JWT_SECRET_KEY must be set in production")
	}

	log.Printf("[config] AppEnv=%s IsStaging=%v IsProduction=%v", AppEnv, IsStaging, IsProduction)
	log.Printf("[config] IsGeminiEnabled=%v GeminiAPIKeyPresent=%v", IsGeminiEnabled, GeminiAPIKey != "")
	log.Printf("[config] GeminiModel=%s", GeminiModel)
	log.Printf("[config] PromptMode=%s", PromptMode)
	log.Printf("[config] RateLimit window=%ds capacity=%d userConc=%d dupWindow=%ds cacheTTL=%ds",
		RateLimitWindowSeconds, RateLimitCapacity, UserConcurrencyLimit, DuplicateWindowSeconds, ChatCacheTTLSeconds)
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
