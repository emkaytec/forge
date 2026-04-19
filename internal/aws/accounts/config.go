package accounts

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var roleARNAccountPattern = regexp.MustCompile(`^arn:aws:iam::([0-9]{12}):`)

// Profile represents one AWS shared-config/shared-credentials profile plus any
// account identifier Forge could infer from it.
type Profile struct {
	Name      string
	AccountID string
}

// LoadProfiles returns the merged set of profiles from the local AWS config and
// credentials files. Missing files are treated as "no profiles available".
func LoadProfiles() ([]Profile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := os.Getenv("AWS_CONFIG_FILE")
	if configPath == "" {
		configPath = filepath.Join(home, ".aws", "config")
	}

	credentialsPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credentialsPath == "" {
		credentialsPath = filepath.Join(home, ".aws", "credentials")
	}

	profiles := map[string]*Profile{}
	if err := loadFile(configPath, true, profiles); err != nil {
		return nil, err
	}
	if err := loadFile(credentialsPath, false, profiles); err != nil {
		return nil, err
	}

	result := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, *profile)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == "default" {
			return true
		}
		if result[j].Name == "default" {
			return false
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// FindProfile looks up a profile by name.
func FindProfile(profiles []Profile, name string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}

	return Profile{}, false
}

func loadFile(path string, configFile bool, profiles map[string]*Profile) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var current string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = profileName(line, configFile)
			if current == "" {
				continue
			}
			ensureProfile(profiles, current)
			continue
		}

		if current == "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		profile := ensureProfile(profiles, current)
		recordAccountHint(profile, strings.TrimSpace(strings.ToLower(key)), strings.TrimSpace(value))
	}

	return scanner.Err()
}

func profileName(section string, configFile bool) string {
	name := strings.TrimSuffix(strings.TrimPrefix(section, "["), "]")
	name = strings.TrimSpace(name)

	if configFile {
		if name == "default" {
			return name
		}
		if strings.HasPrefix(name, "sso-session ") {
			return ""
		}
		if strings.HasPrefix(name, "profile ") {
			return strings.TrimSpace(strings.TrimPrefix(name, "profile "))
		}
	}

	return name
}

func ensureProfile(profiles map[string]*Profile, name string) *Profile {
	if profile, ok := profiles[name]; ok {
		return profile
	}

	profile := &Profile{Name: name}
	profiles[name] = profile
	return profile
}

func recordAccountHint(profile *Profile, key, value string) {
	if profile.AccountID != "" {
		return
	}

	switch key {
	case "sso_account_id", "aws_account_id", "account_id":
		if strings.TrimSpace(value) != "" {
			profile.AccountID = strings.TrimSpace(value)
		}
	case "role_arn":
		matches := roleARNAccountPattern.FindStringSubmatch(strings.TrimSpace(value))
		if len(matches) == 2 {
			profile.AccountID = matches[1]
		}
	}
}
