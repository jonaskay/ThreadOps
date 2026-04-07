package main

type OuterEvent struct {
	Token          string          `json:"token"`
	TeamID         string          `json:"team_id"`
	APIAppID       string          `json:"api_app_id"`
	Event          InnerEvent      `json:"event"`
	Type           string          `json:"type"`
	EventID        string          `json:"event_id"`
	EventTime      int64           `json:"event_time"`
	Authorizations []Authorization `json:"authorizations"`
}

type InnerEvent struct {
	Type    string `json:"type"`
	User    string `json:"user"`
	Text    string `json:"text"`
	TS      string `json:"ts"`
	Channel string `json:"channel"`
	EventTS string `json:"event_ts"`
}

type Authorization struct {
	TeamID              string `json:"team_id"`
	UserID              string `json:"user_id"`
	IsBot               bool   `json:"is_bot"`
	IsEnterpriseInstall bool   `json:"is_enterprise_install"`
}
