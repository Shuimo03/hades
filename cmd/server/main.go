package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"hades/internal/alerts"
	"hades/internal/brief"
	"hades/internal/config"
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/okx"
	"hades/internal/scheduler"
	"hades/internal/tools"
	llmprompt "hades/llm/prompt"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	logLevel := flag.String("logs", "info", "log level: debug, info, warn, error")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *logLevel == "debug" {
		mcp.SetDebug(true)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("[DEBUG] Debug mode enabled")
	}

	if cfg.AppKey == "" || cfg.AppSecret == "" || cfg.AccessToken == "" {
		log.Fatal("Missing required configuration: app_key, app_secret, access_token")
	}

	scheduleLocation := loadLocationOrLocal(preferredScheduleTimezone(cfg))
	lb := mustLongbridgeClient(cfg)
	defer lb.Close()

	okxClient := newOptionalOKXClient(cfg)
	sched := scheduler.New(scheduleLocation)
	alertMgr, alertScope := newAlertManager(cfg, lb)
	notifier := newNotifier(cfg)
	execWindowMgr := newExecutionWindowManager()

	alertMgr.SetCallback(func(alert *alerts.Alert, message string) {
		if planContext := alerts.BuildAlertPlanContext(context.Background(), lb, alert.Symbol, alertMgr.QuoteScopeForAlert(alert)); planContext != "" {
			message += "\n" + planContext
		}
		log.Printf("[Alerts] Triggered: %s", message)
		notifier.Notify(context.Background(), fmt.Sprintf("Signal Alert: %s", alert.Symbol), message)
	})

	execWindowMgr.SetCallback(func(window *alerts.ExecutionWindow) {
		log.Printf("[ExecutionWindow] Triggered: %s", window.Name)
		gen := brief.New(lb, cfg.DailyBrief.Timezone)
		result, err := gen.Generate(context.Background(), brief.BriefVersionPreMarket, nil)
		if err != nil {
			log.Printf("[ExecutionWindow] Failed to generate brief: %v", err)
			return
		}
		notifier.Notify(context.Background(), "Execution Window", fmt.Sprintf("Execution Window: %s\n\n%s", window.Name, result))
	})

	setupDailyBriefJobs(cfg, lb, notifier, sched)
	setupScheduledReviews(cfg, lb, notifier, sched)
	setupSignalAlertChecker(cfg, alertMgr, alertScope, sched)
	restoreExecutionWindows(cfg, execWindowMgr, sched)

	sched.Start()

	serverInstructions, err := llmprompt.LoadServerInstructions("llm/prompt/server_instructions.md")
	if err != nil {
		log.Fatalf("Failed to load MCP server instructions: %v", err)
	}

	server := mcp.NewHTTPServerWithInstructions("longbridge-mcp", "v1.0.0", serverInstructions)
	if err := llmprompt.RegisterMCPPrompts(server); err != nil {
		log.Fatalf("Failed to register MCP prompts: %v", err)
	}
	registerServerTools(server, cfg, lb, okxClient, alertMgr, execWindowMgr, sched)

	http.Handle("/mcp", server)
	http.Handle("/mcp/", server)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("MCP server starting on %s", addr)
	log.Printf("MCP endpoint: http://%s/mcp", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	sched.Stop()
}

func mustLongbridgeClient(cfg *config.Config) *longbridge.Client {
	lb, err := longbridge.NewClient(cfg.AppKey, cfg.AppSecret, cfg.AccessToken)
	if err != nil {
		log.Fatalf("Failed to create LongBridge client: %v", err)
	}
	return lb
}

func newOptionalOKXClient(cfg *config.Config) *okx.Client {
	if cfg.Okx == nil || !cfg.Okx.Enabled {
		return nil
	}
	if cfg.Okx.APIKey == "" || cfg.Okx.SecretKey == "" || cfg.Okx.Passphrase == "" {
		log.Printf("[OKX] Missing api_key/secret_key/passphrase, skip OKX client")
		return nil
	}
	client := okx.NewClient(cfg.Okx.APIKey, cfg.Okx.SecretKey, cfg.Okx.Passphrase, cfg.Okx.BaseURL)
	log.Printf("[OKX] OKX client initialized")
	return client
}

func newAlertManager(cfg *config.Config, lb *longbridge.Client) (*alerts.Manager, longbridge.QuoteSessionScope) {
	alertScope, valid := longbridge.ParseQuoteSessionScope(cfg.SignalAlert.SessionScope)
	if !valid {
		log.Printf("[Alerts] Invalid session_scope %q, fallback to %s", cfg.SignalAlert.SessionScope, alertScope)
	}

	alertMgr := alerts.New(lb, "data/alerts.json", time.Duration(cfg.SignalAlert.CheckInterval)*time.Second, alertScope)
	if err := alertMgr.Load(); err != nil {
		log.Printf("[Alerts] Failed to load alerts: %v", err)
	}
	return alertMgr, alertScope
}

func newNotifier(cfg *config.Config) alerts.Notifier {
	if cfg.Feishu != nil && cfg.Feishu.Enabled && cfg.Feishu.AppID != "" && cfg.Feishu.AppSecret != "" && cfg.Feishu.UserID != "" {
		log.Printf("[Notifier] Using Feishu API (app_id: %s, user_id: %s)", cfg.Feishu.AppID, cfg.Feishu.UserID)
		return alerts.NewFeishuNotifierWithConfig(cfg.Feishu.AppID, cfg.Feishu.AppSecret, cfg.Feishu.UserID)
	}
	if cfg.SignalAlert != nil && cfg.SignalAlert.WebhookURL != "" {
		log.Printf("[Notifier] Using webhook: %s", cfg.SignalAlert.WebhookURL)
		return alerts.NewNotifier(cfg.SignalAlert.WebhookURL)
	}
	log.Printf("[Notifier] No notification configured")
	return alerts.NewNotifier("")
}

func newExecutionWindowManager() *alerts.ExecutionWindowManager {
	execWindowMgr := alerts.NewExecutionWindowManager("data/execution_windows.json")
	if err := execWindowMgr.Load(); err != nil {
		log.Printf("[ExecutionWindow] Failed to load windows: %v", err)
	}
	return execWindowMgr
}

func setupDailyBriefJobs(cfg *config.Config, lb *longbridge.Client, notifier alerts.Notifier, sched *scheduler.Scheduler) {
	if !cfg.DailyBrief.Enabled {
		return
	}

	timezone := cfg.DailyBrief.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	gen := brief.New(lb, timezone)

	if preTime := cfg.DailyBrief.PreMarketTime; preTime != "" {
		spec, err := weekdayTimeSpec(preTime)
		if err != nil {
			log.Printf("[DailyBrief] Invalid pre-market schedule %q: %v", preTime, err)
		} else if err := sched.AddJob("daily_brief_pre_market", spec, func(ctx context.Context) {
			result, err := gen.Generate(ctx, brief.BriefVersionPreMarket, nil)
			if err != nil {
				log.Printf("[DailyBrief] Pre-market failed: %v", err)
				return
			}
			notifier.Notify(ctx, "Daily Brief - 开盘前", result)
		}); err != nil {
			log.Printf("[DailyBrief] Failed to add pre-market job: %v", err)
		}
	}

	if postTime := cfg.DailyBrief.PostMarketTime; postTime != "" {
		spec, err := weekdayTimeSpec(postTime)
		if err != nil {
			log.Printf("[DailyBrief] Invalid post-market schedule %q: %v", postTime, err)
		} else if err := sched.AddJob("daily_brief_post_market", spec, func(ctx context.Context) {
			result, err := gen.Generate(ctx, brief.BriefVersionPostMarket, nil)
			if err != nil {
				log.Printf("[DailyBrief] Post-market failed: %v", err)
				return
			}
			notifier.Notify(ctx, "Daily Brief - 收盘后", result)
		}); err != nil {
			log.Printf("[DailyBrief] Failed to add post-market job: %v", err)
		}
	}
}

func setupScheduledReviews(cfg *config.Config, lb *longbridge.Client, notifier alerts.Notifier, sched *scheduler.Scheduler) {
	if cfg.ReviewSchedule == nil || !cfg.ReviewSchedule.Enabled {
		return
	}

	reviewTimezone := cfg.ReviewSchedule.Timezone
	dailyReviewTool := tools.NewDailyReviewTool(lb)
	weeklyReviewTool := tools.NewWeeklyReviewTool(lb)

	if spec, err := weekdayRangeTimeSpec(cfg.ReviewSchedule.DailyReviewTime, "2-6"); err != nil {
		log.Printf("[Review] Invalid daily review schedule %q: %v", cfg.ReviewSchedule.DailyReviewTime, err)
	} else if err := sched.AddJob("daily_review", spec, func(ctx context.Context) {
		result, err := dailyReviewTool(ctx, map[string]interface{}{
			"timezone": reviewTimezone,
			"periods":  "1d,4h,1h",
			"lookback": 120,
		})
		if err != nil {
			log.Printf("[Review] Daily review failed: %v", err)
			return
		}
		notifier.Notify(ctx, "Daily Review - 每日复盘", formatScheduledReviewMessage(result))
	}); err != nil {
		log.Printf("[Review] Failed to add daily review job: %v", err)
	}

	if spec, err := weekdayRangeTimeSpec(cfg.ReviewSchedule.WeeklyReviewTime, "6"); err != nil {
		log.Printf("[Review] Invalid weekly review schedule %q: %v", cfg.ReviewSchedule.WeeklyReviewTime, err)
	} else if err := sched.AddJob("weekly_review", spec, func(ctx context.Context) {
		result, err := weeklyReviewTool(ctx, map[string]interface{}{
			"timezone": reviewTimezone,
			"periods":  "1d,4h,1h",
			"lookback": 120,
		})
		if err != nil {
			log.Printf("[Review] Weekly review failed: %v", err)
			return
		}
		notifier.Notify(ctx, "Weekly Review - 周复盘", formatScheduledReviewMessage(result))
	}); err != nil {
		log.Printf("[Review] Failed to add weekly review job: %v", err)
	}
}

func setupSignalAlertChecker(cfg *config.Config, alertMgr *alerts.Manager, alertScope longbridge.QuoteSessionScope, sched *scheduler.Scheduler) {
	if !cfg.SignalAlert.Enabled {
		return
	}

	var alertWindow func(time.Time) bool
	if alertScope == longbridge.QuoteSessionScopeRegular {
		window, err := newClockWindowChecker(cfg.SignalAlert.Timezone, cfg.SignalAlert.SessionStart, cfg.SignalAlert.SessionEnd)
		if err != nil {
			log.Printf("[Alerts] Invalid trading window %s-%s: %v", cfg.SignalAlert.SessionStart, cfg.SignalAlert.SessionEnd, err)
		} else {
			alertWindow = window
		}
	}

	if err := sched.AddJob("signal_alert_check", fmt.Sprintf("@every %ds", cfg.SignalAlert.CheckInterval), func(ctx context.Context) {
		if alertWindow != nil && !alertWindow(time.Now()) {
			return
		}
		alertMgr.CheckAll(ctx)
	}); err != nil {
		log.Printf("[Alerts] Failed to add signal alert job: %v", err)
	}
}

func restoreExecutionWindows(cfg *config.Config, execWindowMgr *alerts.ExecutionWindowManager, sched *scheduler.Scheduler) {
	if !cfg.ExecutionWindow.Enabled {
		return
	}

	for _, window := range execWindowMgr.List() {
		if window == nil || !window.Enabled {
			continue
		}
		windowID := window.ID
		if err := sched.AddJob("execution_window_"+windowID, window.Schedule, func(ctx context.Context) {
			execWindowMgr.Trigger(windowID)
		}); err != nil {
			log.Printf("[ExecutionWindow] Failed to restore window %s: %v", window.Name, err)
		}
	}
}

func weekdayTimeSpec(hhmm string) (string, error) {
	return weekdayRangeTimeSpec(hhmm, "1-5")
}

func weekdayRangeTimeSpec(hhmm string, dow string) (string, error) {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * %s", parsed.Minute(), parsed.Hour(), dow), nil
}

func loadLocationOrLocal(timezone string) *time.Location {
	if strings.TrimSpace(timezone) == "" {
		return time.Local
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("[Scheduler] Invalid timezone %s, using local", timezone)
		return time.Local
	}
	return location
}

func preferredScheduleTimezone(cfg *config.Config) string {
	if cfg.ReviewSchedule != nil && strings.TrimSpace(cfg.ReviewSchedule.Timezone) != "" {
		return cfg.ReviewSchedule.Timezone
	}
	if cfg.DailyBrief != nil && strings.TrimSpace(cfg.DailyBrief.Timezone) != "" {
		return cfg.DailyBrief.Timezone
	}
	if cfg.SignalAlert != nil && strings.TrimSpace(cfg.SignalAlert.Timezone) != "" {
		return cfg.SignalAlert.Timezone
	}
	return "Asia/Shanghai"
}

func newClockWindowChecker(timezone, startHHMM, endHHMM string) (func(time.Time) bool, error) {
	location := loadLocationOrLocal(timezone)
	startMinute, err := minutesOfDay(startHHMM)
	if err != nil {
		return nil, err
	}
	endMinute, err := minutesOfDay(endHHMM)
	if err != nil {
		return nil, err
	}

	return func(now time.Time) bool {
		localNow := now.In(location)
		minuteOfDay := localNow.Hour()*60 + localNow.Minute()
		if startMinute <= endMinute {
			return minuteOfDay >= startMinute && minuteOfDay <= endMinute
		}
		return minuteOfDay >= startMinute || minuteOfDay <= endMinute
	}, nil
}

func minutesOfDay(hhmm string) (int, error) {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func formatScheduledReviewMessage(result map[string]interface{}) string {
	payload, _ := result["result"].(map[string]interface{})
	if payload == nil {
		return "复盘结果为空"
	}

	var sections []string
	if summary, _ := payload["summary"].(string); strings.TrimSpace(summary) != "" {
		sections = append(sections, summary)
	}

	if risks := extractStringSlice(payload["risks"]); len(risks) > 0 {
		limit := minInt(len(risks), 3)
		lines := make([]string, 0, limit+1)
		lines = append(lines, "风险提示")
		for _, risk := range risks[:limit] {
			lines = append(lines, "- "+risk)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if periodMap, _ := payload["period"].(map[string]interface{}); periodMap != nil {
		start, _ := periodMap["start"].(string)
		end, _ := periodMap["end"].(string)
		if start != "" || end != "" {
			sections = append(sections, fmt.Sprintf("复盘区间\n- %s ~ %s", start, end))
		}
	}

	if len(sections) == 0 {
		return "复盘已生成，但没有可展示的摘要。"
	}
	return strings.Join(sections, "\n\n")
}

func extractStringSlice(raw interface{}) []string {
	switch value := raw.(type) {
	case []string:
		return value
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if ok && strings.TrimSpace(text) != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
