package resource

import (
	"regexp"

	runtimeevent "observability-platform/internal/runtime/event"
)

var (
	podRegex = regexp.MustCompile(`pod="([^"]+)"`)

	namespaceRegex = regexp.MustCompile(`namespace="([^"]+)"`)

	deploymentRegex = regexp.MustCompile(`deployment="([^"]+)"`)

	nodeRegex = regexp.MustCompile(`node="([^"]+)"`)
)

func InferFromQuery(
	query string,
) runtimeevent.RuntimeResource {

	// Pod
	if match := podRegex.FindStringSubmatch(query); len(match) > 1 {

		res := runtimeevent.RuntimeResource{
			Kind: "Pod",
			Name: match[1],
		}

		if ns := namespaceRegex.FindStringSubmatch(query); len(ns) > 1 {
			res.Namespace = ns[1]
		}

		return res
	}

	// Deployment
	if match := deploymentRegex.FindStringSubmatch(query); len(match) > 1 {

		res := runtimeevent.RuntimeResource{
			Kind: "Deployment",
			Name: match[1],
		}

		if ns := namespaceRegex.FindStringSubmatch(query); len(ns) > 1 {
			res.Namespace = ns[1]
		}

		return res
	}

	// Node
	if match := nodeRegex.FindStringSubmatch(query); len(match) > 1 {

		return runtimeevent.RuntimeResource{
			Kind: "Node",
			Name: match[1],
		}
	}

	return runtimeevent.RuntimeResource{}
}