package tictactoe

import (
	"encoding/json"
)

func MarshalEnvelope(ok bool, data map[string]interface{}, errObj map[string]interface{}) string {
	out := map[string]interface{}{"ok": ok}
	if ok {
		out["data"] = data
	} else {
		out["error"] = errObj
	}
	encoded, _ := json.Marshal(out)
	return string(encoded)
}

type modeRequest struct {
	Mode string `json:"mode"`
}

func ParseModePayload(payload string) (*modeRequest, map[string]interface{}) {
	if payload == "" {
		return &modeRequest{Mode: "classic"}, nil
	}
	var req modeRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return nil, buildError(ErrorCodes.InvalidPayload, "Invalid payload.", nil)
	}
	if req.Mode == "" {
		req.Mode = "classic"
	}
	if req.Mode != "classic" && req.Mode != "timed" {
		return nil, buildError(ErrorCodes.InvalidMode, "Invalid mode.", map[string]interface{}{"allowed": []string{"classic", "timed"}})
	}
	return &req, nil
}

func buildError(code, message string, details map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"code": code, "message": message}
	if details != nil {
		out["details"] = details
	}
	return out
}
