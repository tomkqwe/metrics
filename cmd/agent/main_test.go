package main

import "testing"

func TestNewAgent(t *testing.T) {
	app, err := newAgent()
	if err != nil {
		t.Fatalf("newAgent() error = %v", err)
	}
	if app == nil {
		t.Fatal("newAgent() returned nil app")
	}
}
