package object

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

var defaultAssigner *assigner
var weakAssigner *assigner

func init() {
	defaultAssigner = newAssigner(&AssignConfig{
		TagName:   "json",
		Converter: toLowerCamel,
	})
	weakAssigner = newAssigner(&AssignConfig{
		TagName:          "json",
		Converter:        toLowerCamel,
		WeaklyTypedInput: true,
	})
}

// AssignConfig is the configuration used to create a new decoder
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

	// TagName is the tag name that object reads for field names.
	// This defaults to "json"
	TagName string

	// IncludeIgnoreFields includes all struct fields that were ignored by '-'
	IncludeIgnoreFields bool

	// Converter is the function used to convert the struct field name
	// to map key. Defaults to `Lower Camel`.
	Converter func(fieldName string) string

	// Metadata is the struct that will contain extra metadata about
	// the decoding. If this is nil, then no metadata will be tracked.
	Metadata *Metadata

	// SkipKeys is a list of keys that should be skipped during decoding.
	SkipKeys []string

	// SkipSameValues if true will skip the same values during decoding.
	SkipSameValues bool
}

// Metadata contains information about the decoding process that
// would be tedious or difficult to obtain otherwise.
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

// Assign decodes values from the source object and assigns them to the target object.
// This function uses reflection, so it can handle objects of any type.
// Parameters:
//   - target: Any type, pointer to the object that will be assigned values.
//   - source: Any type, source object whose values will be decoded into target.
//   - configs: Optional function slice for configuring the decoding process,
//     each function receives a *AssignConfig pointer.
//
// Returns:
//   - error: Returns an error if an error occurs during the decoding process.
func Assign(target any, source any, configs ...func(c *AssignConfig)) error {
	return defaultAssigner.Assign(target, source, configs...)
}

type assigner struct {
	config        *AssignConfig
	skipKeysCache map[string]struct{}
}

func newAssigner(c *AssignConfig) *assigner {
	a := &assigner{
		config:        c,
		skipKeysCache: make(map[string]struct{}),
	}

	for _, k := range c.SkipKeys {
		a.skipKeysCache[k] = struct{}{}
	}

	return a
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

	return newAssigner(&config)
}

// Assign decodes and assigns values from the source to the target.
// The target must be a pointer to a value that can be addressed.
// It returns an error if the target is not a pointer or cannot be addressed,
// or if any error occurs during the assignment process.
func (a *assigner) Assign(target, source any, configs ...func(c *AssignConfig)) error {
	// Check that target is a pointer
	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		return errors.New("target must be a pointer")
	}

	// Get the element that the pointer points to
	targetVal = targetVal.Elem()
	if !targetVal.CanAddr() {
		return errors.New("target must be addressable (a pointer)")
	}

	// Apply custom configurations if provided
	as := a
	if len(configs) > 0 {
		as = as.withConfig(configs...)
	}

	sourceVal := reflect.ValueOf(source)

	// Perform the assignment
	return as.assign(targetVal, "", sourceVal, "")
}

// assign decodes an unknown data type into a specific reflection value.
func (a *assigner) assign(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	// Check if we should skip this key based on configuration
	if a.shouldSkipKey(targetKey, sourceKey) {
		return nil
	}

	// Handle typed nil values
	if sourceVal.IsValid() {
		// Check if input is a typed nil. Typed nils won't
		// match the "source == nil" check below, so we handle them here.
		if sourceVal.Kind() == reflect.Ptr && sourceVal.IsNil() {
			sourceVal = reflect.Value{}
		}
	}

	// Handle invalid source values
	if !sourceVal.IsValid() {
		return nil
	}

	// Skip same values if configured to do so
	if a.config.SkipSameValues {
		if reflect.DeepEqual(targetVal.Interface(), sourceVal.Interface()) {
			a.addMetaUnused(sourceKey)
			a.addMetaUnset(targetKey)
			return nil
		}
	}

	if sourceVal.Kind() == reflect.Interface {
		sourceVal = sourceVal.Elem()
	}

	// Process based on target type
	var err error
	targetKind := targetVal.Kind()
	addMetaKey := true

	switch targetKind {
	case reflect.Bool:
		err = a.assignBool(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Interface:
		err = a.assignBasic(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.String:
		err = a.assignString(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		err = a.assignInt(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		err = a.assignUint(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Float32, reflect.Float64:
		err = a.assignFloat(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Struct:
		err = a.assignStruct(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Map:
		err = a.assignMap(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Ptr:
		addMetaKey, err = a.assignPtr(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Slice:
		err = a.assignSlice(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Array:
		err = a.assignArray(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Func:
		err = a.assignFunc(targetVal, targetKey, sourceVal, sourceKey)
	default:
		// Unsupported type
		return fmt.Errorf("%s: unsupported type: %s", targetKey.String(), targetKind)
	}

	// Mark key as used if we're tracking metadata and assignment was successful
	if addMetaKey && err == nil {
		a.addMetaKey(targetKey)
	}

	return err
}

// assignBasic decodes a basic type (bool, int, string, etc.) and sets the
// value to "data" of that type.
func (a *assigner) assignBasic(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	// Handle the case where targetVal is a valid pointer to a valid element
	if targetVal.IsValid() && targetVal.Elem().IsValid() {
		elem := targetVal.Elem()

		// If we can't address this element, then it's not writable. Instead,
		// we make a copy of the value (which is a pointer and therefore
		// writable), decode into that, and replace the whole value.
		copied := false
		if !elem.CanAddr() {
			copied = true

			// Create a new pointer to the element's type
			copy := reflect.New(elem.Type())

			// Set the copy's element to the original element's value
			copy.Elem().Set(elem)

			// Update elem to point to our copy so we decode into it
			elem = copy
		}

		// Decode the value. If there's an error, return it immediately.
		// Also return immediately if we're not working with a copy,
		// which means we decoded directly.
		if err := a.assign(elem, targetKey, sourceVal, sourceKey); err != nil || !copied {
			return err
		}

		// If we used a copy, we need to set the final result
		targetVal.Set(elem.Elem())
		return nil
	}

	// If the input data is a pointer, and the assigned type is the dereference
	// of that exact pointer, then indirect it so that we can assign it.
	// Example: *string to string
	if sourceVal.Kind() == reflect.Ptr && sourceVal.Type().Elem() == targetVal.Type() {
		sourceVal = reflect.Indirect(sourceVal)
	}

	// Handle invalid source values by using the zero value of target type
	if !sourceVal.IsValid() {
		sourceVal = reflect.Zero(targetVal.Type())
	}

	// Check if we can assign the source value to the target
	sourceType := sourceVal.Type()
	if !sourceType.AssignableTo(targetVal.Type()) {
		return fmt.Errorf(
			"'%s' expected type '%s', got '%s'",
			targetKey.String(), targetVal.Type(), sourceType)
	}

	// Perform the assignment
	targetVal.Set(sourceVal)
	return nil
}

// assignString assigns a value to a string target, performing type conversions as needed.
func (a *assigner) assignString(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, _ metaKey) error {
	// Get the source value, dereferencing pointers if necessary
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()

	if isString(sourceKind) {
		// Direct string assignment
		targetVal.SetString(sourceVal.String())
		return nil
	}

	if a.config.WeaklyTypedInput {
		if isBool(sourceKind) {
			// Convert boolean to string ("1" for true, "0" for false)
			if sourceVal.Bool() {
				targetVal.SetString("1")
			} else {
				targetVal.SetString("0")
			}
			return nil
		}

		if isInt(sourceKind) {
			// Convert integer to string
			targetVal.SetString(strconv.FormatInt(sourceVal.Int(), 10))
			return nil
		}

		if isUint(sourceKind) {
			// Convert unsigned integer to string
			targetVal.SetString(strconv.FormatUint(sourceVal.Uint(), 10))
			return nil
		}

		if isFloat(sourceKind) {
			// Convert float to string
			targetVal.SetString(strconv.FormatFloat(sourceVal.Float(), 'f', -1, 64))
			return nil
		}

		if isArraySlice(sourceKind) {
			// Handle slices and arrays
			sourceType := sourceVal.Type()
			elemKind := sourceType.Elem().Kind()

			if elemKind == reflect.Uint8 {
				// Convert byte slice/array to string
				var uints []uint8
				if sourceKind == reflect.Array {
					// For arrays, copy elements to a slice
					uints = make([]uint8, sourceVal.Len())
					for i := range uints {
						uints[i] = sourceVal.Index(i).Interface().(uint8)
					}
				} else {
					// For slices, direct type assertion
					uints = sourceVal.Interface().([]uint8)
				}
				targetVal.SetString(string(uints))
				return nil
			}

		}
	}

	return fmt.Errorf(
		"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
		targetKey.String(),
		targetVal.Type(),
		sourceVal.Type(),
		sourceVal.Interface(),
	)
}

func (a *assigner) assignInt(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, _ metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()
	sourceType := sourceVal.Type()

	if isInt(sourceKind) {
		targetVal.SetInt(sourceVal.Int())
		return nil
	}

	if isUint(sourceKind) {
		targetVal.SetInt(int64(sourceVal.Uint()))
		return nil
	}

	if isFloat(sourceKind) {
		targetVal.SetInt(int64(sourceVal.Float()))
		return nil
	}

	if a.config.WeaklyTypedInput {
		if isBool(sourceKind) {
			if sourceVal.Bool() {
				targetVal.SetInt(1)
			} else {
				targetVal.SetInt(0)
			}
			return nil
		}

		if isString(sourceKind) {
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
			return nil
		}
	}

	if sourceType.PkgPath() == "encoding/json" && sourceType.Name() == "Number" {
		jn := sourceVal.Interface().(json.Number)
		i, err := jn.Int64()
		if err != nil {
			return fmt.Errorf(
				"error parsing json.Number into %s: %s", targetKey.String(), err)
		}
		targetVal.SetInt(i)
		return nil
	}

	return fmt.Errorf(
		"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
		targetKey.String(),
		targetVal.Type(),
		sourceVal.Type(),
		sourceVal.Interface(),
	)
}

func (a *assigner) assignUint(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, _ metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()
	sourceType := sourceVal.Type()

	if isInt(sourceKind) {
		i := sourceVal.Int()
		if i < 0 && !a.config.WeaklyTypedInput {
			return fmt.Errorf("cannot parse '%s', %d overflows uint",
				targetKey.String(), i)
		}
		targetVal.SetUint(uint64(i))
		return nil
	}

	if isUint(sourceKind) {
		targetVal.SetUint(sourceVal.Uint())
		return nil
	}

	if isFloat(sourceKind) {
		f := sourceVal.Float()
		if f < 0 && !a.config.WeaklyTypedInput {
			return fmt.Errorf("cannot parse '%s', %f overflows uint",
				targetKey.String(), f)
		}
		targetVal.SetUint(uint64(f))
		return nil
	}

	if a.config.WeaklyTypedInput {
		if isBool(sourceKind) {
			if sourceVal.Bool() {
				targetVal.SetUint(1)
			} else {
				targetVal.SetUint(0)
			}
			return nil
		}

		if isString(sourceKind) {
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
			return nil
		}
	}

	if isJsonNumber(sourceType) {
		jn, ok := sourceVal.Interface().(json.Number)
		if !ok {
			return fmt.Errorf("expected json.Number, got different type for '%s'", targetKey.String())
		}
		i, err := strconv.ParseUint(string(jn), 0, 64)
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.String(), err)
		}
		targetVal.SetUint(i)
		return nil
	}

	return fmt.Errorf(
		"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
		targetKey.String(),
		targetVal.Type(),
		sourceVal.Type(),
		sourceVal.Interface(),
	)
}

func (a *assigner) assignBool(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()

	if isBool(sourceKind) {
		targetVal.SetBool(sourceVal.Bool())
		return nil
	}

	if a.config.WeaklyTypedInput {
		if isInt(sourceKind) {
			targetVal.SetBool(sourceVal.Int() != 0)
			return nil
		}

		if isUint(sourceKind) {
			targetVal.SetBool(sourceVal.Uint() != 0)
			return nil
		}

		if isFloat(sourceKind) {
			targetVal.SetBool(sourceVal.Float() != 0)
			return nil
		}

		if isString(sourceKind) {
			b, err := strconv.ParseBool(sourceVal.String())
			if err == nil {
				targetVal.SetBool(b)
			} else if sourceVal.String() == "" {
				targetVal.SetBool(false)
			} else {
				return fmt.Errorf("cannot parse '%s' as bool: %s", sourceKey.String(), err)
			}
			return nil
		}
	}

	return fmt.Errorf(
		"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
		targetKey.String(),
		targetVal.Type(),
		sourceVal.Type(),
		sourceVal.Interface(),
	)
}

func (a *assigner) assignFloat(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, _ metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()
	sourceType := sourceVal.Type()

	if isInt(sourceKind) {
		targetVal.SetFloat(float64(sourceVal.Int()))
		return nil
	}

	if isUint(sourceKind) {
		targetVal.SetFloat(float64(sourceVal.Uint()))
		return nil
	}

	if isFloat(sourceKind) {
		f := sourceVal.Float()
		if err := a.checkNaNAndInf(targetKey, f); err != nil {
			if a.config.WeaklyTypedInput {
				targetVal.SetFloat(0)
			} else {
				return err
			}
		} else {
			targetVal.SetFloat(f)
		}
		return nil
	}

	if a.config.WeaklyTypedInput {
		if isBool(sourceKind) {
			if sourceVal.Bool() {
				targetVal.SetFloat(1)
			} else {
				targetVal.SetFloat(0)
			}
			return nil
		}

		if isString(sourceKind) {
			str := sourceVal.String()
			if str == "" {
				str = "0"
			}

			f, err := strconv.ParseFloat(str, targetVal.Type().Bits())
			if err != nil {
				return fmt.Errorf("cannot parse '%s' as float: %s", targetKey.String(), err)
			}

			return a.setFloatValue(targetVal, targetKey, f)
		}
	}

	if isJsonNumber(sourceType) {
		// We need to get the interface to type assert to json.Number
		sourceInterface := sourceVal.Interface()
		jn, ok := sourceInterface.(json.Number)
		if !ok {
			return fmt.Errorf("error decoding json.Number into %s: type assertion failed", targetKey.String())
		}
		i, err := jn.Float64()
		if err != nil {
			return fmt.Errorf(
				"error decoding json.Number into %s: %s", targetKey.String(), err)
		}
		return a.setFloatValue(targetVal, targetKey, i)
	}

	return fmt.Errorf(
		"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
		targetKey.String(),
		targetVal.Type(),
		sourceVal.Type(),
		sourceVal.Interface(),
	)
}

// setFloatValue sets the float value after checking for NaN and Inf
func (a *assigner) setFloatValue(targetVal reflect.Value, key metaKey, f float64) error {
	if err := a.checkNaNAndInf(key, f); err != nil {
		if a.config.WeaklyTypedInput {
			targetVal.SetFloat(0)
			return nil
		}
		return err
	}
	targetVal.SetFloat(f)
	return nil
}

// checkNaNAndInf checks if a float value is NaN or Infinity and returns appropriate error if needed
func (a *assigner) checkNaNAndInf(key metaKey, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("error decoding '%s': NaN or Inf values are not allowed", key.String())
	}
	return nil
}

func (a *assigner) assignMap(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)

	// Handle nil case explicitly
	if !sourceVal.IsValid() {
		return fmt.Errorf("'%s' expected a map, got nil", targetKey.String())
	}

	sourceKind := sourceVal.Kind()

	if isMap(sourceKind) {
		return a.assignMapFromMap(targetVal, targetKey, sourceVal, sourceKey)
	}

	if isStruct(sourceKind) {
		return a.assignMapFromStruct(targetVal, targetKey, sourceVal, sourceKey)
	}

	if a.config.WeaklyTypedInput && isArraySlice(sourceKind) {
		return a.assignMapFromSlice(targetVal, targetKey, sourceVal, sourceKey)
	}

	return fmt.Errorf("'%s' expected a map, got '%s'", targetKey.String(), sourceVal.Kind())
}

func (a *assigner) assignMapFromSlice(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	if sourceVal.IsNil() {
		return nil
	}

	targetMapType := targetVal.Type()
	targetKeyType := targetMapType.Key()
	targetElemType := targetMapType.Elem()

	if sourceVal.Len() == 0 {
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
			srcElem,
			sourceKey.newChild(reflect.Slice, k),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *assigner) assignMapFromMap(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	targetValType := targetVal.Type()
	targetValKeyType := targetValType.Key()
	targetValElemType := targetValType.Elem()

	if sourceVal.IsNil() {
		return nil
	}

	// Accumulate errors
	errors := make([]string, 0)

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
		if err := weakAssigner.assign(currentKey, "", srcKey, ""); err != nil {
			errors = appendErrors(errors, err)
			continue
		}

		// Next decode the data into the proper type
		if err := a.assign(targetElem, childTargetKey, sourceElem, childSourceKey); err != nil {
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

func (a *assigner) assignMapFromStruct(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
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
		if err := weakAssigner.assign(keyVal, "", srcField.ActualNameVal(), ""); err != nil {
			return fmt.Errorf("error converting map key '%s': %w", srcField.actualName, err)
		}

		srcFieldKind := srcField.fieldVal.Kind()

		if isStruct(srcFieldKind) { // this is an embedded struct, so handle it differently
			sourceFieldType := srcField.fieldVal.Type()
			// Check if struct can be directly assigned to map element
			if sourceFieldType.AssignableTo(targetElemType) {
				targetVal.SetMapIndex(keyVal, srcField.fieldVal)
				a.addMetaKey(targetFieldKey)
				continue
			}

			// Create a new map for nested struct
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

			continue
		}

		if srcField.omitempty && isEmptyValue(srcField.fieldVal) {
			a.addMetaUnused(sourceFieldKey)
			continue
		}

		targetVal.SetMapIndex(keyVal, srcField.fieldVal)
		a.addMetaKey(targetFieldKey)
	}

	return nil
}

func (a *assigner) assignPtr(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) (bool, error) {
	// If the input data is nil, then we want to just set the output
	// pointer to be nil as well.
	if isPtrAble(sourceVal.Kind()) {
		isNil := sourceVal.IsNil()
		if !isNil {
			switch v := reflect.Indirect(sourceVal); v.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
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

		if err := a.assign(reflect.Indirect(realVal), targetKey, sourceVal, sourceKey); err != nil {
			return false, err
		}

		targetVal.Set(realVal)
	} else {
		if err := a.assign(reflect.Indirect(targetVal), targetKey, sourceVal, sourceKey); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (a *assigner) assignFunc(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, _ metaKey) error {
	// Create an element of the concrete (non pointer) type and decode
	// into that. Then set the value of the pointer to this type.
	sourceVal = reflect.Indirect(sourceVal)
	if targetVal.Type() != sourceVal.Type() {
		return fmt.Errorf(
			"'%s' expected type '%s', got unconvertible type '%s', value: '%v'",
			targetKey.String(),
			targetVal.Type(),
			sourceVal.Type(),
			sourceVal.Interface(),
		)
	}
	targetVal.Set(sourceVal)
	return nil
}

func (a *assigner) assignSlice(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()

	targetValType := targetVal.Type()
	targetValElemType := targetValType.Elem()
	sliceType := reflect.SliceOf(targetValElemType)

	// If we have a non array/slice type then we first attempt to convert.
	if !isArraySlice(sourceKind) {
		if !a.config.WeaklyTypedInput {
			return fmt.Errorf(
				"'%s': source data must be an array or slice, got %s",
				targetKey.String(),
				sourceKind,
			)
		}

		switch {
		// Slice and array we use the normal logic
		case sourceKind == reflect.Slice, sourceKind == reflect.Array:
			break

		// Empty maps turn into empty slices
		case sourceKind == reflect.Map:
			if sourceVal.Len() == 0 {
				targetVal.Set(reflect.MakeSlice(sliceType, 0, 0))
				a.addMetaKey(targetKey)
				return nil
			}
			// Create slice of maps of other sizes
			return a.assignSlice(targetVal, targetKey, a.wrapSlice(sourceVal), sourceKey)

		case sourceKind == reflect.String && targetValElemType.Kind() == reflect.Uint8:
			// Convert sourceVal from type string to type []byte
			return a.assignSlice(targetVal, targetKey, reflect.ValueOf([]byte(sourceVal.String())), sourceKey)

		// All other types we try to convert to the slice type
		// and "lift" it into it. i.e. a string becomes a string slice.
		default:
			// Just re-try this function with data as a slice.
			return a.assignSlice(targetVal, targetKey, a.wrapSlice(sourceVal), sourceKey)
		}
	}

	// If the input value is nil, then don't allocate since empty != nil
	if sourceKind != reflect.Array && sourceVal.IsNil() {
		return nil
	}

	// Make a new slice to hold our result, same size as the original data.
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

		// Ensure target slice has enough capacity
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

		if err := a.assign(targetField, targetFieldKey, sourceElem, sourceFieldKey); err != nil {
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

func (a *assigner) wrapSlice(val reflect.Value) reflect.Value {
	valType := val.Type()
	sliceType := reflect.SliceOf(valType)
	sliceValue := reflect.MakeSlice(sliceType, 1, 1)
	sliceValue.Index(0).Set(val)
	return sliceValue
}

func (a *assigner) assignArray(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()
	targetValType := targetVal.Type()
	targetValElemType := targetValType.Elem()
	arrayType := reflect.ArrayOf(targetValType.Len(), targetValElemType)

	valArray := targetVal

	if valArray.Interface() == reflect.Zero(valArray.Type()).Interface() {
		// Check input type
		if sourceKind != reflect.Array && sourceKind != reflect.Slice {
			if a.config.WeaklyTypedInput {
				switch {
				// Empty maps turn into empty arrays
				case sourceKind == reflect.Map:
					if sourceVal.Len() == 0 {
						targetVal.Set(reflect.Zero(arrayType))
						a.addMetaKey(targetKey)
						return nil
					}

				// All other types we try to convert to the array type
				// and "lift" it into it. i.e. a string becomes a string array.
				default:
					newSlice := reflect.MakeSlice(reflect.SliceOf(sourceVal.Type()), 1, 1)
					newSlice.Index(0).Set(sourceVal)
					// Just re-try this function with source as a slice.
					return a.assignArray(targetVal, targetKey, newSlice, sourceKey)
				}
			}

			return fmt.Errorf(
				"'%s': source data must be an array or slice, got %s", targetKey.String(), sourceKind)

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
		if err := a.assign(targetField, targetFieldKey, sourceElem, sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}
	}

	// Initialize remaining elements to zero values if source is shorter than target array
	if sourceVal.Len() < arrayType.Len() {
		zeroVal := reflect.Zero(targetValElemType)
		for i := sourceVal.Len(); i < arrayType.Len(); i++ {
			valArray.Index(i).Set(zeroVal)
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

func (a *assigner) assignStruct(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {

	sourceVal = reflect.Indirect(sourceVal)
	sourceKind := sourceVal.Kind()

	switch sourceKind {
	case reflect.Map:
		return a.assignStructFromMap(targetVal, targetKey, sourceVal, sourceKey)
	case reflect.Struct:
		return a.assignStructFromStruct(targetVal, targetKey, sourceVal, sourceKey)
	}
	return fmt.Errorf("'%s' expected a map, got '%s'", targetKey.String(), sourceKind)
}

type fieldInfo struct {
	field          reflect.StructField
	fieldVal       reflect.Value
	displayName    string
	displayNameVal reflect.Value
	actualName     string
	actualNameVal  reflect.Value
	omitempty      bool
}

func (info *fieldInfo) DisplayNameVal() reflect.Value {
	if !info.displayNameVal.IsValid() {
		info.displayNameVal = reflect.ValueOf(info.displayName)
	}
	return info.displayNameVal
}

func (info *fieldInfo) ActualNameVal() reflect.Value {
	if !info.actualNameVal.IsValid() {
		info.actualNameVal = reflect.ValueOf(info.actualName)
	}
	return info.actualNameVal
}

func (a *assigner) flattenStruct(val reflect.Value) map[string]fieldInfo {

	// This slice will keep track of all the structs we'll be decoding.
	// There can be more than one struct if there are embedded structs
	// that are squashed.
	structs := make([]reflect.Value, 1, 5)
	structs[0] = val

	// Estimate capacity to improve performance
	fields := make(map[string]fieldInfo, val.NumField())

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

			// Only check IsZero if omitempty is true to avoid unnecessary expensive operations
			if omitempty && isZeroValue(fieldVal) {
				continue
			}

			if field.Anonymous { // Field is an embedded type
				if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct { // Field is an embedded pointer to struct

					if fieldVal.IsNil() && fieldVal.CanSet() {
						fieldVal.Set(reflect.New(field.Type.Elem())) // Initialize fieldVal
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

			// Check if field already exists to avoid overwriting
			if _, exist := fields[field.Name]; exist {
				// Already exists, ignore embed struct's field name
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

// isZeroValue is a more efficient version of reflect.Value.IsZero
// It avoids the expensive IsZero call for common types
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Array:
		// For arrays, we need to check each element
		for i := 0; i < v.Len(); i++ {
			if !isZeroValue(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	case reflect.String:
		return v.Len() == 0
	case reflect.Struct:
		// For structs, we need to check each field
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		// Fall back to the standard IsZero for any other types
		return v.IsZero()
	}
}

func (a *assigner) assignStructFromMap(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
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

	// Pre-create mapKey value for performance optimization
	mapKey := reflect.New(sourceTypeKey).Elem()

	errors := make([]string, 0)
	for _, targetField := range targetFields {

		if err := weakAssigner.assign(mapKey, "", targetField.ActualNameVal(), ""); err != nil {
			errors = appendErrors(errors, err)
			continue
		}

		targetFieldKey := targetKey.newChild(reflect.Struct, targetField.displayName)

		value := sourceVal.MapIndex(mapKey)
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

		// Remove processed key
		delete(unusedMapKeys, targetField.actualName)

		if err := a.assign(targetField.fieldVal, targetFieldKey, value, sourceFieldKey); err != nil {
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

func (a *assigner) assignStructFromStruct(targetVal reflect.Value, targetKey metaKey, sourceVal reflect.Value, sourceKey metaKey) error {
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

		// Remove processed key
		delete(sourceFields, tfieldName)

		if err := a.assign(targetField.fieldVal, targetFieldKey, sourceField.fieldVal, sourceFieldKey); err != nil {
			errors = appendErrors(errors, err)
		}
	}

	for displayName := range sourceFields {
		a.addMetaUnused(sourceKey.newChild(reflect.Struct, displayName))
	}

	if len(errors) > 0 {
		return &Error{errors}
	}

	return nil
}

func (a *assigner) shouldSkipKey(targetKey, sourceKey metaKey) bool {
	// Skip empty keys as they should never be skipped
	if targetKey == "" || sourceKey == "" {
		return false
	}

	// Check if target key should be skipped based on config
	if _, exist := a.skipKeysCache[string(targetKey)]; exist {
		return true
	}

	// Check if source key should be skipped based on config
	if _, exist := a.skipKeysCache[string(sourceKey)]; exist {
		return true
	}

	return false
}

func (a *assigner) addMetaKey(targetKey metaKey) {
	// Return early if metadata is not configured
	if a.config.Metadata == nil {
		return
	}

	// Skip empty keys
	if targetKey.IsEmpty() {
		return
	}

	// Append the key to metadata keys list
	a.config.Metadata.Keys = append(a.config.Metadata.Keys, string(targetKey))
}

func (a *assigner) addMetaUnused(sourceKey metaKey) {
	if a.config.Metadata == nil {
		return
	}

	if sourceKey.IsEmpty() {
		return
	}

	a.config.Metadata.Unused = append(a.config.Metadata.Unused, string(sourceKey))
}

func (a *assigner) addMetaUnset(targetKey metaKey) {
	if a.config.Metadata == nil {
		return
	}

	if targetKey.IsEmpty() {
		return
	}

	a.config.Metadata.Unset = append(a.config.Metadata.Unset, string(targetKey))
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
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
	// For other unhandled types (like struct, complex numbers, etc.), not considered empty by default
	return false
}

func (a *assigner) parseTag(field reflect.StructField) (actualName string, omitempty, skip bool) {
	tagValue := field.Tag.Get(a.config.TagName)
	// Determine the name of the key in the map
	pieces := strings.Split(tagValue, ",")

	displayName := field.Name

	if len(pieces) == 0 || pieces[0] == "" {
		actualName = a.config.Converter(displayName)
	} else if pieces[0] == "-" {
		if a.config.IncludeIgnoreFields {
			actualName = a.config.Converter(displayName)
		} else {
			skip = true
		}
	} else {
		actualName = pieces[0]
	}

	for _, piece := range pieces {
		if piece == "omitempty" {
			omitempty = true
		}
	}

	return
}

type metaKey string

func (k metaKey) String() string {
	return string(k)
}

func (k metaKey) IsEmpty() bool {
	return k == ""
}

func (k metaKey) newChild(parentKind reflect.Kind, fieldName string) metaKey {
	n := genFullKey(parentKind, string(k), fieldName)
	return metaKey(n)
}

func genFullKey(parentKind reflect.Kind, parentFull, keyName string) string {
	// When parentKind is 0 (reflect.Invalid), directly return keyName
	if parentKind == 0 {
		return keyName
	}

	// If parentFull is empty, directly return keyName (regardless of whether keyName is empty)
	if parentFull == "" {
		return keyName
	}

	// If keyName is empty, return parentFull
	if keyName == "" {
		return parentFull
	}

	// Determine connector based on parentKind type
	switch parentKind {
	case reflect.Map, reflect.Array, reflect.Slice:
		// For simple concatenation like this, direct string concatenation is actually faster
		// than using strings.Builder due to compiler optimizations
		return parentFull + "[" + keyName + "]"
	default:
		// For simple concatenation like this, direct string concatenation is actually faster
		// than using strings.Builder due to compiler optimizations
		return parentFull + "." + keyName
	}
}

func isUint(kind reflect.Kind) bool {
	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isInt(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func isFloat(kind reflect.Kind) bool {
	switch kind {
	case reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isBool(kind reflect.Kind) bool {
	return kind == reflect.Bool
}

func isString(kind reflect.Kind) bool {
	return kind == reflect.String
}

func isMap(kind reflect.Kind) bool {
	return kind == reflect.Map
}

func isStruct(kind reflect.Kind) bool {
	return kind == reflect.Struct
}

func isArraySlice(kind reflect.Kind) bool {
	return kind == reflect.Array || kind == reflect.Slice
}

func isJsonNumber(typ reflect.Type) bool {
	return typ.PkgPath() == "encoding/json" && typ.Name() == "Number"
}

func isPtrAble(kind reflect.Kind) bool {
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return true
	default:
		return false
	}
}
