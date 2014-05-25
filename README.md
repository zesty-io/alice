# Alice 

`Alice` provides a convenient way to chain 
your HTTP middleware functions and the app handler.

In short, it transforms `Middleware1(Middleware2(Middleware3(App)))`
to `alice.New(Middleware1, Middleware, Middleware3).Then(App)`.

### Why?

None of the other middleware chaining solutions
behaves exactly like Alice.
Alice is as minimal as a chaining solution gets.
In its essence, it's just a for loop that does the wrapping for you.

### Usage

Your middleware constructors should have the form of

    func (http.Handler) http.Handler

Some middleware provide this out of the box.
For ones that don't, it's trivial to write one yourself.

```go
func myStripPrefix(h http.Handler) http.Handler {
    return http.StripPrefix("/old", h)
}
```

This complete example shows the full power of Alice.

```go
package main

import (
    "net/http"
    "time"

    "github.com/PuerkitoBio/throttled"
    "github.com/justinas/alice"
    "github.com/justinas/nosurf"
)

func timeoutHandler(h http.Handler) http.Handler {
    return http.TimeoutHandler(h, 1*time.Second, "timed out")
}

func myApp(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello world!"))
}

func main() {
    th := throttled.Interval(throttled.PerSec(10), 1, &throttled.VaryBy{Path: true}, 50)
    myHandler := http.HandlerFunc(myApp)

    chain := alice.New(th.Throttle, timeoutHandler, nosurf.NewPure).Then(myHandler)
    http.ListenAndServe(":8000", chain)
}
```

Here, the request will pass [throttled](https://github.com/PuerkitoBio/throttled) first,
then an http.TimeoutHandler we've set up,
then [nosurf](https://github.com/justinas/nosurf)
and will finally reach our handler.

Note that Alice makes **no guarantees** for
how one or another piece of  middleware will behave.
It does not execute all handlers sequentially
but wraps them in one another.

If a piece of middleware were to stop the chain,
the request will not reach the inner handlers.
This is intentional behavior.

### Contributing

0. Find an issue that bugs you / open a new one.
1. Discuss.
2. Branch off the `develop` branch, commit, test.
3. Make a pull request / attach the commits to the issue.