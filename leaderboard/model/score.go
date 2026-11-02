package model

import (
	"errors"
	"math"
)

// MaxScore is the inclusive integer bound supported by the score API and Redis.
const MaxScore = 1 << 53

var ErrInvalidScore = errors.New("score must be a finite integer between -9007199254740992 and 9007199254740992")

// ValidateScore checks a protobuf score before conversion to an integer.
func ValidateScore(score float64) error {
	if math.IsNaN(score) || math.IsInf(score, 0) || math.Trunc(score) != score || math.Abs(score) > MaxScore {
		return ErrInvalidScore
	}
	return nil
}
