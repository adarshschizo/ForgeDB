package record

import (
	"math"
	"testing"
)

func TestBinaryRowRoundTrip(t *testing.T) {
	row := Row{Values: []Value{
		{Kind: Integer, I: -42},
		{Kind: Text, S: "Adarsh"},
		{Kind: Boolean, B: true},
		{Kind: Float, F: math.Pi},
		{Kind: Null},
	}}
	b, err := EncodeRow(row)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRow(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Values) != 5 || got.Values[0].I != -42 || got.Values[1].S != "Adarsh" ||
		!got.Values[2].B || got.Values[3].F != math.Pi || got.Values[4].Kind != Null {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}
