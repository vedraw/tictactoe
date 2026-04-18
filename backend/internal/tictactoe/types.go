package tictactoe

import "github.com/heroiclabs/nakama-common/runtime"

type Player struct {
	UserID   string          `json:"userId"`
	Username string          `json:"username"`
	Presence runtime.Presence `json:"-"`
}

type MatchState struct {
	Mode            string            `json:"mode"`
	Board           []string          `json:"board"`
	Players         []Player          `json:"players"`
	PlayerSymbols   map[string]string `json:"playerSymbols"`
	TurnUserID      string            `json:"turnUserId"`
	Winner          string            `json:"winner"`
	WinnerName      string            `json:"winnerName"`
	Status          string            `json:"status"`
	MoveCount       int               `json:"moveCount"`
	TurnDeadlineSec int64             `json:"turnDeadlineSec"`
	TurnDurationSec int64             `json:"turnDurationSec"`
	EndedAtTick     int64             `json:"endedAtTick"`
}

type rateBucket struct {
	WindowStart int64
	Count       int
}
