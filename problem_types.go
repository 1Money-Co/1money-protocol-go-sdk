package onemoney

import "strings"

// ProblemTypePrefix starts every problem type URI of the node.
const ProblemTypePrefix = "https://developer.1moneynetwork.com/problem/"

// IsKnownType reports whether Type is in the node's namespace (it starts with
// ProblemTypePrefix), including a type that is newer than this SDK. The match
// is exact and case-sensitive; "about:blank" is not in it.
//
// Deprecated: the SDK keeps no copy of the node's catalog. Problem.Action is
// already empty unless the node sent it, so decide on Action, and on a submit
// use *SubmitError.
func (p Problem) IsKnownType() bool {
	return strings.HasPrefix(p.Type, ProblemTypePrefix)
}
