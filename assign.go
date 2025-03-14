package object

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var defaultAssigner *assigner
var weakAssigner *assigner

func init() {
	defaultAssigner = &assigner{
		config: &AssignConfig{
			TagName:   "json",
			Converter: toLowerCamel,
		},
	}
	weakAssigner = &assigner{
		config: &AssignConfig{
			TagName:          "json",
			Converter:        toLowerCamel,
			WeaklyTypedInput: true,
		},
	}
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

// Metadata contains information about decoding a structure that
// is tedious or difficult to get otherwise.
type Metadata struct {
	// Keys are the target object keys of the structure which were successfully assigned
	Keys []string

	// Unused are the keys that were found in the source but
	// weren't decoded since there was no matching field in the target object
	Unused []string

	// Unset are the field names that were found in the target object
	// but weren't set in the decoding process since there was no matching value
	// in the input
	Unset []string
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
	return defaultAssigner.Assign(target, source, configs...)
}

type assigner struct {
	config *AssignConfig
}

func (a *assigner) withConfig(configs ...func(c *AssignConfig)) *assigner {
	config := *a.config // copy config

	for _, fn := range configs {
		fn(&config)
	}

	if config.Metadata != nil {
		if config.Metadata.Keys == nil {
			config.Metadata.Keys = []string{}
		}
		if config.Metadata.Unused == nil {
			config.Metadata.Unused = []string{}
		}
		if config.Metadata.Unset == nil {
			config.Metadata.Unset = []string{}
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
		as = as.withConfig(configs...)
	}

	return as.assign(targetVal, "", source, "")
}

// Decodes an unknown data type into a specific reflection value.
func (a *assigner) assign(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {

	if a.shouldSkipKey(targetKey, sourceKey) {
		return nil
	}

	sourceVal := reflect.ValueOf(source)

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
		return fmt.Errorf("%s: unsupported type: %s", targetKey.String(), targetKind)
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
func (a *assigner) assignBasic(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
			targetKey.String(), targetVal.Type(), sourceType)
	}

	targetVal.Set(sourceVal)
	return nil
}

func (a *assigner) assignString(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
			targetKey.String(), targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignInt(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
			return fmt.Errorf("cannot parse '%s' as int: %s", targetKey.String(), err)
		}
	case sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number":
		jn := source.(json.Number)
		i, err := jn.Int64()
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.String(), err)
		}
		targetVal.SetInt(i)
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.String(), targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignUint(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	sourceKind := getKind(sourceVal)
	sourceType := sourceVal.Type()

	switch {
	case sourceKind == reflect.Int:
		i := sourceVal.Int()
		if i < 0 && !a.config.WeaklyTypedInput {
			return fmt.Errorf("cannot parse '%s', %d overflows uint",
				targetKey.String(), i)
		}
		targetVal.SetUint(uint64(i))
	case sourceKind == reflect.Uint:
		targetVal.SetUint(sourceVal.Uint())
	case sourceKind == reflect.Float32:
		f := sourceVal.Float()
		if f < 0 && !a.config.WeaklyTypedInput {
			return fmt.Errorf("cannot parse '%s', %f overflows uint",
				targetKey.String(), f)
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
			return fmt.Errorf("cannot parse '%s' as uint: %s", targetKey.String(), err)
		}
	case sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number":
		jn := source.(json.Number)
		i, err := strconv.ParseUint(string(jn), 0, 64)
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.String(), err)
		}
		targetVal.SetUint(i)
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.String(), targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignBool(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
			return fmt.Errorf("cannot parse '%s' as bool: %s", targetKey.String(), err)
		}
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.String(), targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignFloat(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
			return fmt.Errorf("cannot parse '%s' as float: %s", targetKey.String(), err)
		}
	case sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number":
		jn := source.(json.Number)
		i, err := jn.Float64()
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.String(), err)
		}
		targetVal.SetFloat(i)
	default:
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.String(), targetVal.Type(), sourceVal.Type(), source)
	}

	return nil
}

func (a *assigner) assignMap(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {

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
		return fmt.Errorf("'%s' expected a map, got '%s'", targetKey.String(), sourceVal.Kind())
	}
}

func (a *assigner) assignMapFromSlice(targetVal reflect.Value, targetKey kkk, sourceVal reflect.Value, sourceKey kkk) error {
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
			sourceKey.newChild(reflect.Slice, k),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *assigner) assignMapFromMap(targetVal reflect.Value, targetKey kkk, sourceVal reflect.Value, sourceKey kkk) error {
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

		childTargetKey := targetKey.newChild(reflect.Map, kStr)
		childSourceKey := sourceKey.newChild(reflect.Map, kStr)

		if a.shouldSkipKey(childTargetKey, childSourceKey) {
			continue
		}

		// First decode the key into the proper type
		currentKey := reflect.Indirect(reflect.New(targetValKeyType))
		if err := weakAssigner.assign(currentKey, "", srcKey.Interface(), ""); err != nil {
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

func (a *assigner) assignMapFromStruct(targetVal reflect.Value, targetKey kkk, sourceVal reflect.Value, sourceKey kkk) error {
	targetMapType := targetVal.Type()
	targetKeyType := targetMapType.Key()
	targetElemType := targetMapType.Elem()

	if targetVal.IsNil() {
		targetVal.Set(reflect.MakeMap(reflect.MapOf(targetKeyType, targetElemType)))
	}

	sourceFields := a.flattenStruct(sourceVal)
	for _, srcField := range sourceFields {
		// Next get the actual value of this field and verify it is assignable
		// to the map value.
		if !srcField.fieldVal.Type().AssignableTo(targetVal.Type().Elem()) {
			return fmt.Errorf("cannot assign type '%s' to map value field of type '%s'", srcField.fieldVal.Type(), targetVal.Type().Elem())
		}

		targetFieldKey := targetKey.newChild(reflect.Map, srcField.actualName)
		sourceFieldKey := sourceKey.newChild(reflect.Struct, srcField.displayName)

		if a.shouldSkipKey(targetFieldKey, sourceFieldKey) {
			continue
		}

		keyVal := reflect.Indirect(reflect.New(targetKeyType))
		weakAssigner.assign(keyVal, "", srcField.actualName, "")

		switch srcField.fieldVal.Kind() {

		// this is an embedded struct, so handle it differently
		case reflect.Struct:

			sourceFieldType := srcField.fieldVal.Type()
			// struct 是否可以塞入 map
			if sourceFieldType.AssignableTo(targetElemType) {
				targetVal.SetMapIndex(keyVal, srcField.fieldVal)
				a.addMetaKey(targetFieldKey)
				continue
			}

			targetChild := map[string]any{}
			targetChildVal := reflect.ValueOf(targetChild)
			if !targetChildVal.Type().AssignableTo(targetElemType) {
				a.addMetaUnused(sourceFieldKey)
				continue
			}

			if err := a.assignMapFromStruct(targetChildVal, targetFieldKey, srcField.fieldVal, sourceFieldKey); err != nil {
				return err
			}

			targetVal.SetMapIndex(keyVal, targetChildVal)
			a.addMetaKey(targetFieldKey)

		default:

			if srcField.omitempty && isEmptyValue(srcField.fieldVal) {
				a.addMetaUnused(sourceFieldKey)
				continue
			}

			targetVal.SetMapIndex(keyVal, srcField.fieldVal)
			a.addMetaKey(targetFieldKey)
		}
	}

	return nil
}

func (a *assigner) assignPtr(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) (bool, error) {
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

func (a *assigner) assignFunc(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
	// Create an element of the concrete (non pointer) type and decode
	// into that. Then set the value of the pointer to this type.
	sourceVal := reflect.Indirect(reflect.ValueOf(source))
	if targetVal.Type() != sourceVal.Type() {
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.String(), targetVal.Type(), sourceVal.Type(), source)
	}
	targetVal.Set(sourceVal)
	return nil
}

func (a *assigner) assignSlice(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
			"'%s': source data must be an array or slice, got %s", targetKey.String(), sourceValKind)
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

		targetFieldKey := targetKey.newChild(reflect.Slice, k)
		sourceFieldKey := sourceKey.newChild(reflect.Slice, k)

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

func (a *assigner) assignArray(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {
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
				"'%s': source data must be an array or slice, got %s", targetKey.String(), sourceValKind)

		}
		if sourceVal.Len() > arrayType.Len() {
			return fmt.Errorf(
				"'%s': expected source data to have length less or equal to %d, got %d", targetKey.String(), arrayType.Len(), sourceVal.Len())

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

		targetFieldKey := targetKey.newChild(reflect.Array, k)
		sourceFieldKey := sourceKey.newChild(reflect.Array, k)

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

func (a *assigner) assignStruct(targetVal reflect.Value, targetKey kkk, source any, sourceKey kkk) error {

	sourceVal := reflect.Indirect(reflect.ValueOf(source))

	sourceValKind := sourceVal.Kind()
	switch sourceValKind {
	case reflect.Map:
		return a.assignStructFromMap(targetVal, targetKey, sourceVal, sourceKey)

	case reflect.Struct:
		return a.assignStructFromStruct(targetVal, targetKey, sourceVal, sourceKey)

	default:
		return fmt.Errorf("'%s' expected a map, got '%s'", targetKey.String(), sourceVal.Kind())
	}
}

type fieldInfo struct {
	field       reflect.StructField
	fieldVal    reflect.Value
	displayName string
	actualName  string
	omitempty   bool
}

func (a *assigner) flattenStruct(val reflect.Value) map[string]fieldInfo {

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
				field:       field,
				fieldVal:    fieldVal,
				displayName: field.Name,
				actualName:  actualName,
				omitempty:   omitempty,
			}
		}

	}

	return fields

}

func (a *assigner) assignStructFromMap(targetVal reflect.Value, targetKey kkk, sourceVal reflect.Value, sourceKey kkk) error {
	sourceType := sourceVal.Type()
	sourceTypeKey := sourceType.Key()
	if kind := sourceTypeKey.Kind(); kind != reflect.String && kind != reflect.Interface {
		return fmt.Errorf(
			"'%s' needs a map with string keys, has '%s' keys",
			targetKey.String(), sourceTypeKey.Kind())
	}

	unusedMapKeys := make(map[string]struct{})
	for _, k := range sourceVal.MapKeys() {
		unusedMapKeys[k.String()] = struct{}{}
	}

	targetFields := a.flattenStruct(targetVal)

	errors := make([]string, 0)
	for _, targetField := range targetFields {

		mapKey := reflect.New(sourceTypeKey)
		if err := weakAssigner.assign(mapKey, "", targetField.actualName, ""); err != nil {
			errors = appendErrors(errors, err)
			continue
		}

		targetFieldKey := targetKey.newChild(reflect.Struct, targetField.displayName)

		value := sourceVal.MapIndex(reflect.Indirect(mapKey))
		if !value.IsValid() {
			a.addMetaUnset(targetFieldKey)
			continue
		}

		sourceFieldKey := sourceKey.newChild(reflect.Map, targetField.actualName)

		if a.shouldSkipKey(targetFieldKey, sourceFieldKey) {
			continue
		}

		if !targetField.fieldVal.CanSet() {
			a.addMetaUnset(targetFieldKey)
			continue
		}

		// 已处理的删除key
		delete(unusedMapKeys, targetField.actualName)

		if err := a.assign(targetField.fieldVal, targetFieldKey, value.Interface(), sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}

	}

	for k := range unusedMapKeys {
		a.addMetaUnused(sourceKey.newChild(reflect.Map, k))
	}

	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) assignStructFromStruct(targetVal reflect.Value, targetKey kkk, sourceVal reflect.Value, sourceKey kkk) error {
	targetFields := a.flattenStruct(targetVal)
	sourceFields := a.flattenStruct(sourceVal)

	errors := make([]string, 0)
	for tfieldName, targetField := range targetFields {

		targetFieldKey := targetKey.newChild(reflect.Struct, targetField.displayName)

		sourceField, exist := sourceFields[tfieldName]
		if !exist {
			a.addMetaUnset(targetFieldKey)
			continue
		}

		sourceFieldKey := sourceKey.newChild(reflect.Struct, sourceField.displayName)

		if a.shouldSkipKey(targetFieldKey, sourceFieldKey) {
			continue
		}

		if !sourceField.fieldVal.IsValid() {
			a.addMetaUnused(sourceFieldKey)
			continue
		}

		if !targetField.fieldVal.CanSet() {
			a.addMetaUnset(targetFieldKey)
			continue
		}

		// 已处理的删除key
		delete(sourceFields, tfieldName)

		if err := a.assign(targetField.fieldVal, targetFieldKey, sourceField.fieldVal.Interface(), sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}

	}

	for displayName, _ := range sourceFields {
		a.addMetaUnused(sourceKey.newChild(reflect.Struct, displayName))
	}

	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) shouldSkipKey(targetKey, sourceKey kkk) bool {

	if targetKey == "" || sourceKey == "" {
		return false
	}

	for _, keyToSkip := range a.config.SkipKeys {
		if string(targetKey) == keyToSkip ||
			string(sourceKey) == keyToSkip {
			a.addMetaUnused(sourceKey)
			a.addMetaUnset(targetKey)
			return true
		}
	}
	return false
}

func (a *assigner) addMetaKey(targetKey kkk) {
	if a.config.Metadata == nil {
		return
	}

	if targetKey.IsEmpty() {
		return
	}

	a.config.Metadata.Keys = append(a.config.Metadata.Keys, string(targetKey))
}

func (a *assigner) addMetaUnused(sourceKey kkk) {
	if a.config.Metadata == nil {
		return
	}

	if sourceKey.IsEmpty() {
		return
	}

	a.config.Metadata.Unused = append(a.config.Metadata.Unused, string(sourceKey))
}

func (a *assigner) addMetaUnset(targetKey kkk) {
	if a.config.Metadata == nil {
		return
	}

	if targetKey.IsEmpty() {
		return
	}

	a.config.Metadata.Unset = append(a.config.Metadata.Unset, string(targetKey))
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

type kkk string

func (k kkk) String() string {
	return string(k)
}

func (k kkk) IsEmpty() bool {
	return k == ""
}

func (k kkk) newChild(parentKind reflect.Kind, fieldName string) kkk {
	n := genFullKey(parentKind, string(k), fieldName)
	return kkk(n)
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
