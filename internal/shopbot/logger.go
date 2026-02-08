package shopbot

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ShopLogger provides rich, QoL-focused logging for the shop bot.
type ShopLogger struct {
	*slog.Logger
	cfg         LoggingConfig
	logFile     *os.File
	sessionStart time.Time
	stats       *SessionStats
}

// SessionStats tracks shopping session statistics for diagnostics.
type SessionStats struct {
	GamesPlayed      int
	ShopRefreshes    int
	ClawsScanned     int
	ClawsMatched     int
	ClawsPurchased   int
	GoldSpent        int
	PathingAttempts  int
	PathingFailures  int
	AnyaInteractions int
	Errors           []LoggedError
	StartTime        time.Time
}

// LoggedError records an error with context.
type LoggedError struct {
	Time    time.Time
	Context string
	Message string
}

// NewShopLogger creates a new shop bot logger with the given configuration.
func NewShopLogger(cfg LoggingConfig) (*ShopLogger, error) {
	sl := &ShopLogger{
		cfg:          cfg,
		sessionStart: time.Now(),
		stats: &SessionStats{
			StartTime: time.Now(),
		},
	}

	level := slog.LevelInfo
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if cfg.LogToFile {
		logDir := cfg.LogDir
		if logDir == "" {
			logDir = "logs/shopbot"
		}
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("creating log directory %s: %w", logDir, err)
		}

		logPath := filepath.Join(logDir, fmt.Sprintf("shopbot_%s.log", time.Now().Format("2006-01-02_15-04-05")))
		f, err := os.Create(logPath)
		if err != nil {
			return nil, fmt.Errorf("creating log file %s: %w", logPath, err)
		}
		sl.logFile = f
		writers = append(writers, f)
	}

	multiWriter := io.MultiWriter(writers...)
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level: level,
	})
	sl.Logger = slog.New(handler)

	return sl, nil
}

// Close closes the log file if one was opened.
func (sl *ShopLogger) Close() {
	if sl.logFile != nil {
		sl.logFile.Close()
	}
}

// LogClawScan logs a claw scan result with full detail.
func (sl *ShopLogger) LogClawScan(clawName string, quality string, skills map[string]int, matched bool, reason string) {
	if !sl.cfg.LogShopping {
		return
	}

	skillStr := formatSkills(skills)
	if matched {
		sl.Logger.Info("CLAW MATCH FOUND",
			slog.String("claw", clawName),
			slog.String("quality", quality),
			slog.String("skills", skillStr),
			slog.String("reason", reason),
		)
	} else {
		sl.Logger.Debug("Claw scanned (no match)",
			slog.String("claw", clawName),
			slog.String("quality", quality),
			slog.String("skills", skillStr),
			slog.String("reason", reason),
		)
	}
}

// LogPathingStep logs a pathing decision.
func (sl *ShopLogger) LogPathingStep(from, to string, distance int, step int, totalSteps int) {
	if !sl.cfg.LogPathing {
		return
	}
	sl.Logger.Debug("Pathing",
		slog.String("from", from),
		slog.String("to", to),
		slog.Int("distance", distance),
		slog.Int("step", step),
		slog.Int("totalSteps", totalSteps),
	)
}

// LogGameState logs a game state change.
func (sl *ShopLogger) LogGameState(event string, details map[string]interface{}) {
	if !sl.cfg.LogGameState {
		return
	}
	attrs := []any{slog.String("event", event)}
	for k, v := range details {
		attrs = append(attrs, slog.Any(k, v))
	}
	sl.Logger.Info("GameState", attrs...)
}

// LogError records an error for the diagnostic summary.
func (sl *ShopLogger) LogError(context, msg string) {
	sl.stats.Errors = append(sl.stats.Errors, LoggedError{
		Time:    time.Now(),
		Context: context,
		Message: msg,
	})
	sl.Logger.Error(msg, slog.String("context", context))
}

// PrintDiagnosticSummary outputs a human-readable summary of what happened.
func (sl *ShopLogger) PrintDiagnosticSummary() {
	elapsed := time.Since(sl.stats.StartTime)

	summary := fmt.Sprintf(`
========================================
  D2R SHOP BOT - SESSION SUMMARY
========================================
  Duration:           %s
  Games Played:       %d
  Shop Refreshes:     %d
  Claws Scanned:      %d
  Claws Matched:      %d
  Claws Purchased:    %d
  Gold Spent:         %d
  Pathing Attempts:   %d
  Pathing Failures:   %d
  Anya Interactions:  %d
========================================`,
		elapsed.Round(time.Second),
		sl.stats.GamesPlayed,
		sl.stats.ShopRefreshes,
		sl.stats.ClawsScanned,
		sl.stats.ClawsMatched,
		sl.stats.ClawsPurchased,
		sl.stats.GoldSpent,
		sl.stats.PathingAttempts,
		sl.stats.PathingFailures,
		sl.stats.AnyaInteractions,
	)

	if len(sl.stats.Errors) > 0 {
		summary += fmt.Sprintf("\n\n  ERRORS (%d total):\n", len(sl.stats.Errors))
		// Show last 10 errors
		start := 0
		if len(sl.stats.Errors) > 10 {
			start = len(sl.stats.Errors) - 10
		}
		for _, e := range sl.stats.Errors[start:] {
			summary += fmt.Sprintf("    [%s] %s: %s\n", e.Time.Format("15:04:05"), e.Context, e.Message)
		}
	}

	if sl.stats.ClawsScanned > 0 && sl.stats.ClawsMatched == 0 {
		summary += "\n  DIAGNOSTIC: Scanned claws but found no matches."
		summary += "\n  -> Check your filter profiles - skills or min_total_skills may be too strict."
		summary += "\n  -> Try lowering min_total_skills or adding more optional_skills."
	}

	if sl.stats.PathingFailures > 0 {
		pct := float64(sl.stats.PathingFailures) / float64(sl.stats.PathingAttempts) * 100
		summary += fmt.Sprintf("\n  DIAGNOSTIC: Pathing failed %.0f%% of the time.", pct)
		if pct > 50 {
			summary += "\n  -> The bot may be stuck or the collision grid isn't loading."
			summary += "\n  -> Try increasing pathing_timeout or restarting D2R."
		}
	}

	if sl.stats.AnyaInteractions == 0 && sl.stats.GamesPlayed > 0 {
		summary += "\n  DIAGNOSTIC: Never interacted with Anya."
		summary += "\n  -> The bot may not be reaching Harrogath or Anya's position."
		summary += "\n  -> Check if you're in the correct difficulty and have Harrogath waypoint."
	}

	summary += "\n========================================"
	fmt.Println(summary)

	if sl.logFile != nil {
		fmt.Fprintln(sl.logFile, summary)
	}
}

func formatSkills(skills map[string]int) string {
	if len(skills) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(skills))
	for name, level := range skills {
		parts = append(parts, fmt.Sprintf("+%d %s", level, name))
	}
	return strings.Join(parts, ", ")
}
