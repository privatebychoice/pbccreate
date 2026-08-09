package server

import (
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestBuildChecklist(t *testing.T) {
	ready := checklistInput{
		Placements:   []placementView{{Deliverables: []store.Deliverable{{Done: true}, {Done: true}}}},
		Attributions: []store.Attribution{{IncludedInDescription: true}},
		Thumbnails:   []store.Thumbnail{{ID: 1}},
		RenderedDesc: "Intro text",
		Tags:         []store.Tag{{Name: "privacy"}},
		Publications: []store.Publication{{OutputFile: "/masters/final.mp4"}},
	}

	tests := []struct {
		name      string
		mutate    func(in *checklistInput)
		wantReady bool
		// wantStates maps a check label to its expected state.
		wantStates map[string]checkState
	}{
		{
			name:      "all ready",
			mutate:    func(*checklistInput) {},
			wantReady: true,
			wantStates: map[string]checkState{
				"Sponsor deliverables": checkOK,
				"Attributions":         checkOK,
				"Thumbnail":            checkOK,
				"Description & tags":   checkOK,
				"Output file":          checkOK,
			},
		},
		{
			name: "empty item: nothing set",
			mutate: func(in *checklistInput) {
				in.Placements = nil
				in.Attributions = nil
				in.Thumbnails = nil
				in.RenderedDesc = ""
				in.Tags = nil
				in.Publications = nil
			},
			wantReady: false,
			wantStates: map[string]checkState{
				"Sponsor deliverables": checkNA,
				"Attributions":         checkNA,
				"Thumbnail":            checkFail,
				"Description & tags":   checkFail,
				"Output file":          checkFail,
			},
		},
		{
			name: "deliverable outstanding blocks readiness",
			mutate: func(in *checklistInput) {
				in.Placements = []placementView{{Deliverables: []store.Deliverable{{Done: true}, {Done: false}}}}
			},
			wantReady:  false,
			wantStates: map[string]checkState{"Sponsor deliverables": checkFail},
		},
		{
			name: "attribution not marked for credits blocks readiness",
			mutate: func(in *checklistInput) {
				in.Attributions = []store.Attribution{{IncludedInDescription: true}, {IncludedInDescription: false}}
			},
			wantReady:  false,
			wantStates: map[string]checkState{"Attributions": checkFail},
		},
		{
			name: "description present but no tags",
			mutate: func(in *checklistInput) {
				in.Tags = nil
			},
			wantReady:  false,
			wantStates: map[string]checkState{"Description & tags": checkFail},
		},
		{
			name: "publication without an output file blocks readiness",
			mutate: func(in *checklistInput) {
				in.Publications = []store.Publication{{Platform: "YouTube"}}
			},
			wantReady:  false,
			wantStates: map[string]checkState{"Output file": checkFail},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ready
			tt.mutate(&in)
			items, gotReady := buildChecklist(in)
			if gotReady != tt.wantReady {
				t.Errorf("ready = %v, want %v", gotReady, tt.wantReady)
			}
			byLabel := make(map[string]checkState, len(items))
			for _, it := range items {
				byLabel[it.Label] = it.State
			}
			for label, want := range tt.wantStates {
				if got := byLabel[label]; got != want {
					t.Errorf("check %q state = %q, want %q", label, got, want)
				}
			}
		})
	}
}
