// Package stats provides the small set of statistical tools Heramba's research
// experiments use to report honest uncertainty: Wilson score intervals for
// proportions (correct at the low base rates and small samples security
// evaluation lives at), a nonparametric percentile bootstrap for mean metrics
// and their differences, and PR-AUC / confusion-matrix helpers.
//
// Everything here is deterministic given a seed. No result is reported without
// an interval; a point estimate alone is never enough.
package stats

import (
	"math"
	"math/rand"
	"sort"
)

// Z95 is the standard-normal quantile for a two-sided 95% interval.
const Z95 = 1.959963984540054

// Interval is a point estimate with a (default 95%) confidence interval.
type Interval struct {
	Point float64 `json:"point"`
	Lo    float64 `json:"lo"`
	Hi    float64 `json:"hi"`
	N     int     `json:"n"`
}

// Wilson returns the Wilson score interval for a binomial proportion. Unlike the
// normal approximation it stays within [0,1] and is well-behaved when the count
// is 0, n, or the sample is small — exactly the regime of a 1% FPR target.
func Wilson(successes, n int, z float64) Interval {
	if n == 0 {
		return Interval{Point: 0, Lo: 0, Hi: 0, N: 0}
	}
	p := float64(successes) / float64(n)
	nf := float64(n)
	z2 := z * z
	denom := 1 + z2/nf
	center := (p + z2/(2*nf)) / denom
	margin := (z * math.Sqrt((p*(1-p)+z2/(4*nf))/nf)) / denom
	lo := center - margin
	hi := center + margin
	return Interval{Point: p, Lo: clamp01(lo), Hi: clamp01(hi), N: n}
}

// Wilson95 is Wilson at the 95% level.
func Wilson95(successes, n int) Interval { return Wilson(successes, n, Z95) }

// MeanStd returns the sample mean and (n-1) standard deviation.
func MeanStd(xs []float64) (mean, sd float64) {
	n := len(xs)
	if n == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean = sum / float64(n)
	if n == 1 {
		return mean, 0
	}
	var ss float64
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(ss / float64(n-1))
}

// Bootstrap returns the mean and a percentile 95% interval from `iters`
// nonparametric resamples. Deterministic given seed.
func Bootstrap(xs []float64, iters int, seed int64) Interval {
	n := len(xs)
	if n == 0 {
		return Interval{}
	}
	mean, _ := MeanStd(xs)
	if iters < 1 {
		return Interval{Point: mean, Lo: mean, Hi: mean, N: n}
	}
	rng := rand.New(rand.NewSource(seed))
	means := make([]float64, iters)
	for b := 0; b < iters; b++ {
		var sum float64
		for i := 0; i < n; i++ {
			sum += xs[rng.Intn(n)]
		}
		means[b] = sum / float64(n)
	}
	sort.Float64s(means)
	return Interval{Point: mean, Lo: percentile(means, 2.5), Hi: percentile(means, 97.5), N: n}
}

// BootstrapDiff returns the mean difference (a-b) with a percentile 95% interval
// over independent resamples — used to report effect sizes between arms.
func BootstrapDiff(a, b []float64, iters int, seed int64) Interval {
	if len(a) == 0 || len(b) == 0 {
		return Interval{}
	}
	ma, _ := MeanStd(a)
	mb, _ := MeanStd(b)
	diff := ma - mb
	if iters < 1 {
		return Interval{Point: diff, Lo: diff, Hi: diff, N: len(a) + len(b)}
	}
	rng := rand.New(rand.NewSource(seed))
	diffs := make([]float64, iters)
	for k := 0; k < iters; k++ {
		diffs[k] = resampleMean(a, rng) - resampleMean(b, rng)
	}
	sort.Float64s(diffs)
	return Interval{Point: diff, Lo: percentile(diffs, 2.5), Hi: percentile(diffs, 97.5), N: len(a) + len(b)}
}

func resampleMean(xs []float64, rng *rand.Rand) float64 {
	var sum float64
	for range xs {
		sum += xs[rng.Intn(len(xs))]
	}
	return sum / float64(len(xs))
}

// Confusion is a binary confusion matrix at a fixed threshold.
type Confusion struct {
	TP, FP, TN, FN int
}

// ConfusionAt scores predictions as positive when score >= threshold.
func ConfusionAt(scores []float64, labels []bool, threshold float64) Confusion {
	var c Confusion
	for i := range scores {
		pos := scores[i] >= threshold
		switch {
		case pos && labels[i]:
			c.TP++
		case pos && !labels[i]:
			c.FP++
		case !pos && labels[i]:
			c.FN++
		default:
			c.TN++
		}
	}
	return c
}

// Recall (sensitivity).
func (c Confusion) Recall() float64 { return ratio(c.TP, c.TP+c.FN) }

// FPR (false-positive rate).
func (c Confusion) FPR() float64 { return ratio(c.FP, c.FP+c.TN) }

// Precision.
func (c Confusion) Precision() float64 { return ratio(c.TP, c.TP+c.FP) }

// PRAUC is the area under the precision-recall curve (average precision),
// computed by sweeping every distinct score threshold and integrating precision
// over recall with the trapezoid rule. Robust to class imbalance.
func PRAUC(scores []float64, labels []bool) float64 {
	n := len(scores)
	if n == 0 {
		return 0
	}
	type pt struct {
		s float64
		y bool
	}
	pts := make([]pt, n)
	totalPos := 0
	for i := range scores {
		pts[i] = pt{scores[i], labels[i]}
		if labels[i] {
			totalPos++
		}
	}
	if totalPos == 0 {
		return 0
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].s > pts[j].s })

	var tp, fp int
	prevRecall := 0.0
	var auc float64
	prevPrec := 1.0
	i := 0
	for i < len(pts) {
		thr := pts[i].s
		for i < len(pts) && pts[i].s == thr {
			if pts[i].y {
				tp++
			} else {
				fp++
			}
			i++
		}
		recall := float64(tp) / float64(totalPos)
		prec := 1.0
		if tp+fp > 0 {
			prec = float64(tp) / float64(tp+fp)
		}
		auc += (recall - prevRecall) * (prec + prevPrec) / 2
		prevRecall = recall
		prevPrec = prec
	}
	return auc
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
