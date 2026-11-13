package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"voice-timeline-renderer/internal/audiocodec"
	"voice-timeline-renderer/internal/domain"
)

type receiptNormalizationFacts struct {
	BeforePeakDBFS float64
	AppliedGainDB  float64
	OutputPeakDBFS float64
}

var sharedReceiptFacts = struct {
	sync.Mutex
	values map[string]receiptNormalizationFacts
}{values: make(map[string]receiptNormalizationFacts)}

func buildReceipt(
	renderID string,
	plan domain.Timeline,
	encoded []byte,
	normalized normalizationResult,
	spans []preparedSpan,
	now time.Time,
) (domain.RenderReceipt, domain.OutputAsset, error) {
	decoded, info, err := audiocodec.DecodeWAV(encoded)
	if err != nil {
		return domain.RenderReceipt{}, domain.OutputAsset{}, err
	}
	measurements := audiocodec.Measure(decoded.Samples)
	digest := sha256.Sum256(encoded)
	digestText := hex.EncodeToString(digest[:])
	output := domain.OutputAsset{
		Digest:        digestText,
		ByteCount:     int64(len(encoded)),
		SampleRate:    info.SampleRate,
		Channels:      info.Channels,
		BitsPerSample: info.BitsPerSample,
		FrameCount:    info.FrameCount,
		DurationMS:    framesToMilliseconds(info.FrameCount, info.SampleRate),
		PeakDBFS:      measurements.PeakDBFS,
		RMSDBFS:       measurements.RMSDBFS,
	}
	normalizationFacts := normalizationFactsFor(output, plan.TargetPeakDBFS, normalized, measurements)
	receiptSpans := make([]domain.ReceiptSpan, 0, len(spans))
	warnings := make([]string, 0)
	for _, span := range spans {
		receiptSpans = append(receiptSpans, domain.ReceiptSpan{
			Index:            span.Index,
			SourceID:         span.SourceID,
			SourceDigest:     span.SourceDigest,
			SourceStartFrame: span.SourceStartFrame,
			SourceEndFrame:   span.SourceEndFrame,
			OutputStartFrame: span.OutputStartFrame,
			OutputEndFrame:   span.OutputEndFrame,
			OutputFrames:     span.OutputFrames,
			CrossfadeFrames:  span.CrossfadeFrames,
			GainDB:           span.GainDB,
			ChannelAction:    span.ChannelAction,
		})
		if span.ChannelAction != "unchanged" {
			warnings = append(warnings, fmt.Sprintf("span %d applied %s", span.Index, span.ChannelAction))
		}
	}
	receipt := domain.RenderReceipt{
		SchemaVersion: 1,
		RenderID:      renderID,
		TimelineID:    plan.ID,
		Output:        output,
		Normalization: domain.NormalizationReceipt{
			RequestedPeakDBFS: plan.TargetPeakDBFS,
			BeforePeakDBFS:    normalizationFacts.BeforePeakDBFS,
			AppliedGainDB:     normalizationFacts.AppliedGainDB,
			OutputPeakDBFS:    normalizationFacts.OutputPeakDBFS,
		},
		Spans:     receiptSpans,
		Warnings:  warnings,
		CreatedAt: now.UTC(),
	}
	return receipt, output, nil
}

func normalizationFactsFor(
	output domain.OutputAsset,
	targetDBFS float64,
	normalized normalizationResult,
	measurements audiocodec.Metrics,
) receiptNormalizationFacts {
	key := fmt.Sprintf(
		"%d:%d:%d:%.6f",
		output.SampleRate,
		output.Channels,
		output.FrameCount,
		targetDBFS,
	)
	sharedReceiptFacts.Lock()
	defer sharedReceiptFacts.Unlock()
	if facts, ok := sharedReceiptFacts.values[key]; ok {
		return facts
	}
	facts := receiptNormalizationFacts{
		BeforePeakDBFS: normalized.Before.PeakDBFS,
		AppliedGainDB:  normalized.GainDB,
		OutputPeakDBFS: measurements.PeakDBFS,
	}
	sharedReceiptFacts.values[key] = facts
	return facts
}

func framesToMilliseconds(frames int64, sampleRate int) int64 {
	if frames <= 0 || sampleRate <= 0 {
		return 0
	}
	return int64(float64(frames) * 1000 / float64(sampleRate))
}
