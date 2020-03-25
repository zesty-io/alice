// Package alice provides a convenient way to chain http handlers.
package alice

import (
	"net/http"
)

// A constructor for a piece of middleware.
// Some middleware use this constructor out of the box,
// so in most cases you can just pass somepackage.New
type Constructor func(http.Handler) http.Handler

// Chain acts as a list of http.Handler constructors.
// Chain is effectively immutable:
// once created, it will always hold
// the same set of constructors in the same order.
type Chain struct {
	constructors []Constructor
	endware      []Endware
}

// New creates a new chain,
// memorizing the given list of middleware constructors.
// New serves no other function,
// constructors are only called upon a call to Then().
func New(constructors ...Constructor) Chain {
	return Chain{append(([]Constructor)(nil), constructors...), nil}
}

// Then chains the middleware and endware and returns the final http.Handler.
//     New(m1, m2, m3).Then(h, e1, e2, e3)
// is equivalent to:
//     m1(m2(m3(h(e1(e2(e3))))))
// When the request comes in, it will be passed to m1, then m2, then m3,
// then the given handler (who serves the response), then e1, e2, e3
// (assuming every middleware/endware calls the following one).
//
// A chain can be safely reused by calling Then() several times.
//     stdStack := alice.New(ratelimitHandler, csrfHandler)
//     indexPipe = stdStack.Then(indexHandler, endwareFns)
//     authPipe = stdStack.Then(authHandler, endwareFns)
// Note that constructors are called on every call to Then()
// and thus several instances of the same middleware will be created
// when a chain is reused in this way.
// For proper middleware/endware, this should cause no problems.
//
// Then() treats nil as http.DefaultServeMux.
func (c Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)

		for _, endwareFn := range c.endware {
			endwareFn(w, r)
		}
	})

	for i := range c.constructors {
		h = c.constructors[len(c.constructors)-1-i](h)
	}

	return h
}

// ThenFunc works identically to Then, but takes
// a HandlerFunc instead of a Handler.
//
// The following two statements are equivalent:
//     c.Then(http.HandlerFunc(fn))
//     c.ThenFunc(fn)
//
// ThenFunc provides all the guarantees of Then.
func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	if fn == nil {
		return c.Then(nil)
	}
	return c.Then(fn)
}

// Append extends a chain, adding the specified constructors
// as the last ones in the request flow.
//
// Append returns a new chain, leaving the original one untouched.
//
//     stdChain := alice.New(m1, m2)
//     extChain := stdChain.Append(m3, m4)
//     // requests in stdChain go m1 -> m2
//     // requests in extChain go m1 -> m2 -> m3 -> m4
func (c Chain) Append(constructors ...Constructor) Chain {
	newCons := make([]Constructor, 0, len(c.constructors)+len(constructors))
	newCons = append(newCons, c.constructors...)
	newCons = append(newCons, constructors...)

	return Chain{newCons, c.endware}
}

// Extend extends a chain by adding the specified chain
// as the last one in the request flow.
//
// Extend returns a new chain, leaving the original one untouched.
//
//     stdChain := alice.New(m1, m2)
//     ext1Chain := alice.New(m3, m4)
//     ext2Chain := stdChain.Extend(ext1Chain)
//     // requests in stdChain go  m1 -> m2
//     // requests in ext1Chain go m3 -> m4
//     // requests in ext2Chain go m1 -> m2 -> m3 -> m4
//
// Another example:
//  aHtmlAfterNosurf := alice.New(m2)
// 	aHtml := alice.New(m1, func(h http.Handler) http.Handler {
// 		csrf := nosurf.New(h)
// 		csrf.SetFailureHandler(aHtmlAfterNosurf.ThenFunc(csrfFail))
// 		return csrf
// 	}).Extend(aHtmlAfterNosurf)
//		// requests to aHtml hitting nosurfs success handler go m1 -> nosurf -> m2 -> target-handler
//		// requests to aHtml hitting nosurfs failure handler go m1 -> nosurf -> m2 -> csrfFail
func (c Chain) Extend(chain Chain) Chain {
	newC := c.Append(chain.constructors...)
	newC = newC.AppendEndware(chain.endware...)
	return newC
}

// Endware is functionality executed after a response
// is sent to the requester. It is used for any actions the server
// wishes to take after fulfilling a user's request. It is a
// func(http.ResponseWriter, *http.Request) so values from
// the request can be used.
//
// *Note:* This will not let you re-use values from
// the Request or the Response that can no longer be used.
//
// e.g. re-reading a Request body, re-setting the Response headers, etc.
type Endware func(http.ResponseWriter, *http.Request)

// After creates a new chain with the current chain's constructors
// and the provided endware. Endware is executed after both the
// constructors and the main handler are called.
func (c Chain) After(endwareFns ...Endware) Chain {
	return Chain{c.constructors, endwareFns}
}

// AppendEndware extends a chain, adding the specified endware
// as the last ones in the request flow.
//
// Append returns a new chain, leaving the original one untouched.
//
//     stdChain := alice.New(m1).After(e1, e2)
//     extChain := stdChain.AppendEndware(e3, e4)
//	   stdHandler := stdChain.Then(h)
//	   extHandler := extChain.Then(h)
//     // requests in stdHandler go m1 -> h -> e1 -> e2
//     // requests in extHandler go m1 -> h -> e1 -> e2 -> e3 -> e4
func (c Chain) AppendEndware(endware ...Endware) Chain {
	newEnds := make([]Endware, 0, len(c.endware)+len(endware))
	newEnds = append(newEnds, c.endware...)
	newEnds = append(newEnds, endware...)

	return Chain{c.constructors, newEnds}
}
