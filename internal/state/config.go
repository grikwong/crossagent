package state

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Config represents a workflow's config file (key=value format).
type Config struct {
	Repo           string
	AddDirs        []string
	Created        string
	Project        string
	PlanAgent      string
	ReviewAgent    string
	ImplementAgent string
	VerifyAgent    string
	RetryCount     int
	MaxRetries     int
	FollowupRound  int
}


// ReadConfig reads a workflow config file into a Config struct.
func ReadConfig(dir string) (*Config, error) {
	path := filepath.Join(dir, "config")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+1:]
		setConfigField(cfg, key, val)
	}
	return cfg, scanner.Err()
}

func setConfigField(cfg *Config, key, val string) {
	switch key {
	case "repo":
		cfg.Repo = val
	case "add_dirs":
		if val != "" {
			cfg.AddDirs = strings.Split(val, ",")
		}
	case "created":
		cfg.Created = val
	case "project":
		cfg.Project = val
	case "plan_agent":
		cfg.PlanAgent = val
	case "review_agent":
		cfg.ReviewAgent = val
	case "implement_agent":
		cfg.ImplementAgent = val
	case "verify_agent":
		cfg.VerifyAgent = val
	case "retry_count":
		cfg.RetryCount = atoi(val)
	case "max_retries":
		cfg.MaxRetries = atoi(val)
	case "followup_round":
		cfg.FollowupRound = atoi(val)
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// WriteConfig writes a Config struct to the workflow config file atomically.
func WriteConfig(dir string, cfg *Config) error {
	path := filepath.Join(dir, "config")
	var lines []string

	addLine := func(key, val string) {
		if val != "" {
			lines = append(lines, key+"="+val)
		}
	}

	addLine("repo", cfg.Repo)
	if len(cfg.AddDirs) > 0 {
		addLine("add_dirs", strings.Join(cfg.AddDirs, ","))
	}
	addLine("created", cfg.Created)
	addLine("project", cfg.Project)
	addLine("plan_agent", cfg.PlanAgent)
	addLine("review_agent", cfg.ReviewAgent)
	addLine("implement_agent", cfg.ImplementAgent)
	addLine("verify_agent", cfg.VerifyAgent)
	if cfg.RetryCount > 0 {
		addLine("retry_count", fmt.Sprintf("%d", cfg.RetryCount))
	}
	if cfg.MaxRetries > 0 {
		addLine("max_retries", fmt.Sprintf("%d", cfg.MaxRetries))
	}
	if cfg.FollowupRound > 0 {
		addLine("followup_round", fmt.Sprintf("%d", cfg.FollowupRound))
	}

	content := strings.Join(lines, "\n") + "\n"
	return atomicWrite(path, []byte(content))
}

// GetConf reads a single config key from the workflow config file.
// Returns empty string if key not found or file doesn't exist (matches bash behavior).
func GetConf(dir, key string) (string, error) {
	path := filepath.Join(dir, "config")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	prefix := key + "="
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			return line[len(prefix):], nil
		}
	}
	return "", scanner.Err()
}

// SetConf atomically sets a single config key in the workflow config file.
// Uses file locking to prevent race conditions (improvement over bash).
func SetConf(dir, key, value string) error {
	path := filepath.Join(dir, "config")
	lockPath := path + ".lock"

	// Acquire file lock
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	defer func() {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		// Do NOT remove the lock file — removing it allows a new inode to be
		// created, which breaks mutual exclusion for processes waiting on the
		// old file descriptor.
	}()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Read existing lines
	var lines []string
	found := false

	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, key+"=") {
				lines = append(lines, key+"="+value)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
		f.Close()
		if err := scanner.Err(); err != nil {
			return err
		}
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	content := strings.Join(lines, "\n") + "\n"
	return atomicWrite(path, []byte(content))
}

// atomicWrite writes data to a file atomically via temp file + rename.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
