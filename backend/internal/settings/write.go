package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry describes one key for the admin endpoint: what it is, what it is set
// to, and what it would be if nothing overrode it.
type Entry struct {
	Key         Key    `json:"key"`
	Value       any    `json:"value"`
	Default     any    `json:"default"`
	Kind        Kind   `json:"type"`
	Min         *int   `json:"min,omitempty"`
	Max         *int   `json:"max,omitempty"`
	Overridden  bool   `json:"overridden"`
	Description string `json:"description"`
}

// Describe lists every registered key against the active snapshot.
func Describe(s *Snapshot) []Entry {
	out := make([]Entry, 0, len(Registry))
	for k, spec := range Registry {
		e := Entry{
			Key:         k,
			Default:     spec.Default,
			Kind:        spec.Kind,
			Description: spec.Description,
		}
		if spec.Bounded {
			lo, hi := int(spec.Min), int(spec.Max)
			e.Min, e.Max = &lo, &hi
		}
		switch spec.Kind {
		case KindInt:
			e.Value = s.Int(k)
		case KindBool:
			e.Value = s.Bool(k)
		case KindString:
			e.Value = s.String(k)
		}
		e.Overridden = e.Value != spec.Default
		out = append(out, e)
	}
	return out
}

// ErrUnknownKey is returned for a key that is not in the Registry.
var ErrUnknownKey = fmt.Errorf("unknown setting")

// Validate checks a proposed value against the registry. The database trigger
// is the authoritative boundary — this exists so the API can reject with a
// useful message instead of surfacing a raw SQL error.
func Validate(k Key, value any) error {
	spec, ok := Registry[k]
	if !ok {
		return ErrUnknownKey
	}

	switch spec.Kind {
	case KindInt:
		f, ok := toFloat(value)
		if !ok {
			return fmt.Errorf("%s expects a number", k)
		}
		if f != math.Trunc(f) {
			return fmt.Errorf("%s expects a whole number, got %v", k, f)
		}
		if spec.Bounded && (f < spec.Min || f > spec.Max) {
			return fmt.Errorf("%s must be between %d and %d, got %v",
				k, int(spec.Min), int(spec.Max), f)
		}
	case KindBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s expects a boolean", k)
		}
	case KindString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s expects a string", k)
		}
	}
	return nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// Set writes a value. Only `value` and `updated_by` are updated; the trigger
// rejects any attempt to alter a key's type or bounds, and fires the NOTIFY
// that makes the change live.
func Set(ctx context.Context, pool *pgxpool.Pool, k Key, value any, updatedBy string) error {
	if err := Validate(k, value); err != nil {
		return err
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode value: %w", err)
	}

	tag, err := pool.Exec(ctx,
		`UPDATE settings SET value = $2, updated_by = $3 WHERE key = $1`,
		string(k), encoded, updatedBy)
	if err != nil {
		return fmt.Errorf("write setting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s is registered in code but missing from the database", ErrUnknownKey, k)
	}
	return nil
}
