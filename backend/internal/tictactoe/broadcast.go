package tictactoe

import (
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

func sendError(dispatcher runtime.MatchDispatcher, state *MatchState, userID, code, message string, details map[string]interface{}) error {
	presences := []runtime.Presence{}
	for _, p := range state.Players {
		if p.UserID == userID && p.Presence != nil {
			presences = append(presences, p.Presence)
			break
		}
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"ok":    false,
		"error": buildError(code, message, details),
	})
	return dispatcher.BroadcastMessage(OpError, payload, presences, nil, true)
}

func broadcastSystem(dispatcher runtime.MatchDispatcher, message string) error {
	payload, _ := json.Marshal(map[string]interface{}{"message": message})
	return dispatcher.BroadcastMessage(OpSystem, payload, nil, nil, true)
}

func broadcastState(dispatcher runtime.MatchDispatcher, state *MatchState) error {
	payload, _ := json.Marshal(state)
	return dispatcher.BroadcastMessage(OpState, payload, nil, nil, true)
}
