package main

import "testing"

func TestShellJoinQuotesArguments(t *testing.T) {
	got := shellJoin("/path/with a space/psql", []string{"-h", "127.0.0.1", "-p", "5432"})
	want := `'/path/with a space/psql' '-h' '127.0.0.1' '-p' '5432'`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestShellJoinEscapesSingleQuotes(t *testing.T) {
	got := shellJoin("it's/psql", nil)
	want := `'it'\''s/psql'`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
