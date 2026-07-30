package redis

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestParseRedisFloat(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    float64
		wantErr bool
	}{
		{name: "string", value: "1.25", want: 1.25},
		{name: "float64", value: 2.5, want: 2.5},
		{name: "int64", value: int64(3), want: 3},
		{name: "invalid string", value: "invalid", wantErr: true},
		{name: "unexpected type", value: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRedisFloat(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("expected %v, got %v, %v", tt.want, got, err)
			}
		})
	}
}

func TestParseRedisInt64(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    int64
		wantErr bool
	}{
		{name: "int64", value: int64(3), want: 3},
		{name: "string", value: "4", want: 4},
		{name: "invalid string", value: "invalid", wantErr: true},
		{name: "unexpected type", value: 5.0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRedisInt64(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("expected %d, got %d, %v", tt.want, got, err)
			}
		})
	}
}

func TestValidateTieBreakMembers(t *testing.T) {
	tests := []struct {
		name    string
		order   string
		members []*Member
		wantErr bool
	}{
		{name: "ascending", order: "asc", members: []*Member{{Member: "alice", Score: 1}}},
		{name: "descending", order: "desc", members: []*Member{{Member: "alice", Score: 1}}},
		{name: "invalid order", order: "invalid", wantErr: true},
		{name: "nil member", order: "desc", members: []*Member{nil}, wantErr: true},
		{name: "NaN score", order: "desc", members: []*Member{{Member: "alice", Score: math.NaN()}}, wantErr: true},
		{name: "infinite score", order: "desc", members: []*Member{{Member: "alice", Score: math.Inf(1)}}, wantErr: true},
		{
			name:    "duplicate member",
			order:   "desc",
			members: []*Member{{Member: "alice", Score: 1}, {Member: "alice", Score: 2}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTieBreakMembers(tt.order, tt.members)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTieBreakScriptErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := goredis.NewClient(&goredis.Options{Addr: "localhost:1"})
	t.Cleanup(func() {
		_ = client.Close()
	})
	keys := TieBreakKeys{
		Scores:    "{script-errors}:scores",
		ScoresAsc: "{script-errors}:scores-asc",
		Members:   "{script-errors}:members",
		Sequence:  "{script-errors}:sequence",
		TTL:       "{script-errors}:ttl",
	}
	member := &Member{Member: "alice", Score: 1, TTL: time.Now().Add(time.Minute)}

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "upsert",
			call: func() error {
				_, err := upsertMembersWithTieBreak(ctx, client, keys, "desc", member)
				return err
			},
		},
		{
			name: "increment",
			call: func() error {
				_, err := incrementWithTieBreak(ctx, client, keys, member.Member, "desc", 1)
				return err
			},
		},
		{
			name: "get members",
			call: func() error {
				_, err := getMembersWithTieBreak(ctx, client, keys, "desc", false, member.Member)
				return err
			},
		},
		{
			name: "get rank",
			call: func() error {
				_, err := getRankWithTieBreak(ctx, client, keys, member.Member, "desc")
				return err
			},
		},
		{
			name: "remove members",
			call: func() error {
				return removeMembersWithTieBreak(ctx, client, keys, member.Member)
			},
		},
		{
			name: "expire keys",
			call: func() error {
				return expireTieBreakKeysAt(ctx, client, keys, time.Now().Add(time.Minute))
			},
		},
		{
			name: "set member TTL",
			call: func() error {
				return setMembersTTLWithTieBreak(ctx, client, keys, member)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var generalError *GeneralError
			if err := tt.call(); !errors.As(err, &generalError) {
				t.Fatalf("expected a general Redis error, got %v", err)
			}
		})
	}
}

func TestIncrementWithTieBreakValidation(t *testing.T) {
	keys := TieBreakKeys{}
	tests := []struct {
		name      string
		order     string
		increment float64
	}{
		{name: "invalid order", order: "invalid", increment: 1},
		{name: "NaN increment", order: "desc", increment: math.NaN()},
		{name: "infinite increment", order: "desc", increment: math.Inf(1)},
		{name: "out-of-range increment", order: "desc", increment: maxExactRedisScore + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := incrementWithTieBreak(
				context.Background(),
				nil,
				keys,
				"alice",
				tt.order,
				tt.increment,
			); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
