package sqlparams

import (
	"reflect"
	"slices"
	"strings"
)

type SqlParams []any

// SqlParams creates a new instance of SqlParams
func New(params ...any) SqlParams {
	return append(SqlParams{}, params...)
}

// AddParam adds a new parameter to SqlParams and returns a "?" string for use in queries
func (p *SqlParams) AddParam(param any) string {
	*p = append(*p, param)
	return "?"
}

// annoying workaround for mysql driver.
// can't pass a slice to IN(?), must be IN(?,?..n)
// AddParams adds multiple parameters to SqlParams, returning a string of "?,?,?...n" based on the number of arguments passed
// it expands slices to correctly pass each entry into an IN() statement.
func (p *SqlParams) AddParams(params ...any) string {
	var paramCount int
	for _, param := range params {
		switch v := reflect.ValueOf(param); v.Kind() {
		case reflect.Slice:
			if v.Len() == 0 {
				continue
			}
			paramCount += v.Len()
			*p = slices.Grow(*p, v.Len())

			switch v.Index(0).Kind() {
			case reflect.String:
				for i := range v.Len() {
					*p = append(*p, v.Index(i).Interface().(string))
				}
			default:
				panic("Unhandled slice type")
			}
		default:
			paramCount++
			*p = append(*p, param)
		}
	}
	return "?" + strings.Repeat(",?", paramCount-1)
}
