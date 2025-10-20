package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
)

type AppConfig struct {
	HTTPPort                  string
	Env                       string
	DatabaseDSN               string
	DBDriver                  string
	SwaggerEnable             bool
	DataDir                   string
	SkipWAConnect             bool
	MasterToken               string
	Postgres                  PostgresConfig
	Storage                   StorageConfig
	CommunityEventsWebhookURL string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
	PublicURL string
}

func (s StorageConfig) Enabled() bool {
	return s.Endpoint != "" && s.AccessKey != "" && s.SecretKey != "" && s.Bucket != ""
}

func Load() *AppConfig {
	pg := PostgresConfig{
		Host:     getEnv("POSTGRES_HOST", ""),
		Port:     getEnv("POSTGRES_PORT", ""),
		User:     getEnv("POSTGRES_USER", ""),
		Password: getEnv("POSTGRES_PASSWORD", ""),
		DBName:   getEnv("POSTGRES_DB", ""),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	storage := StorageConfig{
		Endpoint:  getEnv("STORAGE_ENDPOINT", ""),
		AccessKey: getEnv("STORAGE_ACCESS_KEY", ""),
		SecretKey: getEnv("STORAGE_SECRET_KEY", ""),
		Bucket:    getEnv("STORAGE_BUCKET", ""),
		Region:    getEnv("STORAGE_REGION", ""),
		UseSSL:    getEnv("STORAGE_USE_SSL", "false") == "true",
		PublicURL: getEnv("STORAGE_PUBLIC_URL", ""),
	}

	// Backward compatibility: allow MINIO_* env vars when STORAGE_* not provided.
	if storage.Endpoint == "" {
		storage.Endpoint = getEnv("MINIO_ENDPOINT", "")
	}
	if storage.AccessKey == "" {
		storage.AccessKey = getEnv("MINIO_ACCESS_KEY", "")
	}
	if storage.SecretKey == "" {
		storage.SecretKey = getEnv("MINIO_SECRET_KEY", "")
	}
	if storage.Bucket == "" {
		storage.Bucket = getEnv("MINIO_BUCKET", "")
	}
	if storage.Region == "" {
		storage.Region = getEnv("MINIO_REGION", "")
	}
	if !storage.UseSSL {
		storage.UseSSL = getEnv("MINIO_USE_SSL", "false") == "true"
	}
	if storage.PublicURL == "" {
		storage.PublicURL = getEnv("MINIO_PUBLIC_URL", "")
	}

	dsn := getEnv("DATABASE_DSN", "")
	driver := strings.ToLower(getEnv("DB_DRIVER", ""))

	if driver == "" {
		lower := strings.ToLower(dsn)
		switch {
		case strings.HasPrefix(lower, "postgres"):
			driver = "postgres"
		case pg.Host != "":
			driver = "postgres"
		default:
			driver = "sqlite"
		}
	}

	if driver == "postgres" {
		if dsn == "" {
			dsn = buildPostgresDSN(pg)
		}
	} else {
		if dsn == "" {
			dsn = "file:whatsapp.db?_foreign_keys=on"
		}
	}

	cfg := &AppConfig{
		HTTPPort:                  getEnv("HTTP_PORT", "8080"),
		Env:                       getEnv("APP_ENV", "development"),
		DatabaseDSN:               dsn,
		DBDriver:                  driver,
		SwaggerEnable:             getEnv("SWAGGER_ENABLE", "true") == "true",
		DataDir:                   getEnv("DATA_DIR", "data"),
		SkipWAConnect:             getEnv("WA_SKIP_CONNECT", "false") == "true",
		MasterToken:               getEnv("API_MASTER_TOKEN", ""),
		Postgres:                  pg,
		Storage:                   storage,
		CommunityEventsWebhookURL: strings.TrimSpace(getEnv("COMMUNITY_EVENTS_WEBHOOK_URL", "")),
	}
	return cfg
}

func buildPostgresDSN(pg PostgresConfig) string {
	host := pg.Host
	if host == "" {
		host = "localhost"
	}
	port := pg.Port
	if port == "" {
		port = "5432"
	}
	ssl := pg.SSLMode
	if ssl == "" {
		ssl = "disable"
	}

	u := &url.URL{Scheme: "postgres"}
	if pg.User != "" {
		if pg.Password != "" {
			u.User = url.UserPassword(pg.User, pg.Password)
		} else {
			u.User = url.User(pg.User)
		}
	}
	if host != "" {
		if port != "" {
			u.Host = fmt.Sprintf("%s:%s", host, port)
		} else {
			u.Host = host
		}
	}
	if pg.DBName != "" {
		u.Path = pg.DBName
	}
	q := u.Query()
	if ssl != "" {
		q.Set("sslmode", ssl)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func MustLoad() *AppConfig {
	cfg := Load()
	if cfg.HTTPPort == "" {
		log.Fatal("HTTP_PORT required")
	}
	if cfg.DBDriver == "postgres" && cfg.DatabaseDSN == "" {
		log.Fatal("DATABASE_DSN required for postgres driver")
	}
	return cfg
}
