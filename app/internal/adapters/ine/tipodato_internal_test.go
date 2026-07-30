package ine

// Task 2a.1 (RED): wireObservation must decode INE's real tip=A
// T3_TipoDato field and must not declare Secreto, which does not exist
// in the tip=A response the adapter actually requests (spec
// source-ingestion-ine, "No phantom field survives"). White-box (package
// ine, not ine_test) because it inspects the unexported wireObservation
// type directly, mirroring client_internal_test.go's own pattern.

import (
	"reflect"
	"testing"
)

func TestWireObservation_NoSecretoFieldDeclaredAndTipoDatoTagged(t *testing.T) {
	typ := reflect.TypeOf(wireObservation{})

	if _, ok := typ.FieldByName("Secreto"); ok {
		t.Error("wireObservation must not declare a Secreto field: it does not exist in the tip=A response INE actually requests")
	}

	field, ok := typ.FieldByName("TipoDato")
	if !ok {
		t.Fatal("wireObservation must declare a TipoDato field decoding T3_TipoDato")
	}
	if got := field.Tag.Get("json"); got != "T3_TipoDato" {
		t.Errorf("expected TipoDato's json tag to be %q, got %q", "T3_TipoDato", got)
	}
}
