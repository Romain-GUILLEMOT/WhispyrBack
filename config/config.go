package config

import (
	"log"
	"os"
)

var cfg *Config

type Config struct {
	Port             string
	Debug            bool
	JWTSecret        string
	ScyllaHost       string
	ScyllaPort       string
	ScyllaUser       string
	ScyllaPass       string
	ScyllaKeyspace   string
	RedisHost        string
	RedisPass        string
	RedisDB          string
	SmtpHost         string
	SmtpPort         string
	SmtpUser         string
	SmtpPass         string
	ErrorReportEmail string
	MinioEndpoint    string
	MinioAccessKey   string
	MinioSecretKey   string
	MinioBucket      string
	MinioURL         string
}

func LoadConfig() {
	if cfg != nil {
		return
	}

	cfg = &Config{
		Port:             os.Getenv("PORT"),
		Debug:            os.Getenv("APP_DEBUG") == "true",
		JWTSecret:        os.Getenv("JWT_SECRET"),
		ScyllaHost:       os.Getenv("SCYLLA_HOST"),
		ScyllaPort:       os.Getenv("SCYLLA_PORT"),
		ScyllaUser:       os.Getenv("SCYLLA_USER"),
		ScyllaPass:       os.Getenv("SCYLLA_PASSWORD"),
		ScyllaKeyspace:   os.Getenv("SCYLLA_KEYSPACE"),
		RedisHost:        os.Getenv("REDIS_HOST"),
		RedisPass:        os.Getenv("REDIS_PASSWORD"),
		RedisDB:          os.Getenv("REDIS_DB"),
		SmtpHost:         os.Getenv("SMTP_HOST"),
		SmtpPort:         os.Getenv("SMTP_PORT"),
		SmtpUser:         os.Getenv("SMTP_USER"),
		SmtpPass:         os.Getenv("SMTP_PASS"),
		ErrorReportEmail: os.Getenv("ERROR_REPORT_EMAIL"),
		MinioEndpoint:    os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey:   os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:   os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:      os.Getenv("MINIO_BUCKET"),
		MinioURL:         os.Getenv("MINIO_URL"),
	}
}

func GetConfig() *Config {
	if cfg == nil {
		log.Fatal("Config not loaded — call LoadConfig() first")
	}
	return cfg
}
