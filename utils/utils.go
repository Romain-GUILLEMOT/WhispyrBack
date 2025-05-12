package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"reflect"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func DebugScyllaInsert(label string, v interface{}) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if typ.Kind() != reflect.Struct {
		fmt.Println("❌ DebugScyllaInsert ne supporte que les structs.")
		return
	}

	fmt.Printf("📦 [DEBUG SCYLLA INSERT] %s\n", label)
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i).Interface()

		fmt.Printf("🧩 %s (%s): %v\n", field.Name, field.Type, value)
	}
	fmt.Println("✅ Fin du dump struct\n")
}
func DebugCQL(query string, values ...any) {
	fmt.Println("🟢 Requête CQL (debug approximatif) :")
	for i, val := range values {
		fmt.Printf("  Paramètre %d: %v (%T)\n", i+1, val, val)
	}
	fmt.Println("🧾 Requête brute :", query)
}
func RandomString64() (string, error) {
	bytes := make([]byte, 32) // 32 bytes = 64 chars en hex
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
