## Cosmos-based blockchain block fetcher

Only `go` is needed to run this project. All other dependencies have been
vendored and included in this repository already.

To start fetching blocks, you may use the following command:
```
$ go run . --start-height 25085433 --end-height 25086634 --node-url="https://rpc.osmosis.zone/" --parallelism 3 --output=/absolute/path/to/output.json
```

To run all the available tests, use:
```
$ go test -race -shuffle on .
```

### Going Beyond: Additional Changes and Potential Improvements
The current implementation was made with a very specific set of requirements
in mind, but to make this more production-ready, the following changes
could be made:
- Add metrics (e.g., Prometheus) to measure the duration of each request, 
the size of their responses, the number of retries.
- Add CI workflows to test and lint the code on each push.
- Handle more sophisticated error scenarios, e.g., when the server is 
completely down (returns 500) and the system runs out of retries.
