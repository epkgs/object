package object

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var weakAssigner *assigner

func init() {
	weakAssigner = newAssigner(func(c *AssignConfig) {
		c.WeaklyTypedInput = true
	})
}

// AssignConfig is the configuration that is used to create a new decoder
// and allows customization of various aspects of decoding.
type AssignConfig struct {
	// If WeaklyTypedInput is true, the decoder will make the following
	// "weak" conversions:
	//
	//   - bools to string (true = "1", false = "0")
	//   - numbers to string (base 10)
	//   - bools to int/uint (true = 1, false = 0)
	//   - strings to int/uint (base implied by prefix)
	//   - int to bool (true if value != 0)
	//   - string to bool (accepts: 1, t, T, TRUE, true, True, 0, f, F,
	//     FALSE, false, False. Anything else is an error)
	//   - empty array = empty map and vice versa
	//   - negative numbers to overflowed uint values (base 10)
	//   - slice of maps to a merged map
	//   - single values are converted to slices if required. Each
	//     element is weakly decoded. For example: "4" can become []int{4}
	//     if the target type is an int slice.
	//
	WeaklyTypedInput bool

	// The tag name that object reads for field names. This
	// defaults to "json"
	TagName string

	// IncludeIgnoreFields include all struct fields were ignore by '-'
	IncludeIgnoreFields bool

	// Converter is the function used to convert the struct field name
	// to map key. Defaults to `Lower Camel`.
	Converter func(fieldName string) string

	// Metadata is the struct that will contain extra metadata about
	// the decoding. If this is nil, then no metadata will be tracked.
	Metadata *Metadata

	// SkipKeys is a list of keys that should be skipped during decoding.
	SkipKeys []string

	// SkipSameValues is true will skipped the same values during decoding.
	SkipSameValues bool
}

type Key struct {
	display string // display name
	actual  string // actual name

	displayFull string
	actualFull  string

	parentKind reflect.Kind
	parent     *Key
	children   []*Key
}

func newKey(display string, actual string) *Key {
	k := &Key{
		display: display,
		actual:  actual,
	}

	return k
}

func (k *Key) newChild(parentKind reflect.Kind, display string, actual string) *Key {

	child := &Key{
		display: display,
		actual:  actual,

		displayFull: genFullKey(parentKind, k.displayFull, display),
		actualFull:  genFullKey(parentKind, k.actualFull, actual),

		parent:     k,
		parentKind: parentKind,
	}

	if k.children == nil {
		k.children = []*Key{}
	}
	k.children = append(k.children, child)

	return child
}

func (k *Key) IsEmpty() bool {
	if k == nil {
		return true
	}
	return k.display == "" && k.parent == nil
}

type Keys map[string]*Key

func (k *Keys) FullDisplayNames() []string {
	keys := []string{}
	for _, key := range *k {
		keys = append(keys, key.displayFull)
	}
	return keys
}

func (k *Keys) FullActualNames() []string {
	keys := []string{}
	for _, key := range *k {
		keys = append(keys, key.actualFull)
	}
	return keys
}

func (k *Keys) RootDisplayNames() []string {
	keys := []string{}
	for _, key := range *k {
		if !key.parent.IsEmpty() {
			continue
		}
		keys = append(keys, key.displayFull)
	}
	return keys
}

func (k *Keys) RootActualNames() []string {
	keys := []string{}
	for _, key := range *k {
		if !key.parent.IsEmpty() {
			continue
		}
		keys = append(keys, key.actualFull)
	}
	return keys
}

func (k *Keys) Add(key *Key) *Keys {
	(*k)[key.displayFull] = key
	return k
}

// Metadata contains information about decoding a structure that
// is tedious or difficult to get otherwise.
type Metadata struct {
	// Keys are the target object keys of the structure which were successfully assigned
	keys Keys

	// Unused are the keys that were found in the source but
	// weren't decoded since there was no matching field in the target object
	unused Keys

	// Unset are the field names that were found in the target object
	// but weren't set in the decoding process since there was no matching value
	// in the input
	unset Keys

	_keys       []string
	_keysFull   []string
	_unused     []string
	_unusedFull []string
	_unset      []string
	_unsetFull  []string
}

func (m *Metadata) Keys() []string {
	if m._keys == nil {
		m._keys = m.keys.RootDisplayNames()
	}
	return m._keys
}
func (m *Metadata) KeysFull() []string {
	if m._keysFull == nil {
		m._keysFull = m.keys.FullDisplayNames()
	}
	return m._keysFull
}

func (m *Metadata) Unused() []string {
	if m._unused == nil {
		m._unused = m.unused.RootActualNames()
	}
	return m._unused
}
func (m *Metadata) UnusedFull() []string {
	if m._unusedFull == nil {
		m._unusedFull = m.unused.FullActualNames()
	}
	return m._unused
}

func (m *Metadata) Unset() []string {
	if m._unset == nil {
		m._unset = m.unset.RootDisplayNames()
	}
	return m._unset
}
func (m *Metadata) UnsetFull() []string {
	if m._unsetFull == nil {
		m._unsetFull = m.unset.FullDisplayNames()
	}
	return m._unset
}

// Assign 将 source 对象的值解码并赋值给 target 对象。
// 该函数使用反射机制，因此可以处理任意类型的对象。
// 参数:
//   - target: 任意类型，将被赋值的对象指针。
//   - source: 任意类型，源对象，其值将被解码到 target。
//   - configs: 可选的函数切片，用于配置解码过程，每个函数接收一个 *AssignConfig 指针。
//
// 返回值:
//   - error: 如果解码过程中发生错误，则返回错误。
func Assign(target any, source any, configs ...func(c *AssignConfig)) error {
	// 创建一个实例，并应用配置函数。
	assigner := newAssigner(configs...)
	// 使用解码器将 source 的值解码到 target。
	return assigner.Assign(target, source)
}

type assigner struct {
	config *AssignConfig
}

func newAssigner(configs ...func(c *AssignConfig)) *assigner {
	config := AssignConfig{
		TagName:   "json",
		Converter: toLowerCamel,
	}

	for _, fn := range configs {
		fn(&config)
	}

	if config.Metadata != nil {
		if config.Metadata.keys == nil {
			config.Metadata.keys = make(Keys, 0)
		}
		if config.Metadata.unused == nil {
			config.Metadata.unused = make(Keys, 0)
		}
		if config.Metadata.unset == nil {
			config.Metadata.unset = make(Keys, 0)
		}
	}
	return &assigner{
		config: &config,
	}
}

func (a *assigner) Assign(target, source any, configs ...func(c *AssignConfig)) error {
	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		return errors.New("target must be a pointer")
	}

	targetVal = targetVal.Elem()
	if !targetVal.CanAddr() {
		return errors.New("target must be addressable (a pointer)")
	}

	as := a
	if len(configs) > 0 {
		config := *a.config // copy config

		for _, fn := range configs {
			fn(&config)
		}

		as = &assigner{
			config: &config,
		}
	}

	return as.assign(targetVal, nil, source, nil)
}

// Decodes an unknown data type into a specific reflection value.
func (a *assigner) assign(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {

	if a.shouldSkipKey(targetKey, sourceKey) {
		return nil
	}

	sourceVal := reflect.ValueOf(source)

	if targetKey == nil {
		targetKey = newKey("", "")
	}

	if sourceKey == nil {
		sourceKey = newKey("", "")
	}

	if source != nil {
		// We need to check here if input is a typed nil. Typed nils won't
		// match the "input == nil" below so we check that here.
		if sourceVal.Kind() == reflect.Ptr && sourceVal.IsNil() {
			source = nil
		}
	}

	if source == nil {
		return nil
	}

	if !sourceVal.IsValid() {
		// If the source value is invalid, then we just set the value
		// to be the zero value.
		targetVal.Set(reflect.Zero(targetVal.Type()))

		a.addMetaKey(targetKey)
		return nil
	}

	if a.config.SkipSameValues {
		if reflect.DeepEqual(targetVal.Interface(), source) {
			a.addMetaUnused(sourceKey)
			a.addMetaUnset(targetKey)
			return nil
		}
	}

	var err error
	targetKind := getKind(targetVal)
	addMetaKey := true
	switch targetKind {
	case reflect.Bool:
		err = a.assignBool(targetVal, targetKey, source, sourceKey)
	case reflect.Interface:
		err = a.assignBasic(targetVal, targetKey, source, sourceKey)
	case reflect.String:
		err = a.assignString(targetVal, targetKey, source, sourceKey)
	case reflect.Int:
		err = a.assignInt(targetVal, targetKey, source, sourceKey)
	case reflect.Uint:
		err = a.assignUint(targetVal, targetKey, source, sourceKey)
	case reflect.Float32:
		err = a.assignFloat(targetVal, targetKey, source, sourceKey)
	case reflect.Struct:
		err = a.assignStruct(targetVal, targetKey, source, sourceKey)
	case reflect.Map:
		err = a.assignMap(targetVal, targetKey, source, sourceKey)
	case reflect.Ptr:
		addMetaKey, err = a.assignPtr(targetVal, targetKey, source, sourceKey)
	case reflect.Slice:
		err = a.assignSlice(targetVal, targetKey, source, sourceKey)
	case reflect.Array:
		err = a.assignArray(targetVal, targetKey, source, sourceKey)
	case reflect.Func:
		err = a.assignFunc(targetVal, targetKey, source, sourceKey)
	default:
		// If we reached this point then we weren't able to decode it
		return fmt.Errorf("%s: unsupported type: %s", targetKey.displayFull, targetKind)
	}

	// If we reached here, then we successfully decoded SOMETHING, so
	// mark the key as used if we're tracking metainput.
	if addMetaKey {
		a.addMetaKey(targetKey)
	}

	return err
}

// This decodes a basic type (bool, int, string, etc.) and sets the
// value to "data" of that type.
func (a *assigner) assignBasic(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	if targetVal.IsValid() && targetVal.Elem().IsValid() { // TODO: 搞明白这里的作用和意义
		elem := targetVal.Elem()

		// If we can't address this element, then its not writable. Instead,
		// we make a copy of the value (which is a pointer and therefore
		// writable), decode into that, and replace the whole value.
		copied := false
		if !elem.CanAddr() {
			copied = true

			// Make any
			copy := reflect.New(elem.Type())

			// any = elem
			copy.Elem().Set(elem)

			// Set elem so we decode into it
			elem = copy
		}

		// Decode. If we have an error then return. We also return right
		// away if we're not a copy because that means we decoded directly.
		if err := a.assign(elem, targetKey, source, sourceKey); err != nil || !copied {
			return err
		}

		// If we're a copy, we need to set te final result
		targetVal.Set(elem.Elem())
		return nil
	}

	sourceVal := reflect.ValueOf(source)

	// If the input data is a pointer, and the assigned type is the dereference
	// of that exact pointer, then indirect it so that we can assign it.
	// Example: *string to string
	if sourceVal.Kind() == reflect.Ptr && sourceVal.Type().Elem() == targetVal.Type() {
		sourceVal = reflect.Indirect(sourceVal)
	}

	if !sourceVal.IsValid() {
		sourceVal = reflect.Zero(targetVal.Type())
	}

	sourceType := sourceVal.Type()
	if !sourceType.AssignableTo(targetVal.Type()) {
		return fmt.Errorf(
			"'%s' expected type '%s', got '%s'",
			targetKey.displayFull, targetVal.Type(), sourceType)
	}

	targetVal.Set(sourceVal)
	return nil
}

func (a *assigner) assignString(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceKind := getKind(sourceVal)

	converted := true
	switch {
	case sourceKind == reflect.String:
		targetVal.SetString(sourceVal.String())
	case sourceKind == reflect.Bool && a.config.WeaklyTypedInput:
		if sourceVal.Bool() {
			targetVal.SetString("1")
		} else {
			targetVal.SetString("0")
		}
	case sourceKind == reflect.Int && a.config.WeaklyTypedInput:
		targetVal.SetString(strconv.FormatInt(sourceVal.Int(), 10))
	case sourceKind == reflect.Uint && a.config.WeaklyTypedInput:
		targetVal.SetString(strconv.FormatUint(sourceVal.Uint(), 10))
	case sourceKind == reflect.Float32 && a.config.WeaklyTypedInput:
		targetVal.SetString(strconv.FormatFloat(sourceVal.Float(), 'f', -1, 64))
	case sourceKind == reflect.Slice && a.config.WeaklyTypedInput,
		sourceKind == reflect.Array && a.config.WeaklyTypedInput:
		sourceType := sourceVal.Type()
		elemKind := sourceType.Elem().Kind()
		switch elemKind {
		case reflect.Uint8:
			var uints []uint8
			if sourceKind == reflect.Array {
				uints = make([]uint8, sourceVal.Len())
				for i := range uints {
					uints[i] = sourceVal.Index(i).Interface().(uint8)
				}
			} else {
				uints = sourceVal.Interface().([]uint8)
			}
			targetVal.SetString(string(uints))
		default:
			converted = false
		}
	default:
		converted = false
	}

	if !converted {
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.displayFull, targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignInt(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceKind := getKind(sourceVal)
	sourceType := sourceVal.Type()

	switch {
	case sourceKind == reflect.Int:
		targetVal.SetInt(sourceVal.Int())
	case sourceKind == reflect.Uint:
		targetVal.SetInt(int64(sourceVal.Uint()))
	case sourceKind == reflect.Float32:
		targetVal.SetInt(int64(sourceVal.Float()))
	case sourceKind == reflect.Bool && a.config.WeaklyTypedInput:
		if sourceVal.Bool() {
			targetVal.SetInt(1)
		} else {
			targetVal.SetInt(0)
		}
	case sourceKind == reflect.String && a.config.WeaklyTypedInput:
		str := sourceVal.String()
		if str == "" {
			str = "0"
		}

		i, err := strconv.ParseInt(str, 0, targetVal.Type().Bits())
		if err == nil {
			targetVal.SetInt(i)
		} else {
			return fmt.Errorf("cannot parse '%s' as int: %s", targetKey.displayFull, err)
		}
	case sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number":
		jn := source.(json.Number)
		i, err := jn.Int64()
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.displayFull, err)
		}
		targetVal.SetInt(i)
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.displayFull, targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignUint(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceKind := getKind(sourceVal)
	sourceType := sourceVal.Type()

	switch {
	case sourceKind == reflect.Int:
		i := sourceVal.Int()
		if i < 0 && !a.config.WeaklyTypedInput {
			return fmt.Errorf("cannot parse '%s', %d overflows uint",
				targetKey.displayFull, i)
		}
		targetVal.SetUint(uint64(i))
	case sourceKind == reflect.Uint:
		targetVal.SetUint(sourceVal.Uint())
	case sourceKind == reflect.Float32:
		f := sourceVal.Float()
		if f < 0 && !a.config.WeaklyTypedInput {
			return fmt.Errorf("cannot parse '%s', %f overflows uint",
				targetKey.displayFull, f)
		}
		targetVal.SetUint(uint64(f))
	case sourceKind == reflect.Bool && a.config.WeaklyTypedInput:
		if sourceVal.Bool() {
			targetVal.SetUint(1)
		} else {
			targetVal.SetUint(0)
		}
	case sourceKind == reflect.String && a.config.WeaklyTypedInput:
		str := sourceVal.String()
		if str == "" {
			str = "0"
		}

		i, err := strconv.ParseUint(str, 0, targetVal.Type().Bits())
		if err == nil {
			targetVal.SetUint(i)
		} else {
			return fmt.Errorf("cannot parse '%s' as uint: %s", targetKey.displayFull, err)
		}
	case sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number":
		jn := source.(json.Number)
		i, err := strconv.ParseUint(string(jn), 0, 64)
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.displayFull, err)
		}
		targetVal.SetUint(i)
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.displayFull, targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignBool(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceKind := getKind(sourceVal)

	switch {
	case sourceKind == reflect.Bool:
		targetVal.SetBool(sourceVal.Bool())
	case sourceKind == reflect.Int && a.config.WeaklyTypedInput:
		targetVal.SetBool(sourceVal.Int() != 0)
	case sourceKind == reflect.Uint && a.config.WeaklyTypedInput:
		targetVal.SetBool(sourceVal.Uint() != 0)
	case sourceKind == reflect.Float32 && a.config.WeaklyTypedInput:
		targetVal.SetBool(sourceVal.Float() != 0)
	case sourceKind == reflect.String && a.config.WeaklyTypedInput:
		b, err := strconv.ParseBool(sourceVal.String())
		if err == nil {
			targetVal.SetBool(b)
		} else if sourceVal.String() == "" {
			targetVal.SetBool(false)
		} else {
			return fmt.Errorf("cannot parse '%s' as bool: %s", targetKey.displayFull, err)
		}
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.displayFull, targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignFloat(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceKind := getKind(sourceVal)
	sourceType := sourceVal.Type()

	switch {
	case sourceKind == reflect.Int:
		targetVal.SetFloat(float64(sourceVal.Int()))
	case sourceKind == reflect.Uint:
		targetVal.SetFloat(float64(sourceVal.Uint()))
	case sourceKind == reflect.Float32:
		targetVal.SetFloat(sourceVal.Float())
	case sourceKind == reflect.Bool && a.config.WeaklyTypedInput:
		if sourceVal.Bool() {
			targetVal.SetFloat(1)
		} else {
			targetVal.SetFloat(0)
		}
	case sourceKind == reflect.String && a.config.WeaklyTypedInput:
		str := sourceVal.String()
		if str == "" {
			str = "0"
		}

		f, err := strconv.ParseFloat(str, targetVal.Type().Bits())
		if err == nil {
			targetVal.SetFloat(f)
		} else {
			return fmt.Errorf("cannot parse '%s' as float: %s", targetKey.displayFull, err)
		}
	case sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number":
		jn := source.(json.Number)
		i, err := jn.Float64()
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.displayFull, err)
		}
		targetVal.SetFloat(i)
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.displayFull, targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignMap(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {

	sourceVal := reflect.Indirect(reflect.ValueOf(source))

	// Check input type and based on the input type jump to the proper func
	switch sourceVal.Kind() {
	case reflect.Map:
		return a.assignMapFromMap(targetVal, targetKey, sourceVal, sourceKey)

	case reflect.Struct:
		return a.assignMapFromStruct(targetVal, targetKey, sourceVal, sourceKey)

	case reflect.Array, reflect.Slice:
		if a.config.WeaklyTypedInput {
			return a.assignMapFromSlice(targetVal, targetKey, sourceVal, sourceKey)
		}

		fallthrough

	default:
		return fmt.Errorf("'%s' expected a map, got '%s'", targetKey.displayFull, sourceVal.Kind())
	}
}

func (a *assigner) assignMapFromSlice(targetVal reflect.Value, targetKey *Key, sourceVal reflect.Value, sourceKey *Key) error {
	if sourceVal.IsNil() {
		return nil
	}

	targetMapType := targetVal.Type()
	targetKeyType := targetMapType.Key()
	targetElemType := targetMapType.Elem()

	if !sourceVal.IsNil() && sourceVal.Len() == 0 {
		targetVal.Set(reflect.MakeMap(reflect.MapOf(targetKeyType, targetElemType)))
		a.addMetaKey(targetKey)
		return nil
	}

	if targetVal.IsNil() {
		targetVal.Set(reflect.MakeMap(reflect.MapOf(targetKeyType, targetElemType)))
	}

	for i := 0; i < sourceVal.Len(); i++ {
		k := strconv.Itoa(i)
		srcElem := sourceVal.Index(i)
		err := a.assign(
			targetVal,
			targetKey,
			srcElem.Interface(),
			sourceKey.newChild(reflect.Slice, k, k),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *assigner) assignMapFromMap(targetVal reflect.Value, targetKey *Key, sourceVal reflect.Value, sourceKey *Key) error {
	targetValType := targetVal.Type()
	targetValKeyType := targetValType.Key()
	targetValElemType := targetValType.Elem()

	if sourceVal.IsNil() {
		return nil
	}

	// Accumulate errors
	errors := make([]string, 0)

	if sourceVal.IsNil() {
		return nil
	}

	// If the input data is empty, then we just match what the input data is.
	if sourceVal.Len() == 0 {
		targetVal.Set(reflect.MakeMap(reflect.MapOf(targetValKeyType, targetValElemType)))
		a.addMetaKey(targetKey)
		return nil
	}

	if targetVal.IsNil() {
		targetVal.Set(reflect.MakeMap(reflect.MapOf(targetValKeyType, targetValElemType)))
	}

	for _, srcKey := range sourceVal.MapKeys() {
		kStr := fmt.Sprintf("%v", srcKey.Interface())

		targetElem := reflect.Indirect(reflect.New(targetValElemType))
		sourceElem := sourceVal.MapIndex(srcKey)

		childTargetKey := targetKey.newChild(reflect.Map, kStr, kStr)
		childSourceKey := sourceKey.newChild(reflect.Map, kStr, kStr)

		if a.shouldSkipKey(childTargetKey, childSourceKey) {
			continue
		}

		// First decode the key into the proper type
		currentKey := reflect.Indirect(reflect.New(targetValKeyType))
		if err := weakAssigner.assign(currentKey, nil, srcKey.Interface(), nil); err != nil {
			errors = appendErrors(errors, err)
			continue
		}

		// Next decode the data into the proper type
		if err := a.assign(targetElem, childTargetKey, sourceElem.Interface(), childSourceKey); err != nil {
			errors = appendErrors(errors, err)
			continue
		}

		targetVal.SetMapIndex(currentKey, targetElem)
	}

	// If we had errors, return those
	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) assignMapFromStruct(targetVal reflect.Value, targetKey *Key, sourceVal reflect.Value, sourceKey *Key) error {
	targetMapType := targetVal.Type()
	targetKeyType := targetMapType.Key()
	targetElemType := targetMapType.Elem()

	if targetVal.IsNil() {
		targetVal.Set(reflect.MakeMap(reflect.MapOf(targetKeyType, targetElemType)))
	}

	sourceFields := a.flattenStruct(sourceVal, sourceKey)
	for _, skf := range sourceFields {
		// Next get the actual value of this field and verify it is assignable
		// to the map value.
		if !skf.fieldVal.Type().AssignableTo(targetVal.Type().Elem()) {
			return fmt.Errorf("cannot assign type '%s' to map value field of type '%s'", skf.fieldVal.Type(), targetVal.Type().Elem())
		}

		targetFieldKey := targetKey.newChild(reflect.Struct, skf.key.actual, skf.key.actual)

		if a.shouldSkipKey(targetFieldKey, skf.key) {
			continue
		}

		keyVal := reflect.Indirect(reflect.New(targetKeyType))
		weakAssigner.assign(keyVal, nil, targetFieldKey.actual, nil)

		switch skf.fieldVal.Kind() {

		// this is an embedded struct, so handle it differently
		case reflect.Struct:

			sourceFieldType := skf.fieldVal.Type()
			// struct 是否可以塞入 map
			if sourceFieldType.AssignableTo(targetElemType) {
				targetVal.SetMapIndex(keyVal, skf.fieldVal)
				a.addMetaKey(targetFieldKey)
				continue
			}

			targetChild := map[string]any{}
			targetChildVal := reflect.ValueOf(targetChild)
			if !targetChildVal.Type().AssignableTo(targetElemType) {
				a.addMetaUnused(skf.key)
				continue
			}

			if err := a.assignMapFromStruct(targetChildVal, targetFieldKey, skf.fieldVal, skf.key); err != nil {
				return err
			}

			targetVal.SetMapIndex(keyVal, targetChildVal)
			a.addMetaKey(targetFieldKey)

		default:

			if skf.omitempty && isEmptyValue(skf.fieldVal) {
				a.addMetaUnused(skf.key)
				continue
			}

			targetVal.SetMapIndex(keyVal, skf.fieldVal)
			a.addMetaKey(targetFieldKey)
		}
	}

	return nil
}

func (a *assigner) assignPtr(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) (bool, error) {
	// If the input data is nil, then we want to just set the output
	// pointer to be nil as well.
	isNil := source == nil
	if !isNil {
		switch v := reflect.Indirect(reflect.ValueOf(source)); v.Kind() {
		case reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Ptr,
			reflect.Slice:
			isNil = v.IsNil()
		}
	}
	if isNil {
		if !targetVal.IsNil() && targetVal.CanSet() {
			nilValue := reflect.New(targetVal.Type()).Elem()
			targetVal.Set(nilValue)
		}

		return true, nil
	}

	// Create an element of the concrete (non pointer) type and decode
	// into that. Then set the value of the pointer to this type.
	targetValType := targetVal.Type()
	targetValElemType := targetValType.Elem()
	if targetVal.CanSet() {
		realVal := targetVal
		if targetVal.IsNil() {
			realVal = reflect.New(targetValElemType)
		}

		if err := a.assign(reflect.Indirect(realVal), targetKey, source, sourceKey); err != nil {
			return false, err
		}

		targetVal.Set(realVal)
	} else {
		if err := a.assign(reflect.Indirect(targetVal), targetKey, source, sourceKey); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (a *assigner) assignFunc(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	// Create an element of the concrete (non pointer) type and decode
	// into that. Then set the value of the pointer to this type.
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	if targetVal.Type() != sourceVal.Type() {
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.displayFull, targetVal.Type(), sourceVal.Type(), source)
	}
	targetVal.Set(sourceVal)
	return nil
}

func (a *assigner) assignSlice(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceValKind := sourceVal.Kind()
	targetValType := targetVal.Type()
	targetValElemType := targetValType.Elem()
	sliceType := reflect.SliceOf(targetValElemType)

	// If we have a non array/slice type then we first attempt to convert.
	if sourceValKind != reflect.Array && sourceValKind != reflect.Slice {
		if a.config.WeaklyTypedInput {
			switch {
			// Slice and array we use the normal logic
			case sourceValKind == reflect.Slice, sourceValKind == reflect.Array:
				break

			// Empty maps turn into empty slices
			case sourceValKind == reflect.Map:
				if sourceVal.Len() == 0 {
					targetVal.Set(reflect.MakeSlice(sliceType, 0, 0))
					a.addMetaKey(targetKey)
					return nil
				}
				// Create slice of maps of other sizes
				return a.assignSlice(targetVal, targetKey, []any{source}, sourceKey)

			case sourceValKind == reflect.String && targetValElemType.Kind() == reflect.Uint8:
				return a.assignSlice(targetVal, targetKey, []byte(sourceVal.String()), sourceKey)

			// All other types we try to convert to the slice type
			// and "lift" it into it. i.e. a string becomes a string slice.
			default:
				// Just re-try this function with data as a slice.
				return a.assignSlice(targetVal, targetKey, []any{source}, sourceKey)
			}
		}

		return fmt.Errorf(
			"'%s': source data must be an array or slice, got %s", targetKey.displayFull, sourceValKind)
	}

	// If the input value is nil, then don't allocate since empty != nil
	if sourceValKind != reflect.Array && sourceVal.IsNil() {
		return nil
	}

	targetValSlice := targetVal
	if targetValSlice.IsNil() {
		// Make a new slice to hold our result, same size as the original data.
		targetValSlice = reflect.MakeSlice(sliceType, sourceVal.Len(), sourceVal.Len())
	} else if targetValSlice.Len() > sourceVal.Len() {
		targetValSlice = targetValSlice.Slice(0, sourceVal.Len())
	}

	// Accumulate any errors
	errors := make([]string, 0)

	for i := 0; i < sourceVal.Len(); i++ {
		sourceElem := sourceVal.Index(i)
		for targetValSlice.Len() <= i {
			targetValSlice = reflect.Append(targetValSlice, reflect.Zero(targetValElemType))
		}
		targetField := targetValSlice.Index(i)

		k := strconv.Itoa(i)

		targetFieldKey := targetKey.newChild(reflect.Slice, k, k)
		sourceFieldKey := sourceKey.newChild(reflect.Slice, k, k)

		if a.shouldSkipKey(targetFieldKey, sourceFieldKey) {
			continue
		}

		if err := a.assign(targetField, targetFieldKey, sourceElem.Interface(), sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}
	}

	// Finally, set the value to the slice we built up
	targetVal.Set(targetValSlice)

	// If there were errors, we return those
	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) assignArray(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceValKind := sourceVal.Kind()
	targetValType := targetVal.Type()
	targetValElemType := targetValType.Elem()
	arrayType := reflect.ArrayOf(targetValType.Len(), targetValElemType)

	valArray := targetVal

	if valArray.Interface() == reflect.Zero(valArray.Type()).Interface() {
		// Check input type
		if sourceValKind != reflect.Array && sourceValKind != reflect.Slice {
			if a.config.WeaklyTypedInput {
				switch {
				// Empty maps turn into empty arrays
				case sourceValKind == reflect.Map:
					if sourceVal.Len() == 0 {
						targetVal.Set(reflect.Zero(arrayType))
						a.addMetaKey(targetKey)
						return nil
					}

				// All other types we try to convert to the array type
				// and "lift" it into it. i.e. a string becomes a string array.
				default:
					// Just re-try this function with source as a slice.
					return a.assignArray(targetVal, targetKey, []any{source}, sourceKey)
				}
			}

			return fmt.Errorf(
				"'%s': source data must be an array or slice, got %s", targetKey.displayFull, sourceValKind)

		}
		if sourceVal.Len() > arrayType.Len() {
			return fmt.Errorf(
				"'%s': expected source data to have length less or equal to %d, got %d", targetKey.displayFull, arrayType.Len(), sourceVal.Len())

		}

		// Make a new array to hold our result, same size as the original data.
		valArray = reflect.New(arrayType).Elem()
	}

	// Accumulate any errors
	errors := make([]string, 0)

	for i := 0; i < sourceVal.Len(); i++ {
		sourceElem := sourceVal.Index(i)
		targetField := valArray.Index(i)

		k := strconv.Itoa(i)

		targetFieldKey := targetKey.newChild(reflect.Array, k, k)
		sourceFieldKey := sourceKey.newChild(reflect.Array, k, k)

		if a.shouldSkipKey(targetFieldKey, sourceFieldKey) {
			continue
		}
		if err := a.assign(targetField, targetFieldKey, sourceElem.Interface(), sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}
	}

	// Finally, set the value to the array we built up
	targetVal.Set(valArray)

	// If there were errors, we return those
	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) assignStruct(targetVal reflect.Value, targetKey *Key, source any, sourceKey *Key) error {

	sourceVal := reflect.Indirect(reflect.ValueOf(source))

	sourceValKind := sourceVal.Kind()
	switch sourceValKind {
	case reflect.Map:
		return a.assignStructFromMap(targetVal, targetKey, sourceVal, sourceKey)

	case reflect.Struct:
		return a.assignStructFromStruct(targetVal, targetKey, sourceVal, sourceKey)

	default:
		return fmt.Errorf("'%s' expected a map, got '%s'", targetKey.displayFull, sourceVal.Kind())
	}
}

type fieldInfo struct {
	key       *Key
	field     reflect.StructField
	fieldVal  reflect.Value
	omitempty bool
}

func (a *assigner) flattenStruct(val reflect.Value, key *Key) map[string]fieldInfo {

	// This slice will keep track of all the structs we'll be decoding.
	// There can be more than one struct if there are embedded structs
	// that are squashed.
	structs := make([]reflect.Value, 1, 5)
	structs[0] = val

	fields := map[string]fieldInfo{}

	for len(structs) > 0 {
		structVal := structs[0]
		structs = structs[1:]

		structType := structVal.Type()
		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)
			fieldVal := structVal.Field(i)

			if !field.IsExported() {
				continue
			}

			actualName, omitempty, skip := a.parseTag(field)
			if skip {
				continue
			}

			if omitempty && fieldVal.IsZero() {
				continue
			}

			if field.Anonymous { // 字段为内嵌类型

				if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct { // 字段为内嵌指针结构体

					if fieldVal.IsNil() && fieldVal.CanSet() {
						fieldVal.Set(reflect.New(field.Type.Elem())) // 初始化 fieldVal
						fieldVal = fieldVal.Elem()
					} else {
						fieldVal = fieldVal.Elem()
						if !fieldVal.IsValid() {
							ftype := field.Type.Elem()
							fieldVal = reflect.Indirect(reflect.New(ftype))
						}
					}
				}

				if fieldVal.Kind() == reflect.Struct {
					structs = append(structs, fieldVal)
					continue
				}
			}

			if _, exist := fields[field.Name]; exist {
				// 已存在，embed struct 的 field name 则忽略
				continue
			}

			fields[field.Name] = fieldInfo{
				key:       key.newChild(fieldVal.Type().Kind(), field.Name, actualName),
				field:     field,
				fieldVal:  fieldVal,
				omitempty: omitempty,
			}
		}

	}

	return fields

}

func (a *assigner) assignStructFromMap(targetVal reflect.Value, targetKey *Key, sourceVal reflect.Value, sourceKey *Key) error {
	sourceType := sourceVal.Type()
	sourceTypeKey := sourceType.Key()
	if kind := sourceTypeKey.Kind(); kind != reflect.String && kind != reflect.Interface {
		return fmt.Errorf(
			"'%s' needs a map with string keys, has '%s' keys",
			targetKey.displayFull, sourceTypeKey.Kind())
	}

	unusedMapKeys := make(map[string]struct{})
	for _, k := range sourceVal.MapKeys() {
		unusedMapKeys[k.String()] = struct{}{}
	}

	targetFields := a.flattenStruct(targetVal, targetKey)

	errors := make([]string, 0)
	for _, tkf := range targetFields {
		keyName := tkf.key.actual
		sourceFieldKey := sourceKey.newChild(reflect.Map, keyName, keyName)

		mapKey := reflect.New(sourceTypeKey)
		if err := weakAssigner.assign(mapKey, nil, keyName, nil); err != nil {
			errors = appendErrors(errors, err)
			continue
		}

		value := sourceVal.MapIndex(reflect.Indirect(mapKey))
		if !value.IsValid() {
			a.addMetaUnset(tkf.key)
			continue
		}

		if a.shouldSkipKey(tkf.key, sourceFieldKey) {
			continue
		}

		if !tkf.fieldVal.CanSet() {
			a.addMetaUnset(tkf.key)
			continue
		}

		// 已处理的删除key
		delete(unusedMapKeys, sourceFieldKey.actual)

		if err := a.assign(tkf.fieldVal, tkf.key, value.Interface(), sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}

	}

	if len(unusedMapKeys) > 0 {
		for k := range unusedMapKeys {
			a.addMetaUnused(sourceKey.newChild(reflect.Map, k, k))
		}
	}

	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) assignStructFromStruct(targetVal reflect.Value, targetKey *Key, sourceVal reflect.Value, sourceKey *Key) error {
	targetFields := a.flattenStruct(targetVal, targetKey)
	sourceFields := a.flattenStruct(sourceVal, sourceKey)

	errors := make([]string, 0)
	for tfieldName, tkf := range targetFields {
		skf, exist := sourceFields[tfieldName]
		if !exist {
			a.addMetaUnset(tkf.key)
			continue
		}

		if a.shouldSkipKey(tkf.key, skf.key) {
			continue
		}

		if !skf.fieldVal.IsValid() {
			a.addMetaUnused(skf.key)
			continue
		}

		if !tkf.fieldVal.CanSet() {
			a.addMetaUnset(tkf.key)
			continue
		}

		// 已处理的删除key
		delete(sourceFields, tfieldName)

		if err := a.assign(tkf.fieldVal, tkf.key, skf.fieldVal.Interface(), skf.key); err != nil {
			errors = appendErrors(errors, err)
		}

	}

	if len(sourceFields) > 0 {
		for _, skf := range sourceFields {
			a.addMetaUnused(skf.key)
		}
	}

	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) shouldSkipKey(targetKey, sourceKey *Key) bool {

	if targetKey == nil || sourceKey == nil {
		return false
	}

	for _, keyToSkip := range a.config.SkipKeys {
		if targetKey.displayFull == keyToSkip ||
			targetKey.actualFull == keyToSkip ||
			sourceKey.displayFull == keyToSkip ||
			sourceKey.actualFull == keyToSkip {
			a.addMetaUnused(sourceKey)
			a.addMetaUnset(targetKey)
			return true
		}
	}
	return false
}

func (a *assigner) addMetaKey(targetKey *Key) {
	if a.config.Metadata == nil {
		return
	}

	if targetKey.IsEmpty() {
		return
	}

	a.config.Metadata.keys.Add(targetKey)
}

func (a *assigner) addMetaUnused(sourceKey *Key) {
	if a.config.Metadata == nil {
		return
	}

	if sourceKey == nil {
		return
	}

	a.config.Metadata.unused.Add(sourceKey)
}

func (a *assigner) addMetaUnset(targetKey *Key) {
	if a.config.Metadata == nil {
		return
	}

	if targetKey == nil {
		return
	}

	a.config.Metadata.unset.Add(targetKey)
}

func isEmptyValue(v reflect.Value) bool {
	switch getKind(v) {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func getKind(val reflect.Value) reflect.Kind {
	kind := val.Kind()

	switch {
	case kind >= reflect.Int && kind <= reflect.Int64:
		return reflect.Int
	case kind >= reflect.Uint && kind <= reflect.Uint64:
		return reflect.Uint
	case kind >= reflect.Float32 && kind <= reflect.Float64:
		return reflect.Float32
	default:
		return kind
	}
}

func (a *assigner) parseTag(field reflect.StructField) (actualName string, omitempty, skip bool) {
	tagValue := field.Tag.Get(a.config.TagName)
	// Determine the name of the key in the map
	pices := strings.Split(tagValue, ",")

	displayName := field.Name

	if len(pices) == 0 || pices[0] == "" {
		actualName = a.config.Converter(displayName)
	} else if pices[0] == "-" {
		if a.config.IncludeIgnoreFields {
			actualName = a.config.Converter(displayName)
		} else {
			skip = true
		}
	} else {
		actualName = pices[0]
	}

	for _, pice := range pices {
		if pice == "omitempty" {
			omitempty = true
		}
	}

	return
}

func genFullKey(parentKind reflect.Kind, parentFull, keyName string) string {
	if parentKind != 0 {
		if parentFull == "" {
			return keyName
		}

		if keyName == "" {
			return parentFull
		}

		switch parentKind {
		case reflect.Map, reflect.Array, reflect.Slice:
			return parentFull + "[" + keyName + "]"
		default:
			return parentFull + "." + keyName
		}
	}
	return keyName
}
