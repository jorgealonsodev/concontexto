// Package useragent holds the single User-Agent every outbound HTTP
// client this project owns puts on the wire, and the evidence for why
// leaving it unset is a production outage rather than a cosmetic
// omission.
//
// It is a package, not four copies of a string literal, for one reason:
// the rationale below is what has value, and a rationale duplicated four
// times is a rationale that rots in three of them. The other shared
// adapter constants (defaultHTTPTimeout, defaultMaxResponseBytes) are
// deliberately NOT centralised here -- those are per-source tuning knobs
// that may legitimately diverge per endpoint. This one must not diverge:
// a source operator looking at their access logs has to see one client,
// not four.
package useragent

// UserAgent identifies this project to every source it calls.
//
// # Why an explicit value is load-bearing, not politeness
//
// Measured live 2026-07-30. Every ingest of every configured series
// failed against INE:
//
//	ingest: series tasa-de-paro-epa: ingestion: fetching tasa-de-paro-epa:
//	retryable-transport: requesting
//	https://servicios.ine.es/wstempus/js/ES/DATOS_SERIE/EPA453100?nult=9999&tip=A:
//	context deadline exceeded (Client.Timeout exceeded while awaiting headers)
//
// Ruled out first, each by direct measurement, so that nobody repeats
// them: container networking (curl from the app container's exact network
// namespace returned HTTP 200 in 0.19s), DNS, CA certificates, HTTP proxy
// (ProxyFromEnvironment returns nil), IPv6 (only an A record exists;
// forcing IPv4 changed nothing), request concurrency (six concurrent
// full-history requests all returned 200 under 0.55s), and HTTP/2
// (forcing HTTP/1.1 changed nothing). Go's crypto/tls handshake to the
// host completed in 83ms, and a RAW HTTP/1.1 request written over that
// same Go TLS connection returned HTTP/1.1 200 OK in ~250ms -- so the
// network, the TLS stack and the endpoint were all healthy throughout.
//
// Varying exactly one header over an identical raw Go TLS connection to
// servicios.ine.es:443, requesting
// /wstempus/js/ES/DATOS_SERIE/EPA453100?nult=9999&tip=A:
//
//	User-Agent sent                                                    Result
//	-----------------------------------------------------------------  -------------------------------------------------
//	Go-http-client/1.1                                                 NO RESPONSE AT ALL -- connection hangs past 10s
//	Go-http-client/2.0                                                 HTTP/1.1 200 OK in 205ms
//	(none at all)                                                      HTTP/1.1 200 OK in 219ms
//	concontexto (+https://github.com/jorgealonsodev/concontexto)       HTTP/1.1 200 OK in 208ms
//
// "Go-http-client/1.1" is EXACTLY what net/http puts on the wire over
// HTTP/1.1 when a request carries no User-Agent. INE's edge blackholes
// that one specific token: it does not answer slowly, it does not answer
// with an error, it does not answer.
//
// The consequence for whoever considers deleting this: the failure is
// invisible in development (an httptest server answers anything), appears
// only against the real endpoint, costs a full 30s client timeout PER
// ATTEMPT times the retry budget, and reports itself as
// "retryable-transport ... context deadline exceeded" -- a message that
// points at timeouts, networking and the source's health, i.e. at four
// hours of bisection in every direction except this one. Raising the
// timeout does not help. The response never comes.
//
// # Why this exact string
//
// It is a bare RFC 9110 §10.1.5 product token plus a comment. The product
// names this project; the comment carries a contact URL, which is the
// convention public open-data APIs expect -- INE and Eurostat have no
// other channel through which to tell us we are misbehaving, and an
// unidentified client is the first thing an operator rate-limits.
//
// No version token. RFC 9110's product-version is optional, and this
// repository has no version constant and no build-time version stamp
// (Dockerfile builds with -ldflags="-s -w" only). Inventing "0.1" here
// would put a number on the wire that nothing in the build can keep
// honest, and web/package.json's 0.1.0 belongs to the Astro site, not to
// this binary. Declaring no version is the accurate statement; add one
// here the day the build actually stamps one.
//
// It does not impersonate a browser or curl. Making INE's edge answer by
// pretending to be software we are not would "work", and is exactly what
// this project's honesty-to-sources principle forbids -- and it would
// also make us invisible in the access logs of every source that has been
// nothing but cooperative.
const UserAgent = "concontexto (+https://github.com/jorgealonsodev/concontexto)"
