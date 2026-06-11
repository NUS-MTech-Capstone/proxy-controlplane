package service

import "testing"

func TestObserverWebsocketURLPreservesExistingQuery(t *testing.T) {
	got := observerWebsocketURL("wss://proxy.example/sessions/sess-1/ws?token=abc")
	want := "wss://proxy.example/sessions/sess-1/ws?mode=observe&token=abc"
	if got != want {
		t.Fatalf("observerWebsocketURL() = %q, want %q", got, want)
	}
}
