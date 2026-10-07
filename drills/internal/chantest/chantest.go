// Package chantest holds small helpers shared by the drill tests. They are
// meant to run inside a testing/synctest bubble, where time is fake.
package chantest

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"
)

// Recv receives one value from ch, giving up after d.
func Recv[T any](ch <-chan T, d time.Duration) (v T, ok, timedOut bool) {
	select {
	case v, ok = <-ch:
		return v, ok, false
	case <-time.After(d):
		return v, false, true
	}
}

// Drain reads ch until it is closed, failing if that takes longer than d.
func Drain[T any](t *testing.T, ch <-chan T, d time.Duration) []T {
	t.Helper()
	if ch == nil {
		t.Fatal("got a nil channel")
	}
	var got []T
	for {
		v, ok, timedOut := Recv(ch, d)
		switch {
		case timedOut:
			t.Fatalf("channel not closed after %v (got %v so far)", d, got)
		case !ok:
			return got
		}
		got = append(got, v)
	}
}

// ExpectClosedNow fails if ch is not closed right now. Call it after
// synctest.Wait. A value waiting on ch means a sender is still parked on a
// send with no escape hatch.
func ExpectClosedNow[T any](t *testing.T, ch <-chan T, who string) {
	t.Helper()
	select {
	case v, ok := <-ch:
		if ok {
			t.Errorf("%s was still blocked sending %v after cancel with no reader: "+
				"that send needs an escape hatch", who, v)
			Drain(t, ch, time.Second)
		}
	default:
		t.Errorf("%s neither exited nor offered a value after cancel; is it closing its channel?", who)
	}
}

// Feed sends vs on a new channel, one every gap, then closes it. It gives up
// when ctx is done, so the code under test can't make it leak.
func Feed[T any](ctx context.Context, gap time.Duration, vs ...T) <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for _, v := range vs {
			select {
			case <-time.After(gap):
			case <-ctx.Done():
				return
			}
			select {
			case ch <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// CountSelects parses file and returns how many select statements function
// fn contains, including ones inside closures.
func CountSelects(t *testing.T, file, fn string) int {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	n, found := 0, false
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == fn && fd.Body != nil {
			found = true
			ast.Inspect(fd.Body, func(node ast.Node) bool {
				if _, ok := node.(*ast.SelectStmt); ok {
					n++
				}
				return true
			})
		}
	}
	if !found {
		t.Fatalf("func %s not found in %s", fn, file)
	}
	return n
}
