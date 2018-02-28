package plugins

import (
	"math"
	"reflect"
	"testing"
)

type operation int

const (
	increase operation = 0
	decrease operation = 1
)

type action struct {
	idx int
	op  operation
}

type possibilityAction struct {
	actions   []action
	targetLen int
}

type possibilityActionTest struct {
	input  possibilityAction
	output []int // output only needs possibilityAction.possibility
}

/// generateTargetsWithPossiblityTest
type pipelinePossibility struct {
	pipelineNames []string
	possibility   []int
}

type generateTargetsWithPossibilityTest struct {
	input pipelinePossibility
	out   pipelinePossibility
}

var generateTargetsWithPossibility = []generateTargetsWithPossibilityTest{
	/// edge cases
	// if we only have one pipeline, then it must in the target list
	{
		input: pipelinePossibility{
			pipelineNames: []string{"pipe1"},
			possibility:   []int{50},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"pipe1"},
			possibility:   []int{100},
		},
	},
	{
		input: pipelinePossibility{
			pipelineNames: []string{"pipe1"},
			possibility:   []int{1},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"pipe1"},
			possibility:   []int{100},
		},
	},
	{
		input: pipelinePossibility{
			pipelineNames: []string{"pipe1"},
			possibility:   []int{100},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"pipe1"},
			possibility:   []int{100},
		},
	},
	// if we don't have possibility, then all the pipelines will be in the target list
	{
		input: pipelinePossibility{
			pipelineNames: []string{"pipe1", "pipe2"},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"pipe1", "pipe2"},
			possibility:   []int{100, 100},
		},
	},
	{
		input: pipelinePossibility{
			pipelineNames: []string{"pipe1", "pipe2", "pipe3"},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"pipe1", "pipe2", "pipe3"},
			possibility:   []int{100, 100, 100},
		},
	},
	/// normal case
	{
		input: pipelinePossibility{
			pipelineNames: []string{"apipe", "bpipe", "cpipe", "dpipe"},
			possibility:   []int{12, 25, 50, 50},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"apipe", "bpipe", "cpipe", "dpipe"},
			possibility:   []int{12, 25, 50, 100}, // last pipeline always has 100% possibility
		},
	},
	{
		input: pipelinePossibility{
			pipelineNames: []string{"apipe", "bpipe", "cpipe", "dpipe"},
			possibility:   []int{1, 2, 4, 1},
		},
		out: pipelinePossibility{
			pipelineNames: []string{"apipe", "bpipe", "cpipe", "dpipe"},
			possibility:   []int{1, 2, 4, 100},
		},
	},
}

func TestGenerateTargetsWithPossibility(t *testing.T) {
	experimentCount := 1000000 // experiment count should be large enough to get statistics distribution
	for ttIdx, tt := range generateTargetsWithPossibility {
		counter := make([]int, len(tt.input.pipelineNames))
		for i := 0; i < experimentCount; i++ {
			targets := generateTargets(tt.input.pipelineNames, tt.input.possibility)
			for _, target := range targets {
				if tt.input.pipelineNames[target.idx] != target.pipelineName {
					t.Errorf("generateTargets idx error in %dth test case", ttIdx)
				}
				counter[target.idx]++
			}
		}
		for idx, c := range counter {
			t.Logf("pipelineName: %s, counter: %d", tt.out.pipelineNames[idx], c)
			real := float64(c) / float64(experimentCount) * 100.0
			// 0.1 precison is enough for us
			if math.Abs(float64(tt.out.possibility[idx])-real) > 0.1 {
				t.Errorf("generateTargets possibility error in %dth test case: pipelineName: %s, want %f, real is %f", ttIdx, tt.out.pipelineNames[idx], float64(tt.out.possibility[idx]), real)
			}
		}
	}
}

var possibilityActions = []possibilityActionTest{
	/// edge case
	// empty action
	{
		input: possibilityAction{
			actions:   []action{},
			targetLen: 3,
		},
		output: []int{100, 100, 100},
	},
	// only one elements
	{
		input: possibilityAction{
			actions: []action{
				{
					idx: 0,
					op:  increase,
				},
			},
			targetLen: 1,
		},
		output: []int{100},
	},
	{
		input: possibilityAction{
			actions: []action{
				action{
					idx: 0,
					op:  decrease,
				},
			},
			targetLen: 1,
		},
		output: []int{50},
	},
	{
		input: possibilityAction{
			actions: []action{
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
				action{
					idx: 0,
					op:  decrease,
				},
			},
			targetLen: 1,
		},
		output: []int{1},
	},
	/// normal case
	{
		input: possibilityAction{
			actions: []action{
				action{
					idx: 0,
					op:  decrease, // 0->50
				},
				action{
					idx: 1,
					op:  decrease, // 1->50
				},
				action{
					idx: 1,
					op:  decrease, // 1->25
				},
				action{
					idx: 1,
					op:  decrease, // 1->12
				},
				action{
					idx: 1,
					op:  increase, // 1->50, fast recovery
				},
				action{
					idx: 0,
					op:  increase, // 0->100
				},
			},

			targetLen: 2,
		},
		output: []int{100, 50},
	},
}

func TestTargetsPossibility(t *testing.T) {
	for _, tt := range possibilityActions {
		targetsPossibility := newTargetsPossibility(tt.input.targetLen)
		for _, action := range tt.input.actions {
			switch action.op {
			case increase:
				targetsPossibility.increase(action.idx)
			case decrease:
				targetsPossibility.decrease(action.idx)
			}
		}
		real := targetsPossibility.getAll()
		if !reflect.DeepEqual(real, tt.output) {
			t.Errorf("test targetsPossibility error: expected possibility: %v, real is :%v", tt.output, real)
		}
	}
}
