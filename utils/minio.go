package utils

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client

func MinioInit() {
	cfg := config.GetConfig()
	endpoint := cfg.MinioEndpoint
	accessKey := cfg.MinioAccessKey
	secretKey := cfg.MinioSecretKey
	useSSL := false
	if cfg.Debug {
		useSSL = false
	}

	Client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		Fatal("❌ MinIO init failed: %v", err)
	}

	MinioClient = Client
}
