package tictactoe

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakeMatchRPC struct {
	listOut   []*api.Match
	listErr   error
	createOut string
	createErr error
	listCalls int
	createCalls int
}

func (f *fakeMatchRPC) MatchCreate(ctx context.Context, module string, params map[string]interface{}) (string, error) {
	f.createCalls++
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createOut, nil
}

func (f *fakeMatchRPC) MatchList(ctx context.Context, limit int, authoritative bool, label string, minSize, maxSize *int, query string) ([]*api.Match, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listOut, nil
}

type fakeLeaderboardRPC struct {
	records     []*api.LeaderboardRecord
	nextCursor  string
	err         error
	listCalls int
}

func (f *fakeLeaderboardRPC) LeaderboardRecordsList(ctx context.Context, id string, ownerIDs []string, limit int, cursor string, expiry int64) ([]*api.LeaderboardRecord, []*api.LeaderboardRecord, string, string, error) {
	f.listCalls++
	if f.err != nil {
		return nil, nil, "", "", f.err
	}
	return f.records, nil, f.nextCursor, "", nil
}

func envelopeOK(t *testing.T, raw string) bool {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m["ok"] == true
}

func envelopeErrCode(t *testing.T, raw string) string {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if m["ok"] != false {
		t.Fatalf("expected ok false: %v", m)
	}
	return m["error"].(map[string]interface{})["code"].(string)
}

func TestRPCCreateMatch(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-create-1")
		f := &fakeMatchRPC{createOut: "mid123"}
		out, err := RPCCreateMatch(ctxU, f, `{"mode":"timed"}`)
		if err != nil {
			t.Fatal(err)
		}
		if !envelopeOK(t, out) || f.createCalls != 1 {
			t.Fatalf("out=%q calls=%d", out, f.createCalls)
		}
	})
	t.Run("invalid mode", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-create-2")
		f := &fakeMatchRPC{}
		out, err := RPCCreateMatch(ctxU, f, `{"mode":"blitz"}`)
		if err != nil {
			t.Fatal(err)
		}
		if envelopeErrCode(t, out) != ErrorCodes.InvalidMode || f.createCalls != 0 {
			t.Fatal(out, f.createCalls)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-create-rl")
		f := &fakeMatchRPC{createOut: "x"}
		for i := 0; i < RateLimitCreateMatch; i++ {
			_, _ = RPCCreateMatch(ctxU, f, `{}`)
		}
		out, err := RPCCreateMatch(ctxU, f, `{}`)
		if err != nil {
			t.Fatal(err)
		}
		if envelopeErrCode(t, out) != ErrorCodes.RateLimitRPC {
			t.Fatal(out)
		}
	})
	t.Run("nakama error surfaces", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-create-err")
		f := &fakeMatchRPC{createErr: errors.New("boom")}
		_, err := RPCCreateMatch(ctxU, f, `{}`)
		if err == nil || err.Error() != "boom" {
			t.Fatal(err)
		}
	})
}

func TestRPCListMatches(t *testing.T) {
	ctx := context.Background()
	ResetRPCRateLimiterForTest()
	ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-list-1")
	f := &fakeMatchRPC{listOut: []*api.Match{{MatchId: "m1"}}}
	out, err := RPCListMatches(ctxU, f, `{"mode":"classic"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !envelopeOK(t, out) || f.listCalls != 1 {
		t.Fatalf("calls=%d out=%s", f.listCalls, out)
	}
	var env map[string]interface{}
	_ = json.Unmarshal([]byte(out), &env)
	data := env["data"].(map[string]interface{})
	if data["mode"] != "classic" {
		t.Fatal(data)
	}
}

func TestRPCFindMatchJoinsExisting(t *testing.T) {
	ctx := context.Background()
	ResetRPCRateLimiterForTest()
	ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-find-1")
	f := &fakeMatchRPC{listOut: []*api.Match{{MatchId: "existing"}}}
	out, err := RPCFindMatch(ctxU, f, `{"mode":"classic"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !envelopeOK(t, out) || f.listCalls != 1 || f.createCalls != 0 {
		t.Fatalf("list=%d create=%d", f.listCalls, f.createCalls)
	}
	var env map[string]interface{}
	_ = json.Unmarshal([]byte(out), &env)
	data := env["data"].(map[string]interface{})
	if data["matchId"] != "existing" || data["created"] != false {
		t.Fatal(data)
	}
}

func TestRPCFindMatchCreatesWhenEmpty(t *testing.T) {
	ctx := context.Background()
	ResetRPCRateLimiterForTest()
	ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-find-2")
	f := &fakeMatchRPC{listOut: nil, createOut: "newid"}
	out, err := RPCFindMatch(ctxU, f, `{"mode":"timed"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !envelopeOK(t, out) || f.listCalls != 1 || f.createCalls != 1 {
		t.Fatalf("list=%d create=%d", f.listCalls, f.createCalls)
	}
	var env map[string]interface{}
	_ = json.Unmarshal([]byte(out), &env)
	data := env["data"].(map[string]interface{})
	if data["matchId"] != "newid" || data["created"] != true || data["mode"] != "timed" {
		t.Fatal(data)
	}
}

func TestRPCFindMatchListError(t *testing.T) {
	ctx := context.Background()
	ResetRPCRateLimiterForTest()
	ctxU := context.WithValue(ctx, runtime.RUNTIME_CTX_USER_ID, "rpc-find-err")
	f := &fakeMatchRPC{listErr: errors.New("list down")}
	_, err := RPCFindMatch(ctxU, f, `{}`)
	if err == nil || err.Error() != "list down" {
		t.Fatal(err)
	}
}

func TestRPCListLeaderboard(t *testing.T) {
	ctx := context.Background()
	t.Run("maps metadata", func(t *testing.T) {
		md, _ := json.Marshal(map[string]interface{}{"wins": 3, "losses": 1, "draws": 2, "streak": 2})
		f := &fakeLeaderboardRPC{records: []*api.LeaderboardRecord{{
			Rank:     1,
			Username: wrapperspb.String("alice"),
			Score:    99,
			Metadata: string(md),
		}}, nextCursor: "c1"}
		out, err := RPCListLeaderboard(ctx, f, `{}`)
		if err != nil || f.listCalls != 1 {
			t.Fatal(err, f.listCalls)
		}
		if !envelopeOK(t, out) {
			t.Fatal(out)
		}
		var env map[string]interface{}
		_ = json.Unmarshal([]byte(out), &env)
		data := env["data"].(map[string]interface{})
		lb := data["leaderboard"].([]interface{})
		if len(lb) != 1 {
			t.Fatal(lb)
		}
		row := lb[0].(map[string]interface{})
		if int(row["wins"].(float64)) != 3 || int(row["streak"].(float64)) != 2 {
			t.Fatal(row)
		}
		if data["cursor"] != "c1" {
			t.Fatal("cursor", data["cursor"])
		}
	})
	t.Run("nil username becomes empty string", func(t *testing.T) {
		f := &fakeLeaderboardRPC{records: []*api.LeaderboardRecord{{Rank: 2, Score: 1, Metadata: "{}"}}}
		out, err := RPCListLeaderboard(ctx, f, `{}`)
		if err != nil {
			t.Fatal(err)
		}
		var env map[string]interface{}
		_ = json.Unmarshal([]byte(out), &env)
		row := env["data"].(map[string]interface{})["leaderboard"].([]interface{})[0].(map[string]interface{})
		if row["username"] != "" {
			t.Fatal(row)
		}
	})
	t.Run("empty metadata", func(t *testing.T) {
		f := &fakeLeaderboardRPC{records: []*api.LeaderboardRecord{{
			Rank: 5, Username: wrapperspb.String("bob"), Score: 1, Metadata: "",
		}}}
		out, err := RPCListLeaderboard(ctx, f, `{}`)
		if err != nil {
			t.Fatal(err)
		}
		var env map[string]interface{}
		_ = json.Unmarshal([]byte(out), &env)
		lb := env["data"].(map[string]interface{})["leaderboard"].([]interface{})
		row := lb[0].(map[string]interface{})
		if int(row["wins"].(float64)) != 0 {
			t.Fatal(row)
		}
	})
	t.Run("error propagates", func(t *testing.T) {
		f := &fakeLeaderboardRPC{err: errors.New("lb err")}
		_, err := RPCListLeaderboard(ctx, f, `{}`)
		if err == nil || err.Error() != "lb err" {
			t.Fatal(err)
		}
	})
}
