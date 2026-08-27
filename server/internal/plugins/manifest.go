package plugins

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var pluginKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var permissionHostPattern = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9][A-Za-z0-9.-]*(:[0-9]{1,5})?$`)

func ParseManifest(raw []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: decode manifest: %s", ErrInvalidPlugin, err.Error())
	}

	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func ValidateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != 2 {
		return fmt.Errorf("%w: schemaVersion must be 2", ErrInvalidPlugin)
	}
	if manifest.Kind != "source" {
		return fmt.Errorf("%w: kind must be source", ErrInvalidPlugin)
	}

	if manifest.PluginKey == "" || manifest.PluginKey != strings.TrimSpace(manifest.PluginKey) {
		return fmt.Errorf("%w: pluginKey cannot contain surrounding whitespace", ErrInvalidPlugin)
	}
	if !pluginKeyPattern.MatchString(manifest.PluginKey) {
		return fmt.Errorf("%w: pluginKey must use lowercase letters, digits, and dashes", ErrInvalidPlugin)
	}

	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPlugin)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidPlugin)
	}
	if manifest.Runtime.Type != "node" && manifest.Runtime.Type != "python" {
		return fmt.Errorf("%w: runtime.type must be node or python", ErrInvalidPlugin)
	}
	if manifest.FetchPolicy.Type != FetchPolicyTypeFixedInterval {
		return fmt.Errorf("%w: fetchPolicy.type must be fixed_interval", ErrInvalidPlugin)
	}
	if manifest.FetchPolicy.Minutes <= 0 {
		return fmt.Errorf("%w: fetchPolicy.minutes must be positive", ErrInvalidPlugin)
	}
	if len(manifest.Entrypoints.Validate.Command) == 0 ||
		strings.TrimSpace(manifest.Entrypoints.Validate.Command[0]) == "" {
		return fmt.Errorf("%w: entrypoints.validate.command is required", ErrInvalidPlugin)
	}
	if len(manifest.Entrypoints.Fetch.Command) == 0 ||
		strings.TrimSpace(manifest.Entrypoints.Fetch.Command[0]) == "" {
		return fmt.Errorf("%w: entrypoints.fetch.command is required", ErrInvalidPlugin)
	}
	if err := validatePermissions(manifest.Permissions); err != nil {
		return err
	}
	if err := validateFieldSpecs(manifest.WorkspaceConfigSchema, true); err != nil {
		return err
	}

	return nil
}

func validatePermissions(permissions *PluginPermissions) error {
	if permissions == nil {
		return nil
	}
	if permissions.Network == nil {
		return nil
	}

	switch permissions.Network.Mode {
	case NetworkPermissionNone, NetworkPermissionAll:
		return nil
	case NetworkPermissionDeclaredHosts:
		if len(permissions.Network.Hosts) == 0 {
			return fmt.Errorf("%w: permissions.network.hosts is required when mode is declared_hosts", ErrInvalidPlugin)
		}
		seenHosts := map[string]struct{}{}
		for _, host := range permissions.Network.Hosts {
			normalized := strings.ToLower(strings.TrimSpace(host))
			if normalized == "" || !permissionHostPattern.MatchString(normalized) {
				return fmt.Errorf("%w: invalid permissions.network.hosts entry %q", ErrInvalidPlugin, host)
			}
			if _, exists := seenHosts[normalized]; exists {
				return fmt.Errorf("%w: duplicate permissions.network.hosts entry %q", ErrInvalidPlugin, normalized)
			}
			seenHosts[normalized] = struct{}{}
		}
		return nil
	default:
		return fmt.Errorf("%w: permissions.network.mode must be none, declared_hosts, or all", ErrInvalidPlugin)
	}
}

func validateFieldSpecs(fields []FieldSpec, allowSecret bool) error {
	seenKeys := map[string]struct{}{}
	for _, field := range fields {
		if strings.TrimSpace(field.Key) == "" {
			return fmt.Errorf("%w: schema field key is required", ErrInvalidPlugin)
		}
		if field.Key != strings.TrimSpace(field.Key) {
			return fmt.Errorf("%w: schema field key %q cannot contain surrounding whitespace", ErrInvalidPlugin, field.Key)
		}

		key := field.Key
		label := strings.TrimSpace(field.Label)
		if label == "" {
			return fmt.Errorf("%w: schema field label is required for %s", ErrInvalidPlugin, key)
		}
		if _, exists := seenKeys[key]; exists {
			return fmt.Errorf("%w: duplicate schema field %s", ErrInvalidPlugin, key)
		}
		seenKeys[key] = struct{}{}

		switch field.Type {
		case FieldTypeText, FieldTypeTextarea, FieldTypeURL, FieldTypeNumber, FieldTypeSelect, FieldTypeCheckbox:
		case FieldTypeSecret:
			if !allowSecret {
				return fmt.Errorf("%w: secret fields are not allowed here", ErrInvalidPlugin)
			}
		default:
			return fmt.Errorf("%w: unsupported field type %s", ErrInvalidPlugin, field.Type)
		}

		if field.Type == FieldTypeSelect {
			if len(field.Options) == 0 {
				return fmt.Errorf("%w: select field %s must define options", ErrInvalidPlugin, key)
			}
			optionValues := map[string]struct{}{}
			for _, option := range field.Options {
				value := strings.TrimSpace(option.Value)
				if strings.TrimSpace(option.Label) == "" || value == "" {
					return fmt.Errorf("%w: select field %s has invalid option", ErrInvalidPlugin, key)
				}
				if _, exists := optionValues[value]; exists {
					return fmt.Errorf("%w: select field %s has duplicate option %s", ErrInvalidPlugin, key, value)
				}
				optionValues[value] = struct{}{}
			}
		}

		if field.DefaultValue != nil {
			if _, _, errs := NormalizeConfigValues([]FieldSpec{field}, map[string]any{key: field.DefaultValue}, allowSecret); len(errs) > 0 {
				return fmt.Errorf("%w: invalid defaultValue for %s: %s", ErrInvalidPlugin, key, errs[0].Message)
			}
		}
	}

	return nil
}

func NormalizeConfigValues(fields []FieldSpec, raw map[string]any, allowSecret bool) (map[string]any, map[string]string, []FieldError) {
	normalized := make(map[string]any, len(fields))
	secrets := map[string]string{}
	errors := make([]FieldError, 0)
	allowedKeys := make(map[string]FieldSpec, len(fields))

	for _, field := range fields {
		allowedKeys[field.Key] = field
	}

	for key := range raw {
		if _, exists := allowedKeys[key]; !exists {
			errors = append(errors, FieldError{
				Field:   key,
				Message: "包含未声明字段",
			})
		}
	}

	for _, field := range fields {
		value, exists := raw[field.Key]
		if !exists {
			value = field.DefaultValue
		}

		if isBlankFieldValue(field.Type, value) {
			if field.Required && isBlankFieldValue(field.Type, field.DefaultValue) {
				errors = append(errors, FieldError{
					Field:   field.Key,
					Message: "此字段不能为空",
				})
			}
			continue
		}

		normalizedValue, err := normalizeFieldValue(field, value, allowSecret)
		if err != nil {
			errors = append(errors, FieldError{
				Field:   field.Key,
				Message: err.Error(),
			})
			continue
		}

		if field.Type == FieldTypeSecret {
			secret, _ := normalizedValue.(string)
			secrets[field.Key] = secret
			continue
		}

		normalized[field.Key] = normalizedValue
	}

	if len(errors) > 0 {
		slices.SortFunc(errors, func(a, b FieldError) int {
			return cmp.Compare(a.Field, b.Field)
		})
	}

	return normalized, secrets, errors
}

func normalizeFieldValue(field FieldSpec, value any, allowSecret bool) (any, error) {
	switch field.Type {
	case FieldTypeText, FieldTypeTextarea, FieldTypeSecret:
		if field.Type == FieldTypeSecret && !allowSecret {
			return nil, fmt.Errorf("secret 字段当前不支持")
		}
		stringValue := strings.TrimSpace(stringFromValue(value))
		if stringValue == "" {
			return nil, fmt.Errorf("请输入有效文本")
		}
		return stringValue, nil
	case FieldTypeURL:
		stringValue := strings.TrimSpace(stringFromValue(value))
		if stringValue == "" {
			return nil, fmt.Errorf("请输入有效 URL")
		}
		parsed, err := url.ParseRequestURI(stringValue)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("请输入有效 URL")
		}
		return stringValue, nil
	case FieldTypeNumber:
		numberValue, err := intFromValue(value)
		if err != nil {
			return nil, fmt.Errorf("请输入有效数字")
		}
		return numberValue, nil
	case FieldTypeCheckbox:
		booleanValue, err := boolFromValue(value)
		if err != nil {
			return nil, fmt.Errorf("请输入有效布尔值")
		}
		return booleanValue, nil
	case FieldTypeSelect:
		stringValue := strings.TrimSpace(stringFromValue(value))
		if stringValue == "" {
			return nil, fmt.Errorf("请选择有效选项")
		}

		for _, option := range field.Options {
			if option.Value == stringValue {
				return stringValue, nil
			}
		}

		return nil, fmt.Errorf("请选择有效选项")
	default:
		return nil, fmt.Errorf("不支持的字段类型")
	}
}

func isBlankFieldValue(fieldType FieldType, value any) bool {
	if value == nil {
		return true
	}

	switch fieldType {
	case FieldTypeCheckbox:
		return false
	default:
		return strings.TrimSpace(stringFromValue(value)) == ""
	}
}

func stringFromValue(value any) string {
	switch current := value.(type) {
	case string:
		return current
	case json.Number:
		return current.String()
	case fmt.Stringer:
		return current.String()
	case float64:
		return strconv.FormatFloat(current, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(current), 'f', -1, 32)
	case int:
		return strconv.Itoa(current)
	case int8:
		return strconv.FormatInt(int64(current), 10)
	case int16:
		return strconv.FormatInt(int64(current), 10)
	case int32:
		return strconv.FormatInt(int64(current), 10)
	case int64:
		return strconv.FormatInt(current, 10)
	case uint:
		return strconv.FormatUint(uint64(current), 10)
	case uint8:
		return strconv.FormatUint(uint64(current), 10)
	case uint16:
		return strconv.FormatUint(uint64(current), 10)
	case uint32:
		return strconv.FormatUint(uint64(current), 10)
	case uint64:
		return strconv.FormatUint(current, 10)
	case bool:
		if current {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", current)
	}
}

func intFromValue(value any) (int, error) {
	switch current := value.(type) {
	case int:
		return current, nil
	case int8:
		return int(current), nil
	case int16:
		return int(current), nil
	case int32:
		return int(current), nil
	case int64:
		return int(current), nil
	case uint:
		return int(current), nil
	case uint8:
		return int(current), nil
	case uint16:
		return int(current), nil
	case uint32:
		return int(current), nil
	case uint64:
		return int(current), nil
	case float64:
		if current != math.Trunc(current) {
			return 0, fmt.Errorf("invalid integer")
		}
		return int(current), nil
	case float32:
		if float64(current) != math.Trunc(float64(current)) {
			return 0, fmt.Errorf("invalid integer")
		}
		return int(current), nil
	case json.Number:
		intValue, err := current.Int64()
		return int(intValue), err
	default:
		parsed, err := strconv.Atoi(strings.TrimSpace(stringFromValue(value)))
		if err != nil {
			return 0, err
		}
		return parsed, nil
	}
}

func boolFromValue(value any) (bool, error) {
	switch current := value.(type) {
	case bool:
		return current, nil
	case string:
		switch strings.TrimSpace(strings.ToLower(current)) {
		case "1", "true", "yes", "on":
			return true, nil
		case "0", "false", "no", "off":
			return false, nil
		default:
			return false, fmt.Errorf("invalid boolean")
		}
	default:
		return false, fmt.Errorf("invalid boolean")
	}
}
