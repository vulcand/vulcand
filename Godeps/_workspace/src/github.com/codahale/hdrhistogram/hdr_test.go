package hdrhistogram_test

import (
	"reflect"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codahale/hdrhistogram"
)

func TestValueAtQuantile(t *testing.T) {
	h := hdrhistogram.New(1, 10000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	data := []struct {
		q float64
		v int64
	}{
		{q: 50, v: 500223},
		{q: 75, v: 750079},
		{q: 90, v: 900095},
		{q: 95, v: 950271},
		{q: 99, v: 990207},
		{q: 99.9, v: 999423},
		{q: 99.99, v: 999935},
	}

	for _, d := range data {
		if v := h.ValueAtQuantile(d.q); v != d.v {
			t.Errorf("P%v was %v, but expected %v", d.q, v, d.v)
		}
	}
}

func TestMean(t *testing.T) {
	h := hdrhistogram.New(1, 10000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.Mean(), 500000.013312; v != want {
		t.Errorf("Mean was %v, but expected %v", v, want)
	}
}

func TestStdDev(t *testing.T) {
	h := hdrhistogram.New(1, 10000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.StdDev(), 288675.1403682715; v != want {
		t.Errorf("StdDev was %v, but expected %v", v, want)
	}
}

func TestMax(t *testing.T) {
	h := hdrhistogram.New(1, 10000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.Max(), int64(999936); v != want {
		t.Errorf("Max was %v, but expected %v", v, want)
	}
}

func TestReset(t *testing.T) {
	h := hdrhistogram.New(1, 10000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	h.Reset()

	if v, want := h.Max(), int64(0); v != want {
		t.Errorf("Max was %v, but expected %v", v, want)
	}
}

func TestMerge(t *testing.T) {
	h1 := hdrhistogram.New(1, 1000, 3)
	h2 := hdrhistogram.New(1, 1000, 3)

	for i := 0; i < 100; i++ {
		if err := h1.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	for i := 100; i < 200; i++ {
		if err := h2.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	h1.Merge(h2)

	if v, want := h1.ValueAtQuantile(50), int64(99); v != want {
		t.Errorf("Median was %v, but expected %v", v, want)
	}
}

func TestMin(t *testing.T) {
	h := hdrhistogram.New(1, 10000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if v, want := h.Min(), int64(0); v != want {
		t.Errorf("Min was %v, but expected %v", v, want)
	}
}

func TestByteSize(t *testing.T) {
	h := hdrhistogram.New(1, 100000, 3)

	if v, want := h.ByteSize(), 65604; v != want {
		t.Errorf("ByteSize was %v, but expected %d", v, want)
	}
}

func TestRecordCorrectedValue(t *testing.T) {
	h := hdrhistogram.New(1, 100000, 3)

	if err := h.RecordCorrectedValue(10, 100); err != nil {
		t.Fatal(err)
	}

	if v, want := h.ValueAtQuantile(75), int64(10); v != want {
		t.Errorf("Corrected value was %v, but expected %v", v, want)
	}
}

func TestRecordCorrectedValueStall(t *testing.T) {
	h := hdrhistogram.New(1, 100000, 3)

	if err := h.RecordCorrectedValue(1000, 100); err != nil {
		t.Fatal(err)
	}

	if v, want := h.ValueAtQuantile(75), int64(800); v != want {
		t.Errorf("Corrected value was %v, but expected %v", v, want)
	}
}

func TestCumulativeDistribution(t *testing.T) {
	h := hdrhistogram.New(1, 100000000, 3)

	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	actual := h.CumulativeDistribution()
	expected := []hdrhistogram.Bracket{
		hdrhistogram.Bracket{Quantile: 0, Count: 1},
		hdrhistogram.Bracket{Quantile: 50, Count: 500224},
		hdrhistogram.Bracket{Quantile: 75, Count: 750080},
		hdrhistogram.Bracket{Quantile: 87.5, Count: 875008},
		hdrhistogram.Bracket{Quantile: 93.75, Count: 937984},
		hdrhistogram.Bracket{Quantile: 96.875, Count: 969216},
		hdrhistogram.Bracket{Quantile: 98.4375, Count: 984576},
		hdrhistogram.Bracket{Quantile: 99.21875, Count: 992256},
		hdrhistogram.Bracket{Quantile: 99.609375, Count: 996352},
		hdrhistogram.Bracket{Quantile: 99.8046875, Count: 998400},
		hdrhistogram.Bracket{Quantile: 99.90234375, Count: 999424},
		hdrhistogram.Bracket{Quantile: 99.951171875, Count: 999936},
		hdrhistogram.Bracket{Quantile: 99.9755859375, Count: 999936},
		hdrhistogram.Bracket{Quantile: 99.98779296875, Count: 999936},
		hdrhistogram.Bracket{Quantile: 99.993896484375, Count: 1000000},
		hdrhistogram.Bracket{Quantile: 100, Count: 1000000},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("CF was %#v, but expected %#v", actual, expected)
	}
}

func BenchmarkHistogramRecordValue(b *testing.B) {
	h := hdrhistogram.New(1, 10000000, 3)
	for i := 0; i < 1000000; i++ {
		if err := h.RecordValue(int64(i)); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		h.RecordValue(100)
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		hdrhistogram.New(1, 120000, 3) // this could track 1ms-2min
	}
}
