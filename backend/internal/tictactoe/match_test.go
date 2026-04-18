package tictactoe

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type mockPresence struct {
	uid, sid, node, user string
}

func (m mockPresence) GetHidden() bool                        { return false }
func (m mockPresence) GetPersistence() bool                     { return false }
func (m mockPresence) GetUsername() string                      { return m.user }
func (m mockPresence) GetStatus() string                        { return "" }
func (m mockPresence) GetReason() runtime.PresenceReason        { return runtime.PresenceReasonUnknown }
func (m mockPresence) GetUserId() string                        { return m.uid }
func (m mockPresence) GetSessionId() string                     { return m.sid }
func (m mockPresence) GetNodeId() string                        { return m.node }

type mockMatchData struct {
	mockPresence
	op   int64
	data []byte
}

func (m mockMatchData) GetOpCode() int64      { return m.op }
func (m mockMatchData) GetData() []byte       { return m.data }
func (m mockMatchData) GetReliable() bool     { return true }
func (m mockMatchData) GetReceiveTime() int64 { return 0 }

type recordDispatcher struct {
	broadcasts []struct {
		op    int64
		data  []byte
		nPres int
	}
	labels []string
}

func (d *recordDispatcher) BroadcastMessage(opCode int64, data []byte, presences []runtime.Presence, sender runtime.Presence, reliable bool) error {
	d.broadcasts = append(d.broadcasts, struct {
		op    int64
		data  []byte
		nPres int
	}{opCode, append([]byte(nil), data...), len(presences)})
	return nil
}

func (d *recordDispatcher) BroadcastMessageDeferred(opCode int64, data []byte, presences []runtime.Presence, sender runtime.Presence, reliable bool) error {
	return nil
}

func (d *recordDispatcher) MatchKick(presences []runtime.Presence) error { return nil }

func (d *recordDispatcher) MatchLabelUpdate(label string) error {
	d.labels = append(d.labels, label)
	return nil
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...interface{})                         {}
func (noopLogger) Info(string, ...interface{})                          {}
func (noopLogger) Warn(string, ...interface{})                          {}
func (noopLogger) Error(string, ...interface{})                         {}
func (noopLogger) WithField(string, interface{}) runtime.Logger         { return noopLogger{} }
func (noopLogger) WithFields(map[string]interface{}) runtime.Logger    { return noopLogger{} }
func (noopLogger) Fields() map[string]interface{}                     { return nil }

type recordNK struct {
	storageWrites int
	lbWrites      int
}

func (r *recordNK) StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return nil, nil
}

func (r *recordNK) StorageWrite(ctx context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	r.storageWrites++
	return nil, nil
}

func (r *recordNK) LeaderboardRecordWrite(ctx context.Context, id, ownerID, username string, score, subscore int64, metadata map[string]interface{}, overrideOperator *int) (*api.LeaderboardRecord, error) {
	r.lbWrites++
	return &api.LeaderboardRecord{}, nil
}

func TestMatchInitModesAndLabel(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	st, rate, label := m.MatchInit(ctx, noopLogger{}, nil, nil, nil)
	ms := st.(*MatchState)
	if rate != 2 || ms.Mode != "classic" || ms.Status != StatusWaiting || ms.TurnDurationSec != 0 {
		t.Fatalf("classic init: %+v tick=%d label=%q", ms, rate, label)
	}
	if label != `{"mode":"classic","status":"waiting"}` {
		t.Fatal(label)
	}
	st, _, label = m.MatchInit(ctx, noopLogger{}, nil, nil, map[string]interface{}{"mode": "timed"})
	ms = st.(*MatchState)
	if ms.Mode != "timed" || ms.TurnDurationSec != 30 {
		t.Fatal("timed init")
	}
	if label != `{"mode":"timed","status":"waiting"}` {
		t.Fatal(label)
	}
	st, _, _ = m.MatchInit(ctx, noopLogger{}, nil, nil, map[string]interface{}{"mode": "nope"})
	ms = st.(*MatchState)
	if ms.Mode != "classic" {
		t.Fatal("unknown mode falls back to classic")
	}
}

func TestMatchJoinAttemptCorners(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	p := mockPresence{uid: "u1", sid: "s1", node: "n1", user: "a"}

	t.Run("full match rejects", func(t *testing.T) {
		ms := &MatchState{Status: StatusWaiting, Players: []Player{{UserID: "x"}, {UserID: "y"}}}
		_, ok, _ := m.MatchJoinAttempt(ctx, noopLogger{}, nil, nil, d, 0, ms, p, nil)
		if ok {
			t.Fatal("expected reject when 2 players already in state")
		}
	})
	t.Run("finished rejects", func(t *testing.T) {
		ms := &MatchState{Status: StatusFinished, Players: []Player{}}
		_, ok, _ := m.MatchJoinAttempt(ctx, noopLogger{}, nil, nil, d, 0, ms, p, nil)
		if ok {
			t.Fatal("finished")
		}
	})
	t.Run("abandoned rejects", func(t *testing.T) {
		ms := &MatchState{Status: StatusAbandoned, Players: []Player{}}
		_, ok, _ := m.MatchJoinAttempt(ctx, noopLogger{}, nil, nil, d, 0, ms, p, nil)
		if ok {
			t.Fatal("abandoned")
		}
	})
	t.Run("waiting accepts", func(t *testing.T) {
		ms := &MatchState{Status: StatusWaiting, Players: []Player{}}
		_, ok, _ := m.MatchJoinAttempt(ctx, noopLogger{}, nil, nil, d, 0, ms, p, nil)
		if !ok {
			t.Fatal("expected accept")
		}
	})
}

func TestMatchJoinTwoPlayersActivates(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := &MatchState{Status: StatusWaiting, Mode: "classic", Board: emptyBoard(), PlayerSymbols: map[string]string{}}
	pr1 := mockPresence{uid: "p1", sid: "s1", node: "n", user: "P1"}
	_ = m.MatchJoin(ctx, noopLogger{}, nil, nil, d, 0, ms, []runtime.Presence{pr1})
	if len(ms.Players) != 1 || ms.PlayerSymbols["p1"] != "X" || ms.Status != StatusWaiting {
		t.Fatalf("after one join: %+v", ms)
	}
	pr2 := mockPresence{uid: "p2", sid: "s2", node: "n", user: "P2"}
	_ = m.MatchJoin(ctx, noopLogger{}, nil, nil, d, 1, ms, []runtime.Presence{pr2})
	if ms.Status != StatusActive || ms.TurnUserID != "p1" || ms.PlayerSymbols["p2"] != "O" {
		t.Fatalf("after two: %+v", ms)
	}
	if len(d.labels) < 1 || d.labels[len(d.labels)-1] != `{"mode":"classic","status":"active"}` {
		t.Fatalf("labels: %v", d.labels)
	}
}

func TestMatchJoinRejoinRefreshesPresence(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := &MatchState{Status: StatusWaiting, Mode: "classic", Board: emptyBoard(), Players: []Player{{UserID: "p1", Username: "old"}}, PlayerSymbols: map[string]string{"p1": "X"}}
	pr := mockPresence{uid: "p1", sid: "newsession", node: "n", user: "newname"}
	_ = m.MatchJoin(ctx, noopLogger{}, nil, nil, d, 0, ms, []runtime.Presence{pr})
	if ms.Players[0].Username != "newname" || ms.Players[0].Presence == nil {
		t.Fatal("presence refresh")
	}
}

func TestMatchLeaveAbandonWaiting(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := &MatchState{Status: StatusWaiting, Mode: "classic", Board: emptyBoard(), Players: []Player{{UserID: "solo", Username: "S"}}, PlayerSymbols: map[string]string{"solo": "X"}}
	pr := mockPresence{uid: "solo", sid: "s", node: "n", user: "S"}
	_ = m.MatchLeave(ctx, noopLogger{}, nil, nil, d, 3, ms, []runtime.Presence{pr})
	if ms.Status != StatusAbandoned || ms.EndedAtTick != 3 || len(ms.Players) != 0 {
		t.Fatalf("abandon: %+v", ms)
	}
	if len(d.labels) == 0 || d.labels[len(d.labels)-1] != `{"mode":"classic","status":"abandoned"}` {
		t.Fatalf("abandon label: %v", d.labels)
	}
}

func TestMatchLeaveActiveDisconnectAwardsWin(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := &MatchState{
		Status: StatusActive, Mode: "classic", Board: emptyBoard(),
		Players: []Player{{UserID: "w", Username: "W"}, {UserID: "l", Username: "L"}},
		PlayerSymbols: map[string]string{"w": "X", "l": "O"},
		TurnUserID:    "w",
	}
	leaver := mockPresence{uid: "l", sid: "s", node: "n", user: "L"}
	_ = m.MatchLeave(ctx, noopLogger{}, nil, nil, d, 7, ms, []runtime.Presence{leaver})
	if ms.Status != StatusFinished || ms.Winner != "w" || ms.WinnerName != "W" {
		t.Fatalf("disconnect win: %+v", ms)
	}
	if len(d.labels) == 0 || d.labels[len(d.labels)-1] != `{"mode":"classic","status":"finished"}` {
		t.Fatalf("labels: %v", d.labels)
	}
}

func TestMatchLoopMoveThrottleSecondMessageSameTick(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := activeTwoPlayerState("a", "b")
	ms.TurnUserID = "a"
	ms.PlayerSymbols["a"] = "X"
	ms.PlayerSymbols["b"] = "O"
	payload, _ := json.Marshal(map[string]int{"position": 0})
	msg := mockMatchData{
		mockPresence: mockPresence{uid: "a", user: "A"},
		op:           int64(OpMove),
		data:         payload,
	}
	_ = m.MatchLoop(ctx, noopLogger{}, nil, nil, d, 1, ms, []runtime.MatchData{msg, msg})
	if ms.Board[0] != "X" {
		t.Fatal("only one move should apply")
	}
	var errCount int
	for _, b := range d.broadcasts {
		if b.op != int64(OpError) {
			continue
		}
		var env map[string]interface{}
		_ = json.Unmarshal(b.data, &env)
		em := env["error"].(map[string]interface{})
		if em["code"] == ErrorCodes.RateLimitMove {
			errCount++
		}
	}
	if errCount < 1 {
		t.Fatalf("expected rate limit error broadcast, got %+v", d.broadcasts)
	}
}

func TestMatchLoopInvalidPayload(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := activeTwoPlayerState("a", "b")
	ms.TurnUserID = "a"
	ms.PlayerSymbols["a"] = "X"
	ms.PlayerSymbols["b"] = "O"
	msg := mockMatchData{mockPresence: mockPresence{uid: "a", user: "A"}, op: int64(OpMove), data: []byte(`{`)}
	_ = m.MatchLoop(ctx, noopLogger{}, nil, nil, d, 1, ms, []runtime.MatchData{msg})
	found := false
	for _, b := range d.broadcasts {
		if b.op != int64(OpError) {
			continue
		}
		var env map[string]interface{}
		_ = json.Unmarshal(b.data, &env)
		if env["error"].(map[string]interface{})["code"] == ErrorCodes.InvalidPayload {
			found = true
		}
	}
	if !found {
		t.Fatal("invalid payload error")
	}
}

func TestMatchLoopIgnoresNonMoveOpcode(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := activeTwoPlayerState("a", "b")
	ms.TurnUserID = "a"
	ms.PlayerSymbols["a"] = "X"
	ms.PlayerSymbols["b"] = "O"
	msg := mockMatchData{mockPresence: mockPresence{uid: "a", user: "A"}, op: 99, data: []byte(`{"position":0}`)}
	out := m.MatchLoop(ctx, noopLogger{}, nil, nil, d, 1, ms, []runtime.MatchData{msg})
	oms := out.(*MatchState)
	if oms.Board[0] != "" {
		t.Fatal("board unchanged")
	}
}

func TestMatchTerminateSetsFinished(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := &MatchState{Status: StatusActive, Board: emptyBoard()}
	_ = m.MatchTerminate(ctx, noopLogger{}, nil, nil, d, 9, ms, 0)
	if ms.Status != StatusFinished || ms.EndedAtTick != 9 {
		t.Fatal(ms)
	}
}

func TestMatchSignal(t *testing.T) {
	t.Parallel()
	var m TicTacToeMatch
	ctx := context.Background()
	d := &recordDispatcher{}
	ms := &MatchState{}
	out, s := m.MatchSignal(ctx, noopLogger{}, nil, nil, d, 0, ms, "ping")
	if s != "ok" || out != ms {
		t.Fatal(s)
	}
}

func TestPersistMatchResultSkipsLessThanTwoPlayers(t *testing.T) {
	t.Parallel()
	nk := &recordNK{}
	ms := &MatchState{Players: []Player{{UserID: "only"}}, Winner: "only", Status: StatusFinished}
	persistMatchResult(context.Background(), nk, ms)
	if nk.storageWrites != 0 {
		t.Fatal("should not persist with <2 players")
	}
}

func emptyBoard() []string {
	return []string{"", "", "", "", "", "", "", "", ""}
}
