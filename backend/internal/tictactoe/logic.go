package tictactoe

import "github.com/heroiclabs/nakama-common/runtime"

type matchLeaveEffect int

const (
	matchLeaveNone matchLeaveEffect = iota
	matchLeavePersistFinished
	matchLeaveAbandoned
)

func matchLeaveApply(ms *MatchState, presences []runtime.Presence, tick int64) matchLeaveEffect {
	for _, p := range presences {
		for i := len(ms.Players) - 1; i >= 0; i-- {
			if ms.Players[i].UserID == p.GetUserId() {
				ms.Players = append(ms.Players[:i], ms.Players[i+1:]...)
			}
		}
	}
	if ms.Status == StatusActive && len(ms.Players) == 1 {
		ms.Winner = ms.Players[0].UserID
		ms.WinnerName = ms.Players[0].Username
		ms.Status = StatusFinished
		ms.EndedAtTick = tick
		return matchLeavePersistFinished
	}
	if len(ms.Players) == 0 && ms.Status == StatusWaiting {
		ms.Status = StatusAbandoned
		ms.EndedAtTick = tick
		return matchLeaveAbandoned
	}
	return matchLeaveNone
}

func timedTurnExpired(ms *MatchState, nowUnix int64) bool {
	return ms.Status == StatusActive && ms.Mode == "timed" && ms.TurnDeadlineSec > 0 && nowUnix >= ms.TurnDeadlineSec
}

func applyTimedTurnForfeitCore(ms *MatchState, tick int64) {
	winner := OtherPlayer(ms.Players, ms.TurnUserID)
	if winner != nil {
		ms.Winner = winner.UserID
		ms.WinnerName = winner.Username
	}
	ms.Status = StatusFinished
	ms.EndedAtTick = tick
}

func moveValidationCode(ms *MatchState, userID string, position int) string {
	if userID != ms.TurnUserID {
		return ErrorCodes.NotYourTurn
	}
	if position < 0 || position >= BoardSize {
		return ErrorCodes.InvalidPos
	}
	if ms.Board[position] != "" {
		return ErrorCodes.CellOccupied
	}
	if _, ok := ms.PlayerSymbols[userID]; !ok {
		return ErrorCodes.PlayerNotIn
	}
	return ""
}

func applyMoveAfterValidation(ms *MatchState, userID, username string, position int, tick int64, nowUnix int64) {
	symbol := ms.PlayerSymbols[userID]
	ms.Board[position] = symbol
	ms.MoveCount++

	if IsWinningBoard(ms.Board, symbol) {
		ms.Winner = userID
		ms.WinnerName = username
		ms.Status = StatusFinished
		ms.EndedAtTick = tick
		return
	}
	if ms.MoveCount == BoardSize {
		ms.Status = StatusFinished
		ms.EndedAtTick = tick
		return
	}
	next := OtherPlayer(ms.Players, userID)
	if next != nil {
		ms.TurnUserID = next.UserID
		if ms.TurnDurationSec > 0 {
			ms.TurnDeadlineSec = nowUnix + ms.TurnDurationSec
		}
	}
}
