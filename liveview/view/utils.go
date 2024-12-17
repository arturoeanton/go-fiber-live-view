package view

import (
	"os"
)

func ContainsString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		return false
	}
	return true
}

func FileToString(name string) (string, error) {
	content, err := os.ReadFile(name)
	return string(content), err
}

func StringToFile(filenanme string, content string) error {
	d1 := []byte(content)
	err := os.WriteFile(filenanme, d1, 0644)
	return err
}
