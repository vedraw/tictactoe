package tictactoe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/heroiclabs/nakama-common/runtime"
)

// TicTacToeMatch implements runtime.Match for authoritative tic-tac-toe.
type TicTacToeMatch struct{}

// NewMatch is the Nakama match constructor.
func NewMatch(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
	return &TicTacToeMatch{}, nil
}

func (m *TicTacToeMatch) MatchInit(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	mode := "classic"
	if params != nil {
		if v, ok := params["mode"].(string); ok && v == "timed" {
			mode = "timed"
		}
	}
	state := &MatchState{
		Mode:            mode,
		Board:           []string{"", "", "", "", "", "", "", "", ""},
		Players:         []Player{},
		PlayerSymbols:   map[string]string{},
		Status:          StatusWaiting,
		TurnDurationSec: 0,
	}
	if mode == "timed" {
		state.TurnDurationSec = 30
	}
	label := fmt.Sprintf(`{"mode":"%s","status":"%s"}`, mode, StatusWaiting)
	return state, 2, label
}

func (m *TicTacToeMatch) MatchJoinAttempt(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presence runtime.Presence, metadata map[string]string) (interface{}, bool, string) {
	ms := state.(*MatchState)
	if len(ms.Players) >= 2 {
		return state, false, "Match is full."
	}
	if ms.Status == StatusFinished || ms.Status == StatusAbandoned {
		return state, false, "Match is no longer active."
	}
	return state, true, ""
}

func (m *TicTacToeMatch) MatchJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	ms := state.(*MatchState)
	for _, p := range presences {
		found := false
		for i := range ms.Players {
			if ms.Players[i].UserID == p.GetUserId() {
				ms.Players[i].Presence = p
				ms.Players[i].Username = p.GetUsername()
				found = true
				break
			}
		}
		if !found {
			player := Player{UserID: p.GetUserId(), Username: p.GetUsername(), Presence: p}
			ms.Players = append(ms.Players, player)
			if _, ok := ms.PlayerSymbols[player.UserID]; !ok {
				if len(ms.PlayerSymbols) == 0 {
					ms.PlayerSymbols[player.UserID] = "X"
				} else {
					ms.PlayerSymbols[player.UserID] = "O"
				}
			}
		}
	}

	if len(ms.Players) == 2 && ms.Status == StatusWaiting {
		ms.Status = StatusActive
		ms.TurnUserID = ms.Players[0].UserID
		if ms.TurnDurationSec > 0 {
			ms.TurnDeadlineSec = time.Now().Unix() + ms.TurnDurationSec
		}
		_ = dispatcher.MatchLabelUpdate(fmt.Sprintf(`{"mode":"%s","status":"%s"}`, ms.Mode, StatusActive))
		_ = broadcastSystem(dispatcher, "Match started")
	}

	_ = broadcastState(dispatcher, ms)
	return ms
}

func (m *TicTacToeMatch) MatchLeave(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	ms := state.(*MatchState)
	switch matchLeaveApply(ms, presences, tick) {
	case matchLeavePersistFinished:
		persistMatchResult(ctx, nkPersistenceForMatch(nk), ms)
		_ = dispatcher.MatchLabelUpdate(fmt.Sprintf(`{"mode":"%s","status":"%s"}`, ms.Mode, StatusFinished))
	case matchLeaveAbandoned:
		_ = dispatcher.MatchLabelUpdate(fmt.Sprintf(`{"mode":"%s","status":"%s"}`, ms.Mode, StatusAbandoned))
	}
	_ = broadcastState(dispatcher, ms)
	return ms
}

func (m *TicTacToeMatch) MatchLoop(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, messages []runtime.MatchData) interface{} {
	ms := state.(*MatchState)
	moveAttempts := map[string]int{}
	stateDirty := false
	now := time.Now().Unix()

	if timedTurnExpired(ms, now) {
		applyTimedTurnForfeitCore(ms, tick)
		persistMatchResult(ctx, nkPersistenceForMatch(nk), ms)
		_ = dispatcher.MatchLabelUpdate(fmt.Sprintf(`{"mode":"%s","status":"%s"}`, ms.Mode, StatusFinished))
		_ = broadcastSystem(dispatcher, "Turn timed out")
		stateDirty = true
	}

	for _, message := range messages {
		if message.GetOpCode() != OpMove || ms.Status != StatusActive {
			continue
		}
		userID := message.GetUserId()
		moveAttempts[userID]++
		if moveAttempts[userID] > RateLimitMovePerTick {
			_ = sendError(dispatcher, ms, userID, ErrorCodes.RateLimitMove, "Move rate limit exceeded.", map[string]interface{}{
				"perTickLimit": RateLimitMovePerTick,
				"tick":         tick,
			})
			continue
		}

		var payload struct {
			Position int `json:"position"`
		}
		if err := json.Unmarshal(message.GetData(), &payload); err != nil {
			_ = sendError(dispatcher, ms, userID, ErrorCodes.InvalidPayload, "Invalid payload.", nil)
			continue
		}
		if code := moveValidationCode(ms, userID, payload.Position); code != "" {
			var msg string
			switch code {
			case ErrorCodes.NotYourTurn:
				msg = "Not your turn."
			case ErrorCodes.InvalidPos:
				msg = "Invalid board position."
			case ErrorCodes.CellOccupied:
				msg = "Cell is already occupied."
			case ErrorCodes.PlayerNotIn:
				msg = "You are not part of this match."
			default:
				msg = "Invalid move."
			}
			_ = sendError(dispatcher, ms, userID, code, msg, nil)
			continue
		}

		stateDirty = true
		applyMoveAfterValidation(ms, userID, message.GetUsername(), payload.Position, tick, now)

		if ms.Status == StatusFinished {
			persistMatchResult(ctx, nkPersistenceForMatch(nk), ms)
			_ = dispatcher.MatchLabelUpdate(fmt.Sprintf(`{"mode":"%s","status":"%s"}`, ms.Mode, StatusFinished))
		}
	}

	if stateDirty {
		_ = broadcastState(dispatcher, ms)
	}
	return ms
}

func (m *TicTacToeMatch) MatchTerminate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, graceSeconds int) interface{} {
	ms := state.(*MatchState)
	ms.Status = StatusFinished
	ms.EndedAtTick = tick
	_ = broadcastState(dispatcher, ms)
	return ms
}

func (m *TicTacToeMatch) MatchSignal(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, data string) (interface{}, string) {
	return state, "ok"
}
