// Package alert implements the rule engine: it evaluates rules against
// each Snapshot and dispatches notifications to configured channels.
//
// Rules and channels are stored as YAML — see internal/config for the
// runtime config and an alerts.yaml file for rule definitions.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	t "github.com/mojtaba/portsleuth/backend/internal/collector"
)

// Op is a comparison operator.
type Op string

const (
	OpGT  Op = ">"
	OpGTE Op = ">="
	OpLT  Op = "<"
	OpLTE Op = "<="
	OpEQ  Op = "=="
)

// Metric is the field a rule observes.
type Metric string

const (
	MetricPortBps    Metric = "port_bps"
	MetricTotalRxBps Metric = "total_rx_bps"
	MetricTotalTxBps Metric = "total_tx_bps"
	MetricTotalBps   Metric = "total_bps"
	MetricConnCount  Metric = "connection_count" // per port
)

// Rule is a single alert definition.
type Rule struct {
	ID         string     `yaml:"id" json:"id"`
	Name       string     `yaml:"name" json:"name"`
	Enabled    bool       `yaml:"enabled" json:"enabled"`
	Metric     Metric     `yaml:"metric" json:"metric"`
	Operator   Op         `yaml:"operator" json:"operator"`
	Threshold  float64    `yaml:"threshold" json:"threshold"`
	Port       uint16     `yaml:"port,omitempty" json:"port,omitempty"` // for port-specific rules
	Protocol   t.Protocol `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	ForSeconds int        `yaml:"for_seconds" json:"forSeconds"`
	Channels   []string   `yaml:"channels" json:"channels"` // names of configured channels

	// Internal evaluation state — not persisted.
	conditionSince time.Time `yaml:"-" json:"-"`
	lastFiredAt    time.Time `yaml:"-" json:"-"`
}

// RuleSet is what we read from disk.
type RuleSet struct {
	Rules    []Rule    `yaml:"rules"`
	Channels []Channel `yaml:"channels"`
}

// Channel is a notification destination.
type Channel struct {
	Name     string            `yaml:"name" json:"name"`
	Type     string            `yaml:"type" json:"type"` // webhook | telegram | email
	Webhook  string            `yaml:"webhook,omitempty" json:"webhook,omitempty"`
	Telegram TelegramConfig    `yaml:"telegram,omitempty" json:"telegram,omitempty"`
	Email    EmailConfig       `yaml:"email,omitempty" json:"email,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"-"`
}

// TelegramConfig holds bot token + chat ID.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token" json:"-"` // never serialized to API responses
	ChatID   string `yaml:"chat_id" json:"chatId,omitempty"`
}

// EmailConfig is SMTP-based.
type EmailConfig struct {
	SMTPHost string `yaml:"smtp_host" json:"smtpHost,omitempty"`
	SMTPPort int    `yaml:"smtp_port" json:"smtpPort,omitempty"`
	From     string `yaml:"from" json:"from,omitempty"`
	To       string `yaml:"to" json:"to,omitempty"`
	Username string `yaml:"username" json:"-"`
	Password string `yaml:"password" json:"-"`
}

// Event is what we emit when a rule fires.
type Event struct {
	RuleID    string    `json:"ruleId"`
	RuleName  string    `json:"ruleName"`
	Metric    Metric    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Port      uint16    `json:"port,omitempty"`
	TS        time.Time `json:"ts"`
}

// Engine evaluates rules and dispatches notifications.
type Engine struct {
	mu        sync.RWMutex
	path      string
	rules     []Rule
	channels  map[string]Channel
	events    []Event // ring buffer of recent events for the UI
	maxEvents int
	logger    Logger
}

// Logger lets the engine report dispatch errors.
type Logger interface {
	Printf(format string, args ...any)
}

// NewEngine loads rules from path. If path doesn't exist, starts empty.
func NewEngine(path string, logger Logger) (*Engine, error) {
	e := &Engine{
		path:      path,
		channels:  map[string]Channel{},
		maxEvents: 100,
		logger:    logger,
	}
	if path == "" {
		return e, nil
	}
	if data, err := os.ReadFile(path); err == nil {
		var rs RuleSet
		if err := yaml.Unmarshal(data, &rs); err != nil {
			return nil, fmt.Errorf("parse rules: %w", err)
		}
		e.rules = rs.Rules
		for _, c := range rs.Channels {
			e.channels[c.Name] = c
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return e, nil
}

// Rules returns a snapshot of current rules.
func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// Channels returns the channels (without secrets).
func (e *Engine) Channels() []Channel {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Channel, 0, len(e.channels))
	for _, c := range e.channels {
		safe := c
		safe.Telegram.BotToken = ""
		safe.Email.Username = ""
		safe.Email.Password = ""
		out = append(out, safe)
	}
	return out
}

// RecentEvents returns the latest events (newest first).
func (e *Engine) RecentEvents() []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Event, len(e.events))
	copy(out, e.events)
	return out
}

// Evaluate is called by the sampler with each new snapshot.
// Rules in conditionSince > forSeconds fire and reset the timer.
func (e *Engine) Evaluate(snap t.Snapshot) []Event {
	now := snap.TS
	e.mu.Lock()
	defer e.mu.Unlock()
	var fired []Event
	for i := range e.rules {
		r := &e.rules[i]
		if !r.Enabled {
			r.conditionSince = time.Time{}
			continue
		}
		val, ok := extractValue(*r, snap)
		if !ok {
			continue
		}
		if !compare(r.Operator, val, r.Threshold) {
			r.conditionSince = time.Time{}
			continue
		}
		if r.conditionSince.IsZero() {
			r.conditionSince = now
		}
		if int(now.Sub(r.conditionSince).Seconds()) < r.ForSeconds {
			continue
		}
		// Throttle: don't refire within forSeconds * 2.
		if !r.lastFiredAt.IsZero() && now.Sub(r.lastFiredAt) < time.Duration(r.ForSeconds*2)*time.Second {
			continue
		}
		evt := Event{
			RuleID:    r.ID,
			RuleName:  r.Name,
			Metric:    r.Metric,
			Value:     val,
			Threshold: r.Threshold,
			Port:      r.Port,
			TS:        now,
		}
		fired = append(fired, evt)
		r.lastFiredAt = now
		e.appendEvent(evt)
		// Dispatch outside the lock; copy needed channel names.
		channels := make([]string, len(r.Channels))
		copy(channels, r.Channels)
		go e.dispatch(evt, channels)
	}
	return fired
}

func (e *Engine) appendEvent(evt Event) {
	e.events = append([]Event{evt}, e.events...)
	if len(e.events) > e.maxEvents {
		e.events = e.events[:e.maxEvents]
	}
}

func extractValue(r Rule, snap t.Snapshot) (float64, bool) {
	switch r.Metric {
	case MetricTotalRxBps:
		return snap.Totals.RxBytesPerSec, true
	case MetricTotalTxBps:
		return snap.Totals.TxBytesPerSec, true
	case MetricTotalBps:
		return snap.Totals.RxBytesPerSec + snap.Totals.TxBytesPerSec, true
	case MetricPortBps:
		for _, p := range snap.Ports {
			if p.LocalPort == r.Port && (r.Protocol == "" || p.Protocol == r.Protocol) {
				return p.TotalBps, true
			}
		}
	case MetricConnCount:
		for _, p := range snap.Ports {
			if p.LocalPort == r.Port && (r.Protocol == "" || p.Protocol == r.Protocol) {
				return float64(p.ConnectionCount), true
			}
		}
	}
	return 0, false
}

func compare(op Op, a, b float64) bool {
	switch op {
	case OpGT:
		return a > b
	case OpGTE:
		return a >= b
	case OpLT:
		return a < b
	case OpLTE:
		return a <= b
	case OpEQ:
		return a == b
	}
	return false
}

// dispatch sends the event to all referenced channels.
func (e *Engine) dispatch(evt Event, names []string) {
	for _, name := range names {
		e.mu.RLock()
		ch, ok := e.channels[name]
		e.mu.RUnlock()
		if !ok {
			continue
		}
		var err error
		switch ch.Type {
		case "webhook":
			err = sendWebhook(ch.Webhook, ch.Headers, evt)
		case "telegram":
			err = sendTelegram(ch.Telegram, evt)
		case "email":
			err = sendEmail(ch.Email, evt)
		default:
			err = fmt.Errorf("unknown channel type %q", ch.Type)
		}
		if err != nil && e.logger != nil {
			e.logger.Printf("alert dispatch %s: %v", name, err)
		}
	}
}

// SetRules replaces the rule list (used by the API to update from the UI).
func (e *Engine) SetRules(ctx context.Context, rules []Rule) error {
	e.mu.Lock()
	e.rules = rules
	e.mu.Unlock()
	return e.persist()
}

// SetChannels replaces channels.
func (e *Engine) SetChannels(channels []Channel) error {
	e.mu.Lock()
	e.channels = map[string]Channel{}
	for _, c := range channels {
		e.channels[c.Name] = c
	}
	e.mu.Unlock()
	return e.persist()
}

func (e *Engine) persist() error {
	if e.path == "" {
		return nil
	}
	e.mu.RLock()
	rs := RuleSet{Rules: e.rules}
	for _, c := range e.channels {
		rs.Channels = append(rs.Channels, c)
	}
	e.mu.RUnlock()
	data, err := yaml.Marshal(rs)
	if err != nil {
		return err
	}
	return os.WriteFile(e.path, data, 0640)
}

// ============================================================
// Channel implementations
// ============================================================

func sendWebhook(rawURL string, headers map[string]string, evt Event) error {
	if rawURL == "" {
		return fmt.Errorf("webhook URL empty")
	}
	body, _ := json.Marshal(evt)
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

func sendTelegram(tg TelegramConfig, evt Event) error {
	if tg.BotToken == "" || tg.ChatID == "" {
		return fmt.Errorf("telegram not configured")
	}
	msg := fmt.Sprintf(
		"🚨 *PortSleuth alert*\n*%s*\nmetric: `%s`\nvalue: `%.2f` (threshold: `%.2f`)\nport: `%d`\nat %s",
		evt.RuleName, evt.Metric, evt.Value, evt.Threshold, evt.Port, evt.TS.Format(time.RFC3339),
	)
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tg.BotToken)
	form := url.Values{}
	form.Set("chat_id", tg.ChatID)
	form.Set("text", msg)
	form.Set("parse_mode", "Markdown")
	resp, err := http.PostForm(endpoint, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram returned %d", resp.StatusCode)
	}
	return nil
}

// sendEmail uses net/smtp under the hood.
func sendEmail(_ EmailConfig, _ Event) error {
	return fmt.Errorf("email not implemented yet — use webhook or telegram")
}

// _ keep imports
var _ = strings.TrimSpace
