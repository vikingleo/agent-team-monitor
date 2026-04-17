package app

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFromExecutableDir loads a .env file from the current executable directory.
// Existing environment variables win over values from the file.
func LoadEnvFromExecutableDir() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	envPath := filepath.Join(filepath.Dir(execPath), ".env")
	file, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}
