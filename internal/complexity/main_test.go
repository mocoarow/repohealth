package complexity_test

import (
	"os"
	"path/filepath"
	"testing"
)

const aquaConfigEnv = "AQUA_GLOBAL_CONFIG"

// aqua resolves biome from the nearest aqua.yaml; temporary repositories live outside this module and need this fallback.
func TestMain(m *testing.M) {
	if os.Getenv(aquaConfigEnv) == "" {
		cfg, err := filepath.Abs(filepath.Join("..", "..", "aqua.yaml"))
		if err != nil {
			panic(err)
		}

		if err := os.Setenv(aquaConfigEnv, cfg); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}
