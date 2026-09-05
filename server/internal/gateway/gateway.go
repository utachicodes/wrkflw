// Package gateway stores the per-account messaging gateway configuration
// that the frwrd daemon pulls. The app is the single control surface:
// channels, allowlists, and per-thread routes are edited here, and the
// daemon on the owner's machine fetches them with an account-scoped
// credential. Agent credentials are rejected by the route guards.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/utachicodes/wrkflw/server/internal/database"
)

const (
	maxTokenChars   = 500
	maxAllowEntries = 100
	maxRoutes       = 50
	maxRouteChars   = 200
)

// Route selects a backend for one conversation thread.
type Route struct {
	Thread string `json:"thread"`
	Agent  string `json:"agent"`
}

// TelegramConfig mirrors the gateway's [telegram] section.
type TelegramConfig struct {
	BotToken     string  `json:"botToken"`
	AllowUserIDs []int64 `json:"allowUserIds"`
	AllowChatIDs []int64 `json:"allowChatIds"`
}

// SlackConfig mirrors the gateway's [slack] section.
type SlackConfig struct {
	AppToken     string   `json:"appToken"`
	BotToken     string   `json:"botToken"`
	AllowUserIDs []string `json:"allowUserIds"`
}

// IMessageConfig mirrors the gateway's [imessage] section.
type IMessageConfig struct {
	SelfHandles []string `json:"selfHandles"`
	AllowFrom   []string `json:"allowFrom"`
}

// Delivery names the fallback channel for scheduled job output.
type Delivery struct {
	Channel string `json:"channel"`
	Target  string `json:"target"`
}

// Config is one account's gateway desired state.
type Config struct {
	Channel          string         `json:"channel"`
	Agent            string         `json:"agent"`
	Telegram         TelegramConfig `json:"telegram"`
	Slack            SlackConfig    `json:"slack"`
	IMessage         IMessageConfig `json:"imessage"`
	PrimaryDelivery  Delivery       `json:"primaryDelivery"`
	Routes           []Route        `json:"routes"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	LastPulledAt     *time.Time     `json:"lastPulledAt,omitempty"`
}

// ErrInvalidConfig reports a rejected gateway configuration.
var ErrInvalidConfig = errors.New("invalid gateway configuration")

// Validate reports whether c is storable.
func (c Config) Validate() error {
	switch c.Channel {
	case "", "telegram", "slack", "imessage":
	default:
		return errors.New("channel must be telegram, slack, or imessage")
	}
	switch c.Agent {
	case "", "codex", "claude", "pi":
	default:
		return errors.New(`agent must be codex, claude, or pi`)
	}
	for _, token := range []string{c.Telegram.BotToken, c.Slack.AppToken, c.Slack.BotToken} {
		if len(token) > maxTokenChars {
			return errors.New("channel tokens are too long")
		}
	}
	for _, list := range [][]int64{c.Telegram.AllowUserIDs, c.Telegram.AllowChatIDs} {
		if len(list) > maxAllowEntries {
			return errors.New("allowlist is too long")
		}
	}
	for _, list := range [][]string{c.Slack.AllowUserIDs, c.IMessage.SelfHandles, c.IMessage.AllowFrom} {
		if len(list) > maxAllowEntries {
			return errors.New("allowlist is too long")
		}
		for _, entry := range list {
			if strings.TrimSpace(entry) == "" || len(entry) > maxRouteChars {
				return errors.New("allowlist entries must be non-empty")
			}
		}
	}
	switch c.PrimaryDelivery.Channel {
	case "", "telegram", "slack", "imessage":
	default:
		return errors.New("primary delivery channel must be telegram, slack, or imessage")
	}
	if len(c.PrimaryDelivery.Target) > maxRouteChars {
		return errors.New("primary delivery target is too long")
	}
	if len(c.Routes) > maxRoutes {
		return errors.New("too many routes")
	}
	for _, route := range c.Routes {
		if strings.TrimSpace(route.Thread) == "" || strings.TrimSpace(route.Agent) == "" {
			return errors.New("routes need a thread and an agent")
		}
		if len(route.Thread) > maxRouteChars || len(route.Agent) > maxRouteChars {
			return errors.New("routes are too long")
		}
	}
	return nil
}

// Store persists gateway configs scoped to the owning account.
type Store struct {
	db *database.Pool
}

// NewStore returns a Store backed by db.
func NewStore(db *database.Pool) *Store {
	return &Store{db: db}
}

func emptyStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

func emptyIntSlice(value []int64) []int64 {
	if value == nil {
		return []int64{}
	}
	return value
}

// Get returns the account's config, or a zero config when unset.
func (s *Store) Get(ctx context.Context, userID string) (Config, error) {
	var c Config
	var routes []byte
	var lastPulledAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT channel, agent,
			telegram_bot_token, telegram_allow_user_ids, telegram_allow_chat_ids,
			slack_app_token, slack_bot_token, slack_allow_user_ids,
			imessage_self_handles, imessage_allow_from,
			primary_channel, primary_target, routes,
			updated_at, last_pulled_at
		FROM gateway_configs
		WHERE user_id = $1
	`, userID).Scan(
		&c.Channel, &c.Agent,
		&c.Telegram.BotToken, &c.Telegram.AllowUserIDs, &c.Telegram.AllowChatIDs,
		&c.Slack.AppToken, &c.Slack.BotToken, &c.Slack.AllowUserIDs,
		&c.IMessage.SelfHandles, &c.IMessage.AllowFrom,
		&c.PrimaryDelivery.Channel, &c.PrimaryDelivery.Target, &routes,
		&c.UpdatedAt, &lastPulledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Config{
				Telegram: TelegramConfig{
					AllowUserIDs: []int64{},
					AllowChatIDs: []int64{},
				},
				Slack: SlackConfig{
					AllowUserIDs: []string{},
				},
				IMessage: IMessageConfig{
					SelfHandles: []string{},
					AllowFrom:   []string{},
				},
				Routes: []Route{},
			}, nil
		}
		return Config{}, err
	}
	if len(routes) > 0 {
		if err := json.Unmarshal(routes, &c.Routes); err != nil {
			return Config{}, err
		}
	}
	if c.Routes == nil {
		c.Routes = []Route{}
	}
	c.Telegram.AllowUserIDs = emptyIntSlice(c.Telegram.AllowUserIDs)
	c.Telegram.AllowChatIDs = emptyIntSlice(c.Telegram.AllowChatIDs)
	c.Slack.AllowUserIDs = emptyStringSlice(c.Slack.AllowUserIDs)
	c.IMessage.SelfHandles = emptyStringSlice(c.IMessage.SelfHandles)
	c.IMessage.AllowFrom = emptyStringSlice(c.IMessage.AllowFrom)
	c.LastPulledAt = lastPulledAt
	return c, nil
}

// Upsert validates and stores the account's config.
func (s *Store) Upsert(ctx context.Context, userID string, c Config) (Config, error) {
	if err := c.Validate(); err != nil {
		return Config{}, errors.Join(ErrInvalidConfig, err)
	}
	routes, err := json.Marshal(emptyRoutes(c.Routes))
	if err != nil {
		return Config{}, err
	}
	var stored Config
	var storedRoutes []byte
	var lastPulledAt *time.Time
	err = s.db.QueryRow(ctx, `
		INSERT INTO gateway_configs (
			user_id, channel, agent,
			telegram_bot_token, telegram_allow_user_ids, telegram_allow_chat_ids,
			slack_app_token, slack_bot_token, slack_allow_user_ids,
			imessage_self_handles, imessage_allow_from,
			primary_channel, primary_target, routes,
			updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			$10, $11,
			$12, $13, $14,
			now()
		)
		ON CONFLICT (user_id) DO UPDATE SET
			channel = EXCLUDED.channel,
			agent = EXCLUDED.agent,
			telegram_bot_token = EXCLUDED.telegram_bot_token,
			telegram_allow_user_ids = EXCLUDED.telegram_allow_user_ids,
			telegram_allow_chat_ids = EXCLUDED.telegram_allow_chat_ids,
			slack_app_token = EXCLUDED.slack_app_token,
			slack_bot_token = EXCLUDED.slack_bot_token,
			slack_allow_user_ids = EXCLUDED.slack_allow_user_ids,
			imessage_self_handles = EXCLUDED.imessage_self_handles,
			imessage_allow_from = EXCLUDED.imessage_allow_from,
			primary_channel = EXCLUDED.primary_channel,
			primary_target = EXCLUDED.primary_target,
			routes = EXCLUDED.routes,
			updated_at = now()
		RETURNING channel, agent,
			telegram_bot_token, telegram_allow_user_ids, telegram_allow_chat_ids,
			slack_app_token, slack_bot_token, slack_allow_user_ids,
			imessage_self_handles, imessage_allow_from,
			primary_channel, primary_target, routes,
			updated_at, last_pulled_at
	`,
		userID, c.Channel, c.Agent,
		c.Telegram.BotToken, emptyIntSlice(c.Telegram.AllowUserIDs), emptyIntSlice(c.Telegram.AllowChatIDs),
		c.Slack.AppToken, c.Slack.BotToken, emptyStringSlice(c.Slack.AllowUserIDs),
		emptyStringSlice(c.IMessage.SelfHandles), emptyStringSlice(c.IMessage.AllowFrom),
		c.PrimaryDelivery.Channel, c.PrimaryDelivery.Target, routes,
	).Scan(
		&stored.Channel, &stored.Agent,
		&stored.Telegram.BotToken, &stored.Telegram.AllowUserIDs, &stored.Telegram.AllowChatIDs,
		&stored.Slack.AppToken, &stored.Slack.BotToken, &stored.Slack.AllowUserIDs,
		&stored.IMessage.SelfHandles, &stored.IMessage.AllowFrom,
		&stored.PrimaryDelivery.Channel, &stored.PrimaryDelivery.Target, &storedRoutes,
		&stored.UpdatedAt, &lastPulledAt,
	)
	if err != nil {
		return Config{}, err
	}
	if len(storedRoutes) > 0 {
		if err := json.Unmarshal(storedRoutes, &stored.Routes); err != nil {
			return Config{}, err
		}
	}
	if stored.Routes == nil {
		stored.Routes = []Route{}
	}
	stored.LastPulledAt = lastPulledAt
	return stored, nil
}

// MarkPulled records that the owner's daemon fetched the config and returns
// the current config. Gateways poll this; the app reads without stamping.
func (s *Store) MarkPulled(ctx context.Context, userID string) (Config, error) {
	c, err := s.Get(ctx, userID)
	if err != nil {
		return Config{}, err
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO gateway_configs (user_id, updated_at, last_pulled_at)
		VALUES ($1, now(), now())
		ON CONFLICT (user_id) DO UPDATE SET last_pulled_at = now()
	`, userID); err != nil {
		return Config{}, err
	}
	now := time.Now()
	c.LastPulledAt = &now
	return c, nil
}

func emptyRoutes(routes []Route) []Route {
	if routes == nil {
		return []Route{}
	}
	return routes
}
