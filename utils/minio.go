package utils

import (
	"context"
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

// DeleteObject supprime un seul objet de MinIO par son nom.
func DeleteObject(objectName string) error {
	if objectName == "" || MinioClient == nil {
		return nil
	}
	cfg := config.GetConfig()
	opts := minio.RemoveObjectOptions{}
	err := MinioClient.RemoveObject(context.Background(), cfg.MinioBucket, objectName, opts)
	if err != nil {
		Error("Failed to remove object from MinIO", "bucket", cfg.MinioBucket, "object", objectName, "err", err)
		return err
	}
	Info("Successfully removed object from MinIO", "object", objectName)
	return nil
}

// DeleteObjects supprime plusieurs objets de MinIO de manière efficace en utilisant un channel.
// C'est la méthode à privilégier pour le nettoyage en masse.
func DeleteObjects(ctx context.Context, objectNames []string) {
	if len(objectNames) == 0 || MinioClient == nil {
		return
	}
	cfg := config.GetConfig()
	opts := minio.RemoveObjectsOptions{
		GovernanceBypass: true,
	}

	objectsCh := make(chan minio.ObjectInfo)

	// Goroutine pour envoyer les noms d'objets sur le channel.
	go func() {
		defer close(objectsCh)
		for _, name := range objectNames {
			if name != "" {
				objectsCh <- minio.ObjectInfo{Key: name}
			}
		}
	}()

	// MinioClient.RemoveObjects lit depuis le channel et supprime les objets en parallèle.
	errorCh := MinioClient.RemoveObjects(ctx, cfg.MinioBucket, objectsCh, opts)

	// On écoute et loggue les erreurs de suppression éventuelles.
	for e := range errorCh {
		Error("Failed to remove object during bulk deletion", "object", e.ObjectName, "err", e.Err)
	}
	Info("Finished bulk deletion process.", "attempted_count", len(objectNames))
}
