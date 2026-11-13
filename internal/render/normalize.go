package render

import (
	"math"

	"voice-timeline-renderer/internal/audiocodec"
)

type normalizationResult struct {
	Before  audiocodec.Metrics
	After   audiocodec.Metrics
	GainDB  float64
	Samples []float32
}

func normalizePeak(samples []float32, targetDBFS float64) normalizationResult {
	before := audiocodec.Measure(samples)
	gain, gainDB := normalizationGain(samples, targetDBFS)
	for index, sample := range samples {
		value := float64(sample) * gain
		if value > 1 {
			value = 1
		}
		if value < -1 {
			value = -1
		}
		samples[index] = float32(value)
	}
	return normalizationResult{
		Before:  before,
		After:   audiocodec.Measure(samples),
		GainDB:  gainDB,
		Samples: samples,
	}
}

// normalizationGain derives the peak-normalization gain from the samples of
// the render being finalized. It must never be memoized across renders: two
// timelines can produce the same number of samples at the same target peak
// while carrying completely different signal levels.
func normalizationGain(samples []float32, targetDBFS float64) (float64, float64) {
	target := audiocodec.DBToLinear(targetDBFS)
	peak := beforePeak(samples)
	gain := 1.0
	if peak > 0 {
		gain = target / peak
	}
	if math.IsNaN(gain) || math.IsInf(gain, 0) || gain <= 0 {
		gain = 1
	}
	return gain, audiocodec.LinearToDB(gain)
}

func beforePeak(samples []float32) float64 {
	peak := 0.0
	for _, sample := range samples {
		value := math.Abs(float64(sample))
		if value > peak {
			peak = value
		}
	}
	return peak
}
