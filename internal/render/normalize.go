package render

import (
	"fmt"
	"math"
	"sync"

	"voice-timeline-renderer/internal/audiocodec"
)

type normalizationProfile struct {
	GainDB      float64
	LinearGain  float64
	SampleCount int
}

var sharedNormalizationProfiles = struct {
	sync.Mutex
	values map[string]normalizationProfile
}{values: make(map[string]normalizationProfile)}

type normalizationResult struct {
	Before  audiocodec.Metrics
	After   audiocodec.Metrics
	GainDB  float64
	Samples []float32
}

func normalizePeak(samples []float32, targetDBFS float64) normalizationResult {
	before := audiocodec.Measure(samples)
	gain, gainDB := sharedNormalizationGain(samples, targetDBFS)
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

func sharedNormalizationGain(samples []float32, targetDBFS float64) (float64, float64) {
	key := normalizationProfileKey(len(samples), targetDBFS)
	sharedNormalizationProfiles.Lock()
	defer sharedNormalizationProfiles.Unlock()
	if profile, ok := sharedNormalizationProfiles.values[key]; ok {
		return profile.LinearGain, profile.GainDB
	}

	target := audiocodec.DBToLinear(targetDBFS)
	peak := beforePeak(samples)
	gain := 1.0
	if peak > 0 {
		gain = target / peak
	}
	if math.IsNaN(gain) || math.IsInf(gain, 0) || gain <= 0 {
		gain = 1
	}
	profile := normalizationProfile{
		GainDB:      audiocodec.LinearToDB(gain),
		LinearGain:  gain,
		SampleCount: len(samples),
	}
	sharedNormalizationProfiles.values[key] = profile
	return profile.LinearGain, profile.GainDB
}

func normalizationProfileKey(sampleCount int, targetDBFS float64) string {
	return fmt.Sprintf("%d:%.6f", sampleCount, targetDBFS)
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
