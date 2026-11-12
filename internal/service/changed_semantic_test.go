package service

import (
	"testing"

	"create_sheme/internal/domain"
)

func TestGraphicObjectSemanticChange_IgnoresLayout(t *testing.T) {
	base := domain.GraphicObject{
		Id:  "o1",
		Tag: "T",
		AnchorPoint: []domain.AnchorPoint{{X: "0", Y: "0"}},
		Angle:       "45",
		IsVisibled:  "true",
	}
	moved := base
	moved.AnchorPoint = []domain.AnchorPoint{{X: "10", Y: "20"}}
	moved.Angle = "90"
	moved.IsVisibled = "false"
	if graphicObjectSemanticChange(base, moved) {
		t.Fatal("layout-only change should not be semantic")
	}
}

func TestGraphicObjectSemanticChange_TagIsSemantic(t *testing.T) {
	base := domain.GraphicObject{Id: "o1", Tag: "A"}
	edited := base
	edited.Tag = "B"
	if !graphicObjectSemanticChange(base, edited) {
		t.Fatal("tag change should be semantic")
	}
}

func TestGraphicObjectSemanticChange_MixedPatchSemantic(t *testing.T) {
	base := domain.GraphicObject{
		Id:          "o1",
		Tag:         "A",
		AnchorPoint: []domain.AnchorPoint{{X: "0", Y: "0"}},
	}
	mixed := base
	mixed.AnchorPoint = []domain.AnchorPoint{{X: "5", Y: "5"}}
	mixed.Tag = "B"
	if !graphicObjectSemanticChange(base, mixed) {
		t.Fatal("layout+tag should be semantic")
	}
}
