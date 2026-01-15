package config

import (
	"os"
	"testing"
)

func TestLoad_LocalDefaults(t *testing.T) {
	// Очищаем env
	os.Clearenv()
	os.Setenv("APP_ENV", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AppEnv != EnvLocal {
		t.Errorf("Expected AppEnv=local, got %s", cfg.AppEnv)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Errorf("Expected HTTPAddr=127.0.0.1:8080, got %s", cfg.HTTPAddr)
	}
	if cfg.InventoryGRPCAddr != "127.0.0.1:50051" {
		t.Errorf("Expected InventoryGRPCAddr=127.0.0.1:50051, got %s", cfg.InventoryGRPCAddr)
	}
	if cfg.PaymentGRPCAddr != "127.0.0.1:50052" {
		t.Errorf("Expected PaymentGRPCAddr=127.0.0.1:50052, got %s", cfg.PaymentGRPCAddr)
	}
}

func TestLoad_DockerDefaults(t *testing.T) {
	// Очищаем env
	os.Clearenv()
	os.Setenv("APP_ENV", "docker")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AppEnv != EnvDocker {
		t.Errorf("Expected AppEnv=docker, got %s", cfg.AppEnv)
	}
	if cfg.HTTPAddr != "0.0.0.0:8080" {
		t.Errorf("Expected HTTPAddr=0.0.0.0:8080, got %s", cfg.HTTPAddr)
	}
	if cfg.InventoryGRPCAddr != "inventory:50051" {
		t.Errorf("Expected InventoryGRPCAddr=inventory:50051, got %s", cfg.InventoryGRPCAddr)
	}
	if cfg.PaymentGRPCAddr != "payment:50052" {
		t.Errorf("Expected PaymentGRPCAddr=payment:50052, got %s", cfg.PaymentGRPCAddr)
	}
}


