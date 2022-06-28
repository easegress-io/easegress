# Stream

Most Easegress traffic is message-based, but Easegress v2 pipeline is
protocol independent, that's it supports stream-based traffic like TCP, and
there's stream traffic even in HTTP.

Another new feature in Easegress v2 is multiple requests/responses support,
this requires the payload of a request/response can be read more than once,
for message-based traffic, this is simple and easy to do, as Easegress can
read the full message payload into memory. But for stream-based traffic,
this is impossible, as the payload may require too much memory, and/or take
too much time to read it into memory.

To resolve the above issue, Easegress allow user or developer to configure
whether a request/response is a stream, for example:

* We can set `clientMaxBodySize` of an HTTP server to a negative value to
  tell Easegress the request is a stream, and not a stream otherwise. Please
  refer [HTTP Server](./controllers.md#httpserver) for more information.
* We can set `serverMaxBodySize` of a `Proxy` filter to a negative value to
  tell Easegress the response is a stream, and not a stream otherwise. Please
  refer [Proxy](./filters.md#proxy) for more information.

As we have mentioned above, the payload of a stream-based request/response
cannot be read once, so some features are not possible for these
requests/responses, including:

* In the `template` of `RequestBuilder` or `ResponseBuilder`, you cannot
  access the payload(in HTTP, the body) of an existing stream-based
  request/response, while it is fine to access other information of the
  request/response.
* Stream-based request/response cannot be cached by `Proxy`.
* The `HeaderToJSON` filter does not support stream-based requests/responses.
* You cannot access the payload of stream-based request/response in a
  `WasmHost` filter.
