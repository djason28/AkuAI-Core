package config

import (
	"log"
	"os"
	"slices"
	"strconv"

	"github.com/joho/godotenv"
)

// Variabel global yang Anda minta
var (
	GeminiAPIKey     string
	GeminiModel      string
	GoogleAPIKey     string
	GoogleAPI_CX     string
	TursoDatabaseURL string
	TursoAuthToken   string
	AppEnv           string
	IsStaging        bool
	IsProduction     bool
	// IsGeminiEnabled is a flag to enable/disable Gemini API usage (enum: "1" or "0")
	IsGeminiEnabled bool

	// optional helper vars (dipakai di app)
	JWTSecret string
	Port      string

	// runtime tunables
	RateLimitWindowSeconds int
	RateLimitCapacity      int
	UserConcurrencyLimit   int
	DuplicateWindowSeconds int
	ChatCacheTTLSeconds    int
	ChatCacheMaxItems      int
)

// loadAppEnv: hanya memuat .env jika bukan production.
// sesuai instruksi Anda: jika APP_ENV == "production" => return (jangan load .env)
func loadAppEnv() {
	// baca APP_ENV dari environment saat ini
	AppEnv = os.Getenv("APP_ENV")

	// do not load .env file in production
	if AppEnv == "production" {
		return
	}

	// untuk non-production: load .env (fatal jika gagal)
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func init() {
	// panggil loader yang memutuskan apakah .env harus diload
	loadAppEnv()

	// baca variabel environment (baik dari .env yang sudah dimuat atau env host)
	GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
	GeminiModel = os.Getenv("GEMINI_MODEL")
	GoogleAPI_CX = os.Getenv("GOOGLE_API_CX")
	GoogleAPIKey = os.Getenv("GOOGLE_API_KEY")

	TursoDatabaseURL = os.Getenv("TURSO_DATABASE_URL")
	TursoAuthToken = os.Getenv("TURSO_AUTH_TOKEN")

	// baca APP_ENV lagi dari environment (agar konsisten)
	AppEnv = os.Getenv("APP_ENV")

	// validasi value APP_ENV
	if !slices.Contains([]string{"staging", "production"}, AppEnv) {
		log.Fatal("environment variable APP_ENV must be 'staging' or 'production'")
	}

	IsStaging = AppEnv == "staging"
	IsProduction = AppEnv == "production"

	// IS_GEMINI_ENABLED: "1" untuk enabled, selain itu false
	IsGeminiEnabled = os.Getenv("IS_GEMINI_ENABLED") == "1"

	// default model if not provided; can be overridden via GEMINI_MODEL env
	if GeminiModel == "" {
		GeminiModel = "gemini-2.0-flash"
	}

	// optional: JWT secret dan port (fallback)
	JWTSecret = os.Getenv("JWT_SECRET_KEY")
	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "5000"
	}

	// Tunables with defaults
	RateLimitWindowSeconds = atoiOr(os.Getenv("RATE_LIMIT_WINDOW_SECONDS"), 10)
	RateLimitCapacity = atoiOr(os.Getenv("RATE_LIMIT_CAPACITY"), 5)
	UserConcurrencyLimit = atoiOr(os.Getenv("USER_CONCURRENCY_LIMIT"), 2)
	DuplicateWindowSeconds = atoiOr(os.Getenv("DUPLICATE_WINDOW_SECONDS"), 45)
	ChatCacheTTLSeconds = atoiOr(os.Getenv("CHAT_CACHE_TTL_SECONDS"), 600)
	ChatCacheMaxItems = atoiOr(os.Getenv("CHAT_CACHE_MAX_ITEMS"), 500)

	// jika production dan JWT secret kosong -> fatal (safety)
	if IsProduction && JWTSecret == "" {
		log.Fatal("JWT_SECRET_KEY must be set in production")
	}

	// Log important config values to help debug environment
	log.Printf("[config] AppEnv=%s IsStaging=%v IsProduction=%v", AppEnv, IsStaging, IsProduction)
	log.Printf("[config] IsGeminiEnabled=%v GeminiAPIKeyPresent=%v", IsGeminiEnabled, GeminiAPIKey != "")
	log.Printf("[config] GeminiModel=%s", GeminiModel)
	log.Printf("[config] RateLimit window=%ds capacity=%d userConc=%d dupWindow=%ds cacheTTL=%ds cacheMax=%d",
		RateLimitWindowSeconds, RateLimitCapacity, UserConcurrencyLimit, DuplicateWindowSeconds, ChatCacheTTLSeconds, ChatCacheMaxItems)
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
