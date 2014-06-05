# mio (aka metered IO)

Package mio provides tools to combine io.Reader/io.Writer and
[metrics.Histogram][1] interfaces, so that every non-empty Read or Write call
latency value is sampled to Histogram.

See [documentation on godoc.org][doc].

This package is intended to be used with [metrics][1] package.

[1]: https://godoc.org/github.com/artyom/metrics#Histogram
[2]: https://github.com/artyom/metrics
[doc]: https://godoc.org/github.com/artyom/mio
