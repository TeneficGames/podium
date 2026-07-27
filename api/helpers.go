//  podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package api

import (
	"context"
	"fmt"

	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// EasyJSONUnmarshaler describes a struct able to unmarshal json
type EasyJSONUnmarshaler interface {
	UnmarshalEasyJSON(l *jlexer.Lexer)
}

// EasyJSONMarshaler describes a struct able to marshal json
type EasyJSONMarshaler interface {
	MarshalEasyJSON(w *jwriter.Writer)
}

func newFailMsg(msg string) string {
	return fmt.Sprintf(`{"success":false,"reason":"%s"}`, msg)
}

func withSegment(name string, ctx context.Context, f func() error) error {
	_, span := otel.Tracer("github.com/TeneficGames/podium/api").Start(ctx, name)
	defer span.End()

	err := f()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
