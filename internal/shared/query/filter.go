package query

// Operator type
type Operator string

const (
	OpEq      Operator = "="
	OpNe      Operator = "!="
	OpGt      Operator = ">"
	OpLt      Operator = "<"
	OpLike    Operator = "LIKE"
	OpIn      Operator = "IN"
	OpBetween Operator = "BETWEEN"
)

// Filter struct (support nested group + AND/OR)
type Filter struct {
	Field     string
	Op        Operator
	Value     interface{} // untuk BETWEEN: [2]value {start, end}
	Connector string      // AND / OR
	Group     []Filter    // nested group
}
