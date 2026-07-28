package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.yml")
	content := []byte(`
platform_db:
  host: 127.0.0.1
  user: app_user
  password: test_password
  name: db_platform
session:
  cookie_name: test_session
  expire_hours: 12
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	if err := LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	dbConfig, err := getPlatformDBConfig()
	if err != nil {
		t.Fatal(err)
	}
	if dbConfig.Port != 3306 {
		t.Fatalf("default port = %d, want 3306", dbConfig.Port)
	}
	if sessionConfig := GetSessionConfig(); sessionConfig.CookieName != "test_session" || sessionConfig.ExpireHours != 12 {
		t.Fatalf("unexpected session config: %+v", sessionConfig)
	}
}

func TestLoadConfigRejectsMissingPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.yml")
	content := []byte(`
platform_db:
  host: 127.0.0.1
  user: app_user
  name: db_platform
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	if err := LoadConfig(path); err == nil {
		t.Fatal("LoadConfig should reject an empty platform_db.password")
	}
}

func TestBuildPlatformDBDSNKeepsNativePasswordSupport(t *testing.T) {
	dsn := buildPlatformDBDSN(PlatformDBConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "app_user",
		Password: "p@ss(word)",
		Name:     "db_platform",
	})

	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if !parsed.AllowNativePasswords {
		t.Fatal("generated DSN disabled mysql_native_password authentication")
	}
	if parsed.Passwd != "p@ss(word)" {
		t.Fatalf("password round trip = %q", parsed.Passwd)
	}
}
