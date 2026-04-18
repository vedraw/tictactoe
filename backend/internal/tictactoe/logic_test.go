package tictactoe

import (
	"testing"

	"github.com/heroiclabs/nakama-common/runtime"
)

func TestTimedTurnExpired(t *testing.T) {
	t.Parallel()
	ms := &MatchState{Status: StatusActive, Mode: "timed", TurnDeadlineSec: 100}
	if timedTurnExpired(ms, 99) {
		t.Fatal("before deadline")
	}
	if !timedTurnExpired(ms, 100) {
		t.Fatal("at deadline")
	}
	if !timedTurnExpired(ms, 101) {
		t.Fatal("after deadline")
	}
	ms.Mode = "classic"
	if timedTurnExpired(ms, 200) {
		t.Fatal("classic never expires by clock")
	}
	ms.Mode = "timed"
	ms.TurnDeadlineSec = 0
	if timedTurnExpired(ms, 200) {
		t.Fatal("no deadline set")
	}
	ms.TurnDeadlineSec = 50
	ms.Status = StatusWaiting
	if timedTurnExpired(ms, 200) {
		t.Fatal("not active")
	}
}

func TestApplyTimedTurnForfeitCore(t *testing.T) {
	t.Parallel()
	ms := &MatchState{
		Status:     StatusActive,
		Players:    []Player{{UserID: "a", Username: "A"}, {UserID: "b", Username: "B"}},
		TurnUserID: "a",
	}
	applyTimedTurnForfeitCore(ms, 42)
	if ms.Status != StatusFinished || ms.EndedAtTick != 42 {
		t.Fatal("status/tick")
	}
	if ms.Winner != "b" || ms.WinnerName != "B" {
		t.Fatal("winner should be non-acting player", ms.Winner, ms.WinnerName)
	}
}

func TestMoveValidationCode(t *testing.T) {
	t.Parallel()
	ms := activeTwoPlayerState("a", "b")
	ms.TurnUserID = "a"
	if code := moveValidationCode(ms, "b", 0); code != ErrorCodes.NotYourTurn {
		t.Fatalf("wrong turn: %q", code)
	}
	if code := moveValidationCode(ms, "a", -1); code != ErrorCodes.InvalidPos {
		t.Fatalf("pos low: %q", code)
	}
	if code := moveValidationCode(ms, "a", 9); code != ErrorCodes.InvalidPos {
		t.Fatalf("pos high: %q", code)
	}
	ms.Board[0] = "X"
	if code := moveValidationCode(ms, "a", 0); code != ErrorCodes.CellOccupied {
		t.Fatalf("occupied: %q", code)
	}
	ms.Board[0] = ""
	delete(ms.PlayerSymbols, "a")
	if code := moveValidationCode(ms, "a", 1); code != ErrorCodes.PlayerNotIn {
		t.Fatalf("not in match: %q", code)
	}
	ms.PlayerSymbols["a"] = "X"
	if code := moveValidationCode(ms, "a", 1); code != "" {
		t.Fatalf("valid move: %q", code)
	}
}

func TestApplyMoveAfterValidationWinRow(t *testing.T) {
	t.Parallel()
	ms := activeTwoPlayerState("a", "b")
	ms.TurnUserID = "a"
	ms.PlayerSymbols["a"] = "X"
	ms.PlayerSymbols["b"] = "O"
	const tick int64 = 5
	const now int64 = 1000
	applyMoveAfterValidation(ms, "a", "Alice", 0, tick, now)
	applyMoveAfterValidation(ms, "b", "Bob", 3, tick, now)
	applyMoveAfterValidation(ms, "a", "Alice", 1, tick, now)
	applyMoveAfterValidation(ms, "b", "Bob", 4, tick, now)
	applyMoveAfterValidation(ms, "a", "Alice", 2, tick, now)
	if ms.Status != StatusFinished || ms.Winner != "a" {
		t.Fatalf("win row: status=%s winner=%q", ms.Status, ms.Winner)
	}
	if ms.MoveCount != 5 {
		t.Fatal("move count")
	}
}

func TestApplyMoveAfterValidationDraw(t *testing.T) {
	t.Parallel()
	ms := activeTwoPlayerState("a", "b")
	ms.TurnUserID = "a"
	ms.PlayerSymbols["a"] = "X"
	ms.PlayerSymbols["b"] = "O"
	ms.Board = []string{
		"X", "O", "X",
		"O", "O", "X",
		"O", "X", "",
	}
	ms.MoveCount = 8
	ms.TurnUserID = "b"
	applyMoveAfterValidation(ms, "b", "B", 8, 99, 1000)
	if ms.Status != StatusFinished || ms.Winner != "" || ms.MoveCount != 9 {
		t.Fatalf("draw: status=%s winner=%q moves=%d", ms.Status, ms.Winner, ms.MoveCount)
	}
}

func TestApplyMoveAfterValidationTurnAndDeadline(t *testing.T) {
	t.Parallel()
	ms := activeTwoPlayerState("a", "b")
	ms.Mode = "timed"
	ms.TurnDurationSec = 30
	ms.TurnUserID = "a"
	ms.PlayerSymbols["a"] = "X"
	ms.PlayerSymbols["b"] = "O"
	ms.TurnDeadlineSec = 500
	applyMoveAfterValidation(ms, "a", "A", 4, 1, 1000)
	if ms.TurnUserID != "b" {
		t.Fatal("turn flip")
	}
	if ms.TurnDeadlineSec != 1030 {
		t.Fatalf("deadline: got %d want 1030", ms.TurnDeadlineSec)
	}
	ms.TurnDurationSec = 0
	applyMoveAfterValidation(ms, "b", "B", 0, 2, 2000)
	if ms.TurnUserID != "a" {
		t.Fatal("turn back")
	}
	if ms.TurnDeadlineSec != 1030 {
		t.Fatal("classic timeduration 0 should not update deadline", ms.TurnDeadlineSec)
	}
}

func TestMatchLeaveApply(t *testing.T) {
	t.Parallel()
	t.Run("abandon waiting solo", func(t *testing.T) {
		ms := &MatchState{Status: StatusWaiting, Players: []Player{{UserID: "solo"}}}
		pr := mockPresence{uid: "solo", user: "S"}
		if e := matchLeaveApply(ms, []runtime.Presence{pr}, 3); e != matchLeaveAbandoned {
			t.Fatal(e)
		}
		if ms.Status != StatusAbandoned || len(ms.Players) != 0 {
			t.Fatal(ms)
		}
	})
	t.Run("active disconnect win", func(t *testing.T) {
		ms := &MatchState{
			Status: StatusActive,
			Players: []Player{{UserID: "w"}, {UserID: "l"}},
			PlayerSymbols: map[string]string{"w": "X", "l": "O"},
		}
		pr := mockPresence{uid: "l", user: "L"}
		if e := matchLeaveApply(ms, []runtime.Presence{pr}, 9); e != matchLeavePersistFinished {
			t.Fatal(e)
		}
		if ms.Winner != "w" || ms.Status != StatusFinished {
			t.Fatal(ms)
		}
	})
	t.Run("no terminal change", func(t *testing.T) {
		ms := &MatchState{Status: StatusWaiting, Players: []Player{{UserID: "a"}, {UserID: "b"}}}
		pr := mockPresence{uid: "a", user: "A"}
		if e := matchLeaveApply(ms, []runtime.Presence{pr}, 1); e != matchLeaveNone {
			t.Fatal(e)
		}
		if len(ms.Players) != 1 || ms.Status != StatusWaiting {
			t.Fatal(ms)
		}
	})
}

func activeTwoPlayerState(ua, ub string) *MatchState {
	return &MatchState{
		Status: StatusActive,
		Mode:   "classic",
		Board:  []string{"", "", "", "", "", "", "", "", ""},
		Players: []Player{
			{UserID: ua, Username: "A"},
			{UserID: ub, Username: "B"},
		},
		PlayerSymbols: map[string]string{ua: "X", ub: "O"},
		TurnUserID:    ua,
	}
}
