package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppKey      string       `yaml:"app_key"`
	AppSecret   string       `yaml:"app_secret"`
	AccessToken string       `yaml:"access_token"`
	Region      string       `yaml:"region"`
	Server      ServerConfig `yaml:"server"`
	Okx         *OkxConfig   `yaml:"okx"`

	// Backward-compatible legacy server fields.
	ServerHost string `yaml:"host"`
	ServerPort int    `yaml:"port"`

	// Daily Brief Configuration
	DailyBrief *DailyBriefConfig `yaml:"daily_brief"`

	// Signal Alert Configuration
	SignalAlert *SignalAlertConfig `yaml:"signal_alert"`

	// Review Schedule Configuration
	ReviewSchedule *ReviewScheduleConfig `yaml:"review_schedule"`

	// Execution Window Configuration
	ExecutionWindow *ExecutionWindowConfig `yaml:"execution_window"`

	// Feishu Configuration
	Feishu *FeishuConfig `yaml:"feishu"`

	// Log Configuration
	Log *LogConfig `yaml:"log"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DailyBriefConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Timezone       string `yaml:"timezone"`
	PreMarketTime  string `yaml:"pre_market_time"`
	PostMarketTime string `yaml:"post_market_time"`
	WebhookURL     string `yaml:"webhook_url"`
}

type SignalAlertConfig struct {
	Enabled       bool   `yaml:"enabled"`
	CheckInterval int    `yaml:"check_interval"`
	WebhookURL    string `yaml:"webhook_url"`
	Timezone      string `yaml:"timezone"`
	SessionStart  string `yaml:"session_start"`
	SessionEnd    string `yaml:"session_end"`
}

type ReviewScheduleConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Timezone         string `yaml:"timezone"`
	DailyReviewTime  string `yaml:"daily_review_time"`
	WeeklyReviewTime string `yaml:"weekly_review_time"`
}

type ExecutionWindowConfig struct {
	Enabled bool `yaml:"enabled"`
}

// OkxConfig holds OKX API credentials and options
type OkxConfig struct {
	Enabled    bool   `yaml:"enabled"`
	APIKey     string `yaml:"api_key"`
	SecretKey  string `yaml:"secret_key"`
	Passphrase string `yaml:"passphrase"`
	BaseURL    string `yaml:"base_url"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level string `yaml:"level"` // debug, info, warn, error
}

// FeishuConfig holds Feishu notification configuration
type FeishuConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	UserID    string `yaml:"user_id"` // 用户 open_id
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Use environment variables as fallback
	if cfg.AppKey == "" {
		cfg.AppKey = os.Getenv("LONGPORT_APP_KEY")
	}
	if cfg.AppSecret == "" {
		cfg.AppSecret = os.Getenv("LONGPORT_APP_SECRET")
	}
	if cfg.AccessToken == "" {
		cfg.AccessToken = os.Getenv("LONGPORT_ACCESS_TOKEN")
	}

	// OKX env fallbacks
	if cfg.Okx != nil {
		if cfg.Okx.APIKey == "" {
			cfg.Okx.APIKey = os.Getenv("OKX_API_KEY")
		}
		if cfg.Okx.SecretKey == "" {
			cfg.Okx.SecretKey = os.Getenv("OKX_SECRET_KEY")
		}
		if cfg.Okx.Passphrase == "" {
			cfg.Okx.Passphrase = os.Getenv("OKX_PASSPHRASE")
		}
		if cfg.Okx.BaseURL == "" {
			cfg.Okx.BaseURL = os.Getenv("OKX_BASE_URL")
		}
	}

	// Set environment variables for LongBridge SDK
	if cfg.AppKey != "" {
		os.Setenv("LONGPORT_APP_KEY", cfg.AppKey)
	}
	if cfg.AppSecret != "" {
		os.Setenv("LONGPORT_APP_SECRET", cfg.AppSecret)
	}
	if cfg.AccessToken != "" {
		os.Setenv("LONGPORT_ACCESS_TOKEN", cfg.AccessToken)
	}
	if cfg.Region != "" {
		os.Setenv("LONGPORT_REGION", cfg.Region)
	}
	if cfg.Log != nil && cfg.Log.Level != "" {
		os.Setenv("LONGPORT_LOG_LEVEL", cfg.Log.Level)
	}
	// Set OKX env for downstream libraries / debugging
	if cfg.Okx != nil {
		if cfg.Okx.APIKey != "" {
			os.Setenv("OKX_API_KEY", cfg.Okx.APIKey)
		}
		if cfg.Okx.SecretKey != "" {
			os.Setenv("OKX_SECRET_KEY", cfg.Okx.SecretKey)
		}
		if cfg.Okx.Passphrase != "" {
			os.Setenv("OKX_PASSPHRASE", cfg.Okx.Passphrase)
		}
		if cfg.Okx.BaseURL != "" {
			os.Setenv("OKX_BASE_URL", cfg.Okx.BaseURL)
		}
	}

	// Set defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = cfg.ServerHost
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = cfg.ServerPort
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	// Set Daily Brief defaults
	if cfg.DailyBrief == nil {
		cfg.DailyBrief = &DailyBriefConfig{
			Enabled:        true,
			Timezone:       "Asia/Shanghai",
			PreMarketTime:  "09:00",
			PostMarketTime: "16:00",
		}
	}

	// Set Signal Alert defaults
	if cfg.SignalAlert == nil {
		cfg.SignalAlert = &SignalAlertConfig{
			Enabled:       true,
			CheckInterval: 60,
			Timezone:      "Asia/Shanghai",
			SessionStart:  "21:30",
			SessionEnd:    "04:00",
		}
	} else {
		if cfg.SignalAlert.Timezone == "" {
			cfg.SignalAlert.Timezone = "Asia/Shanghai"
		}
		if cfg.SignalAlert.SessionStart == "" {
			cfg.SignalAlert.SessionStart = "21:30"
		}
		if cfg.SignalAlert.SessionEnd == "" {
			cfg.SignalAlert.SessionEnd = "04:00"
		}
	}

	if cfg.ReviewSchedule == nil {
		cfg.ReviewSchedule = &ReviewScheduleConfig{
			Enabled:          true,
			Timezone:         "Asia/Shanghai",
			DailyReviewTime:  "07:30",
			WeeklyReviewTime: "08:30",
		}
	} else {
		if cfg.ReviewSchedule.Timezone == "" {
			cfg.ReviewSchedule.Timezone = "Asia/Shanghai"
		}
		if cfg.ReviewSchedule.DailyReviewTime == "" {
			cfg.ReviewSchedule.DailyReviewTime = "07:30"
		}
		if cfg.ReviewSchedule.WeeklyReviewTime == "" {
			cfg.ReviewSchedule.WeeklyReviewTime = "08:30"
		}
	}

	// Set Execution Window defaults
	if cfg.ExecutionWindow == nil {
		cfg.ExecutionWindow = &ExecutionWindowConfig{
			Enabled: true,
		}
	}

	// Set Feishu defaults
	if cfg.Feishu == nil {
		cfg.Feishu = &FeishuConfig{
			Enabled: false,
		}
	}

	// Set Log defaults
	if cfg.Log == nil {
		cfg.Log = &LogConfig{
			Level: "info",
		}
	}

	// Set OKX defaults
	if cfg.Okx == nil {
		cfg.Okx = &OkxConfig{Enabled: false}
	}

	return &cfg, nil
}
