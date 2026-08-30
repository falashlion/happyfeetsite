package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// dotenvFiles are loaded in order; later files win over earlier ones, but a
// variable already present in the real environment always wins over all of
// them. That keeps container deployments (which inject env directly) and
// docker-compose `env_file:` authoritative, while letting a developer running
// the binary on the host drop host-specific overrides into `.env.local`.
var dotenvFiles = []string{".env", ".env.local"}

// LoadDotenv reads the dotenv files from dir (and, if not found there, from up
// to three parent directories) into the process environment. Missing files are
// not an error — production runs on injected env vars only.
func LoadDotenv(dir string) {
	root := findRoot(dir)
	// Keys this call has already written. Without it, a value set from `.env`
	// would look like a real environment variable to `.env.local` and the
	// override would be silently dropped.
	fromDotenv := map[string]bool{}
	for _, name := range dotenvFiles {
		applyEnvFile(filepath.Join(root, name), fromDotenv)
	}
}

// findRoot walks up from dir looking for the directory that holds a .env file.
// Falls back to dir so callers still get a well-defined path.
func findRoot(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(abs, ".env")); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return dir
}

func applyEnvFile(path string, fromDotenv map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, val, ok := parseEnvLine(sc.Text())
		if !ok {
			continue
		}
		if _, present := os.LookupEnv(key); present && !fromDotenv[key] {
			continue // real environment wins
		}
		if err := os.Setenv(key, val); err == nil {
			fromDotenv[key] = true
		}
	}
}

// parseEnvLine handles `KEY=value`, `export KEY=value`, quoted values and
// trailing ` # comment` on unquoted values. Blank and comment lines are skipped.
func parseEnvLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	value = strings.TrimSpace(line[eq+1:])
	if key == "" {
		return "", "", false
	}

	switch {
	case len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"':
		value = value[1 : len(value)-1]
	case len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'':
		value = value[1 : len(value)-1]
	default:
		// Strip an inline comment, which must be preceded by whitespace so that
		// values such as a URL fragment or `#`-bearing password survive.
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
	}
	return key, value, true
}
