package config

import (
	"log"
	"os"
	"slices"

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

	// jika production dan JWT secret kosong -> fatal (safety)
	if IsProduction && JWTSecret == "" {
		log.Fatal("JWT_SECRET_KEY must be set in production")
	}

	// Log important config values to help debug environment
	log.Printf("[config] AppEnv=%s IsStaging=%v IsProduction=%v", AppEnv, IsStaging, IsProduction)
	log.Printf("[config] IsGeminiEnabled=%v GeminiAPIKeyPresent=%v", IsGeminiEnabled, GeminiAPIKey != "")
	log.Printf("[config] GeminiModel=%s", GeminiModel)
}
