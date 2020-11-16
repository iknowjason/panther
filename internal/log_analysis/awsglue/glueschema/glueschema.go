package glueschema

/**
 * Panther is a Cloud-Native SIEM for the Modern Security Team.
 * Copyright (C) 2020 Panther Labs Inc
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/structtag"
	"github.com/pkg/errors"

	"github.com/panther-labs/panther/pkg/stringset"
)

// Type is a string representing a type in Glue schema
type Type string

// Scalar types
const (
	TypeString    Type = "string"
	TypeBool      Type = "boolean"
	TypeTimestamp Type = "timestamp"
	TypeTinyInt   Type = "tinyint"
	TypeSmallInt  Type = "smallint"
	TypeInt       Type = "int"
	TypeBigInt    Type = "bigint"
	TypeDouble    Type = "double"
	TypeFloat     Type = "float"
)

// MaxCommentLength is the maximum size of column comments allowed by Glue
const MaxCommentLength = 255

// TruncateComments is a flag used to allow mage to disable comment truncation during mage:doc
var TruncateComments = true

func (t Type) String() string {
	return string(t)
}

func ArrayOf(typ Type) Type {
	return "array<" + typ + ">"
}

func MapOf(key, typ Type) Type {
	return "map<" + key + "," + typ + ">"
}

func StructOf(cols []Column) Type {
	typ := strings.Builder{}
	typ.WriteString("struct<")
	for i, col := range cols {
		if i > 0 {
			typ.WriteByte(',')
		}
		typ.WriteString(col.Name)
		typ.WriteByte(':')
		typ.WriteString(string(col.Type))
	}
	typ.WriteString(">")
	return Type(typ.String())
}

func InferColumns(schema interface{}) ([]Column, error) {
	if schema == nil {
		return nil, errors.New("nil schema value")
	}
	return inferTypeColumns(reflect.TypeOf(schema), nil)
}

func InferColumnsWithMappings(schema interface{}) ([]Column, map[string]string, error) {
	if schema == nil {
		return nil, nil, errors.New("nil schema value")
	}
	names := collisions{}
	columns, err := inferTypeColumns(reflect.TypeOf(schema), names)
	if err != nil {
		return nil, nil, err
	}
	mappings := names.caseSensitiveMappings()
	return columns, mappings, nil
}

func inferTypeColumns(typ reflect.Type, names map[string][]string) ([]Column, error) {
	cols, err := inferStructColumns(typ, nil, names)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return cols, nil
}

func inferStructColumns(typ reflect.Type, path []string, names collisions) ([]Column, error) {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, errors.Errorf("non struct type %v at %q", typ, strings.Join(path, ","))
	}
	fields := appendStructFieldsJSON(nil, typ)
	columns := make([]Column, 0, len(fields))
	uniqueNames := make(map[string]string, len(fields))
	for i := range fields {
		field := &fields[i]
		fieldPath := append(path, field.Name)
		col := Column{}
		if err := inferColumn(&col, field, fieldPath, names); err != nil {
			return nil, err
		}
		if col.Name == "" {
			continue
		}
		if _, duplicate := uniqueNames[col.Name]; duplicate {
			return nil, newSchemaError(path, "duplicate column name")
		}
		uniqueNames[col.Name] = col.Name
		columns = append(columns, col)
	}
	return columns, nil
}

func appendStructFieldsJSON(fields []reflect.StructField, typ reflect.Type) []reflect.StructField {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return fields
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			// Possible embedded struct, extend the fields
			fields = appendStructFieldsJSON(fields, field.Type)
			continue
		}
		// Unexported field
		if field.PkgPath != "" {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func inferColumn(col *Column, field *reflect.StructField, path []string, names collisions) error {
	colName, err := fieldColumnName(field)
	if err != nil {
		// We do not want stack in a recursive function
		return errors.WithMessagef(err, "failed to infer column name at %q", strings.Join(path, "."))
	}
	if colName == "" {
		*col = Column{}
		return nil
	}

	// We register a unique field name
	names.observeColumnName(colName)

	colType, err := inferColumnType(field.Type, path, names)
	if err != nil {
		// We do not want stack in a recursive function
		return errors.WithMessagef(err, "failed to infer column type at %q", strings.Join(path, "."))
	}
	*col = Column{
		Name:     colName,
		Type:     colType,
		Required: isFieldRequired(field),
		Comment:  fieldComment(field),
	}
	return nil
}

func fieldComment(field *reflect.StructField) string {
	comment := field.Tag.Get("description")
	comment = strings.TrimSpace(comment)
	if TruncateComments {
		return comment
	}
	if len(comment) < MaxCommentLength {
		return comment
	}
	return comment[:MaxCommentLength-3] + "..."
}

func fieldColumnName(field *reflect.StructField) (string, error) {
	tags, err := structtag.Parse(string(field.Tag))
	if err != nil {
		return "", err
	}
	name := field.Name
	if jsonTag, err := tags.Get("json"); err == nil {
		if jsonTag.Name == "-" {
			return "", nil
		}
		name = jsonTag.Name
	}
	return ColumnName(name), nil
}

// isFieldRequired checks whether a field is required or not.
func isFieldRequired(field *reflect.StructField) bool {
	tag, hasValidate := field.Tag.Lookup("validate")
	if !hasValidate {
		// If the field does not have a 'validate' tag, it is not required
		return false
	}
	// Otherwise, if the 'validate' tag contains 'omitempty' it is not a required field.
	return !strings.Contains(tag, "omitempty")
}

func inferColumnType(typ reflect.Type, path []string, names collisions) (Type, error) {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if custom, ok := defaultMappings[typ]; ok {
		return custom, nil
	}
	switch typ.Kind() {
	case reflect.Struct:
		cols, err := inferStructColumns(typ, path, names)
		if err != nil {
			return "", err
		}
		return StructOf(cols), nil
	case reflect.Slice:
		// Distinguish array element in path
		path = append(path, "[]")
		elem, err := inferColumnType(typ.Elem(), path, names)
		if err != nil {
			return "", err
		}
		return ArrayOf(elem), nil
	case reflect.Map:
		key, err := inferColumnType(typ.Key(), path, nil)
		if err != nil {
			return "", err
		}
		// Make sure we fail if map key is something exotic
		if key != TypeString {
			// We do not want stack for errors
			return "", newSchemaError(path, "invalid map key %q", key)
		}
		// Distinguish map values in path
		path = append(path, "[]")
		val, err := inferColumnType(typ.Elem(), path, names)
		if err != nil {
			return "", err
		}
		return MapOf(key, val), nil
	default:
		if glueType := inferScalarType(typ); glueType != "" {
			return glueType, nil
		}
		return "", newSchemaError(path, "unsupported type %s", typ)
	}
}

func inferScalarType(typ reflect.Type) Type {
	switch kind := typ.Kind(); kind {
	case reflect.String:
		return TypeString
	case reflect.Bool:
		return TypeBool
	case reflect.Float64:
		return TypeDouble
	case reflect.Float32:
		return TypeFloat
	case reflect.Int:
		return TypeBigInt
	case reflect.Int8:
		return TypeTinyInt
	case reflect.Int16:
		return TypeSmallInt
	case reflect.Int32:
		return TypeInt
	case reflect.Int64:
		return TypeBigInt
	case reflect.Uint:
		return TypeBigInt
	case reflect.Uint8:
		return TypeSmallInt
	case reflect.Uint16:
		return TypeInt
	case reflect.Uint32:
		return TypeBigInt
	case reflect.Uint64:
		return TypeBigInt
	default:
		return ""
	}
}

// newSchemaError formats errors without a stack trace for use in recursive functions.
//
// Normally we use errors.Errorf from the 'github.com/pkg/errors' module.
// Those errors include a stack trace. When used in recursive functions, those stack traces
// can be very long and provide no useful info about the error.
// We make sure to always include the path to the error so we relay that information
func newSchemaError(path []string, format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	return errors.WithMessagef(err, "schema error at %q", strings.Join(path, ""))
}

// collisions observe column names across a schema and provide case sensitive mappings
type collisions map[string][]string

// observeColumnName adds name to the collisions
func (c collisions) observeColumnName(name string) {
	if c == nil {
		return
	}
	key := strings.ToLower(name)
	// append distinct names
	c[key] = stringset.Append(c[key], name)
}

func (c collisions) caseSensitiveMappings() map[string]string {
	out := make(map[string]string, len(c))
	for caseInsensitiveName, caseSensitiveNames := range c {
		sort.Strings(caseSensitiveNames)
		for i, caseSensitiveName := range caseSensitiveNames {
			var mapping string
			// We need to make sure adding a suffix won't conflict with existing key names
			for suffix := i; ; suffix++ {
				mapping = caseInsensitiveName
				if suffix > 0 {
					mapping += strconv.Itoa(suffix)
				}
				_, exists := out[mapping]
				if !exists {
					break
				}
			}
			out[mapping] = caseSensitiveName
		}
	}
	return out
}
