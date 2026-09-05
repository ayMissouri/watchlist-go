package handlers

import (
	"encoding/json"
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestUpdateUserRequestPartialFields(t *testing.T) {
	var settingsOnly models.UpdateUserRequest
	if err := json.Unmarshal([]byte(`{"settings":{"subtitle":{"size":18}}}`), &settingsOnly); err != nil {
		t.Fatal(err)
	}
	if settingsOnly.DisplayName != nil {
		t.Errorf("display_name should be untouched, got %q", *settingsOnly.DisplayName)
	}
	if string(settingsOnly.Settings) == "" {
		t.Error("settings not decoded")
	}

	var clearName models.UpdateUserRequest
	if err := json.Unmarshal([]byte(`{"display_name":""}`), &clearName); err != nil {
		t.Fatal(err)
	}
	if clearName.DisplayName == nil || *clearName.DisplayName != "" {
		t.Error("empty display_name should be present and empty")
	}
	if clearName.Settings != nil {
		t.Error("settings should be untouched")
	}
}
