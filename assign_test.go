package object

import (
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"testing"
	"time"
)

type Basic struct {
	Vstring     string
	Vint        int
	Vint8       int8
	Vint16      int16
	Vint32      int32
	Vint64      int64
	Vuint       uint
	Vbool       bool
	Vfloat      float64
	Vextra      string
	vsilent     bool
	Vdata       any
	VjsonInt    int
	VjsonUint   uint
	VjsonUint64 uint64
	VjsonFloat  float64
	VjsonNumber json.Number
	VRenamed    string `json:"custom_name"`
}

type BasicPointer struct {
	Vstring     *string
	Vint        *int
	Vuint       *uint
	Vbool       *bool
	Vfloat      *float64
	Vextra      *string
	vsilent     *bool
	Vdata       *any
	VjsonInt    *int
	VjsonFloat  *float64
	VjsonNumber *json.Number
	VRenamed    *string `json:"custom_name_pointer"`
}

type Embedded struct {
	Basic
	Vunique string
}

type EmbeddedPointer struct {
	*Basic
	Vunique string
}

type EmbeddedPointerSquash struct {
	*Basic
	Vunique string
}

type BasicMapStructure struct {
	Vunique string     `json:"vunique"`
	Vtime   *time.Time `json:"time"`
}

type NestedPointerWithMapstructure struct {
	Vbar *BasicMapStructure `json:"vbar"`
}

type EmbeddedAndNamed struct {
	Basic
	Named   Basic
	Vunique string
}

type SliceAlias []string

type EmbeddedSlice struct {
	SliceAlias `json:"slice_alias"`
	Vunique    string
}

type ArrayAlias [2]string

type EmbeddedArray struct {
	ArrayAlias `json:"array_alias"`
	Vunique    string
}

type Map struct {
	Vfoo   string
	Vother map[string]string
}

type MapOfStruct struct {
	Value map[string]Basic
}

type Nested struct {
	Vfoo string
	Vbar Basic
}

type NestedPointer struct {
	Vfoo string
	Vbar *Basic
}

type NilInterface struct {
	W io.Writer
}

type NilPointer struct {
	Value *string
}

type Slice struct {
	Vfoo string
	Vbar []string
}

type SliceOfByte struct {
	Vfoo string
	Vbar []byte
}

type SliceOfAlias struct {
	Vfoo string
	Vbar SliceAlias
}

type SliceOfStruct struct {
	Value []Basic
}

type SlicePointer struct {
	Vbar *[]string
}

type Array struct {
	Vfoo string
	Vbar [2]string
}

type ArrayOfStruct struct {
	Value [2]Basic
}

type Func struct {
	Foo func() string
}

type Tagged struct {
	Extra string `json:"bar,what,what"`
	Value string `json:"foo"`
}

type Remainder struct {
	A     string
	Extra map[string]any `json:",remain"`
}

type StructWithOmitEmpty struct {
	VisibleStringField string         `json:"visible-string"`
	OmitStringField    string         `json:"omittable-string,omitempty"`
	VisibleIntField    int            `json:"visible-int"`
	OmitIntField       int            `json:"omittable-int,omitempty"`
	VisibleFloatField  float64        `json:"visible-float"`
	OmitFloatField     float64        `json:"omittable-float,omitempty"`
	VisibleSliceField  []any          `json:"visible-slice"`
	OmitSliceField     []any          `json:"omittable-slice,omitempty"`
	VisibleMapField    map[string]any `json:"visible-map"`
	OmitMapField       map[string]any `json:"omittable-map,omitempty"`
	NestedField        *Nested        `json:"visible-nested"`
	OmitNestedField    *Nested        `json:"omittable-nested,omitempty"`
}

type TypeConversionResult struct {
	IntToFloat         float32
	IntToUint          uint
	IntToBool          bool
	IntToString        string
	UintToInt          int
	UintToFloat        float32
	UintToBool         bool
	UintToString       string
	BoolToInt          int
	BoolToUint         uint
	BoolToFloat        float32
	BoolToString       string
	FloatToInt         int
	FloatToUint        uint
	FloatToBool        bool
	FloatToString      string
	SliceUint8ToString string
	StringToSliceUint8 []byte
	ArrayUint8ToString string
	StringToInt        int
	StringToUint       uint
	StringToBool       bool
	StringToFloat      float32
	StringToStrSlice   []string
	StringToIntSlice   []int
	StringToStrArray   [1]string
	StringToIntArray   [1]int
	SliceToMap         map[string]any
	MapToSlice         []any
	ArrayToMap         map[string]any
	MapToArray         [1]any
}

func TestBasicTypes(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring":     "foo",
		"vint":        42,
		"vint8":       42,
		"vint16":      42,
		"vint32":      42,
		"vint64":      42,
		"vuint":       42,
		"vbool":       true,
		"vfloat":      42.42,
		"vsilent":     true,
		"vdata":       42,
		"vjsonInt":    json.Number("1234"),
		"vjsonUint":   json.Number("1234"),
		"vjsonUint64": json.Number("9223372036854775809"), // 2^63 + 1
		"vjsonFloat":  json.Number("1234.5"),
		"vjsonNumber": json.Number("1234.5"),
	}

	var result Basic
	err := Assign(&result, input)
	if err != nil {
		t.Errorf("got an err: %s", err.Error())
		t.FailNow()
	}

	if result.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vstring)
	}

	if result.Vint != 42 {
		t.Errorf("vint value should be 42: %#v", result.Vint)
	}
	if result.Vint8 != 42 {
		t.Errorf("vint8 value should be 42: %#v", result.Vint)
	}
	if result.Vint16 != 42 {
		t.Errorf("vint16 value should be 42: %#v", result.Vint)
	}
	if result.Vint32 != 42 {
		t.Errorf("vint32 value should be 42: %#v", result.Vint)
	}
	if result.Vint64 != 42 {
		t.Errorf("vint64 value should be 42: %#v", result.Vint)
	}

	if result.Vuint != 42 {
		t.Errorf("vuint value should be 42: %#v", result.Vuint)
	}

	if result.Vbool != true {
		t.Errorf("vbool value should be true: %#v", result.Vbool)
	}

	if result.Vfloat != 42.42 {
		t.Errorf("vfloat value should be 42.42: %#v", result.Vfloat)
	}

	if result.Vextra != "" {
		t.Errorf("vextra value should be empty: %#v", result.Vextra)
	}

	if result.vsilent != false {
		t.Error("vsilent should not be set, it is unexported")
	}

	if result.Vdata != 42 {
		t.Error("vdata should be valid")
	}

	if result.VjsonInt != 1234 {
		t.Errorf("vjsonint value should be 1234: %#v", result.VjsonInt)
	}

	if result.VjsonUint != 1234 {
		t.Errorf("vjsonuint value should be 1234: %#v", result.VjsonUint)
	}

	if result.VjsonUint64 != 9223372036854775809 {
		t.Errorf("vjsonuint64 value should be 9223372036854775809: %#v", result.VjsonUint64)
	}

	if result.VjsonFloat != 1234.5 {
		t.Errorf("vjsonfloat value should be 1234.5: %#v", result.VjsonFloat)
	}

	if !reflect.DeepEqual(result.VjsonNumber, json.Number("1234.5")) {
		t.Errorf("vjsonnumber value should be '1234.5': %T, %#v", result.VjsonNumber, result.VjsonNumber)
	}
}

func TestBasic_IntWithFloat(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vint": float64(42),
	}

	var result Basic
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}
}

func TestBasic_Merge(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vint": 42,
	}

	var result Basic
	result.Vuint = 100
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}

	expected := Basic{
		Vint:  42,
		Vuint: 100,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("bad: %#v", result)
	}
}

// Test for issue #46.
func TestBasic_Struct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vdata": map[string]any{
			"vstring": "foo",
		},
	}

	var result, inner Basic
	result.Vdata = &inner
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}
	expected := Basic{
		Vdata: &Basic{
			Vstring: "foo",
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("bad: %#v", result)
	}
}

func TestBasic_interfaceStruct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
	}

	var iface any = &Basic{}
	err := Assign(&iface, input)
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}

	expected := &Basic{
		Vstring: "foo",
	}
	if !reflect.DeepEqual(iface, expected) {
		t.Fatalf("bad: %#v", iface)
	}
}

// Issue 187
func TestBasic_interfaceStructNonPtr(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
	}

	var iface any = Basic{}
	err := Assign(&iface, input)
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}

	expected := Basic{
		Vstring: "foo",
	}
	if !reflect.DeepEqual(iface, expected) {
		t.Fatalf("bad: %#v", iface)
	}
}

func TestDecode_Embedded(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
		"Basic": map[string]any{
			"vstring": "innerfoo",
		},
		"vunique": "bar",
	}

	var result Embedded
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vstring)
	}

	if result.Vunique != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result.Vunique)
	}
}

func TestDecode_EmbeddedPointer(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
		"vunique": "bar",
	}

	var result EmbeddedPointer
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := EmbeddedPointer{
		Basic: &Basic{
			Vstring: "foo",
		},
		Vunique: "bar",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("bad: %#v", result)
	}
}

func TestDecode_EmbeddedSlice(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"slice_alias": []string{"foo", "bar"},
		"vunique":     "bar",
	}

	var result EmbeddedSlice
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if !reflect.DeepEqual(result.SliceAlias, SliceAlias([]string{"foo", "bar"})) {
		t.Errorf("slice value: %#v", result.SliceAlias)
	}

	if result.Vunique != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result.Vunique)
	}
}

func TestDecode_EmbeddedArray(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"array_alias": [2]string{"foo", "bar"},
		"vunique":     "bar",
	}

	var result EmbeddedArray
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if !reflect.DeepEqual(result.ArrayAlias, ArrayAlias([2]string{"foo", "bar"})) {
		t.Errorf("array value: %#v", result.ArrayAlias)
	}

	if result.Vunique != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result.Vunique)
	}
}

func TestDecode_decodeSliceWithArray(t *testing.T) {
	t.Parallel()

	var result []int
	input := [1]int{1}
	expected := []int{1}
	if err := Assign(&result, input); err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("wanted %+v, got %+v", expected, result)
	}
}

func TestDecode_EmbeddedNoSquash(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
		"vunique": "bar",
	}

	var result Embedded
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vstring != "foo" {
		t.Errorf("vstring value should be empty: %#v", result.Vstring)
	}

	if result.Vunique != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result.Vunique)
	}
}

func TestDecode_EmbeddedPointerNoSquash(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
		"vunique": "bar",
	}

	result := EmbeddedPointer{
		Basic: &Basic{},
	}

	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if result.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vstring)
	}

	if result.Vunique != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result.Vunique)
	}
}

func TestDecode_EmbeddedPointerSquash_FromStructToMap(t *testing.T) {
	t.Parallel()

	input := EmbeddedPointerSquash{
		Basic: &Basic{
			Vstring: "foo",
		},
		Vunique: "bar",
	}

	var result map[string]any
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result["vstring"] != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result["vstring"])
	}

	if result["vunique"] != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result["vunique"])
	}
}

func TestDecode_EmbeddedPointerSquash_FromMapToStruct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
		"vunique": "bar",
	}

	result := EmbeddedPointerSquash{
		Basic: &Basic{},
	}
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vstring)
	}

	if result.Vunique != "bar" {
		t.Errorf("vunique value should be 'bar': %#v", result.Vunique)
	}
}

func TestDecode_Nil(t *testing.T) {
	t.Parallel()

	var input any
	result := Basic{
		Vstring: "foo",
	}

	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if result.Vstring != "foo" {
		t.Fatalf("bad: %#v", result.Vstring)
	}
}

func TestDecode_NonStruct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"foo": "bar",
		"bar": "baz",
	}

	var result map[string]string
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if result["foo"] != "bar" {
		t.Fatal("foo is not bar")
	}
}

func TestDecode_StructMatch(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vbar": Basic{
			Vstring: "foo",
		},
	}

	var result Nested
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vbar.Vstring != "foo" {
		t.Errorf("bad: %#v", result)
	}
}

func TestDecode_TypeConversion(t *testing.T) {
	input := map[string]any{
		"intToFloat":         42,
		"intToUint":          42,
		"intToBool":          1,
		"intToString":        42,
		"uintToInt":          42,
		"uintToFloat":        42,
		"uintToBool":         42,
		"uintToString":       42,
		"boolToInt":          true,
		"boolToUint":         true,
		"boolToFloat":        true,
		"boolToString":       true,
		"floatToInt":         42.42,
		"floatToUint":        42.42,
		"floatToBool":        42.42,
		"floatToString":      42.42,
		"sliceUint8ToString": []uint8("foo"),
		"stringToSliceUint8": "foo",
		"arrayUint8ToString": [3]uint8{'f', 'o', 'o'},
		"stringToInt":        "42",
		"stringToUint":       "42",
		"stringToBool":       "1",
		"stringToFloat":      "42.42",
		"stringToStrSlice":   "A",
		"stringToIntSlice":   "42",
		"stringToStrArray":   "A",
		"stringToIntArray":   "42",
		"sliceToMap":         []any{},
		"mapToSlice":         map[string]any{},
		"arrayToMap":         []any{},
		"mapToArray":         map[string]any{},
	}

	expectedResultStrict := TypeConversionResult{
		IntToFloat:  42.0,
		IntToUint:   42,
		UintToInt:   42,
		UintToFloat: 42,
		BoolToInt:   0,
		BoolToUint:  0,
		BoolToFloat: 0,
		FloatToInt:  42,
		FloatToUint: 42,
	}

	expectedResultWeak := TypeConversionResult{
		IntToFloat:         42.0,
		IntToUint:          42,
		IntToBool:          true,
		IntToString:        "42",
		UintToInt:          42,
		UintToFloat:        42,
		UintToBool:         true,
		UintToString:       "42",
		BoolToInt:          1,
		BoolToUint:         1,
		BoolToFloat:        1,
		BoolToString:       "1",
		FloatToInt:         42,
		FloatToUint:        42,
		FloatToBool:        true,
		FloatToString:      "42.42",
		SliceUint8ToString: "foo",
		StringToSliceUint8: []byte("foo"),
		ArrayUint8ToString: "foo",
		StringToInt:        42,
		StringToUint:       42,
		StringToBool:       true,
		StringToFloat:      42.42,
		StringToStrSlice:   []string{"A"},
		StringToIntSlice:   []int{42},
		StringToStrArray:   [1]string{"A"},
		StringToIntArray:   [1]int{42},
		SliceToMap:         map[string]any{},
		MapToSlice:         []any{},
		ArrayToMap:         map[string]any{},
		MapToArray:         [1]any{},
	}

	// Test strict type conversion
	var resultStrict TypeConversionResult
	err := Assign(&resultStrict, input)
	if err == nil {
		t.Errorf("should return an error")
	}
	if !reflect.DeepEqual(resultStrict, expectedResultStrict) {
		t.Errorf("expected %v, got: %v", expectedResultStrict, resultStrict)
	}

	// Test weak type conversion
	var resultWeak TypeConversionResult
	err = Assign(&resultWeak, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	})
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}

	if !reflect.DeepEqual(resultWeak, expectedResultWeak) {
		t.Errorf("expected \n%#v, got: \n%#v", expectedResultWeak, resultWeak)
	}
}

func TestMap(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vother": map[any]any{
			"foo": "foo",
			"bar": "bar",
		},
	}

	var result Map
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an error: %s", err)
	}

	if result.Vfoo != "foo" {
		t.Errorf("vfoo value should be 'foo': %#v", result.Vfoo)
	}

	if result.Vother == nil {
		t.Fatal("vother should not be nil")
	}

	if len(result.Vother) != 2 {
		t.Error("vother should have two items")
	}

	if result.Vother["foo"] != "foo" {
		t.Errorf("'foo' key should be foo, got: %#v", result.Vother["foo"])
	}

	if result.Vother["bar"] != "bar" {
		t.Errorf("'bar' key should be bar, got: %#v", result.Vother["bar"])
	}
}

func TestMapMerge(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vother": map[any]any{
			"foo": "foo",
			"bar": "bar",
		},
	}

	var result Map
	result.Vother = map[string]string{"hello": "world"}
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an error: %s", err)
	}

	if result.Vfoo != "foo" {
		t.Errorf("vfoo value should be 'foo': %#v", result.Vfoo)
	}

	expected := map[string]string{
		"foo":   "foo",
		"bar":   "bar",
		"hello": "world",
	}
	if !reflect.DeepEqual(result.Vother, expected) {
		t.Errorf("bad: %#v", result.Vother)
	}
}

func TestMapOfStruct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"value": map[string]any{
			"foo": map[string]string{"vstring": "one"},
			"bar": map[string]string{"vstring": "two"},
		},
	}

	var result MapOfStruct
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err)
	}

	if result.Value == nil {
		t.Fatal("value should not be nil")
	}

	if len(result.Value) != 2 {
		t.Error("value should have two items")
	}

	if result.Value["foo"].Vstring != "one" {
		t.Errorf("foo value should be 'one', got: %s", result.Value["foo"].Vstring)
	}

	if result.Value["bar"].Vstring != "two" {
		t.Errorf("bar value should be 'two', got: %s", result.Value["bar"].Vstring)
	}
}

func p[T any](v T) *T {
	return &v
}

func TestStruct(t *testing.T) {
	t.Parallel()

	var input = BasicPointer{
		Vstring:  p("hello"),
		Vint:     p(42),
		Vuint:    p(uint(42)),
		Vbool:    p(true),
		Vextra:   p("extra"),
		Vdata:    p(any(map[string]any{"foo": "bar"})),
		Vfloat:   p(42.42),
		VRenamed: p("hello renamed"),
	}

	var result Basic
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an error: %s", err)
	}

	if result.Vstring != "hello" {
		t.Errorf("string value should be 'hello', got: %s", result.Vstring)
	}

	if result.Vint != 42 {
		t.Errorf("int value should be 42, got: %d", result.Vint)
	}

	if result.Vuint != 42 {
		t.Errorf("uint value should be 42, got: %d", result.Vuint)
	}

	if result.Vbool != true {
		t.Errorf("bool value should be true, got: %t", result.Vbool)
	}

	if result.Vextra != "extra" {
		t.Errorf("extra value should be 'extra', got: %s", result.Vextra)
	}

	if result.Vdata == nil {
		t.Errorf("data value should be 'bar', got: %s", result.Vdata)
	}

	if m, ok := result.Vdata.(map[string]any); !ok || m["foo"] != "bar" {
		t.Errorf("data value should be 'bar', got: %s", result.Vdata)
	}

	if result.Vfloat != 42.42 {
		t.Errorf("float value should be 42.42, got: %f", result.Vfloat)
	}

	if result.VRenamed != "hello renamed" {
		// # only compare with the struct field name. Not the json tag name
		t.Errorf("renamed value should be 'hello renamed', got: %s", result.VRenamed)
	}
}

func TestNestedType(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vbar": map[string]any{
			"vstring": "foo",
			"vint":    42,
			"vbool":   true,
		},
	}

	var result Nested
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vfoo != "foo" {
		t.Errorf("vfoo value should be 'foo': %#v", result.Vfoo)
	}

	if result.Vbar.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vbar.Vstring)
	}

	if result.Vbar.Vint != 42 {
		t.Errorf("vint value should be 42: %#v", result.Vbar.Vint)
	}

	if result.Vbar.Vbool != true {
		t.Errorf("vbool value should be true: %#v", result.Vbar.Vbool)
	}

	if result.Vbar.Vextra != "" {
		t.Errorf("vextra value should be empty: %#v", result.Vbar.Vextra)
	}
}

func TestNestedTypePointer(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vbar": &map[string]any{
			"vstring": "foo",
			"vint":    42,
			"vbool":   true,
		},
	}

	var result NestedPointer
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vfoo != "foo" {
		t.Errorf("vfoo value should be 'foo': %#v", result.Vfoo)
	}

	if result.Vbar.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vbar.Vstring)
	}

	if result.Vbar.Vint != 42 {
		t.Errorf("vint value should be 42: %#v", result.Vbar.Vint)
	}

	if result.Vbar.Vbool != true {
		t.Errorf("vbool value should be true: %#v", result.Vbar.Vbool)
	}

	if result.Vbar.Vextra != "" {
		t.Errorf("vextra value should be empty: %#v", result.Vbar.Vextra)
	}
}

// Test for issue #46.
func TestNestedTypeInterface(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vbar": &map[string]any{
			"vstring": "foo",
			"vint":    42,
			"vbool":   true,

			"vdata": map[string]any{
				"vstring": "bar",
			},
		},
	}

	var result NestedPointer
	result.Vbar = new(Basic)
	result.Vbar.Vdata = new(Basic)
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got an err: %s", err.Error())
	}

	if result.Vfoo != "foo" {
		t.Errorf("vfoo value should be 'foo': %#v", result.Vfoo)
	}

	if result.Vbar.Vstring != "foo" {
		t.Errorf("vstring value should be 'foo': %#v", result.Vbar.Vstring)
	}

	if result.Vbar.Vint != 42 {
		t.Errorf("vint value should be 42: %#v", result.Vbar.Vint)
	}

	if result.Vbar.Vbool != true {
		t.Errorf("vbool value should be true: %#v", result.Vbar.Vbool)
	}

	if result.Vbar.Vextra != "" {
		t.Errorf("vextra value should be empty: %#v", result.Vbar.Vextra)
	}

	if result.Vbar.Vdata.(*Basic).Vstring != "bar" {
		t.Errorf("vstring value should be 'bar': %#v", result.Vbar.Vdata.(*Basic).Vstring)
	}
}

func TestSlice(t *testing.T) {
	t.Parallel()

	inputStringSlice := map[string]any{
		"vfoo": "foo",
		"vbar": []string{"foo", "bar", "baz"},
	}

	inputStringSlicePointer := map[string]any{
		"vfoo": "foo",
		"vbar": &[]string{"foo", "bar", "baz"},
	}

	outputStringSlice := &Slice{
		"foo",
		[]string{"foo", "bar", "baz"},
	}

	testSliceInput(t, inputStringSlice, outputStringSlice)
	testSliceInput(t, inputStringSlicePointer, outputStringSlice)
}

func TestNotEmptyByteSlice(t *testing.T) {
	t.Parallel()

	inputByteSlice := map[string]any{
		"vfoo": "foo",
		"vbar": []byte(`{"bar": "bar"}`),
	}

	result := SliceOfByte{
		Vfoo: "another foo",
		Vbar: []byte(`{"bar": "bar bar bar bar bar bar bar bar"}`),
	}

	err := Assign(&result, inputByteSlice)
	if err != nil {
		t.Fatalf("got unexpected error: %s", err)
	}

	expected := SliceOfByte{
		Vfoo: "foo",
		Vbar: []byte(`{"bar": "bar"}`),
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("bad: %#v", result)
	}
}

func TestInvalidSlice(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vbar": 42,
	}

	result := Slice{}
	err := Assign(&result, input)
	if err == nil {
		t.Errorf("expected failure")
	}
}

func TestSliceOfStruct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"value": []map[string]any{
			{"vstring": "one"},
			{"vstring": "two"},
		},
	}

	var result SliceOfStruct
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got unexpected error: %s", err)
	}

	if len(result.Value) != 2 {
		t.Fatalf("expected two values, got %d", len(result.Value))
	}

	if result.Value[0].Vstring != "one" {
		t.Errorf("first value should be 'one', got: %s", result.Value[0].Vstring)
	}

	if result.Value[1].Vstring != "two" {
		t.Errorf("second value should be 'two', got: %s", result.Value[1].Vstring)
	}
}

func TestSliceCornerCases(t *testing.T) {
	t.Parallel()

	// Input with a map with zero values
	input := map[string]any{}
	var resultWeak []Basic

	err := Assign(&resultWeak, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	})
	if err != nil {
		t.Fatalf("got unexpected error: %s", err)
	}

	if len(resultWeak) != 0 {
		t.Errorf("length should be 0")
	}
	// Input with more values
	input = map[string]any{
		"vstring": "foo",
	}

	resultWeak = nil
	err = Assign(&resultWeak, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	})
	if err != nil {
		t.Fatalf("got unexpected error: %s", err)
	}

	if resultWeak[0].Vstring != "foo" {
		t.Errorf("value does not match")
	}
}

func TestSliceToMap(t *testing.T) {
	t.Parallel()

	input := []map[string]any{
		{
			"foo": "bar",
		},
		{
			"bar": "baz",
		},
	}

	var result map[string]any
	err := Assign(&result, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	})
	if err != nil {
		t.Fatalf("got an error: %s", err)
	}

	expected := map[string]any{
		"foo": "bar",
		"bar": "baz",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("bad: %#v", result)
	}
}

func TestArray(t *testing.T) {
	t.Parallel()

	inputStringArray := map[string]any{
		"vfoo": "foo",
		"vbar": [2]string{"foo", "bar"},
	}

	inputStringArrayPointer := map[string]any{
		"vfoo": "foo",
		"vbar": &[2]string{"foo", "bar"},
	}

	outputStringArray := &Array{
		"foo",
		[2]string{"foo", "bar"},
	}

	testArrayInput(t, inputStringArray, outputStringArray)
	testArrayInput(t, inputStringArrayPointer, outputStringArray)
}

func TestInvalidArray(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vbar": 42,
	}

	result := Array{}
	err := Assign(&result, input)
	if err == nil {
		t.Errorf("expected failure")
	}
}

func TestArrayOfStruct(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"value": []map[string]any{
			{"vstring": "one"},
			{"vstring": "two"},
		},
	}

	var result ArrayOfStruct
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got unexpected error: %s", err)
	}

	if len(result.Value) != 2 {
		t.Fatalf("expected two values, got %d", len(result.Value))
	}

	if result.Value[0].Vstring != "one" {
		t.Errorf("first value should be 'one', got: %s", result.Value[0].Vstring)
	}

	if result.Value[1].Vstring != "two" {
		t.Errorf("second value should be 'two', got: %s", result.Value[1].Vstring)
	}
}

func TestArrayToMap(t *testing.T) {
	t.Parallel()

	input := []map[string]any{
		{
			"foo": "bar",
		},
		{
			"bar": "baz",
		},
	}

	var result map[string]any
	err := Assign(&result, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	})
	if err != nil {
		t.Fatalf("got an error: %s", err)
	}

	expected := map[string]any{
		"foo": "bar",
		"bar": "baz",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("bad: %#v", result)
	}
}

func TestDecodeTable(t *testing.T) {
	t.Parallel()

	// We need to make new types so that we don't get the short-circuit
	// copy functionality. We want to test the deep copying functionality.
	type BasicCopy Basic
	type NestedPointerCopy NestedPointer
	type MapCopy Map

	tests := []struct {
		name    string
		in      any
		target  any
		out     any
		wantErr bool
	}{
		{
			"basic struct input",
			&Basic{
				Vstring: "vstring",
				Vint:    2,
				Vint8:   2,
				Vint16:  2,
				Vint32:  2,
				Vint64:  2,
				Vuint:   3,
				Vbool:   true,
				Vfloat:  4.56,
				Vextra:  "vextra",
				vsilent: true,
				Vdata:   []byte("data"),
			},
			&map[string]any{},
			&map[string]any{
				"vstring":     "vstring",
				"vint":        2,
				"vint8":       int8(2),
				"vint16":      int16(2),
				"vint32":      int32(2),
				"vint64":      int64(2),
				"vuint":       uint(3),
				"vbool":       true,
				"vfloat":      4.56,
				"vextra":      "vextra",
				"vdata":       []byte("data"),
				"vjsonInt":    0,
				"vjsonUint":   uint(0),
				"vjsonUint64": uint64(0),
				"vjsonFloat":  0.0,
				"vjsonNumber": json.Number(""),
			},
			false,
		},
		{
			"embedded struct input",
			&Embedded{
				Vunique: "vunique",
				Basic: Basic{
					Vstring: "vstring",
					Vint:    2,
					Vint8:   2,
					Vint16:  2,
					Vint32:  2,
					Vint64:  2,
					Vuint:   3,
					Vbool:   true,
					Vfloat:  4.56,
					Vextra:  "vextra",
					vsilent: true,
					Vdata:   []byte("data"),
				},
			},
			&map[string]any{},
			&map[string]any{
				"vunique":     "vunique",
				"vstring":     "vstring",
				"vint":        2,
				"vint8":       int8(2),
				"vint16":      int16(2),
				"vint32":      int32(2),
				"vint64":      int64(2),
				"vuint":       uint(3),
				"vbool":       true,
				"vfloat":      4.56,
				"vextra":      "vextra",
				"vdata":       []byte("data"),
				"vjsonInt":    0,
				"vjsonUint":   uint(0),
				"vjsonUint64": uint64(0),
				"vjsonFloat":  0.0,
				"vjsonNumber": json.Number(""),
			},
			false,
		},
		{
			"struct => struct",
			&Basic{
				Vstring: "vstring",
				Vint:    2,
				Vuint:   3,
				Vbool:   true,
				Vfloat:  4.56,
				Vextra:  "vextra",
				Vdata:   []byte("data"),
				vsilent: true,
			},
			&BasicCopy{},
			&BasicCopy{
				Vstring: "vstring",
				Vint:    2,
				Vuint:   3,
				Vbool:   true,
				Vfloat:  4.56,
				Vextra:  "vextra",
				Vdata:   []byte("data"),
			},
			false,
		},
		{
			"struct => struct with pointers",
			&NestedPointer{
				Vfoo: "hello",
				Vbar: nil,
			},
			&NestedPointerCopy{},
			&NestedPointerCopy{
				Vfoo: "hello",
			},
			false,
		},
		{
			"basic pointer to non-pointer",
			&BasicPointer{
				Vstring: stringPtr("vstring"),
				Vint:    intPtr(2),
				Vuint:   uintPtr(3),
				Vbool:   boolPtr(true),
				Vfloat:  floatPtr(4.56),
				Vdata:   interfacePtr([]byte("data")),
			},
			&Basic{},
			&Basic{
				Vstring: "vstring",
				Vint:    2,
				Vuint:   3,
				Vbool:   true,
				Vfloat:  4.56,
				Vdata:   []byte("data"),
			},
			false,
		},
		{
			"slice non-pointer to pointer",
			&Slice{},
			&SlicePointer{},
			&SlicePointer{},
			false,
		},
		{
			"slice non-pointer to pointer, zero field",
			&Slice{},
			&SlicePointer{
				Vbar: &[]string{"yo"},
			},
			&SlicePointer{},
			false,
		},
		{
			"slice to slice alias",
			&Slice{},
			&SliceOfAlias{},
			&SliceOfAlias{},
			false,
		},
		{
			"nil map to map",
			&Map{},
			&MapCopy{},
			&MapCopy{},
			false,
		},
		{
			"nil map to non-empty map",
			&Map{},
			&MapCopy{Vother: map[string]string{"foo": "bar"}},
			&MapCopy{Vother: map[string]string{"foo": "bar"}},
			false,
		},

		{
			"slice input - should error",
			[]string{"foo", "bar"},
			&map[string]any{},
			&map[string]any{},
			true,
		},
		{
			"struct with slice property",
			&Slice{
				Vfoo: "vfoo",
				Vbar: []string{"foo", "bar"},
			},
			&map[string]any{},
			&map[string]any{
				"vfoo": "vfoo",
				"vbar": []string{"foo", "bar"},
			},
			false,
		},
		{
			"struct with empty slice",
			&map[string]any{
				"vbar": []string{},
			},
			&Slice{},
			&Slice{
				Vbar: []string{},
			},
			false,
		},
		{
			"struct with slice of struct property",
			&SliceOfStruct{
				Value: []Basic{
					{
						Vstring: "vstring",
						Vint:    2,
						Vuint:   3,
						Vbool:   true,
						Vfloat:  4.56,
						Vextra:  "vextra",
						vsilent: true,
						Vdata:   []byte("data"),
					},
				},
			},
			&map[string]any{},
			&map[string]any{
				"value": []Basic{
					{
						Vstring: "vstring",
						Vint:    2,
						Vuint:   3,
						Vbool:   true,
						Vfloat:  4.56,
						Vextra:  "vextra",
						vsilent: true,
						Vdata:   []byte("data"),
					},
				},
			},
			false,
		},
		{
			"struct with map property",
			&Map{
				Vfoo:   "vfoo",
				Vother: map[string]string{"vother": "vother"},
			},
			&map[string]any{},
			&map[string]any{
				"vfoo": "vfoo",
				"vother": map[string]string{
					"vother": "vother",
				}},
			false,
		},
		{
			"tagged struct",
			&Tagged{
				Extra: "extra",
				Value: "value",
			},
			&map[string]string{},
			&map[string]string{
				"bar": "extra",
				"foo": "value",
			},
			false,
		},
		{
			"omit tag struct",
			&struct {
				Value string `json:"value"`
				Omit  string `json:"-"`
			}{
				Value: "value",
				Omit:  "omit",
			},
			&map[string]string{},
			&map[string]string{
				"value": "value",
			},
			false,
		},
		{
			"decode to wrong map type",
			&struct {
				Value string
			}{
				Value: "string",
			},
			&map[string]int{},
			&map[string]int{},
			true,
		},
		// {
		// 	"remainder",
		// 	map[string]any{
		// 		"a": "hello",
		// 		"b": "goodbye",
		// 		"c": "yo",
		// 	},
		// 	&Remainder{},
		// 	&Remainder{
		// 		A: "hello",
		// 		Extra: map[string]any{
		// 			"b": "goodbye",
		// 			"c": "yo",
		// 		},
		// 	},
		// 	false,
		// },
		{
			"remainder with no extra",
			map[string]any{
				"a": "hello",
			},
			&Remainder{},
			&Remainder{
				A:     "hello",
				Extra: nil,
			},
			false,
		},
		{
			"struct with omitempty tag return non-empty values",
			&struct {
				VisibleField any `json:"visible"`
				OmitField    any `json:"omittable,omitempty"`
			}{
				VisibleField: nil,
				OmitField:    "string",
			},
			&map[string]any{},
			&map[string]any{"visible": nil, "omittable": "string"},
			false,
		},
		{
			"struct with omitempty tag ignore empty values",
			&struct {
				VisibleField any `json:"visible"`
				OmitField    any `json:"omittable,omitempty"`
			}{
				VisibleField: nil,
				OmitField:    nil,
			},
			&map[string]any{},
			&map[string]any{"visible": nil},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Assign(tt.target, tt.in); (err != nil) != tt.wantErr {
				t.Fatalf("%q: TestMapOutputForStructuredInputs() unexpected error: %s", tt.name, err)
			}

			if !reflect.DeepEqual(tt.out, tt.target) {
				t.Fatalf("%q: TestMapOutputForStructuredInputs() expected: %#v, got: %#v", tt.name, tt.out, tt.target)
			}
		})
	}
}

func TestInvalidType(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": 42,
	}

	var result Basic
	err := Assign(&result, input)
	if err == nil {
		t.Fatal("error should exist")
	}

	derr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error should be kind of Error, instead: %#v", err)
	}

	if derr.Errors[0] !=
		"'Vstring' expected type 'string', got unconvertible type 'int', value: '42'" {
		t.Errorf("got unexpected error: %s", err)
	}

	inputNegIntUint := map[string]any{
		"vuint": -42,
	}

	err = Assign(&result, inputNegIntUint)
	if err == nil {
		t.Fatal("error should exist")
	}

	derr, ok = err.(*Error)
	if !ok {
		t.Fatalf("error should be kind of Error, instead: %#v", err)
	}

	if derr.Errors[0] != "cannot parse 'Vuint', -42 overflows uint" {
		t.Errorf("got unexpected error: %s", err)
	}

	inputNegFloatUint := map[string]any{
		"vuint": -42.0,
	}

	err = Assign(&result, inputNegFloatUint)
	if err == nil {
		t.Fatal("error should exist")
	}

	derr, ok = err.(*Error)
	if !ok {
		t.Fatalf("error should be kind of Error, instead: %#v", err)
	}

	if derr.Errors[0] != "cannot parse 'Vuint', -42.000000 overflows uint" {
		t.Errorf("got unexpected error: %s", err)
	}
}

func TestDecodeMetadata(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vfoo": "foo",
		"vbar": map[string]any{
			"vstring": "foo",
			"vuint":   42,
			"vsilent": "false",
			"foo":     "bar",
		},
		"bar": "nil",
	}

	var md Metadata
	var result Nested

	err := Assign(&result, input, func(c *AssignConfig) {
		c.Metadata = &md
	})
	if err != nil {
		t.Fatalf("err: %s", err.Error())
	}

	expectedKeys := []string{"Vbar", "Vbar.Vstring", "Vbar.Vuint", "Vfoo"}
	sort.Strings(md.Keys)
	if !reflect.DeepEqual(md.Keys, expectedKeys) {
		t.Fatalf("bad keys: %#v", md.Keys)
	}

	expectedUnused := []string{"bar", "vbar[foo]", "vbar[vsilent]"}
	sort.Strings(md.Unused)
	if !reflect.DeepEqual(md.Unused, expectedUnused) {
		t.Fatalf("bad unused: %#v", md.Unused)
	}
}

func TestMetadata(t *testing.T) {
	t.Parallel()

	type testResult struct {
		Vfoo string
		Vbar BasicPointer
	}

	input := map[string]any{
		"vfoo": "foo",
		"vbar": map[string]any{
			"vstring": "foo",
			"vuint":   42,
			"vsilent": "false",
			"foo":     "bar",
		},
		"bar": "nil",
	}

	var md Metadata
	var result testResult
	err := Assign(&result, input, func(c *AssignConfig) {
		c.Metadata = &md
	})
	if err != nil {
		t.Fatalf("err: %s", err.Error())
	}

	expectedKeys := []string{"Vbar", "Vbar.Vstring", "Vbar.Vuint", "Vfoo"}
	sort.Strings(md.Keys)
	if !reflect.DeepEqual(md.Keys, expectedKeys) {
		t.Fatalf("bad keys: %#v", md.Keys)
	}

	expectedUnused := []string{"bar", "vbar[foo]", "vbar[vsilent]"}
	sort.Strings(md.Unused)
	if !reflect.DeepEqual(md.Unused, expectedUnused) {
		t.Fatalf("bad unused: %#v", md.Unused)
	}

	expectedUnset := []string{
		"Vbar.Vbool", "Vbar.Vdata", "Vbar.Vextra", "Vbar.Vfloat", "Vbar.Vint",
		"Vbar.VjsonFloat", "Vbar.VjsonInt", "Vbar.VjsonNumber"}
	sort.Strings(md.Unset)
	if !reflect.DeepEqual(md.Unset, expectedUnset) {
		t.Fatalf("bad unset: %#v", md.Unset)
	}
}

func TestMetadata_Embedded(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"vstring": "foo",
		"vunique": "bar",
	}

	var md Metadata
	var result Embedded
	err := Assign(&result, input, func(c *AssignConfig) {
		c.Metadata = &md
	})
	if err != nil {
		t.Fatalf("err: %s", err.Error())
	}

	expectedKeys := []string{"Vstring", "Vunique"}
	sort.Strings(md.Keys)
	if !reflect.DeepEqual(md.Keys, expectedKeys) {
		t.Fatalf("bad keys: %#v", md.Keys)
	}

	expectedUnused := []string{}
	if !reflect.DeepEqual(md.Unused, expectedUnused) {
		t.Fatalf("bad unused: %#v", md.Unused)
	}
}

func TestNonPtrValue(t *testing.T) {
	t.Parallel()

	err := Assign(Basic{}, map[string]any{})
	if err == nil {
		t.Fatal("error should exist")
	}

	if err.Error() != "target must be a pointer" {
		t.Errorf("got unexpected error: %s", err)
	}
}

func TestTagged(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"foo": "bar",
		"bar": "value",
	}

	var result Tagged
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if result.Value != "bar" {
		t.Errorf("value should be 'bar', got: %#v", result.Value)
	}

	if result.Extra != "value" {
		t.Errorf("extra should be 'value', got: %#v", result.Extra)
	}
}

func TestWeakDecode(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"foo": "4",
		"bar": "value",
	}

	var result struct {
		Foo int
		Bar string
	}

	if err := Assign(&result, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	}); err != nil {
		t.Fatalf("err: %s", err)
	}
	if result.Foo != 4 {
		t.Fatalf("bad: %#v", result)
	}
	if result.Bar != "value" {
		t.Fatalf("bad: %#v", result)
	}
}

func TestWeakDecodeMetadata(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"foo":        "4",
		"bar":        "value",
		"unused":     "value",
		"unexported": "value",
	}

	var md Metadata
	var result struct {
		Foo        int
		Bar        string
		unexported string
	}

	if err := Assign(&result, input, func(c *AssignConfig) {
		c.WeaklyTypedInput = true
		c.Metadata = &md
	}); err != nil {
		t.Fatalf("err: %s", err)
	}
	if result.Foo != 4 {
		t.Fatalf("bad: %#v", result)
	}
	if result.Bar != "value" {
		t.Fatalf("bad: %#v", result)
	}

	expectedKeys := []string{"Bar", "Foo"}
	sort.Strings(md.Keys)
	if !reflect.DeepEqual(md.Keys, expectedKeys) {
		t.Fatalf("bad keys: %#v", md.Keys)
	}

	expectedUnused := []string{"unexported", "unused"}
	sort.Strings(md.Unused)
	if !reflect.DeepEqual(md.Unused, expectedUnused) {
		t.Fatalf("bad unused: %#v", md.Unused)
	}
}

func TestDecode_StructTaggedWithOmitempty_OmitEmptyValues(t *testing.T) {
	t.Parallel()

	input := &StructWithOmitEmpty{}

	var emptySlice []any
	var emptyMap map[string]any
	var emptyNested *Nested
	expected := &map[string]any{
		"visible-string": "",
		"visible-int":    0,
		"visible-float":  0.0,
		"visible-slice":  emptySlice,
		"visible-map":    emptyMap,
		"visible-nested": emptyNested,
	}

	actual := &map[string]any{}
	Assign(actual, input)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Decode() expected: %#v, got: %#v", expected, actual)
	}
}

func TestDecode_StructTaggedWithOmitempty_KeepNonEmptyValues(t *testing.T) {
	t.Parallel()

	input := &StructWithOmitEmpty{
		VisibleStringField: "",
		OmitStringField:    "string",
		VisibleIntField:    0,
		OmitIntField:       1,
		VisibleFloatField:  0.0,
		OmitFloatField:     1.0,
		VisibleSliceField:  nil,
		OmitSliceField:     []any{1},
		VisibleMapField:    nil,
		OmitMapField:       map[string]any{"k": "v"},
		NestedField:        nil,
		OmitNestedField:    &Nested{},
	}

	var emptySlice []any
	var emptyMap map[string]any
	var emptyNested *Nested
	expected := &map[string]any{
		"visible-string":   "",
		"omittable-string": "string",
		"visible-int":      0,
		"omittable-int":    1,
		"visible-float":    0.0,
		"omittable-float":  1.0,
		"visible-slice":    emptySlice,
		"omittable-slice":  []any{1},
		"visible-map":      emptyMap,
		"omittable-map":    map[string]any{"k": "v"},
		"visible-nested":   emptyNested,
		"omittable-nested": &Nested{},
	}

	actual := &map[string]any{}
	Assign(actual, input)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Decode() expected: %#v, got: %#v", expected, actual)
	}
}

func TestDecode_mapToStruct(t *testing.T) {
	type Target struct {
		String    string
		StringPtr *string
	}

	expected := Target{
		String: "hello",
	}

	var target Target
	err := Assign(&target, map[string]any{
		"string":    "hello",
		"stringPtr": "goodbye",
	})
	if err != nil {
		t.Fatalf("got error: %s", err)
	}

	// Pointers fail reflect test so do those manually
	if target.StringPtr == nil || *target.StringPtr != "goodbye" {
		t.Fatalf("bad: %#v", target)
	}
	target.StringPtr = nil

	if !reflect.DeepEqual(target, expected) {
		t.Fatalf("bad: %#v", target)
	}
}

func TestDecoder_IncludeIgnoreFields(t *testing.T) {
	type Input struct {
		UntaggedNumber int
		TaggedNumber   int    `json:"tagged_number"`
		UntaggedString string `json:"-"`
		TaggedString   string `json:"tagged_string"`
	}
	input := &Input{
		UntaggedNumber: 31,
		TaggedNumber:   42,
		UntaggedString: "hidden",
		TaggedString:   "visible",
	}

	actual := make(map[string]any)
	err := Assign(&actual, input, func(c *AssignConfig) {
		c.IncludeIgnoreFields = true
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := map[string]any{
		"tagged_number":  42,
		"tagged_string":  "visible",
		"untaggedNumber": 31,
		"untaggedString": "hidden",
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Decode() expected: %#v\ngot: %#v", expected, actual)
	}
}

func testSliceInput(t *testing.T, input map[string]any, expected *Slice) {
	var result Slice
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got error: %s", err)
	}

	if result.Vfoo != expected.Vfoo {
		t.Errorf("Vfoo expected '%s', got '%s'", expected.Vfoo, result.Vfoo)
	}

	if result.Vbar == nil {
		t.Fatalf("Vbar a slice, got '%#v'", result.Vbar)
	}

	if len(result.Vbar) != len(expected.Vbar) {
		t.Errorf("Vbar length should be %d, got %d", len(expected.Vbar), len(result.Vbar))
	}

	for i, v := range result.Vbar {
		if v != expected.Vbar[i] {
			t.Errorf(
				"Vbar[%d] should be '%#v', got '%#v'",
				i, expected.Vbar[i], v)
		}
	}
}

func testArrayInput(t *testing.T, input map[string]any, expected *Array) {
	var result Array
	err := Assign(&result, input)
	if err != nil {
		t.Fatalf("got error: %s", err)
	}

	if result.Vfoo != expected.Vfoo {
		t.Errorf("Vfoo expected '%s', got '%s'", expected.Vfoo, result.Vfoo)
	}

	if result.Vbar == [2]string{} {
		t.Fatalf("Vbar a slice, got '%#v'", result.Vbar)
	}

	if len(result.Vbar) != len(expected.Vbar) {
		t.Errorf("Vbar length should be %d, got %d", len(expected.Vbar), len(result.Vbar))
	}

	for i, v := range result.Vbar {
		if v != expected.Vbar[i] {
			t.Errorf(
				"Vbar[%d] should be '%#v', got '%#v'",
				i, expected.Vbar[i], v)
		}
	}
}

func stringPtr(v string) *string  { return &v }
func intPtr(v int) *int           { return &v }
func uintPtr(v uint) *uint        { return &v }
func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }
func interfacePtr(v any) *any     { return &v }
