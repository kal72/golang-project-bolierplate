# query filter utility
membantu membangun query where secara dinamis. filter dibangun didalam bisnis logic (usecase) sebagai parameter untuk repository
## helper version
```go
filters := []query.Filter{
	query.Gt("age", 18),
	query.AndGroup(
		query.Eq("status", "single"),
		query.Or(query.Eq("status", "married")),
	),
	query.Between("created_at", "2025-01-01", "2025-01-31"),
}

db.Scopes(query.ScopeFilters(filters)).Find(&users)

```
```sql
-- output sql
SELECT * FROM users
WHERE age > 18 
  AND (status = 'single' OR status = 'married') 
  AND created_at BETWEEN '2025-01-01' AND '2025-01-31'
```
## builder version
```go
qb := query.NewBuilder().
	AndGroup(func(g *query.Builder) {
		g.Gt("age", 18).Lt("age", 30)
	}).
	OrGroup(func(g *query.Builder) {
		g.Eq("status", "single").OrEq("status", "married")
	}).
	Like("name", "ali")

db.Scopes(qb.Scope()).Find(&users)

```
```sql
-- output sql
SELECT * FROM users
WHERE (age > 18 AND age < 30)
   OR (status = 'single' OR status = 'married')
   AND name LIKE '%ali%'

```