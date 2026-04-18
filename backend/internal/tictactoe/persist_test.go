package tictactoe

import (
	"context"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func TestPersistMatchResultWin(t *testing.T) {
	t.Parallel()
	nk := &recordNK{}
	ms := &MatchState{
		Status: StatusFinished,
		Players: []Player{
			{UserID: "a", Username: "A"},
			{UserID: "b", Username: "B"},
		},
		Winner: "a",
	}
	persistMatchResult(context.Background(), nk, ms)
	if nk.storageWrites != 2 || nk.lbWrites != 2 {
		t.Fatalf("expected two storage writes and two LB writes, got storage=%d lb=%d", nk.storageWrites, nk.lbWrites)
	}
}

func TestPersistMatchResultDraw(t *testing.T) {
	t.Parallel()
	nk := &recordNK{}
	ms := &MatchState{
		Status: StatusFinished,
		Players: []Player{
			{UserID: "a", Username: "A"},
			{UserID: "b", Username: "B"},
		},
		Winner: "",
	}
	persistMatchResult(context.Background(), nk, ms)
	if nk.storageWrites != 2 || nk.lbWrites != 2 {
		t.Fatalf("draw persistence: storage=%d lb=%d", nk.storageWrites, nk.lbWrites)
	}
}

func TestReadStatsMalformedJSON(t *testing.T) {
	t.Parallel()
	nk := &malformedStorageNK{}
	if s := readStats(context.Background(), nk, "u"); s.Wins != 0 || s.Losses != 0 {
		t.Fatalf("expected empty stats, got %+v", s)
	}
}

type malformedStorageNK struct{}

func (m *malformedStorageNK) StorageRead(ctx context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return []*api.StorageObject{{
		Value: `{not-json`,
	}}, nil
}

func (m *malformedStorageNK) StorageWrite(ctx context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	return nil, nil
}

func (m *malformedStorageNK) LeaderboardRecordWrite(ctx context.Context, id, ownerID, username string, score, subscore int64, metadata map[string]interface{}, overrideOperator *int) (*api.LeaderboardRecord, error) {
	return &api.LeaderboardRecord{}, nil
}

func TestNoopPersistenceWrites(t *testing.T) {
	t.Parallel()
	var nk NoopPersistence
	persistMatchResult(context.Background(), nk, &MatchState{
		Status: StatusFinished,
		Players: []Player{
			{UserID: "a", Username: "A"},
			{UserID: "b", Username: "B"},
		},
		Winner: "a",
	})
	// no panic; no observable side channel
}
