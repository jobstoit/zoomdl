package main

import "testing"

func TestNewRecording(t *testing.T) {
	rec := NewRecording("/home/user/root/Some title - this specific thing/GMT20220507-121453-speaker_view.mp4")
	eq(t, "Some title - this specific thing", rec.Class)
	eq(t, 7, rec.Date.Day())
	eq(t, "May", rec.Date.Month().String())
	eq(t, 2022, rec.Date.Year())
}

func eq[T int | string | bool](t *testing.T, expect, actual T) {
	if expect != actual {
		t.Errorf("expected: %v, but got: %v", expect, actual)
	}
}
