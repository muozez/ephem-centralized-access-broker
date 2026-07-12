package cli

import (
	"os"
	"path/filepath"
)

// GetConfigDir returns the path to ~/.ephem directory
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ephem")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// GetTokenPath returns the path to ~/.ephem/token
func GetTokenPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token"), nil
}

// SaveToken writes the token to ~/.ephem/token
func SaveToken(token string) error {
	path, err := GetTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0600)
}

// LoadToken reads the token from ~/.ephem/token
func LoadToken() (string, error) {
	path, err := GetTokenPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RemoveToken deletes the token file
func RemoveToken() error {
	path, err := GetTokenPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
