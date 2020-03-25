// Package alice provides a convenient way to chain http handlers.
package alice

import (
	"fmt"
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
	constructors   []Constructor
	endwareLogging bool
}

// Endware is functionality that is executed after a response is sent to the requester.
// Much like middleware, it is a func(http.ResponseWriter, *http.Request) with the allowance
// that it can return errors.
type Endware struct {
	Fn      func(http.ResponseWriter, *http.Request) error
	ErrorFn func(http.ResponseWriter, *http.Request, error) interface{}
}

// NewEndware creates a new piece of Endware.
// If an error function is provided, then it is included in the Endware.
// Error functions are executed if an error occurs during the Endware's main function.
func NewEndware(
	fn func(w http.ResponseWriter, r *http.Request) error,
	errorFn ...func(http.ResponseWriter, *http.Request, error) interface{}) Endware {
	return Endware{fn, errorFn[0]}
}

// New creates a new chain,
// memorizing the given list of middleware constructors.
// New serves no other function,
// constructors are only called upon a call to Then().
func New(constructors ...Constructor) Chain {
	return Chain{append(([]Constructor)(nil), constructors...), false}
}

// Then chains the middleware and returns the final http.Handler.
//     New(m1, m2, m3).Then(h)
// is equivalent to:
//     m1(m2(m3(h)))
// When the request comes in, it will be passed to m1, then m2, then m3
// and finally, the given handler
// (assuming every middleware calls the following one).
//
// A chain can be safely reused by calling Then() several times.
//     stdStack := alice.New(ratelimitHandler, csrfHandler)
//     indexPipe = stdStack.Then(indexHandler)
//     authPipe = stdStack.Then(authHandler)
// Note that constructors are called on every call to Then()
// and thus several instances of the same middleware will be created
// when a chain is reused in this way.
// For proper middleware, this should cause no problems.
//
// Then() treats nil as http.DefaultServeMux.
func (c Chain) Then(h http.Handler, endwareFns ...Endware) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)

		for i, endwareFn := range endwareFns {
			if err := endwareFn.Fn(w, r); err != nil {
				switch {
				case endwareFn.ErrorFn != nil:
					endwareFn.ErrorFn(w, r, err)
				case c.endwareLogging:
					fmt.Printf("error occurred during endware %v: %v", i, err)
				}
			}
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
func (c Chain) ThenFunc(fn http.HandlerFunc, endwareFns ...Endware) http.Handler {
	if fn == nil {
		return c.Then(nil, endwareFns...)
	}
	return c.Then(fn, endwareFns...)
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

	return Chain{newCons, c.endwareLogging}
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
	return c.Append(chain.constructors...)
}

// SetEndwareLogging will determine whether errors that occur during Endware execution
// are printed to stdout. Default is false for new Chains.
func (c Chain) SetEndwareLogging(logging bool) Chain {
	return Chain{c.constructors, c.endwareLogging}
}
