package influxquerybuilder

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// QueryBuilder QueryBuilder interface
type QueryBuilder interface {
	Select(fields ...string) QueryBuilder
	From(string) QueryBuilder
	Where(string, string, interface{}) QueryBuilder
	And(string, string, interface{}) QueryBuilder
	Or(string, string, interface{}) QueryBuilder
	WhereBrackets(QueryBuilder) QueryBuilder
	AndBrackets(QueryBuilder) QueryBuilder
	OrBrackets(QueryBuilder) QueryBuilder
	GroupBy(string) QueryBuilder
	Fill(interface{}) QueryBuilder
	Limit(uint) QueryBuilder
	Offset(uint) QueryBuilder
	Desc() QueryBuilder
	Asc() QueryBuilder
	Build() string
	Clean() QueryBuilder
	GetQueryStruct() query
}

type tag struct {
	key   string
	op    string
	value interface{}
}

type query struct {
	measurement string
	fields      []string
	where       tag
	and         []tag
	or          []tag
	groupWhere  QueryBuilder
	groupAnd    []QueryBuilder
	groupOr     []QueryBuilder
	groupBy     string
	order       string
	limit       uint
	_limit      bool
	offset      uint
	_offset     bool
	fill        interface{}
}

// New New QueryBuilder
func New() QueryBuilder {
	return &query{}
}

// Clean Clean current builder and get a new one
func (q *query) Clean() QueryBuilder {
	return New()
}

func (q *query) Select(fields ...string) QueryBuilder {
	q.fields = append(q.fields, fields...)
	return q
}

func (q *query) From(measurement string) QueryBuilder {
	q.measurement = measurement
	return q
}

func (q *query) Where(key string, op string, value interface{}) QueryBuilder {
	q.where = tag{key, op, value}
	return q
}

func (q *query) And(key string, op string, value interface{}) QueryBuilder {
	q.and = append(q.and, tag{key, op, value})
	return q
}

func (q *query) Or(key string, op string, value interface{}) QueryBuilder {
	q.or = append(q.or, tag{key, op, value})
	return q
}

func (q *query) WhereBrackets(builder QueryBuilder) QueryBuilder {
	q.groupWhere = builder
	return q
}

func (q *query) AndBrackets(builder QueryBuilder) QueryBuilder {
	q.groupAnd = append(q.groupAnd, builder)
	return q
}

func (q *query) OrBrackets(builder QueryBuilder) QueryBuilder {
	q.groupOr = append(q.groupOr, builder)
	return q
}

func (q *query) GroupBy(time string) QueryBuilder {
	q.groupBy = time
	return q
}

func (q *query) Fill(fill interface{}) QueryBuilder {
	q.fill = fill
	return q
}

func (q *query) Limit(limit uint) QueryBuilder {
	q._limit = true
	q.limit = limit
	return q
}

func (q *query) Offset(offset uint) QueryBuilder {
	q._offset = true
	q.offset = offset
	return q
}

func (q *query) Desc() QueryBuilder {
	q.order = "DESC"
	return q
}

func (q *query) Asc() QueryBuilder {
	q.order = "ASC"
	return q
}

func (q *query) GetQueryStruct() query {
	return *q
}

func (q *query) Build() string {
	var buffer bytes.Buffer

	buffer.WriteString(q.buildFields())
	buffer.WriteString(q.buildFrom())
	buffer.WriteString(q.buildWhere())
	buffer.WriteString(q.buildGroupBy())
	buffer.WriteString(q.buildFill())
	buffer.WriteString(q.buildOrder())
	buffer.WriteString(q.buildLimit())
	buffer.WriteString(q.buildOffset())

	return strings.TrimSpace(buffer.String())
}

var functionMatcher = regexp.MustCompile(`.+\(.+\)$`)

func (q *query) buildFields() string {
	if q.fields == nil {
		return ""
	}

	tmpl := `"%s"`
	fields := make([]string, len(q.fields))

	for i := range fields {
		if functionMatcher.MatchString(q.fields[i]) {
			fields[i] = q.fields[i]
		} else {
			fields[i] = fmt.Sprintf(tmpl, q.fields[i])
		}
	}

	return fmt.Sprintf("SELECT %s ", strings.Join(fields, ","))
}

func (q *query) buildFrom() string {
	if q.measurement == "" {
		return ""
	}

	return fmt.Sprintf(`FROM "%s" `, q.measurement)
}

func (q *query) buildWhere() string {
	var buffer bytes.Buffer
	var whereCriteria string
	andCriteria := make([]string, 0)
	orCriteria := make([]string, 0)

	if q.where != (tag{}) || q.groupWhere != nil {
		if q.where != (tag{}) {
			buffer.WriteString("WHERE ")
			whereCriteria = getCriteriaTemplate(q.where)
			buffer.WriteString(whereCriteria)
			buffer.WriteString(" ")
		} else if q.groupWhere != nil {
			buffer.WriteString("WHERE (")
			buffer.WriteString(strings.Replace(q.groupWhere.Build(), "WHERE ", "", 1))
			buffer.WriteString(") ")
		}

		if q.and != nil {
			buffer.WriteString("AND ")
			for _, tag := range q.and {
				andCriteria = append(
					andCriteria,
					getCriteriaTemplate(tag),
				)
			}
			buffer.WriteString(strings.Join(andCriteria, " AND "))
			buffer.WriteString(" ")
		}

		if q.or != nil {
			buffer.WriteString("OR ")
			for _, tag := range q.or {
				orCriteria = append(
					orCriteria,
					getCriteriaTemplate(tag),
				)
			}
			buffer.WriteString(strings.Join(orCriteria, " OR "))
			buffer.WriteString(" ")
		}

		if q.groupAnd != nil {
			for _, g := range q.groupAnd {
				buffer.WriteString("AND (")
				buffer.WriteString(strings.Replace(g.Build(), "WHERE ", "", 1))
				buffer.WriteString(") ")
			}
		}

		if q.groupOr != nil {
			for _, g := range q.groupOr {
				buffer.WriteString("OR (")
				buffer.WriteString(strings.Replace(g.Build(), "WHERE ", "", 1))
				buffer.WriteString(") ")
			}
		}
	}

	return buffer.String()
}

func (q *query) buildGroupBy() string {
	var buffer bytes.Buffer

	if q.groupBy != "" {
		buffer.WriteString(
			fmt.Sprintf("GROUP BY time(%s)", q.groupBy),
		)

		buffer.WriteString(" ")
	}

	return buffer.String()
}

func (q *query) buildFill() string {
	var buffer bytes.Buffer

	if q.fill != nil {
		buffer.WriteString(
			fmt.Sprintf(`FILL(%s)`, fmt.Sprint(q.fill)),
		)

		buffer.WriteString(" ")
	}

	return buffer.String()
}

func (q *query) buildOrder() string {
	var buffer bytes.Buffer

	if q.order != "" {
		buffer.WriteString(
			fmt.Sprintf(`ORDER BY time %s`, q.order),
		)

		buffer.WriteString(" ")
	}

	return buffer.String()
}

func (q *query) buildLimit() string {
	var buffer bytes.Buffer

	if q._limit {
		buffer.WriteString(
			fmt.Sprintf(`LIMIT %v`, q.limit),
		)

		buffer.WriteString(" ")
	}

	return buffer.String()
}

func (q *query) buildOffset() string {
	var buffer bytes.Buffer

	if q._offset {
		buffer.WriteString(
			fmt.Sprintf(`OFFSET %v`, q.offset),
		)

		buffer.WriteString(" ")
	}

	return buffer.String()
}

func getCriteriaTemplate(tag tag) string {
	switch tag.value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf(`"%s" %s %d`, tag.key, tag.op, tag.value)
	case float32, float64:
		return fmt.Sprintf(`"%s" %s %g`, tag.key, tag.op, tag.value)
	case bool:
		return fmt.Sprintf(`"%s" %s %t`, tag.key, tag.op, tag.value)
	default:
		return fmt.Sprintf(`"%s" %s '%s'`, tag.key, tag.op, tag.value)
	}
}
