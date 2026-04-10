package zjulogin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Username       string
	Password       string
	TYYSSignSecret string
}

func NewFromEnv() (*Auth, error) {
	_ = godotenv.Load(findEnvFile(".env.zju"))

	return New(Config{
		Username:       strings.TrimSpace(os.Getenv("ZJU_USERNAME")),
		Password:       strings.TrimSpace(os.Getenv("ZJU_PASSWORD")),
		TYYSSignSecret: strings.TrimSpace(os.Getenv("TYYS_SIGN_SECRET")),
	})
}

func findEnvFile(name string) string {
	dir, err := os.Getwd()
	if err != nil {
		return name
	}

	for {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return name
		}
		dir = parent
	}
}

func New(cfg Config) (*Auth, error) {
	if strings.TrimSpace(cfg.Username) == "" {
		return nil, fmt.Errorf("ZJU_USERNAME is required")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return nil, fmt.Errorf("ZJU_PASSWORD is required")
	}

	am, err := NewZJUAM(cfg)
	if err != nil {
		return nil, err
	}

	return &Auth{
		am:     am,
		config: cfg,
	}, nil
}
