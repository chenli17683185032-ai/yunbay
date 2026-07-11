package openai

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultModels_GPT56ExactCatalog(t *testing.T) {
	want := []Model{
		{ID: "gpt-5.6", Object: "model", Created: 0, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6"},
		{ID: "gpt-5.6-sol", Object: "model", Created: 0, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Sol"},
		{ID: "gpt-5.6-terra", Object: "model", Created: 0, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Terra"},
		{ID: "gpt-5.6-luna", Object: "model", Created: 0, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Luna"},
	}

	var got []Model
	for _, model := range DefaultModels {
		if strings.HasPrefix(model.ID, "gpt-5.6") {
			got = append(got, model)
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GPT-5.6 catalog = %#v, want exact catalog %#v", got, want)
	}
}

func TestDefaultModelIDs_GPT56ExactCatalog(t *testing.T) {
	want := []string{"gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"}

	var got []string
	for _, id := range DefaultModelIDs() {
		if strings.HasPrefix(id, "gpt-5.6") {
			got = append(got, id)
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GPT-5.6 model IDs = %v, want exact IDs %v", got, want)
	}
}
