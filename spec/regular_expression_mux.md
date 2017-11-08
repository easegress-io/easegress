### Generic Syntax
https://golang.org/pkg/regexp/syntax/
- Official
- Great Document
- O(N) Execution Time
- Group: Path Parameter

## Components
Via: [URL Reference](https://en.wikipedia.org/wiki/URL)
> scheme:[//[user[:password]@]host[:port]][/path][?query][#fragment]

EaseGateway supports them with the regular expression, and here it lists default expression respectively(without user info):

| Component | Default Regular Expression |
|:----|:----|
| scheme | (http|https) |
| host | .* |
| port | \d* |
| path | N/A |
| query | .* |
| fragment | .* |

Plus, a matching priority with type `uint32`.

## Rules
1. All match is the leftmost(non-greedy) match.
2. The URI needs fully match.
3. The matching process goes by priority from higher to lower.
4. The final regular expressions with different priority must not be the same.
5. The final regular expressions with the same priority must have no intersection with each other.

## Design

### Data Structure
```golang
type reMux struct {
    pipelineEntries map[string][]*reEntry
    priorityEntries []*reEntry
}
```

The `reEntry` is the wrapper for outside unified interface `HTTPMuxEntry`, it contains the corresponding regular expression products such as `*regexp.Regexp` which is for once only matching incoming requests.

### Administration
The Add/Delete operations on the mux just execute the corresponding operations on  `pipelineEntries` and `priorityEntries`. Here list some important notes on every operation.

In the add operation, the most important thing is to guarantee that the newcome entry does not conflict with the existed entries. In general, we mainly check the matching uniqueness of every entry. So with comparing with every existed entry, there are three components which decide the conflict rule: priority, pattern(scheme, host, port, path, query, fragment), method. We categorize them into below list(diff aka different for nicely reading):

```
- same priority
  - same pattern
    - same method: method conflict
    - diff method: no conflict
  - diff pattern
    - any intersection: pattern conflict
    - no intersection: no conflit
- diff priority
  - same pattern: pattern conflict (it does not make sense for the one with lower priority)
  - diff pattern: no conflict
```

This is the main conflict checking rules, but owing to the complex runtime environment, there is the situation where the original generation of one plugin has not vanished (which means its entries are still in mux) the next generation has come in. So we ignore the entries belonging to the same plugin and different generation while checking conflict, if the new entry went through all conflict checking, we clear any entries with the same plugin and different generation before we commit to adding it.

So in the delete operation, in the background above after we do common checking the incoming deletion request, then if we found there are any entries with the same plugin and different generation, we just ignore the deletion request.

### Serve HTTP
The design of special `priorityEntries` is for quickly pattern matching, we just go through the entries by priority from higher to lower until URL and method are matched.

### Cache
We use request URL and method as the cache key, the entry matched and the corresponding parameter values as the cache value. Under the totally same routing rules, we always use the cache hit. So after every successful administration operation, we just clear cache to guarantee the right way for next requests.
