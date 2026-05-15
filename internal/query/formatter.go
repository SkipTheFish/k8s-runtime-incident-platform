package query

type QueryFormatter interface {
	Format(result *QueryResult) (*QueryResult, error)
}

type DefaultFormatter struct{}

func (f *DefaultFormatter) Format(result *QueryResult) (*QueryResult, error) {
	return result, nil
}