package plugins

import "time"

type SourceType string
type InstallationStatus string
type BindingStatus string
type FieldType string
type BlockType string
type TriggerKind string
type FetchPolicyType string
type NetworkPermissionMode string

const (
	SourceTypeUpload SourceType = "upload"
	SourceTypeGit    SourceType = "git"

	InstallationStatusInstalling InstallationStatus = "installing"
	InstallationStatusReady      InstallationStatus = "ready"
	InstallationStatusFailed     InstallationStatus = "failed"
	InstallationStatusDisabled   InstallationStatus = "disabled"

	BindingStatusConnected    BindingStatus = "connected"
	BindingStatusDisconnected BindingStatus = "disconnected"
	BindingStatusError        BindingStatus = "error"

	FieldTypeText     FieldType = "text"
	FieldTypeSecret   FieldType = "secret"
	FieldTypeTextarea FieldType = "textarea"
	FieldTypeURL      FieldType = "url"
	FieldTypeNumber   FieldType = "number"
	FieldTypeSelect   FieldType = "select"
	FieldTypeCheckbox FieldType = "checkbox"

	BlockHeading   BlockType = "heading"
	BlockParagraph BlockType = "paragraph"
	BlockImage     BlockType = "image"
	BlockLink      BlockType = "link"
	BlockDivider   BlockType = "divider"
	BlockList      BlockType = "list"
	BlockQuote     BlockType = "quote"

	TriggerKindAutomatic TriggerKind = "automatic"
	TriggerKindManual    TriggerKind = "manual"

	FetchPolicyTypeFixedInterval FetchPolicyType = "fixed_interval"

	NetworkPermissionNone          NetworkPermissionMode = "none"
	NetworkPermissionDeclaredHosts NetworkPermissionMode = "declared_hosts"
	NetworkPermissionAll           NetworkPermissionMode = "all"
)

type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FieldSpec struct {
	Key          string        `json:"key"`
	Label        string        `json:"label"`
	Type         FieldType     `json:"type"`
	Required     bool          `json:"required"`
	Description  string        `json:"description,omitempty"`
	DefaultValue any           `json:"defaultValue,omitempty"`
	Options      []FieldOption `json:"options,omitempty"`
}

type CommandSpec struct {
	Command []string `json:"command"`
}

type RuntimeSpec struct {
	Type string `json:"type"`
}

type FetchPolicy struct {
	Type    FetchPolicyType `json:"type"`
	Minutes int             `json:"minutes"`
}

type Entrypoints struct {
	Validate CommandSpec `json:"validate"`
	Fetch    CommandSpec `json:"fetch"`
}

type NetworkPermission struct {
	Mode  NetworkPermissionMode `json:"mode"`
	Hosts []string              `json:"hosts,omitempty"`
}

type FilesystemPermission struct {
	Temp  bool `json:"temp,omitempty"`
	Cache bool `json:"cache,omitempty"`
}

type PluginPermissions struct {
	Network        *NetworkPermission    `json:"network,omitempty"`
	Filesystem     *FilesystemPermission `json:"filesystem,omitempty"`
	InstallScripts bool                  `json:"installScripts,omitempty"`
}

type Manifest struct {
	SchemaVersion         int                `json:"schemaVersion"`
	Kind                  string             `json:"kind"`
	PluginKey             string             `json:"pluginKey"`
	Name                  string             `json:"name"`
	Version               string             `json:"version"`
	Description           string             `json:"description"`
	Runtime               RuntimeSpec        `json:"runtime"`
	FetchPolicy           FetchPolicy        `json:"fetchPolicy"`
	Entrypoints           Entrypoints        `json:"entrypoints"`
	Permissions           *PluginPermissions `json:"permissions,omitempty"`
	WorkspaceConfigSchema []FieldSpec        `json:"workspaceConfigSchema"`
}

type Installation struct {
	ID           string
	PluginKey    string
	SourceType   SourceType
	DisplayName  string
	Version      string
	RuntimeType  string
	ManifestJSON []byte
	CurrentPath  string
	Status       InstallationStatus
	LastError    *string
	InstalledBy  *string
	// Git install metadata. Empty for upload installs.
	RepoURL       string
	RepoRef       string
	RepoCommitSHA string
	RepoSubdir    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Binding struct {
	ID                   string
	PluginInstallationID string
	UserID               string
	Enabled              bool
	Config               map[string]any
	Ciphertext           []byte
	Nonce                []byte
	Cursor               *string
	MaxPrintsPerRun      int
	MaxPrintsPerDay      int
	Status               BindingStatus
	LastValidatedAt      *time.Time
	LastError            *string
	NextFetchAt          *time.Time
	LastFetchAt          *time.Time
	FetchLeaseUntil      *time.Time
	LastFetchError       *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type BindingInput struct {
	Enabled bool              `json:"enabled"`
	Config  map[string]any    `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool         `json:"valid"`
	Errors []FieldError `json:"errors,omitempty"`
}

type FetchTrigger struct {
	Kind         TriggerKind `json:"kind"`
	ScheduledFor string      `json:"scheduledFor,omitempty"`
	TriggeredAt  string      `json:"triggeredAt"`
	Timezone     string      `json:"timezone"`
}

// ContentBlock is the minimal structural unit a plugin emits.
// Only a small set of block types is supported on purpose; new types are
// additive and should extend this union.
type ContentBlock struct {
	Type  BlockType `json:"type"`
	Level int       `json:"level,omitempty"` // heading: 1..3
	Text  string    `json:"text,omitempty"`  // heading / paragraph / link
	URL   string    `json:"url,omitempty"`   // image / link
	Alt   string    `json:"alt,omitempty"`   // image
	Items []string  `json:"items,omitempty"` // list
	Style string    `json:"style,omitempty"` // list: bullet / ordered
}

// Item represents a single printable unit produced by a plugin.
// ExternalID is the idempotency key — the same (binding, externalId) is
// ingested at most once.
type Item struct {
	ExternalID  string         `json:"externalId"`
	Title       string         `json:"title"`
	SourceLabel string         `json:"sourceLabel,omitempty"`
	PublishedAt *time.Time     `json:"publishedAt,omitempty"`
	Blocks      []ContentBlock `json:"blocks"`
}

// FetchOutput is the canonical response returned by a plugin's fetch entrypoint.
// Items may be empty (no new content). Cursor, if present, is persisted on the
// binding and passed back verbatim on the next fetch.
type FetchOutput struct {
	Items  []Item  `json:"items"`
	Cursor *string `json:"cursor,omitempty"`
}

type InstallationSummary struct {
	ID            string             `json:"id"`
	PluginKey     string             `json:"pluginKey"`
	SourceType    SourceType         `json:"sourceType"`
	DisplayName   string             `json:"displayName"`
	Version       string             `json:"version"`
	RuntimeType   string             `json:"runtimeType"`
	Status        InstallationStatus `json:"status"`
	LastError     string             `json:"lastError,omitempty"`
	Description   string             `json:"description,omitempty"`
	RepoURL       string             `json:"repoUrl,omitempty"`
	RepoRef       string             `json:"repoRef,omitempty"`
	RepoCommitSHA string             `json:"repoCommitSha,omitempty"`
	RepoSubdir    string             `json:"repoSubdir,omitempty"`
	CreatedAt     *time.Time         `json:"createdAt,omitempty"`
	UpdatedAt     *time.Time         `json:"updatedAt,omitempty"`
}

type BindingSummary struct {
	ID              string         `json:"id"`
	Enabled         bool           `json:"enabled"`
	Status          BindingStatus  `json:"status"`
	Config          map[string]any `json:"config"`
	LastValidatedAt *time.Time     `json:"lastValidatedAt,omitempty"`
	LastError       string         `json:"lastError,omitempty"`
	NextFetchAt     *time.Time     `json:"nextFetchAt,omitempty"`
	LastFetchAt     *time.Time     `json:"lastFetchAt,omitempty"`
	LastFetchError  string         `json:"lastFetchError,omitempty"`
}

type PluginDetails struct {
	Installation InstallationSummary `json:"installation"`
	Manifest     Manifest            `json:"manifest"`
	Binding      *BindingSummary     `json:"binding,omitempty"`
}
