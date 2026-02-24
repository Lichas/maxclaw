package agent

import (
	"testing"
)

func TestStepDetector_DetectCompletionWithMarkers(t *testing.T) {
	sd := NewStepDetector()

	tests := []struct {
		name            string
		output          string
		iterationInStep int
		wantComplete    bool
	}{
		// Explicit markers (work immediately, even at iteration 0)
		{name: "[Done] marker", output: "Step 1 is done [Done]", iterationInStep: 0, wantComplete: true},
		{name: "[完成] marker Chinese", output: "步骤完成了 [完成]", iterationInStep: 0, wantComplete: true},
		{name: "[Step Done] marker", output: "Task complete [Step Done]", iterationInStep: 0, wantComplete: true},
		{name: "[步骤完成] marker", output: "已经处理完毕 [步骤完成]", iterationInStep: 0, wantComplete: true},
		{name: "[Complete] marker", output: "Finished [Complete]", iterationInStep: 0, wantComplete: true},
		{name: "[Completed] marker", output: "Done [Completed]", iterationInStep: 0, wantComplete: true},
		{name: "[结束] marker", output: "结束了 [结束]", iterationInStep: 0, wantComplete: true},

		// Transition words (need at least 1 iteration)
		{name: "transition word at iteration 1", output: "现在开始下一步", iterationInStep: 1, wantComplete: true},
		{name: "transition word at iteration 0", output: "现在开始", iterationInStep: 0, wantComplete: false},
		{name: "English transition", output: "Next, let me do this", iterationInStep: 1, wantComplete: true},

		// Timeout fallback (5 iterations)
		{name: "timeout at 5", output: "still working", iterationInStep: 5, wantComplete: true},
		{name: "not timeout at 4", output: "still working", iterationInStep: 4, wantComplete: false},

		// No completion
		{name: "no completion", output: "still working on it", iterationInStep: 2, wantComplete: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sd.DetectCompletion(tt.output, tt.iterationInStep)
			if got != tt.wantComplete {
				t.Errorf("DetectCompletion(%q, %d) = %v, want %v",
					tt.output, tt.iterationInStep, got, tt.wantComplete)
			}
		})
	}
}

func TestStepDetector_MaxIterations(t *testing.T) {
	sd := NewStepDetector()
	if sd.maxIterationsPerStep != 5 {
		t.Errorf("expected maxIterationsPerStep to be 5, got %d", sd.maxIterationsPerStep)
	}
}
