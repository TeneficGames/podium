package model

import (
	"errors"
	"math"
	"testing"
)

func TestValidateScore(t *testing.T) {
	for _, score := range []float64{0, -1, 1, MaxScore, -MaxScore} {
		if err := ValidateScore(score); err != nil {
			t.Fatalf("valid score %v: %v", score, err)
		}
	}
	for _, score := range []float64{1.9, -1.9, math.NaN(), math.Inf(1), math.Inf(-1), MaxScore + 2, -MaxScore - 2} {
		if err := ValidateScore(score); !errors.Is(err, ErrInvalidScore) {
			t.Fatalf("invalid score %v: %v", score, err)
		}
	}
}
