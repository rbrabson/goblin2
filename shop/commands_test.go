package shop

import (
	"encoding/json"
	"testing"

	"github.com/disgoorg/disgo/discord"
)

func TestShopOptionValues(t *testing.T) {
	var options []discord.SlashCommandOption
	if err := json.Unmarshal([]byte(`[
  {"name":"name","type":3,"value":"VIP"},
  {"name":"price","type":4,"value":1250},
  {"name":"auto-renewable","type":5,"value":true}
 ]`), &options); err != nil {
		t.Fatal(err)
	}
	data := discord.SlashCommandInteractionData{Options: make(map[string]discord.SlashCommandOption)}
	for _, option := range options {
		data.Options[option.Name] = option
	}
	if got := stringValue(data, "name"); got != "VIP" {
		t.Errorf("name = %q, want VIP", got)
	}
	if got := intValue(data, "price"); got != 1250 {
		t.Errorf("price = %d, want 1250", got)
	}
	if !boolValue(data, "auto-renewable") {
		t.Error("auto-renewable = false, want true")
	}
	if got := stringValue(data, "duration"); got != "" {
		t.Errorf("missing duration = %q", got)
	}
	if got := intValue(data, "missing"); got != 0 {
		t.Errorf("missing integer = %d", got)
	}
	if boolValue(data, "missing") {
		t.Error("missing boolean = true")
	}
}
