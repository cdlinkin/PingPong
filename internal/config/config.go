package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	LogLevel    string
	LogFormat   string
	UploadDir   string
	EnvFile     bool
	HTTPAddr    string
	Storage     string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool
	LLMBaseURL  string
	LLMModel    string
	LLMAPIKey   string
	LLMTimeout  time.Duration
}

func Load() (Config, error) {
	envErr := godotenv.Load()

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		HTTPAddr:    os.Getenv("HTTP_ADDR"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		LogLevel:    os.Getenv("LOG_LEVEL"),
		LogFormat:   os.Getenv("LOG_FORMAT"),
		UploadDir:   os.Getenv("UPLOAD_DIR"),
		Storage:     os.Getenv("STORAGE"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		LLMBaseURL:  os.Getenv("LLM_BASE_URL"),
		LLMModel:    os.Getenv("LLM_MODEL"),
		LLMAPIKey:   os.Getenv("LLM_API_KEY"),
		LLMTimeout:  60 * time.Second,
		EnvFile:     envErr == nil,
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	if cfg.UploadDir == "" {
		cfg.UploadDir = "uploads"
	}

	if cfg.Storage == "" {
		cfg.Storage = "local"
	}

	if cfg.LLMModel == "" {
		cfg.LLMModel = "qwen2.5:3b"
	}

	if v := os.Getenv("LLM_TIMEOUT"); v != "" {
		timeout, err := time.ParseDuration(v)
		if err != nil || timeout <= 0 {
			return cfg, fmt.Errorf(
				"LLM_TIMEOUT must be a positive duration like 60s or 2m, got %q",
				v,
			)
		}
		cfg.LLMTimeout = timeout
	}

	if cfg.S3Bucket == "" {
		cfg.S3Bucket = "pingpong"
	}

	if v := os.Getenv("S3_USE_SSL"); v != "" {
		useSSL, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf(
				"S3_USE_SSL must be true or false, got %q",
				v,
			)
		}
		cfg.S3UseSSL = useSSL
	}

	if cfg.JWTSecret == "" {
		return cfg, errors.New("JWT_SECRET is not set")
	}

	switch cfg.Storage {
	case "local":
	case "s3":
		if cfg.S3Endpoint == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
			return cfg, errors.New("STORAGE=s3 needs S3_ENDPOINT, S3_ACCESS_KEY and S3_SECRET_KEY")
		}
	default:
		return cfg, fmt.Errorf(
			"STORAGE must be local or s3, got %q",
			cfg.Storage,
		)
	}

	return cfg, nil
}
