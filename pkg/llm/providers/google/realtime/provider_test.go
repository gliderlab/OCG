package realtime

import (
	"testing"

	"google.golang.org/genai"
)

func TestParseStartSensitivity(t *testing.T) {
	if got := parseStartSensitivity("low"); got != genai.StartSensitivityLow {
		t.Fatalf("expected low, got %v", got)
	}
	if got := parseStartSensitivity("high"); got != genai.StartSensitivityHigh {
		t.Fatalf("expected high, got %v", got)
	}
	if got := parseStartSensitivity("unknown"); got != "" {
		t.Fatalf("expected empty for unknown, got %v", got)
	}
}

func TestParseEndSensitivity(t *testing.T) {
	if got := parseEndSensitivity("low"); got != genai.EndSensitivityLow {
		t.Fatalf("expected low, got %v", got)
	}
	if got := parseEndSensitivity("high"); got != genai.EndSensitivityHigh {
		t.Fatalf("expected high, got %v", got)
	}
}

func TestParseActivityHandling(t *testing.T) {
	if got := parseActivityHandling("barge_in"); got != genai.ActivityHandlingStartOfActivityInterrupts {
		t.Fatalf("expected interrupts, got %v", got)
	}
	if got := parseActivityHandling("no_interruption"); got != genai.ActivityHandlingNoInterruption {
		t.Fatalf("expected no interruption, got %v", got)
	}
}

func TestParseTurnCoverage(t *testing.T) {
	if got := parseTurnCoverage("only_activity"); got != genai.TurnCoverageTurnIncludesOnlyActivity {
		t.Fatalf("expected only_activity, got %v", got)
	}
	if got := parseTurnCoverage("all_input"); got != genai.TurnCoverageTurnIncludesAllInput {
		t.Fatalf("expected all_input, got %v", got)
	}
}

func TestParseMediaResolution(t *testing.T) {
	if got := parseMediaResolution("low"); got != genai.MediaResolutionLow {
		t.Fatalf("expected low, got %v", got)
	}
	if got := parseMediaResolution("medium"); got != genai.MediaResolutionMedium {
		t.Fatalf("expected medium, got %v", got)
	}
	if got := parseMediaResolution("high"); got != genai.MediaResolutionHigh {
		t.Fatalf("expected high, got %v", got)
	}
}
