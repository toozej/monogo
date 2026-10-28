# `/pkg`

This directory contains library code for external applications, for example `/pkg/mypubliclib`.
Other projects expect these libraries to work, so assess the public interface before you put code here.
The Go compiler enforces import restrictions for `internal` packages, which makes `internal` a better choice for private code.
The `/pkg` directory communicates that other projects can safely use its code.

Travis Jeffery discusses both directories in the
[`I'll take pkg over internal`](https://travisjeffery.com/ill-take-pkg-over-internal/) blog post.
The article explains situations where `pkg` is a reasonable choice.
