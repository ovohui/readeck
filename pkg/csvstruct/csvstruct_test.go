// SPDX-FileCopyrightText: Copyright (c) 2020 Artyom Pervukhin
//
// SPDX-License-Identifier: MIT

package csvstruct

import (
	"strconv"
	"strings"
	"testing"
)

var sink testDst

type testDst struct {
	Name   string `csv:"name"`
	Age    uint8  `csv:"age"`
	Ignore bool
}

func TestScanner(t *testing.T) {
	var dst testDst
	scan, err := NewScanner([]string{"name", "age"}, &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst != (testDst{}) {
		t.Fatalf("NewScanner modified destination: %+v", dst)
	}
	if err := scan([]string{"John", "42"}, &dst); err != nil {
		t.Fatal(err)
	}
	want := testDst{Name: "John", Age: 42}
	if dst != want {
		t.Fatalf("got: %+v, want: %+v", dst, want)
	}
}

func TestScannerOutOfBounds(t *testing.T) {
	tests := []struct {
		values   []string
		expected testDst
		error    string
	}{
		{
			[]string{"42"},
			testDst{},
			"index 1 out of bounds for row with only 1 fields",
		},
		{
			[]string{"42", "John"},
			testDst{Name: "John", Age: 42},
			"",
		},
		{
			[]string{"42", "John", "true"},
			testDst{Name: "John", Age: 42},
			"",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			dst := testDst{}
			scan, err := NewScanner([]string{"age", "name"}, &dst)
			if err != nil {
				t.Fatal(err)
			}

			err = scan(test.values, &dst)
			if test.error != "" {
				if err == nil || err.Error() != test.error {
					t.Fatalf("no error or error not matching: %s", test.error)
				}
			}

			if dst != test.expected {
				t.Fatalf("%#v is not equal to %#v", dst, test.expected)
			}
		})
	}
}

func TestScannerDifferentType(t *testing.T) {
	scan, err := NewScanner([]string{"name", "age"}, &testDst{})
	if err != nil {
		t.Fatal(err)
	}
	type testDst2 struct {
		Name   string `csv:"name"`
		Age    uint8  `csv:"age"`
		Ignore bool
		_      byte // <- differs from testDst type
	}
	var dst testDst2
	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("using different types in NewScanner and Scanner should have panicked, but it's not")
		}
		if s, ok := p.(string); !ok || !strings.Contains(s, "different type") {
			t.Fatalf("unexpected panic value: %v", p)
		}
	}()
	t.Fatalf("Scanner returned on different types: %v", scan([]string{"John", "42"}, &dst))
}

func BenchmarkScanner(b *testing.B) {
	scan, err := NewScanner([]string{"name", "age"}, &sink)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := scan([]string{"John", "42"}, &sink); err != nil {
			b.Fatal(err)
		}
	}
}
