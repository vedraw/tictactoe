package tictactoe

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

// nkPersistence is the minimal Nakama surface used by match result persistence.
type nkPersistence interface {
	StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error)
	StorageWrite(ctx context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error)
	LeaderboardRecordWrite(ctx context.Context, id, ownerID, username string, score, subscore int64, metadata map[string]interface{}, overrideOperator *int) (*api.LeaderboardRecord, error)
}

// NoopPersistence implements nkPersistence with successful no-ops (used when tests pass nil NakamaModule).
type NoopPersistence struct{}

func (NoopPersistence) StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return nil, nil
}

func (NoopPersistence) StorageWrite(ctx context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	return nil, nil
}

func (NoopPersistence) LeaderboardRecordWrite(ctx context.Context, id, ownerID, username string, score, subscore int64, metadata map[string]interface{}, overrideOperator *int) (*api.LeaderboardRecord, error) {
	return &api.LeaderboardRecord{}, nil
}

func nkPersistenceForMatch(nk runtime.NakamaModule) nkPersistence {
	if nk == nil {
		return NoopPersistence{}
	}
	return nk
}

func persistMatchResult(ctx context.Context, nk nkPersistence, state *MatchState) {
	if len(state.Players) < 2 {
		return
	}
	a := state.Players[0]
	b := state.Players[1]
	if state.Winner != "" {
		updatePlayerStats(ctx, nk, a, ternary(state.Winner == a.UserID, "win", "loss"))
		updatePlayerStats(ctx, nk, b, ternary(state.Winner == b.UserID, "win", "loss"))
		return
	}
	updatePlayerStats(ctx, nk, a, "draw")
	updatePlayerStats(ctx, nk, b, "draw")
}

func updatePlayerStats(ctx context.Context, nk nkPersistence, p Player, outcome string) {
	stats := readStats(ctx, nk, p.UserID)
	switch outcome {
	case "win":
		stats.Wins++
		stats.Streak++
	case "loss":
		stats.Losses++
		stats.Streak = 0
	default:
		stats.Draws++
		stats.Streak = 0
	}
	value, _ := json.Marshal(stats)
	_, _ = nk.StorageWrite(ctx, []*runtime.StorageWrite{{
		Collection:      StatsCollection,
		Key:             StatsKey,
		UserID:          p.UserID,
		Value:           string(value),
		PermissionRead:  2,
		PermissionWrite: 0,
	}})
	score := int64(stats.Wins*3 + stats.Draws + stats.Streak)
	_, _ = nk.LeaderboardRecordWrite(ctx, LeaderboardID, p.UserID, p.Username, score, int64(stats.Streak), map[string]interface{}{
		"wins":   stats.Wins,
		"losses": stats.Losses,
		"draws":  stats.Draws,
		"streak": stats.Streak,
	}, nil)
}

type statsRecord struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Draws  int `json:"draws"`
	Streak int `json:"streak"`
}

func readStats(ctx context.Context, nk nkPersistence, userID string) statsRecord {
	objects, err := nk.StorageRead(ctx, []*runtime.StorageRead{{
		Collection: StatsCollection,
		Key:        StatsKey,
		UserID:     userID,
	}})
	if err != nil || len(objects) == 0 {
		return statsRecord{}
	}
	var out statsRecord
	if err := json.Unmarshal([]byte(objects[0].Value), &out); err != nil {
		return statsRecord{}
	}
	return out
}

func IntFromMetadata(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

func ternary(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
