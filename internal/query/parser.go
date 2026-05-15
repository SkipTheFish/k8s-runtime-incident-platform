package query

type QueryParser interface {
	Parse(q *Query) error
}

type NoopParser struct{}

func (p *NoopParser) Parse(q *Query) error {
	q.AST = nil
	return nil
}