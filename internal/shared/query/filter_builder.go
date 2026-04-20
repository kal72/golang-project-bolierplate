package query

import "gorm.io/gorm"

type Builder struct {
	filters []Filter
}

// NewBuilder untuk mulai bikin filter
func NewBuilder() *Builder {
	return &Builder{}
}

// ---------------- Basic Condition ----------------

func (b *Builder) Eq(field string, value interface{}) *Builder {
	if value != nil && value != "" {
		b.filters = append(b.filters, Filter{Field: field, Op: OpEq, Value: value})
	}
	return b
}

func (b *Builder) OrEq(field string, value interface{}) *Builder {
	if value != nil && value != "" {
		b.filters = append(b.filters, Filter{Field: field, Op: OpEq, Value: value, Connector: "OR"})
	}
	return b
}

func (b *Builder) Gt(field string, value interface{}) *Builder {
	if value != nil {
		b.filters = append(b.filters, Filter{Field: field, Op: OpGt, Value: value})
	}
	return b
}

func (b *Builder) Lt(field string, value interface{}) *Builder {
	if value != nil {
		b.filters = append(b.filters, Filter{Field: field, Op: OpLt, Value: value})
	}
	return b
}

func (b *Builder) Like(field string, value string) *Builder {
	if value != "" {
		b.filters = append(b.filters, Filter{Field: field, Op: OpLike, Value: "%" + value + "%"})
	}
	return b
}

func (b *Builder) In(field string, values interface{}) *Builder {
	b.filters = append(b.filters, Filter{Field: field, Op: OpIn, Value: values})
	return b
}

func (b *Builder) Between(field string, start, end interface{}) *Builder {
	if start != nil && end != nil {
		b.filters = append(b.filters, Filter{Field: field, Op: OpBetween, Value: []interface{}{start, end}})
	}
	return b
}

// ---------------- Grouping ----------------

func (b *Builder) AndGroup(fn func(*Builder)) *Builder {
	sub := NewBuilder()
	fn(sub)
	if len(sub.filters) > 0 {
		b.filters = append(b.filters, Filter{Group: sub.filters})
	}
	return b
}

func (b *Builder) OrGroup(fn func(*Builder)) *Builder {
	sub := NewBuilder()
	fn(sub)
	if len(sub.filters) > 0 {
		b.filters = append(b.filters, Filter{Connector: "OR", Group: sub.filters})
	}
	return b
}

// ---------------- Build & Scope ----------------

func (b *Builder) Build() []Filter {
	return b.filters
}

func (b *Builder) Scope() func(db *gorm.DB) *gorm.DB {
	return ScopeFilters(b.filters)
}
