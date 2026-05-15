package query

import (
	"context"
	"time"
)

type QueryPipeline struct {
	Validator QueryValidator
	Parser    QueryParser
	Isolator  QueryIsolator
	Cache     QueryCache
	Executor  QueryExecutor
	Formatter QueryFormatter
	TTL       time.Duration
}

func NewQueryPipeline(
	validator QueryValidator,
	parser QueryParser,
	isolator QueryIsolator,
	cache QueryCache,
	executor QueryExecutor,
	formatter QueryFormatter,
) *QueryPipeline {
	return &QueryPipeline{
		Validator: validator,
		Parser:    parser,
		Isolator:  isolator,
		Cache:     cache,
		Executor:  executor,
		Formatter: formatter,
		TTL:       30 * time.Second,
	}
}

func (p *QueryPipeline) Run(ctx context.Context, q *Query) (*QueryResult, error) {
	if err := p.Validator.Validate(q); err != nil {
		return nil, err
	}

	if err := p.Parser.Parse(q); err != nil {
		return nil, err
	}

	if err := p.Isolator.Isolate(ctx, q); err != nil {
		return nil, err
	}

	if cached, ok := p.Cache.Get(ctx, q); ok {
		return cached, nil
	}

	result, err := p.Executor.Execute(ctx, q)
	if err != nil {
		return nil, err
	}

	formatted, err := p.Formatter.Format(result)
	if err != nil {
		return nil, err
	}

	p.Cache.Set(ctx, q, formatted, p.TTL)

	return formatted, nil
}