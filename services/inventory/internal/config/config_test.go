package config

import (
	"os"
	"testing"
)

func TestLoad_LocalDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_ENV", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AppEnv != EnvLocal {
		t.Errorf("Expected AppEnv=local, got %s", cfg.AppEnv)
	}
	if cfg.GRPCAddr != "127.0.0.1:50051" {
		t.Errorf("Expected GRPCAddr=127.0.0.1:50051, got %s", cfg.GRPCAddr)
	}
	if cfg.EnableGRPCReflection != false {
		t.Errorf("Expected EnableGRPCReflection=false, got %v", cfg.EnableGRPCReflection)
	}
}

func TestLoad_DockerDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_ENV", "docker")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AppEnv != EnvDocker {
		t.Errorf("Expected AppEnv=docker, got %s", cfg.AppEnv)
	}
	if cfg.GRPCAddr != "0.0.0.0:50051" {
		t.Errorf("Expected GRPCAddr=0.0.0.0:50051, got %s", cfg.GRPCAddr)
	}
}

func TestLoad_EnableReflection(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_ENV", "local")
	os.Setenv("ENABLE_GRPC_REFLECTION", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.EnableGRPCReflection != true {
		t.Errorf("Expected EnableGRPCReflection=true, got %v", cfg.EnableGRPCReflection)
	}
}


