package database

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Update updates a row in the given table,
// setting columns based on the exported fields of `data`.
// If `columns` is provided, only those fields are used.
// If `columns` is empty, *all* exported fields of `data` will be included.
// The condition is `WHERE whereCol = ?`.
func Update(
	table string,
	whereCol string,
	whereVal any,
	data any,
	columns ...string,
) error {
	// 1) Make sure `data` is a struct or pointer to a struct.
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return errors.New("data must be a struct or pointer to a struct")
	}

	// 2) Figure out which fields to use.
	fieldsToUse, args, setClause, shouldReturn, err := clauseBuilder(columns, v)
	if shouldReturn {
		return err
	}

	// The WHERE placeholder is after all SET placeholders
	wherePlaceholderIndex := len(fieldsToUse) + 1

	// 4) Construct final query
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		table,
		setClause,
		whereCol,
		wherePlaceholderIndex,
	)

	// 5) Add the WHERE argument
	args = append(args, whereVal)

	// 6) Execute
	_, err = Pg.Exec(query, args...)
	if err != nil {
		return err
	}
	return nil
}

func clauseBuilder(columns []string, v reflect.Value) ([]string, []any, string, bool, error) {
	var fieldsToUse []string
	var useAllFields bool = len(columns) == 0
	if useAllFields {
		// We'll use all exported struct fields
		fieldsToUse = exportedFieldNames(v)
	} else {
		// We'll only use the user-specified columns
		fieldsToUse = columns
	}

	if len(fieldsToUse) == 0 {
		return nil, nil, "", true, errors.New("no fields/columns to update")
	}

	// 3) Build the SET clause and collect argument values.
	setParts := make([]string, len(fieldsToUse))
	args := make([]any, 0, len(fieldsToUse)+1)

	for i, fieldName := range fieldsToUse {
		// reflect the field by name
		fieldVal := v.FieldByName(fieldName)
		if !fieldVal.IsValid() {
			return nil, nil, "", true, fmt.Errorf("struct is missing field %q", fieldName)
		}

		// Add "col = $X" piece (Postgres placeholders).
		// e.g.: "FirstName = $1"
		setParts[i] = fmt.Sprintf("%s = $%d", fieldName, i+1)
		args = append(args, fieldVal.Interface())
	}

	setClause := strings.Join(setParts, ", ")
	return fieldsToUse, args, setClause, false, nil
}

// exportedFieldNames returns the names of all exported fields in the struct value v.
func exportedFieldNames(v reflect.Value) []string {
	t := v.Type()
	fields := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		// Only include exported fields (which begin with uppercase letter).
		if field.IsExported() {
			fields = append(fields, field.Name)
		}
	}
	return fields
}
