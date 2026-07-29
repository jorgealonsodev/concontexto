package indicators

// Break is one already-resolved editorial break exemption for a single
// series' single period. "Already-resolved" is load-bearing:
// config/rupturas.yaml records a calendar date plus a scope (series /
// dataset / source) that can widen to many series; turning that into
// "this series, this Period, exempted" is a reconcile-time job (slice
// 7, not built yet). Rule 3 (plausibility) only ever consumes the
// already-widened, already-period-keyed result — it must not depend on
// rupturas.yaml or the reconcile step existing (design.md "Breaks
// []indicators.Break // pre-resolved by scope expansion (series ⊂
// dataset ⊂ source)").
type Break struct {
	ID     string
	Period Period
	Kind   string
	Note   string
}
