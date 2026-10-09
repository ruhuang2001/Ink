package workspace

import "time"

type DeviceStatus string
type PrintStatus string
type ConversationMessageRole string
type ThemeMode string
type SourceConnectionStatus string
type LocaleCode string
type LocalePreference string

const (
	DeviceStatusConnected DeviceStatus = "connected"
	DeviceStatusPending   DeviceStatus = "pending"
	DeviceStatusOffline   DeviceStatus = "offline"

	PrintStatusPending   PrintStatus = "pending"
	PrintStatusQueued    PrintStatus = "queued"
	PrintStatusCompleted PrintStatus = "completed"
	PrintStatusFailed    PrintStatus = "failed"
	PrintStatusCancelled PrintStatus = "cancelled"

	ConversationRoleUser      ConversationMessageRole = "user"
	ConversationRoleAssistant ConversationMessageRole = "assistant"

	ThemeModeLight ThemeMode = "light"

	LocaleCodeZhCN LocaleCode = "zh-CN"
	LocaleCodeEnUS LocaleCode = "en-US"

	LocalePreferenceSystem LocalePreference = "system"

	SourceConnectionStatusConnected SourceConnectionStatus = "connected"
	SourceConnectionStatusError     SourceConnectionStatus = "error"
)

type Device struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Status DeviceStatus `json:"status"`
	Note   string       `json:"note"`
}

type ConversationMessage struct {
	ID        string                  `json:"id"`
	Role      ConversationMessageRole `json:"role"`
	Text      string                  `json:"text"`
	CreatedAt string                  `json:"createdAt"`
}

type Conversation struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Preview   string                `json:"preview"`
	UpdatedAt string                `json:"updatedAt"`
	Draft     string                `json:"draft"`
	Messages  []ConversationMessage `json:"messages"`
}

type PrintJob struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Source       string      `json:"source"`
	DeviceID     string      `json:"deviceId"`
	Status       PrintStatus `json:"status"`
	CreatedAt    string      `json:"createdAt"`
	UpdatedAt    string      `json:"updatedAt"`
	Content      string      `json:"content"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
}

type Schedule struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	TimeLabel string `json:"timeLabel"`
	DeviceID  string `json:"deviceId"`
	Enabled   bool   `json:"enabled"`
}

type SourceConnection struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Note   string                 `json:"note"`
	Status SourceConnectionStatus `json:"status"`
}

type Preferences struct {
	LoginProtectionEnabled  bool             `json:"loginProtectionEnabled"`
	SendConfirmationEnabled bool             `json:"sendConfirmationEnabled"`
	TutorialTabEnabled      *bool            `json:"tutorialTabEnabled"`
	Theme                   ThemeMode        `json:"theme"`
	DefaultDeviceID         string           `json:"defaultDeviceId"`
	Locale                  LocalePreference `json:"locale"`
}

type ServiceBinding struct {
	ProviderName *string `json:"providerName"`
	ModelName    string  `json:"modelName"`
	Bound        bool    `json:"bound"`
}

type State struct {
	Revision             int64              `json:"revision,omitzero"`
	Devices              []Device           `json:"devices"`
	Conversations        []Conversation     `json:"conversations"`
	ActiveConversationID string             `json:"activeConversationId"`
	PrintJobs            []PrintJob         `json:"printJobs"`
	Schedules            []Schedule         `json:"schedules"`
	Sources              []SourceConnection `json:"sources"`
	Preferences          Preferences        `json:"preferences"`
	ServiceBinding       ServiceBinding     `json:"serviceBinding"`
}

func SeedState(now time.Time) State {
	firstMessageAt := now.Add(-10 * time.Minute).UTC().Format(time.RFC3339)

	return NormalizeState(State{
		Devices: []Device{
			{
				ID:     "device-desk",
				Name:   "书桌咕咕机",
				Status: DeviceStatusConnected,
				Note:   "默认设备",
			},
			{
				ID:     "device-bedroom",
				Name:   "卧室咕咕机",
				Status: DeviceStatusPending,
				Note:   "睡前提醒",
			},
		},
		Conversations: []Conversation{
			{
				ID:        "conv-today",
				Title:     "今日待办",
				Preview:   "下班前要记得买牛奶和胶带",
				UpdatedAt: now.Add(-2 * time.Minute).UTC().Format(time.RFC3339),
				Draft:     "",
				Messages: []ConversationMessage{
					{
						ID:        "message-today-user",
						Role:      ConversationRoleUser,
						Text:      "帮我整理一张温柔一点的今日提醒，适合打印在小纸条上。",
						CreatedAt: firstMessageAt,
					},
					{
						ID:        "message-today-assistant",
						Role:      ConversationRoleAssistant,
						Text:      "当然可以。你可以写成：今天也别太赶，先把最重要的一件事做好，晚一点记得给自己买杯热饮。",
						CreatedAt: now.Add(-9 * time.Minute).UTC().Format(time.RFC3339),
					},
				},
			},
			{
				ID:        "conv-birthday",
				Title:     "生日祝福",
				Preview:   "想写一句温柔一点的话",
				UpdatedAt: now.Add(-10 * time.Minute).UTC().Format(time.RFC3339),
				Draft:     "",
				Messages: []ConversationMessage{
					{
						ID:        "message-birthday-user",
						Role:      ConversationRoleUser,
						Text:      "想给朋友写一句生日祝福，语气轻一点。",
						CreatedAt: now.Add(-12 * time.Minute).UTC().Format(time.RFC3339),
					},
					{
						ID:        "message-birthday-assistant",
						Role:      ConversationRoleAssistant,
						Text:      "生日快乐，愿你这一岁也有被认真照顾、被温柔对待的日子。",
						CreatedAt: now.Add(-11 * time.Minute).UTC().Format(time.RFC3339),
					},
				},
			},
			{
				ID:        "conv-shopping",
				Title:     "购物清单",
				Preview:   "鸡蛋、吐司、番茄、酸奶",
				UpdatedAt: now.Add(-18 * time.Hour).UTC().Format(time.RFC3339),
				Draft:     "记得补充家里常备的食物。",
				Messages: []ConversationMessage{
					{
						ID:        "message-shopping-user",
						Role:      ConversationRoleUser,
						Text:      "帮我整理一个简洁一点的购物清单。",
						CreatedAt: now.Add(-18 * time.Hour).UTC().Format(time.RFC3339),
					},
					{
						ID:        "message-shopping-assistant",
						Role:      ConversationRoleAssistant,
						Text:      "鸡蛋、吐司、番茄、酸奶，先买这四样就够了。",
						CreatedAt: now.Add(-17 * time.Hour).UTC().Format(time.RFC3339),
					},
				},
			},
		},
		ActiveConversationID: "conv-today",
		PrintJobs: []PrintJob{
			{
				ID:        "print-pending-message",
				Title:     "晚安留言",
				Source:    "对话草稿",
				DeviceID:  "device-bedroom",
				Status:    PrintStatusPending,
				CreatedAt: now.Add(-30 * time.Minute).UTC().Format(time.RFC3339),
				UpdatedAt: now.Add(-30 * time.Minute).UTC().Format(time.RFC3339),
				Content:   "早点休息，今天已经做得很好了。",
			},
			{
				ID:        "print-queued-report",
				Title:     "明日早报",
				Source:    "晨间订阅",
				DeviceID:  "device-desk",
				Status:    PrintStatusQueued,
				CreatedAt: now.Add(-25 * time.Minute).UTC().Format(time.RFC3339),
				UpdatedAt: now.Add(-25 * time.Minute).UTC().Format(time.RFC3339),
				Content:   "明天上午天气晴，记得带水出门。",
			},
			{
				ID:        "print-done-todo",
				Title:     "今日待办",
				Source:    "手动打印",
				DeviceID:  "device-desk",
				Status:    PrintStatusCompleted,
				CreatedAt: now.Add(-70 * time.Minute).UTC().Format(time.RFC3339),
				UpdatedAt: now.Add(-68 * time.Minute).UTC().Format(time.RFC3339),
				Content:   "先完成最重要的一件事。",
			},
			{
				ID:        "print-done-shopping",
				Title:     "购物清单",
				Source:    "手动打印",
				DeviceID:  "device-desk",
				Status:    PrintStatusCompleted,
				CreatedAt: now.Add(-95 * time.Minute).UTC().Format(time.RFC3339),
				UpdatedAt: now.Add(-93 * time.Minute).UTC().Format(time.RFC3339),
				Content:   "鸡蛋、吐司、番茄、酸奶。",
			},
		},
		Schedules: []Schedule{
			{
				ID:        "schedule-morning",
				Title:     "早报摘要",
				Source:    "晨间订阅",
				TimeLabel: "每天 08:00",
				DeviceID:  "device-desk",
				Enabled:   true,
			},
			{
				ID:        "schedule-night",
				Title:     "晚安提醒",
				Source:    "睡前便签",
				TimeLabel: "每天 22:00",
				DeviceID:  "device-bedroom",
				Enabled:   true,
			},
			{
				ID:        "schedule-weekend",
				Title:     "周末清单",
				Source:    "家庭计划",
				TimeLabel: "周六 09:30",
				DeviceID:  "device-desk",
				Enabled:   false,
			},
		},
		Sources: []SourceConnection{
			{
				ID:     "source-worth",
				Name:   "今天值得看",
				Type:   "RSS",
				Note:   "每日文章摘要",
				Status: SourceConnectionStatusConnected,
			},
			{
				ID:     "source-weather",
				Name:   "天气提醒",
				Type:   "在线服务",
				Note:   "晨间天气简报",
				Status: SourceConnectionStatusConnected,
			},
			{
				ID:     "source-calendar",
				Name:   "家庭日历",
				Type:   "日历",
				Note:   "最近同步失败，请重新授权",
				Status: SourceConnectionStatusError,
			},
		},
		Preferences: Preferences{
			LoginProtectionEnabled:  false,
			SendConfirmationEnabled: true,
			TutorialTabEnabled:      new(true),
			Theme:                   ThemeModeLight,
			DefaultDeviceID:         "device-desk",
			Locale:                  LocalePreferenceSystem,
		},
		ServiceBinding: ServiceBinding{
			ProviderName: nil,
			ModelName:    "Ink AI",
			Bound:        false,
		},
	})
}

func EmptyState() State {
	return NormalizeState(State{
		Devices:       []Device{},
		Conversations: []Conversation{},
		PrintJobs:     []PrintJob{},
		Schedules:     []Schedule{},
		Sources:       []SourceConnection{},
		Preferences: Preferences{
			LoginProtectionEnabled:  false,
			SendConfirmationEnabled: true,
			TutorialTabEnabled:      new(true),
			Theme:                   ThemeModeLight,
			Locale:                  LocalePreferenceSystem,
		},
		ServiceBinding: ServiceBinding{
			ProviderName: nil,
			ModelName:    "Ink AI",
			Bound:        false,
		},
	})
}

func NormalizeState(state State) State {
	if state.Devices == nil {
		state.Devices = []Device{}
	}
	if state.Conversations == nil {
		state.Conversations = []Conversation{}
	}
	if state.PrintJobs == nil {
		state.PrintJobs = []PrintJob{}
	}
	if state.Schedules == nil {
		state.Schedules = []Schedule{}
	}
	if state.Sources == nil {
		state.Sources = []SourceConnection{}
	}
	if state.Preferences.TutorialTabEnabled == nil {
		state.Preferences.TutorialTabEnabled = new(true)
	}
	if state.Preferences.Theme == "" {
		state.Preferences.Theme = ThemeModeLight
	}
	if state.Preferences.Locale == "" {
		state.Preferences.Locale = LocalePreferenceSystem
	}
	if state.Preferences.DefaultDeviceID == "" && len(state.Devices) > 0 {
		state.Preferences.DefaultDeviceID = state.Devices[0].ID
	}
	if state.ServiceBinding.ModelName == "" {
		state.ServiceBinding.ModelName = "Ink AI"
	}
	if state.ActiveConversationID == "" && len(state.Conversations) > 0 {
		state.ActiveConversationID = state.Conversations[0].ID
	}

	return state
}
