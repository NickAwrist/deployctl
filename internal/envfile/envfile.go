package envfile

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
)

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ParseAssignment(assignment string) (string, string, error) {
	name, value, ok := strings.Cut(assignment, "=")
	if !ok {
		return "", "", fmt.Errorf("env variable %q must use KEY=VALUE", assignment)
	}

	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return "", "", err
	}

	return name, value, nil
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid env variable name %q", name)
	}

	return nil
}

func Read(path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	variables, err := dotenv.Parse(file)
	if err != nil {
		return nil, err
	}

	return variables, nil
}

func Write(path string, variables map[string]string) error {
	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	sort.Strings(names)

	var builder strings.Builder
	for _, name := range names {
		builder.WriteString(name)
		builder.WriteString("=")
		builder.WriteString(strconv.Quote(variables[name]))
		builder.WriteString("\n")
	}

	return os.WriteFile(path, []byte(builder.String()), 0600)
}
