package minitoml

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Document map[string]map[string]string

func Load(path string) (Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	doc := Document{}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if section == "" {
				return nil, fmt.Errorf("empty TOML section")
			}
			if _, ok := doc[section]; !ok {
				doc[section] = map[string]string{}
			}
			continue
		}
		if section == "" {
			return nil, fmt.Errorf("key outside TOML section: %s", line)
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid TOML line: %s", line)
		}
		doc[section][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return doc, nil
}

func stripComment(line string) string {
	inString := false
	for i, r := range line {
		if r == '"' {
			inString = !inString
		}
		if r == '#' && !inString {
			return line[:i]
		}
	}
	return line
}

func String(section map[string]string, key, fallback string) string {
	raw, ok := section[key]
	if !ok {
		return fallback
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return raw
	}
	return value
}

func Int(section map[string]string, key string, fallback int) int {
	raw, ok := section[key]
	if !ok {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func Bool(section map[string]string, key string, fallback bool) bool {
	raw, ok := section[key]
	if !ok {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
