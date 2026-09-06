package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"ps2go/lib/harness"
)

// encoding/json and reflect on the PS2: the old runtime could marshal but
// not unmarshal (missing reflection); these keep both working.

type Inner struct {
	Name  string  `json:"name"`
	Value float64 `json:"value,omitempty"`
}

type Embedded struct {
	Kind string `json:"kind"`
}

type Thing struct {
	Embedded
	ID      int              `json:"id"`
	Tags    []string         `json:"tags"`
	Inner   Inner            `json:"inner"`
	Ptr     *Inner           `json:"ptr,omitempty"`
	Map     map[string]int   `json:"map"`
	Any     interface{}      `json:"any"`
	Raw     json.RawMessage  `json:"raw"`
	Skip    string           `json:"-"`
	Nested  map[string][]int `json:"nested"`
	When    time.Time        `json:"when"`
	Bytes   []byte           `json:"bytes"`
	U8      uint8            `json:"u8"`
	I64     int64            `json:"i64"`
	F32     float32          `json:"f32"`
	Bool    bool             `json:"bool"`
	Iface   map[string]any   `json:"iface"`
	private int
}

type Custom int

func (c Custom) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"custom-%d\"", int(c))), nil
}
func (c *Custom) UnmarshalJSON(b []byte) error {
	var n int
	_, err := fmt.Sscanf(string(b), "\"custom-%d\"", &n)
	*c = Custom(n)
	return err
}

func sample() Thing {
	return Thing{
		Embedded: Embedded{Kind: "thing"},
		ID:       7, Tags: []string{"a", "b"},
		Inner:  Inner{Name: "in", Value: 1.5},
		Ptr:    &Inner{Name: "ptr"},
		Map:    map[string]int{"x": 1, "y": 2},
		Any:    []interface{}{1.0, "two", true, nil},
		Raw:    json.RawMessage(`{"k":"v"}`),
		Nested: map[string][]int{"n": {1, 2, 3}},
		When:   time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC),
		Bytes:  []byte{1, 2, 3},
		U8:     200, I64: -1 << 40, F32: 0.25, Bool: true,
		Iface:   map[string]any{"f": 2.5, "s": "str"},
		private: 9,
	}
}

func testMarshalStruct() error {
	b, err := json.Marshal(sample())
	if err != nil {
		return err
	}
	harness.Logf("%s", b)
	return nil
}

func testUnmarshalStruct() error {
	b, err := json.Marshal(sample())
	if err != nil {
		return err
	}
	var t Thing
	if err := json.Unmarshal(b, &t); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	want := sample()
	want.private = 0
	got := t
	if !reflect.DeepEqual(got.Tags, want.Tags) || got.ID != want.ID || got.Kind != want.Kind || got.Inner != want.Inner ||
		got.Ptr == nil || *got.Ptr != *want.Ptr || !reflect.DeepEqual(got.Map, want.Map) || !got.When.Equal(want.When) ||
		!reflect.DeepEqual(got.Bytes, want.Bytes) || got.U8 != want.U8 || got.I64 != want.I64 || got.F32 != want.F32 || got.Bool != want.Bool ||
		string(got.Raw) != string(want.Raw) || !reflect.DeepEqual(got.Nested, want.Nested) {
		return fmt.Errorf("round trip differs: %+v", got)
	}
	harness.Logf("any: %#v iface: %#v", got.Any, got.Iface)
	return nil
}

func testUnmarshalInterface() error {
	var v interface{}
	if err := json.Unmarshal([]byte(`{"a":[1,2.5,"x",true,null,{"b":{}}],"n":-3}`), &v); err != nil {
		return err
	}
	harness.Logf("%#v", v)
	m, ok := v.(map[string]interface{})
	if !ok || m["n"] != -3.0 {
		return fmt.Errorf("got %#v", v)
	}
	return nil
}

func testUnmarshalMapSlice() error {
	var m map[string][]Inner
	if err := json.Unmarshal([]byte(`{"k":[{"name":"a","value":1},{"name":"b"}]}`), &m); err != nil {
		return err
	}
	if len(m["k"]) != 2 || m["k"][1].Name != "b" {
		return fmt.Errorf("got %#v", m)
	}
	return nil
}

func testCustomMarshaler() error {
	type W struct {
		C  Custom  `json:"c"`
		PC *Custom `json:"pc"`
	}
	c := Custom(5)
	b, err := json.Marshal(W{C: 3, PC: &c})
	if err != nil {
		return err
	}
	var w W
	if err := json.Unmarshal(b, &w); err != nil {
		return fmt.Errorf("%s: %w", b, err)
	}
	if w.C != 3 || w.PC == nil || *w.PC != 5 {
		return fmt.Errorf("%s -> %+v", b, w)
	}
	return nil
}

func testDecoderStream() error {
	dec := json.NewDecoder(strings.NewReader(`{"id":1} {"id":2} {"id":3}`))
	n := 0
	for dec.More() {
		var t struct {
			ID int `json:"id"`
		}
		if err := dec.Decode(&t); err != nil {
			return err
		}
		n += t.ID
	}
	if n != 6 {
		return fmt.Errorf("sum %d", n)
	}
	return nil
}

func testIndent() error {
	b, err := json.MarshalIndent(map[string]any{"a": 1, "b": []int{1, 2}}, "", "  ")
	if err != nil {
		return err
	}
	harness.Logf("%s", b)
	return nil
}

func testReflectBasics() error {
	t := reflect.TypeOf(sample())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		_ = f.Tag.Get("json")
	}
	v := reflect.ValueOf(&Thing{}).Elem()
	v.FieldByName("ID").SetInt(42)
	v.FieldByName("Tags").Set(reflect.ValueOf([]string{"z"}))
	m := reflect.MakeMap(reflect.TypeOf(map[string]int{}))
	m.SetMapIndex(reflect.ValueOf("k"), reflect.ValueOf(1))
	v.FieldByName("Map").Set(m)
	nv := reflect.New(reflect.TypeOf(Inner{}))
	nv.Elem().Field(0).SetString("new")
	v.FieldByName("Ptr").Set(nv)
	th := v.Interface().(Thing)
	if th.ID != 42 || th.Tags[0] != "z" || th.Map["k"] != 1 || th.Ptr.Name != "new" {
		return fmt.Errorf("%+v", th)
	}
	return nil
}
