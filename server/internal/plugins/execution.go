package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ExecuteFetch invokes the plugin's fetch entrypoint and returns a normalised
// list of items. The caller is responsible for persisting items and the cursor.
func (s *Service) ExecuteFetch(ctx context.Context, installation Installation, binding Binding, secrets map[string]string, trigger FetchTrigger) (FetchOutput, error) {
	manifest, err := ParseManifest(installation.ManifestJSON)
	if err != nil {
		return FetchOutput{}, err
	}

	payload := fetchPayload{
		WorkspaceConfig: cloneMap(binding.Config),
		Secrets:         secrets,
		Cursor:          binding.Cursor,
		Trigger:         trigger,
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return FetchOutput{}, err
	}

	execCtx, cancel := context.WithTimeout(ctx, s.execTimeout)
	defer cancel()

	stdout, stderr, err := s.runner.Run(execCtx, installation.CurrentPath, manifest.Entrypoints.Fetch.Command, input, s.runOptions())
	if err != nil {
		return FetchOutput{}, fmt.Errorf("%w: %s", ErrExecutionFailed, trimExecOutput(stdout, stderr, err))
	}

	var result FetchOutput
	if err := json.Unmarshal(stdout, &result); err != nil {
		return FetchOutput{}, fmt.Errorf("%w: invalid fetch output", ErrExecutionFailed)
	}

	for index := range result.Items {
		if strings.TrimSpace(result.Items[index].SourceLabel) == "" {
			result.Items[index].SourceLabel = installation.DisplayName
		}
	}
	if err := validateFetchOutputLimits(result, s.runtimeLimits); err != nil {
		return FetchOutput{}, fmt.Errorf("%w: %s", ErrExecutionFailed, err.Error())
	}

	return result, nil
}

func (s *Service) runValidation(ctx context.Context, installation Installation, config map[string]any, secrets map[string]string, manifest Manifest) (ValidationResult, error) {
	payload := validationPayload{
		WorkspaceConfig: cloneMap(config),
		Secrets:         secrets,
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return ValidationResult{}, err
	}

	execCtx, cancel := context.WithTimeout(ctx, s.execTimeout)
	defer cancel()

	stdout, stderr, err := s.runner.Run(execCtx, installation.CurrentPath, manifest.Entrypoints.Validate.Command, input, s.runOptions())
	if err != nil {
		return ValidationResult{}, fmt.Errorf("%w: %s", ErrExecutionFailed, trimExecOutput(stdout, stderr, err))
	}

	var result ValidationResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return ValidationResult{}, fmt.Errorf("%w: invalid validation output", ErrExecutionFailed)
	}

	if !result.Valid && len(result.Errors) == 0 {
		result.Errors = []FieldError{{Field: "", Message: "插件校验失败"}}
	}

	return result, nil
}

func (s *Service) installPlugin(ctx context.Context, pluginDir string, manifest Manifest) error {
	var command []string
	switch manifest.Runtime.Type {
	case "node":
		command = []string{"pnpm", "install", "--frozen-lockfile", "--ignore-scripts"}
		if manifest.Permissions != nil && manifest.Permissions.InstallScripts {
			command = []string{"pnpm", "install", "--frozen-lockfile"}
		}
	case "python":
		command = []string{"uv", "sync", "--frozen"}
	default:
		return fmt.Errorf("%w: unsupported runtime %s", ErrInvalidPlugin, manifest.Runtime.Type)
	}

	installCtx, cancel := context.WithTimeout(ctx, s.installTimeout)
	defer cancel()

	stdout, stderr, err := s.runner.Run(installCtx, pluginDir, command, nil, s.runOptions())
	if err != nil {
		return fmt.Errorf("%w: %s", ErrExecutionFailed, trimExecOutput(stdout, stderr, err))
	}

	return nil
}

func (s *Service) runOptions() RunOptions {
	return RunOptions{
		OutputMaxBytes: s.runtimeLimits.OutputMaxBytes,
		EnvAllowlist:   s.runtimeLimits.EnvAllowlist,
	}
}

func validateFetchOutputLimits(output FetchOutput, limits RuntimeLimits) error {
	limits = normalizeRuntimeLimits(limits)
	if len(output.Items) > limits.FetchMaxItems {
		return fmt.Errorf("fetch returned %d items, limit is %d", len(output.Items), limits.FetchMaxItems)
	}

	for itemIndex, item := range output.Items {
		if err := validateItemLimits(item, itemIndex, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateItemLimits(item Item, itemIndex int, limits RuntimeLimits) error {
	if len(item.ExternalID) > limits.FetchMaxTextBytes {
		return fmt.Errorf("items[%d].externalId exceeds %d bytes", itemIndex, limits.FetchMaxTextBytes)
	}
	if len(item.Title) > limits.FetchMaxTextBytes {
		return fmt.Errorf("items[%d].title exceeds %d bytes", itemIndex, limits.FetchMaxTextBytes)
	}
	if len(item.SourceLabel) > limits.FetchMaxTextBytes {
		return fmt.Errorf("items[%d].sourceLabel exceeds %d bytes", itemIndex, limits.FetchMaxTextBytes)
	}
	if len(item.Blocks) > limits.FetchMaxBlocksPerItem {
		return fmt.Errorf("items[%d].blocks has %d blocks, limit is %d", itemIndex, len(item.Blocks), limits.FetchMaxBlocksPerItem)
	}
	for blockIndex, block := range item.Blocks {
		if err := validateBlockLimits(block, itemIndex, blockIndex, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateBlockLimits(block ContentBlock, itemIndex int, blockIndex int, limits RuntimeLimits) error {
	label := fmt.Sprintf("items[%d].blocks[%d]", itemIndex, blockIndex)
	if len(block.Text) > limits.FetchMaxTextBytes {
		return fmt.Errorf("%s.text exceeds %d bytes", label, limits.FetchMaxTextBytes)
	}
	if len(block.Alt) > limits.FetchMaxTextBytes {
		return fmt.Errorf("%s.alt exceeds %d bytes", label, limits.FetchMaxTextBytes)
	}
	if len(block.URL) > limits.FetchMaxURLBytes {
		return fmt.Errorf("%s.url exceeds %d bytes", label, limits.FetchMaxURLBytes)
	}
	return nil
}

func trimExecOutput(stdout []byte, stderr []byte, runErr error) string {
	parts := []string{}
	if text := strings.TrimSpace(string(stdout)); text != "" {
		parts = append(parts, text)
	}
	if text := strings.TrimSpace(string(stderr)); text != "" {
		parts = append(parts, text)
	}
	if runErr != nil && len(parts) == 0 {
		parts = append(parts, runErr.Error())
	}
	return strings.Join(parts, " | ")
}
