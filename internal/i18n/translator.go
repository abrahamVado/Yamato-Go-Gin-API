package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

//1.- Define the default locale used whenever the requested locale is not available.
const defaultLocale = "en"

//2.- Provide shared state for caching parsed translations.
var (
	translations         map[string]map[string]string
	loadTranslationsOnce sync.Once
	loadTranslationsErr  error
)

//3.- Translator provides locale-aware access to translation strings.
type Translator struct {
	locale string
}

//4.- New creates a translator honoring the preferred locale when possible.
func New(preferredLocale string) (*Translator, error) {
	//5.- Ensure that translations are loaded before constructing the translator.
	if err := ensureTranslationsLoaded(); err != nil {
		return nil, err
	}

	//6.- Normalize the requested locale and fall back to the default when unsupported.
	trimmed := strings.TrimSpace(preferredLocale)
	locale := defaultLocale
	if trimmed != "" {
		if _, ok := translations[trimmed]; ok {
			locale = trimmed
		}
	}

	//7.- Return the translator initialized with the effective locale.
	return &Translator{locale: locale}, nil
}

//8.- Translate resolves the given key using the active locale with fallback support.
func (t *Translator) Translate(key string) string {
	//9.- Guard against nil receivers and empty keys by returning the key itself.
	if t == nil || key == "" {
		return key
	}

	//10.- Attempt to find the translation in the preferred locale first.
	if localeMessages, ok := translations[t.locale]; ok {
		if value, found := localeMessages[key]; found {
			return value
		}
	}

	//11.- Fall back to the default locale when the key is not available above.
	if fallbackMessages, ok := translations[defaultLocale]; ok {
		if value, found := fallbackMessages[key]; found {
			return value
		}
	}

	//12.- Return the key itself when no translation is available anywhere.
	return key
}

//13.- ensureTranslationsLoaded parses the locale files a single time.
func ensureTranslationsLoaded() error {
	loadTranslationsOnce.Do(func() {
		//14.- Resolve the filesystem path to the locales directory.
		localesDir, err := resolveLocalesDir()
		if err != nil {
			loadTranslationsErr = err
			return
		}

		//15.- Read every locale directory to discover available translations.
		entries, err := os.ReadDir(localesDir)
		if err != nil {
			loadTranslationsErr = fmt.Errorf("read locales dir: %w", err)
			return
		}

		translations = make(map[string]map[string]string)
		foundLocale := false

		for _, entry := range entries {
			//16.- Skip any filesystem entries that are not directories.
			if !entry.IsDir() {
				continue
			}

			//17.- Build the expected path to the locale's messages.json file.
			localeCode := entry.Name()
			messagePath := filepath.Join(localesDir, localeCode, "messages.json")

			//18.- Read the message file and parse its JSON payload.
			data, err := os.ReadFile(messagePath)
			if err != nil {
				loadTranslationsErr = fmt.Errorf("read locale %s: %w", localeCode, err)
				return
			}

			parsed, err := parseMessages(data)
			if err != nil {
				loadTranslationsErr = fmt.Errorf("parse locale %s: %w", localeCode, err)
				return
			}

			translations[localeCode] = parsed
			foundLocale = true
		}

		if !foundLocale {
			loadTranslationsErr = errors.New("no locale directories found")
		}
	})

	//19.- Return any error that occurred during the initialization above.
	return loadTranslationsErr
}

//20.- resolveLocalesDir determines the absolute path to the locales directory.
func resolveLocalesDir() (string, error) {
	//21.- Use runtime caller information to locate this source file at runtime.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("unable to determine caller information")
	}

	//22.- Construct the path to the locales directory relative to this file.
	dir := filepath.Dir(file)
	localesDir := filepath.Join(dir, "..", "..", "locales")

	//23.- Ensure the resolved directory actually exists on disk.
	if _, err := os.Stat(localesDir); err != nil {
		return "", fmt.Errorf("locate locales dir: %w", err)
	}

	//24.- Return the verified directory path for caller usage.
	return localesDir, nil
}

//25.- parseMessages flattens nested JSON objects into dot-delimited keys.
func parseMessages(data []byte) (map[string]string, error) {
	//26.- Unmarshal the JSON document into a generic map for traversal.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal messages: %w", err)
	}

	//27.- Create the destination map that will store flattened keys.
	flattened := make(map[string]string)
	if err := flattenMap("", raw, flattened); err != nil {
		return nil, err
	}

	//28.- Return the completed flattened map.
	return flattened, nil
}

//29.- flattenMap walks the JSON structure and builds dot-delimited keys.
func flattenMap(prefix string, input map[string]any, output map[string]string) error {
	for key, value := range input {
		//30.- Compose the hierarchical key path progressively.
		nextKey := key
		if prefix != "" {
			nextKey = prefix + "." + key
		}

		switch typed := value.(type) {
		case string:
			//31.- Store string values directly under their computed keys.
			output[nextKey] = typed
		case map[string]any:
			//32.- Recurse into nested objects while preserving the prefix.
			if err := flattenMap(nextKey, typed, output); err != nil {
				return err
			}
		default:
			//33.- Reject unsupported types to surface configuration errors early.
			return fmt.Errorf("unsupported value type at key %s", nextKey)
		}
	}

	//34.- Signal successful traversal without errors.
	return nil
}
