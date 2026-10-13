package settings

import "testing"

func TestSnapshotFallsBackToDefault(t *testing.T) {
	// A key absent from the snapshot must still return the registry default,
	// otherwise a partially-populated settings table would hand callers a zero.
	s := newSnapshot(map[Key]any{})
	if got, want := s.Int(IngestMaxAttempts), Registry[IngestMaxAttempts].Default; got != want {
		t.Errorf("missing key: got %d, want %v", got, want)
	}

	// A value of the wrong type is as unusable as a missing one.
	s = newSnapshot(map[Key]any{IngestMaxAttempts: "three"})
	if got, want := s.Int(IngestMaxAttempts), Registry[IngestMaxAttempts].Default; got != want {
		t.Errorf("wrong-typed key: got %d, want %v", got, want)
	}
}

func TestSnapshotReturnsStoredValue(t *testing.T) {
	s := newSnapshot(map[Key]any{IngestMaxAttempts: 7})
	if got := s.Int(IngestMaxAttempts); got != 7 {
		t.Errorf("got %d, want 7", got)
	}
}

func TestDefaultSnapshotCoversRegistry(t *testing.T) {
	s := defaultSnapshot()
	for k := range Registry {
		if _, ok := s.values[k]; !ok {
			t.Errorf("default snapshot missing %s", k)
		}
	}
}

func TestSecondsAndHours(t *testing.T) {
	s := newSnapshot(map[Key]any{
		IngestSyncIntervalSeconds: 90,
		SessionTTLHours:           2,
	})
	if got := s.Seconds(IngestSyncIntervalSeconds).String(); got != "1m30s" {
		t.Errorf("Seconds: got %s, want 1m30s", got)
	}
	if got := s.Hours(SessionTTLHours).String(); got != "2h0m0s" {
		t.Errorf("Hours: got %s, want 2h0m0s", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		key     Key
		value   any
		wantErr bool
	}{
		{"in range", IngestMaxAttempts, 5, false},
		{"at minimum", IngestMaxAttempts, 1, false},
		{"at maximum", IngestMaxAttempts, 10, false},
		{"below minimum", IngestMaxAttempts, 0, true},
		{"above maximum", IngestMaxAttempts, 11, true},
		{"not a whole number", IngestMaxAttempts, 2.5, true},
		{"wrong type", IngestMaxAttempts, "many", true},
		{"unknown key", Key("api.not.a.real.key"), 1, true},
		{"json float is accepted for int", IngestMaxAttempts, float64(4), false},
		// 0 is meaningful for cache TTLs: it disables caching.
		{"zero allowed where min is zero", CacheSearchTTLSeconds, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.key, tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("expected an error for %v", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		kind    Kind
		want    any
		wantErr bool
	}{
		{"int", "42", KindInt, 42, false},
		{"int rejects fraction", "4.5", KindInt, nil, true},
		{"int rejects string", `"42"`, KindInt, nil, true},
		{"bool", "true", KindBool, true, false},
		{"bool rejects number", "1", KindBool, nil, true},
		{"string", `"hello"`, KindString, "hello", false},
		{"string rejects number", "5", KindString, nil, true},
		{"unknown kind", "1", Kind("weird"), nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decode([]byte(tc.raw), tc.kind)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

func TestDescribeMarksOverrides(t *testing.T) {
	def := Registry[IngestMaxAttempts].Default.(int)
	s := newSnapshot(map[Key]any{IngestMaxAttempts: def + 1})

	var found bool
	for _, e := range Describe(s) {
		if e.Key != IngestMaxAttempts {
			continue
		}
		found = true
		if !e.Overridden {
			t.Error("changed value should be reported as overridden")
		}
		if e.Default != def {
			t.Errorf("default: got %v, want %d", e.Default, def)
		}
		if e.Min == nil || e.Max == nil {
			t.Error("bounded key should report min and max")
		}
	}
	if !found {
		t.Fatal("Describe omitted the key")
	}
}

// Every registry entry must be usable: a default that violates its own bounds
// would mean the process boots on a value the database would reject.
func TestRegistryDefaultsAreValid(t *testing.T) {
	for k, spec := range Registry {
		if err := Validate(k, spec.Default); err != nil {
			t.Errorf("%s has an invalid default: %v", k, err)
		}
		if spec.Bounded && spec.Min > spec.Max {
			t.Errorf("%s has min %v above max %v", k, spec.Min, spec.Max)
		}
	}
}
