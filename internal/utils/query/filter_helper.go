package query

import "gorm.io/gorm"

// ---------------- Helper DSL ----------------

func Eq(field string, value interface{}) Filter { return Filter{Field: field, Op: OpEq, Value: value} }
func Ne(field string, value interface{}) Filter { return Filter{Field: field, Op: OpNe, Value: value} }
func Gt(field string, value interface{}) Filter { return Filter{Field: field, Op: OpGt, Value: value} }
func Lt(field string, value interface{}) Filter { return Filter{Field: field, Op: OpLt, Value: value} }
func Like(field string, value string) Filter {
	return Filter{Field: field, Op: OpLike, Value: "%" + value + "%"}
}
func In(field string, values interface{}) Filter {
	return Filter{Field: field, Op: OpIn, Value: values}
}
func Between(field string, start, end interface{}) Filter {
	return Filter{Field: field, Op: OpBetween, Value: []interface{}{start, end}}
}

// Or(f) → ubah connector jadi OR
func Or(f Filter) Filter {
	f.Connector = "OR"
	return f
}

// AndGroup(filters...) → nested group dengan AND
func AndGroup(filters ...Filter) Filter {
	return Filter{Group: filters}
}

// OrGroup(filters...) → nested group dengan OR
func OrGroup(filters ...Filter) Filter {
	return Filter{Connector: "OR", Group: filters}
}

// ---------------- Scope Recursive ----------------
// digunakan di gorm scopes()
func ScopeFilters(filters []Filter) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, f := range filters {
			// nested group
			if len(f.Group) > 0 {
				if f.Connector == "OR" {
					db = db.Or(func(tx *gorm.DB) *gorm.DB {
						return tx.Scopes(ScopeFilters(f.Group))
					})
				} else {
					db = db.Where(func(tx *gorm.DB) *gorm.DB {
						return tx.Scopes(ScopeFilters(f.Group))
					})
				}
				continue
			}

			// simple clause
			switch f.Op {
			case OpBetween:
				vals, _ := f.Value.([]interface{})
				if f.Connector == "OR" {
					db = db.Or(f.Field+" BETWEEN ? AND ?", vals[0], vals[1])
				} else {
					db = db.Where(f.Field+" BETWEEN ? AND ?", vals[0], vals[1])
				}
			default:
				clause := f.Field + " " + string(f.Op) + " ?"
				if f.Connector == "OR" {
					db = db.Or(clause, f.Value)
				} else {
					db = db.Where(clause, f.Value)
				}
			}
		}
		return db
	}
}
